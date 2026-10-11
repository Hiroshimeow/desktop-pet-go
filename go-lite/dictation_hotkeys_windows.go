//go:build windows

package main

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	dictModAlt               uint32 = 1
	dictModControl           uint32 = 2
	dictModShift             uint32 = 4
	dictModWin               uint32 = 8
	dictModAltGr             uint32 = 16
	dictNoRepeat                    = 0x4000
	dictHookKeyboard                = 13
	dictInjected                    = 0x12 // LLKHF_INJECTED and LLKHF_LOWER_IL_INJECTED
	dictKeyDown                     = 0x100
	dictKeyUp                       = 0x101
	dictSysKeyDown                  = 0x104
	dictSysKeyUp                    = 0x105
	dictHotkeyMessage               = 0x312
	dictEscape                      = 0x1b
	dictReleaseEscapeMessage        = 0x8001
)

type dictationAction uint8

const (
	dictationNone dictationAction = iota
	dictationStart
	dictationStop
	dictationCancel
)

type dictationBinding struct{ vk, mods uint32 }
type dictationBindings struct{ toggle, hold dictationBinding }

// Restrict primary keys to layout-independent function keys or Ctrl+letters.
// Alt/Win, Ctrl+Alt (AltGr), bare text, IME keys and debugger-reserved F12
// are intentionally rejected rather than silently hijacking system shortcuts.
func parseDictationBinding(text string) (dictationBinding, error) {
	var b dictationBinding
	if strings.TrimSpace(text) == "" {
		return b, nil
	}
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(text)), "+")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if i < len(parts)-1 {
			var m uint32
			switch part {
			case "CTRL":
				m = dictModControl
			case "SHIFT":
				m = dictModShift
			default:
				return b, fmt.Errorf("unsupported modifier %q; use Ctrl/Shift", part)
			}
			if b.mods&m != 0 {
				return b, fmt.Errorf("duplicate modifier %q", part)
			}
			b.mods |= m
		} else if strings.HasPrefix(part, "F") {
			n, err := strconv.Atoi(strings.TrimPrefix(part, "F"))
			if err != nil || n < 1 || n > 24 || n == 12 {
				return b, fmt.Errorf("unsupported function key %q", part)
			}
			b.vk = uint32(0x6f + n)
		} else if len(part) == 1 && part[0] >= 'A' && part[0] <= 'Z' && b.mods&dictModControl != 0 {
			b.vk = uint32(part[0])
		} else {
			return b, fmt.Errorf("unsupported key %q", part)
		}
	}
	return b, nil
}
func parseDictationBindings(toggle, hold string) (dictationBindings, error) {
	t, err := parseDictationBinding(toggle)
	if err != nil {
		return dictationBindings{}, err
	}
	h, err := parseDictationBinding(hold)
	if err != nil {
		return dictationBindings{}, err
	}
	if t.vk == 0 && h.vk == 0 {
		return dictationBindings{}, fmt.Errorf("at least one dictation binding is required")
	}
	if t == h {
		return dictationBindings{}, fmt.Errorf("toggle and hold must differ")
	}
	return dictationBindings{t, h}, nil
}

type dictationKeyState struct {
	bindings                 dictationBindings
	down                     [256]bool
	owned                    [256]bool
	busy, recording, holding bool
}

func (s *dictationKeyState) finish() { s.busy = false; s.recording = false; s.holding = false }
func (s *dictationKeyState) event(vk uint32, down bool, mods uint32, ime, ready, injected bool) (dictationAction, bool) {
	if injected {
		return dictationNone, false
	}
	return s.key(vk, down, mods, ime, ready)
}
func (s *dictationKeyState) key(vk uint32, down bool, mods uint32, ime, ready bool) (dictationAction, bool) {
	if vk >= 256 {
		return dictationNone, false
	}
	wasDown := s.down[vk]
	s.down[vk] = down
	if !down {
		owned := s.owned[vk]
		s.owned[vk] = false
		if owned && s.holding && vk == s.bindings.hold.vk && s.recording {
			s.holding = false
			s.recording = false
			return dictationStop, true
		}
		return dictationNone, owned
	}
	if wasDown {
		return dictationNone, s.owned[vk]
	}
	if vk == dictEscape && s.busy && mods == 0 {
		s.owned[vk] = true
		s.recording = false
		s.holding = false
		return dictationCancel, true
	}
	t, h := s.bindings.toggle, s.bindings.hold
	toggle := t.vk != 0 && vk == t.vk && mods == t.mods
	hold := h.vk != 0 && vk == h.vk && mods == h.mods
	if !toggle && !hold {
		return dictationNone, false
	}
	if !s.busy && (!ready || ime) {
		return dictationNone, false
	}
	s.owned[vk] = true
	if s.busy {
		if s.recording && toggle {
			s.recording = false
			s.holding = false
			return dictationStop, true
		}
		return dictationNone, true
	}
	s.busy = true
	s.recording = true
	s.holding = hold
	return dictationStart, true
}

var (
	dictRegisterHotKey   = user32.NewProc("RegisterHotKey")
	dictUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	dictSetHook          = user32.NewProc("SetWindowsHookExW")
	dictUnhook           = user32.NewProc("UnhookWindowsHookEx")
	dictNextHook         = user32.NewProc("CallNextHookEx")
	dictPeekMessage      = user32.NewProc("PeekMessageW")
	dictPostThread       = user32.NewProc("PostThreadMessageW")
	dictThreadID         = kernel32.NewProc("GetCurrentThreadId")
	dictForeground       = user32.NewProc("GetForegroundWindow")
	dictWindowThread     = user32.NewProc("GetWindowThreadProcessId")
	dictGUIInfo          = user32.NewProc("GetGUIThreadInfo")
	dictSendTimeout      = user32.NewProc("SendMessageTimeoutW")
	dictIMEWindow        = syscall.NewLazyDLL("imm32.dll").NewProc("ImmGetDefaultIMEWnd")
	dictActiveHook       atomic.Pointer[dictationHotkeys]
	dictHookCallback     = syscall.NewCallback(dictationKeyboardProc)
)

type dictationFocus struct {
	top, focus, caret            uintptr
	pid                          uint32
	rect                         [4]int32
	valid                        bool
	selectionStart, selectionEnd uint32
	selectionValid               bool
	epoch                        uint64
	cancelEpoch                  uint64
}
type dictationGUIInfo struct {
	size, flags                               uint32
	active, focus, capture, menu, move, caret uintptr
	rect                                      [4]int32
}

func captureDictationFocus() dictationFocus {
	fg, _, _ := dictForeground.Call()
	if fg == 0 {
		return dictationFocus{}
	}
	var pid uint32
	tid, _, _ := dictWindowThread.Call(fg, uintptr(unsafe.Pointer(&pid)))
	var g dictationGUIInfo
	g.size = uint32(unsafe.Sizeof(g))
	ok, _, _ := dictGUIInfo.Call(tid, uintptr(unsafe.Pointer(&g)))
	f := dictationFocus{top: fg, focus: g.focus, caret: g.caret, pid: pid, rect: g.rect, valid: ok != 0 && g.focus != 0}
	captureNativeSelection(&f)
	f.epoch = deliveryEpoch.Load()
	f.cancelEpoch = deliveryCancelEpoch.Load()
	return f
}

// An open IME may own F-keys for candidate selection. Query its native status
// with a finite timeout; a hung/inaccessible IME is not permission to hijack it.
func dictationIMEOwnsKeys(focus uintptr) bool {
	if focus == 0 {
		return true
	}
	hwnd, _, _ := dictIMEWindow.Call(focus)
	if hwnd == 0 {
		return false
	}
	var open uintptr
	ok, _, _ := dictSendTimeout.Call(hwnd, 0x283, 5, 0, 0x2|0x20, 10, uintptr(unsafe.Pointer(&open)))
	return ok == 0 || open != 0
}
func dictationModifiers() uint32 {
	var mods uint32
	for _, pair := range []struct{ vk, mod uint32 }{{0x10, dictModShift}, {0x11, dictModControl}, {0x12, dictModAlt}, {0x5b, dictModWin}, {0x5c, dictModWin}, {0xa5, dictModAltGr}} {
		r, _, _ := pGetAsyncKeyState.Call(uintptr(pair.vk))
		if uint16(r)&0x8000 != 0 {
			mods |= pair.mod
		}
	}
	return mods
}

type dictationInput struct {
	action dictationAction
	target dictationFocus
	err    error
	hold   bool
	at     time.Time
}
type dictationHotkeys struct {
	mu          sync.Mutex
	state       dictationKeyState
	ready       atomic.Bool
	overflow    atomic.Bool
	inputs      chan dictationInput
	tid         uint32
	done        chan struct{}
	stopOnce    sync.Once
	escapeOwned bool
	released    [4]bool // Temporarily pass through unowned bound strokes.
}

func dictationKeyboardProc(code int, wparam, lparam uintptr) uintptr {
	h := dictActiveHook.Load()
	if code >= 0 && lparam != 0 && h != nil {
		kb := (*struct {
			vk, scan, flags, time uint32
			extra                 uintptr
		})(unsafe.Pointer(lparam))
		down := wparam == dictKeyDown || wparam == dictSysKeyDown
		up := wparam == dictKeyUp || wparam == dictSysKeyUp
		if down || up {
			if kb.flags&dictInjected != 0 {
				h.mu.Lock()
				h.passRegisteredStroke(kb.vk, down)
				h.mu.Unlock()
				ret, _, _ := dictNextHook.Call(0, uintptr(code), wparam, lparam)
				return ret
			}
			mods := dictationModifiers()
			h.mu.Lock()
			var focus dictationFocus
			ime := false
			if down && kb.vk < 256 && !h.state.down[kb.vk] && !h.state.busy && h.ready.Load() &&
				((kb.vk == h.state.bindings.toggle.vk && mods == h.state.bindings.toggle.mods) || (kb.vk == h.state.bindings.hold.vk && mods == h.state.bindings.hold.mods)) {
				focus = captureDictationFocus()
				ime = !focus.valid || dictationIMEOwnsKeys(focus.focus)
			}
			action, consume := h.state.key(kb.vk, down, mods, ime, h.ready.Load())
			if action == dictationStart {
				// Esc is exclusive only for the lifetime of the active session.
				ok, _, err := dictRegisterHotKey.Call(0, 3, dictNoRepeat, dictEscape)
				if ok == 0 {
					h.state.finish()
					h.state.owned[kb.vk] = false
					consume = false
					action = dictationNone
					h.enqueue(dictationInput{err: fmt.Errorf("Esc cancellation shortcut unavailable: %w", err)})
				} else {
					h.escapeOwned = true
				}
			}
			if action != dictationNone {
				fenceDictationPhysicalAction(action)
				h.enqueue(dictationInput{action: action, target: focus, hold: h.state.holding, at: time.Now()})
			}
			if !consume {
				h.passRegisteredStroke(kb.vk, down)
			}
			h.mu.Unlock()
			if consume {
				return 1
			}
		}
	}
	ret, _, _ := dictNextHook.Call(0, uintptr(code), wparam, lparam)
	return ret
}

// RegisterHotKey also removes input from the foreground queue, even when a
// low-level hook passes it on. Release the registration for an unowned stroke
// (IME, injected, not ready), and reclaim on its key-up. If another owner wins
// during that interval, fail closed rather than stealing its shortcut.
// Called only on the hook's registration thread, under mu.
func (h *dictationHotkeys) passRegisteredStroke(vk uint32, down bool) {
	for i, b := range []dictationBinding{h.state.bindings.toggle, h.state.bindings.hold, {vk: dictEscape}} {
		id := i + 1
		if b.vk == 0 || vk != b.vk || (id == 3 && !h.escapeOwned) {
			continue
		}
		if down && !h.released[id] {
			dictUnregisterHotKey.Call(0, uintptr(id))
			h.released[id] = true
		}
		if !down && h.released[id] {
			ok, _, err := dictRegisterHotKey.Call(0, uintptr(id), uintptr(b.mods|dictNoRepeat), uintptr(b.vk))
			if ok == 0 {
				h.overflow.Store(true)
				h.ready.Store(false)
				h.enqueue(dictationInput{err: fmt.Errorf("dictation shortcut reclaim conflict: %w", err)})
			} else {
				h.released[id] = false
			}
		}
	}
}

// Reconcile only the owned hold, never synthesize a finalize after a missed
// release (secure-desktop/suspend transitions can swallow key-up).
func (h *dictationHotkeys) reconcileHold(physicallyDown bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state.holding && !physicallyDown {
		vk := h.state.bindings.hold.vk
		h.state.down[vk] = false
		h.state.holding = false
		h.state.recording = false
		fenceDictationPhysicalAction(dictationCancel)
		h.enqueue(dictationInput{action: dictationCancel, at: time.Now()})
	}
}

func (h *dictationHotkeys) enqueue(in dictationInput) {
	select {
	case h.inputs <- in:
	default:
		h.overflow.Store(true)
		h.ready.Store(false)
		fenceDictationPhysicalAction(dictationCancel)
	}
}
func (h *dictationHotkeys) finish() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.finish()
	// Must unregister on the registration thread, not the UI thread.
	if h.tid != 0 {
		dictPostThread.Call(uintptr(h.tid), dictReleaseEscapeMessage, 0, 0)
	}
}

func startDictationHotkeys(bindings dictationBindings) (*dictationHotkeys, error) {
	h := &dictationHotkeys{state: dictationKeyState{bindings: bindings}, inputs: make(chan dictationInput, 32), done: make(chan struct{})}
	if !dictActiveHook.CompareAndSwap(nil, h) {
		return nil, fmt.Errorf("dictation hotkeys already installed")
	}
	started := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(h.done)
		defer dictActiveHook.CompareAndSwap(h, nil)
		tid, _, _ := dictThreadID.Call()
		h.tid = uint32(tid)
		var m msg
		dictPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 0) // Create queue before publishing readiness.
		registered := []uintptr{}
		defer func() {
			for _, id := range registered {
				dictUnregisterHotKey.Call(0, id)
			}
			dictUnregisterHotKey.Call(0, 3)
		}()
		for i, b := range []dictationBinding{bindings.toggle, bindings.hold} {
			if b.vk == 0 {
				continue
			}
			id := uintptr(i + 1)
			ok, _, err := dictRegisterHotKey.Call(0, id, uintptr(b.mods|dictNoRepeat), uintptr(b.vk))
			if ok == 0 {
				started <- fmt.Errorf("dictation shortcut unavailable (key=%x modifiers=%x): %w", b.vk, b.mods, err)
				return
			}
			registered = append(registered, id)
		}
		// Do not treat a key already held during startup as a fresh activation.
		for vk := range h.state.down {
			r, _, _ := pGetAsyncKeyState.Call(uintptr(vk))
			h.state.down[vk] = uint16(r)&0x8000 != 0
		}
		timer, _, timerErr := pSetTimer.Call(0, 0, 100, 0)
		if timer == 0 {
			started <- fmt.Errorf("dictation hold watchdog timer: %w", timerErr)
			return
		}
		defer pKillTimer.Call(0, timer)
		hook, _, err := dictSetHook.Call(dictHookKeyboard, dictHookCallback, 0, 0)
		if hook == 0 {
			started <- fmt.Errorf("install dictation keyboard hook: %w", err)
			return
		}
		defer dictUnhook.Call(hook)
		unwatch, err := watchDictationTarget()
		if err != nil {
			started <- err
			return
		}
		defer unwatch()
		started <- nil
		for {
			r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			if m.Message == WM_TIMER && m.WParam == timer {
				vk := bindings.hold.vk
				if vk != 0 {
					r, _, _ := pGetAsyncKeyState.Call(uintptr(vk))
					h.reconcileHold(uint16(r)&0x8000 != 0)
				}
				continue
			}
			if m.Message == dictReleaseEscapeMessage {
				h.mu.Lock()
				if !h.state.busy && h.escapeOwned {
					dictUnregisterHotKey.Call(0, 3)
					h.escapeOwned = false
					h.released[3] = false
				}
				h.mu.Unlock()
				continue
			}
			if m.Message == dictHotkeyMessage {
				continue
			} // Hook owns physical edges; injected events cannot activate.
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	if err := <-started; err != nil {
		<-h.done
		return nil, err
	}
	return h, nil
}
func (h *dictationHotkeys) stop() {
	if h == nil {
		return
	}
	h.stopOnce.Do(func() { dictPostThread.Call(uintptr(h.tid), 0x12, 0, 0) })
	<-h.done
}

type dictationSession struct {
	sessionID                string
	seq                      uint64
	recording, cancelled     bool
	hold                     bool
	startedAt                time.Time
	target                   dictationFocus
	stable, unstable, result string
	finalSeen, failed        bool
}

func (a *App) processDictationInputs() {
	h := a.DictationKeys
	if h == nil {
		return
	}
	if a.Voice == nil || a.Voice.isStopped() {
		h.ready.Store(false)
		if a.Dictation.sessionID != "" {
			a.cancelDictationDelivery()
			a.Dictation = dictationSession{}
			h.finish()
		}
		return
	}
	ready := a.DictationDelivery.text == "" && a.Voice.ready && !a.Voice.listen && !a.Voice.commandMode && a.Voice.activeTurn == "" && len(a.Voice.queue) == 0
	h.ready.Store(ready && !h.overflow.Load())
	if h.overflow.Swap(false) {
		// A lost release/cancel cannot leave capture active. Retire the bounded
		// sidecar owner and discard every queued edge, not just the latest one.
		a.Voice.cancel()
		a.cancelDictationDelivery()
		a.Dictation = dictationSession{}
		h.finish()
		for {
			select {
			case <-h.inputs:
			default:
				log.Printf("dictation input overloaded; sidecar retired safely")
				return
			}
		}
	}
	for i := 0; i < 32; i++ {
		select {
		case in := <-h.inputs:
			a.handleDictationInput(in)
		default:
			return
		}
	}
}
func (a *App) handleDictationInput(in dictationInput) {
	if in.err != nil {
		log.Printf("dictation unavailable: %v", in.err)
		return
	}
	v := a.Voice
	if v == nil || v.isStopped() {
		if a.DictationKeys != nil {
			a.DictationKeys.finish()
		}
		return
	}
	kind := ""
	switch in.action {
	case dictationStart:
		if a.Dictation.sessionID != "" {
			return
		}
		if a.DictationDelivery.text != "" || !v.ready || v.listen || v.commandMode || v.activeTurn != "" || len(v.queue) != 0 {
			if a.DictationKeys != nil {
				a.DictationKeys.finish()
			}
			return
		}
		a.Dictation = dictationSession{sessionID: fmt.Sprintf("dict-%d-%d", time.Now().UnixNano(), a.DictationSerial), recording: true, target: in.target, hold: in.hold, startedAt: in.at}
		a.DictationSerial++
		a.beginDictationDelivery(in.target)
		kind = "dictation_start"
	case dictationStop:
		if a.Dictation.sessionID == "" || !a.Dictation.recording {
			return
		}
		a.Dictation.recording = false
		kind = "dictation_stop"
		if a.Dictation.hold && !a.Dictation.startedAt.IsZero() && in.at.Sub(a.Dictation.startedAt) < 250*time.Millisecond {
			kind = "dictation_cancel"
			a.Dictation.cancelled = true
			a.cancelDictationDelivery()
		}
	case dictationCancel:
		if a.Dictation.sessionID == "" {
			return
		}
		a.Dictation.cancelled = true
		a.cancelDictationDelivery()
		a.Dictation.recording = false
		kind = "dictation_cancel"
	default:
		return
	}
	if err := v.send(voiceCommand{Type: kind, ProtocolVersion: 1, SessionID: a.Dictation.sessionID}); err != nil {
		a.cancelDictationDelivery()
		a.Dictation = dictationSession{}
		if a.DictationKeys != nil {
			a.DictationKeys.finish()
		}
		log.Printf("dictation command failed: %v", err)
	}
}
func (a *App) handleDictationEvent(ev voiceEvent) bool {
	if ev.SessionID == "" {
		return false
	}
	d := &a.Dictation
	if ev.SessionID != d.sessionID || ev.ProtocolVersion != 1 || ev.Seq <= d.seq {
		return true
	}
	d.seq = ev.Seq
	switch ev.Type {
	case "partial":
		if !d.cancelled {
			d.stable = ev.Stable
			d.unstable = ev.Unstable
		}
	case "final":
		if d.finalSeen || d.recording {
			d.failed = true
		}
		d.finalSeen = true
		if !d.cancelled && !d.failed {
			d.result = ev.Text
		}
	case "cancelled", "error":
		d.failed = true
		a.cancelDictationDelivery()
		if ev.Type == "error" {
			log.Printf("dictation error code=%s", ev.Code)
		}
	case "done":
		if !d.cancelled && !d.failed && !d.recording && d.finalSeen && d.result != "" && ev.Outcome == "final" {
			a.finishDictationDelivery(d.result)
		} else {
			a.cancelDictationDelivery()
		}
		a.Dictation = dictationSession{}
		if a.DictationKeys != nil {
			a.DictationKeys.finish()
		}
	}
	return true
}

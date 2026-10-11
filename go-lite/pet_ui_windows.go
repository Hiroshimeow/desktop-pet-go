//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"desktop-pet-lite-go/internal/agent"
)

const (
	petChatSend   = 2101
	petChatMic    = 2102
	petChatCancel = 2103
	petMicStop    = 2104
	petMicCancel  = 2105
)

var (
	uiSetText      = user32.NewProc("SetWindowTextW")
	uiMove         = user32.NewProc("MoveWindow")
	uiMonitorPoint = user32.NewProc("MonitorFromPoint")
	uiMonitorInfo  = user32.NewProc("GetMonitorInfoW")
	uiSetFocus     = user32.NewProc("SetFocus")
	uiForeground   = user32.NewProc("SetForegroundWindow")
	uiGetDPI       = user32.NewProc("GetDpiForWindow")
)

type petChat struct {
	hwnd, input, output, mic, launcher, launcherMic uintptr
	renderedReply, renderedMic                      string
	busy, speakReply                                bool
	serial                                          uint64
	turn, reply                                     string
	cancel                                          context.CancelFunc
	request                                         func(context.Context, string, func(agent.Event)) (string, error)
}
type petPreview struct {
	hwnd                     uintptr
	status, stable, unstable string
	rendered                 string
	point                    point
}
type uiRect struct{ left, top, right, bottom int32 }

func uiText(hwnd uintptr, text string) {
	p, err := syscall.UTF16PtrFromString(text)
	if err == nil && hwnd != 0 {
		uiSetText.Call(hwnd, uintptr(unsafe.Pointer(p)))
	}
}
func uiWindow(class, title string, ex, style uintptr, x, y, w, h int32, parent, id uintptr) uintptr {
	c, _ := syscall.UTF16PtrFromString(class)
	t, _ := syscall.UTF16PtrFromString(title)
	hwnd, _, err := pCreateWindowExW.Call(ex, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(t)), style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, id, 0, 0)
	if hwnd == 0 {
		log.Printf("pet UI window creation failed class=%s: %v", class, err)
	}
	return hwnd
}

func (a *App) showPetChat() {
	c := &a.Chat
	if c.hwnd == 0 {
		class, _ := syscall.UTF16PtrFromString("DesktopPetChatWindow")
		wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: syscall.NewCallback(petUIProc), ClassName: class}
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		c.hwnd = uiWindow("DesktopPetChatWindow", "Desktop Pet — Chat", WS_EX_TOOLWINDOW, 0x00cf0000, 100, 100, 580, 400, 0, 0)
		if c.hwnd == 0 {
			return
		}
		c.output = uiWindow("EDIT", "", 0x200, 0x502008c4, 12, 12, 540, 235, c.hwnd, 2201) // read-only multiline, Unicode; never clipboard-backed
		c.input = uiWindow("EDIT", "", 0x200, 0x50000004, 12, 255, 540, 50, c.hwnd, 2202)
		uiWindow("BUTTON", "Send", 0, 0x50010000, 12, 315, 110, 30, c.hwnd, petChatSend)
		c.mic = uiWindow("BUTTON", "Mic", 0, 0x50010000, 132, 315, 110, 30, c.hwnd, petChatMic)
		uiWindow("BUTTON", "Cancel", 0, 0x50010000, 252, 315, 110, 30, c.hwnd, petChatCancel)
	}
	uiText(c.output, c.reply)
	pShowWindow.Call(c.hwnd, SW_SHOW)
	uiForeground.Call(c.hwnd) // Text chat is intentional focus; dictation preview is not.
	uiSetFocus.Call(c.input)
}

func petUIProc(hwnd uintptr, message uint32, wp, lp uintptr) uintptr {
	if app != nil && hwnd == app.Chat.launcher && message == 0x21 {
		return 3
	} // MA_NOACTIVATE
	if app != nil {
		switch message {
		case 0x111: // WM_COMMAND
			switch wp & 0xffff {
			case petChatSend:
				if text, err := nativeEditDocument(app.Chat.input); err == nil {
					app.submitPetChat(syscall.UTF16ToString(text), false)
				} else {
					log.Printf("chat input unavailable: %v", err)
				}
			case petChatMic, petMicStop:
				app.togglePetMic()
			case petChatCancel, petMicCancel:
				app.cancelPetChat()
				if app.Dictation.conversation {
					app.handleDictationInput(dictationInput{action: dictationCancel})
				}
			}
			return 0
		case WM_CLOSE:
			app.cancelPetChat()
			if app.Dictation.conversation {
				app.handleDictationInput(dictationInput{action: dictationCancel})
			}
			pShowWindow.Call(hwnd, 0)
			return 0
		}
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wp, lp)
	return r
}

func (a *App) submitPetChat(text string, speak bool) {
	c := &a.Chat
	text = strings.TrimSpace(text)
	if text == "" || c.busy || a.Dictation.sessionID != "" || a.DictationDelivery.text != "" {
		return
	}
	if c.request == nil {
		dir, err := locateVoiceSidecar()
		if err != nil {
			c.reply = "Agent unavailable"
			return
		}
		c.request = agent.NewZeroClaw(filepath.Dir(dir)).ChatEvents
	}
	c.serial++
	c.turn = fmt.Sprintf("pet-chat-%d", c.serial)
	c.busy, c.speakReply = true, speak
	c.reply = "Thinking…"
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	turn, request, events := c.turn, c.request, a.VoiceEvents
	go func() {
		defer cancel()
		requestCtx, requestCancel := context.WithTimeout(ctx, 2*time.Minute)
		reply, err := request(requestCtx, text, nil)
		requestCancel()
		ev := voiceEvent{Type: "pet_chat_reply", TurnID: turn, Text: reply}
		if err != nil {
			ev.Type = "pet_chat_error"
			ev.Detail = "Agent unavailable or request failed"
		}
		select {
		case events <- ev:
		case <-ctx.Done():
		}
	}()
}
func (a *App) cancelPetChat() {
	if a.Chat.cancel != nil {
		a.Chat.cancel()
		a.Chat.cancel = nil
	}
	a.Chat.turn = ""
	a.Chat.busy = false
}
func (a *App) handlePetChatEvent(ev voiceEvent) bool {
	if ev.Type != "pet_chat_reply" && ev.Type != "pet_chat_error" {
		return false
	}
	c := &a.Chat
	if ev.TurnID != c.turn || !c.busy {
		return true
	}
	c.busy = false
	c.reply = ev.Text
	if ev.Type == "pet_chat_error" {
		c.reply = ev.Detail
	}
	if c.speakReply {
		a.Preview.status = c.reply
	}
	if c.speakReply && ev.Type == "pet_chat_reply" && a.Voice != nil && !a.Voice.isStopped() {
		if lang, err := agentReplyLanguage(ev.Text); err == nil {
			a.Voice.enqueueRequests([]voiceSpeakRequest{{Text: ev.Text, Lang: lang}})
		}
	}
	return true
}
func (a *App) togglePetMic() {
	if a.Dictation.conversation && a.Dictation.sessionID != "" {
		a.handleDictationInput(dictationInput{action: dictationStop})
		return
	}
	v := a.Voice
	if a.Chat.busy || a.Dictation.sessionID != "" || a.DictationDelivery.text != "" {
		a.Chat.reply = "Audio or final delivery is busy"
		a.Preview.status = a.Chat.reply
		return
	}
	if v == nil || v.isStopped() {
		a.startVoiceController(false, nil)
		a.Chat.reply = "Local voice starting; click Mic when ready"
		a.Preview.status = a.Chat.reply
		return
	}
	if !v.ready || v.listen || v.commandMode || v.activeTurn != "" || len(v.queue) != 0 {
		a.Chat.reply = "Local voice is not ready or audio is busy"
		a.Preview.status = a.Chat.reply
		return
	}
	a.DictationSerial++
	a.Dictation = dictationSession{sessionID: fmt.Sprintf("mic-%d-%d", time.Now().UnixNano(), a.DictationSerial), recording: true, conversation: true}
	a.Preview.stable, a.Preview.unstable = "", ""
	a.Preview.status = "Voice conversation — recording. Mic/Stop to finish; Cancel to discard."
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&a.Preview.point)))
	if err := v.send(voiceCommand{Type: "dictation_start", ProtocolVersion: 1, SessionID: a.Dictation.sessionID}); err != nil {
		a.Dictation = dictationSession{}
		a.Preview.status = "Local microphone unavailable"
	}
}

func previewPlacement(x, y, w, h int32, r uiRect) (int32, int32, int32, int32) {
	w = min(w, r.right-r.left)
	h = min(h, r.bottom-r.top)
	x = int32(max(int(r.left), int(min(x, r.right-w))))
	y = int32(max(int(r.top), int(min(y, r.bottom-h))))
	return x, y, w, h
}
func (a *App) updatePetUI() {
	c := &a.Chat
	if a.PetChatEnabled && len(a.Pets) > 0 {
		if c.launcher == 0 {
			class, _ := syscall.UTF16PtrFromString("DesktopPetMicWindow")
			wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: syscall.NewCallback(petUIProc), ClassName: class}
			pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
			c.launcher = uiWindow("DesktopPetMicWindow", "Mic", 0x08000088, WS_POPUP, 0, 0, 128, 28, 0, 0)
			c.launcherMic = uiWindow("BUTTON", "Mic", 0, 0x50000000, 0, 0, 64, 28, c.launcher, petChatMic)
			uiWindow("BUTTON", "Cancel", 0, 0x50000000, 64, 0, 64, 28, c.launcher, petMicCancel)
		}
		p := a.Pets[0]
		uiMove.Call(c.launcher, uintptr(int32(p.Pet.X)), uintptr(int32(p.Pet.Y)+int32(p.FrameH)-28), 128, 28, 1)
		visible, _, _ := deliveryVisible.Call(c.launcher)
		if visible == 0 {
			pShowWindow.Call(c.launcher, 4)
		}
	}
	if c.hwnd != 0 || c.launcher != 0 {
		if c.reply != c.renderedReply {
			uiText(c.output, c.reply)
			c.renderedReply = c.reply
		}
		label := "Mic"
		if a.Dictation.conversation && a.Dictation.recording {
			label = "Stop mic"
		}
		if c.renderedMic != label {
			uiText(c.mic, label)
			uiText(c.launcherMic, label)
			c.renderedMic = label
		}
	}
	p := &a.Preview
	label := p.status
	if p.stable != "" {
		label += "\r\nStable: " + p.stable
	}
	if p.unstable != "" {
		label += "\r\nPreview: " + p.unstable
	}
	if label == p.rendered {
		return
	}
	p.rendered = label
	if label == "" {
		if p.hwnd != 0 {
			pShowWindow.Call(p.hwnd, 0)
		}
		return
	}
	if len(a.Pets) == 0 {
		return
	}
	if p.hwnd == 0 {
		p.hwnd = uiWindow("STATIC", "", 0x080800a8, WS_POPUP|0x00800100, 0, 0, 560, 160, 0, 0)
	} // NOACTIVATE + TRANSPARENT + TOOLWINDOW + TOPMOST
	if p.hwnd == 0 {
		return
	}
	user32.NewProc("SetLayeredWindowAttributes").Call(p.hwnd, 0, 255, 2)
	dpi := uintptr(96)
	if uiGetDPI.Find() == nil {
		if d, _, _ := uiGetDPI.Call(p.hwnd); d != 0 {
			dpi = d
		}
	}
	var info struct {
		size          uint32
		monitor, work uiRect
		flags         uint32
	}
	info.size = uint32(unsafe.Sizeof(info))
	packed := uintptr(uint64(uint32(p.point.X)) | uint64(uint32(p.point.Y))<<32)
	monitor, _, _ := uiMonitorPoint.Call(packed, 2)
	uiMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
	x, y, w, h := previewPlacement(p.point.X+16, p.point.Y+20, int32(560*dpi/96), int32(160*dpi/96), info.work)
	uiMove.Call(p.hwnd, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
	uiText(p.hwnd, label)
	pShowWindow.Call(p.hwnd, 4) // SW_SHOWNOACTIVATE
}
func (a *App) closePetUI() {
	a.cancelPetChat()
	for _, hwnd := range []uintptr{a.Chat.hwnd, a.Chat.launcher, a.Preview.hwnd} {
		if hwnd != 0 {
			pDestroyWindow.Call(hwnd)
		}
	}
}

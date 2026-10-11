//go:build windows

package main

import (
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestDictationNativeRegistrationConflictAndCleanup(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// No user application/window receives input in this test.
	b, _ := parseDictationBindings("Ctrl+Shift+F24", "Ctrl+Shift+F23")
	ok, _, err := dictRegisterHotKey.Call(0, 71, uintptr(b.toggle.mods|dictNoRepeat), uintptr(b.toggle.vk))
	if ok == 0 {
		t.Fatalf("cannot reserve disposable shortcut: %v", err)
	}
	h, err := startDictationHotkeys(b)
	if err == nil {
		h.stop()
		t.Fatal("existing global registration must refuse startup")
	}
	dictUnregisterHotKey.Call(0, 71)
	h, err = startDictationHotkeys(b)
	if err != nil {
		t.Fatal(err)
	}
	h.stop()
	h.stop()
	ok, _, err = dictRegisterHotKey.Call(0, 72, uintptr(b.toggle.mods|dictNoRepeat), uintptr(b.toggle.vk))
	if ok == 0 {
		t.Fatalf("shortcut leaked after hook shutdown: %v", err)
	}
	dictUnregisterHotKey.Call(0, 72)
}

func TestDictationNativeInjectedInputPassesToDisposableWindow(t *testing.T) {
	if os.Getenv("DP_DICTATION_NATIVE_TEST") != "1" {
		t.Skip("requires explicit disposable-window native input test")
	}
	if dictationModifiers() != 0 {
		t.Fatal("release modifiers before native input test; no input sent")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	old, _, _ := dictForeground.Call()
	setForeground := user32.NewProc("SetForegroundWindow")
	setFocus := user32.NewProc("SetFocus")
	getText := user32.NewProc("GetWindowTextW")
	send := user32.NewProc("SendInput")
	class, _ := syscall.UTF16PtrFromString("EDIT")
	hwnd, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, 0x10cf0000, 20, 20, 320, 120, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(err)
	}
	defer pDestroyWindow.Call(hwnd)
	defer setForeground.Call(old)
	setForeground.Call(hwnd)
	setFocus.Call(hwnd)
	if fg, _, _ := dictForeground.Call(); fg != hwnd {
		t.Fatal("disposable window did not receive foreground; no input sent")
	}
	var f5Received atomic.Int32
	setProc := user32.NewProc("SetWindowLongPtrW")
	callProc := user32.NewProc("CallWindowProcW")
	var original uintptr
	callback := syscall.NewCallback(func(w uintptr, m uint32, wp, lp uintptr) uintptr {
		if m == dictKeyDown && wp == 0x74 {
			f5Received.Add(1)
		}
		r, _, _ := callProc.Call(original, w, uintptr(m), wp, lp)
		return r
	})
	original, _, _ = setProc.Call(hwnd, ^uintptr(3), callback) // GWLP_WNDPROC = -4
	defer setProc.Call(hwnd, ^uintptr(3), original)
	b, _ := parseDictationBindings("F5", "F6")
	h, err := startDictationHotkeys(b)
	if err != nil {
		t.Fatal(err)
	}
	defer h.stop()
	h.ready.Store(true)
	// Native INPUT ABI: union offset 8, total 40 bytes on amd64.
	type input struct {
		kind             uint32
		pad              uint32
		vk, scan         uint16
		flags, timestamp uint32
		extra            uintptr
		tail             [8]byte
	}
	if unsafe.Sizeof(input{}) != 40 {
		t.Fatal("unsupported INPUT ABI for native test")
	}
	for _, vk := range []uint16{0x74, 0x75, 0x1b, 'A'} {
		if fg, _, _ := dictForeground.Call(); fg != hwnd {
			t.Fatal("foreground changed; no more input sent")
		}
		pair := [2]input{{kind: 1, vk: vk}, {kind: 1, vk: vk, flags: 2}}
		n, _, err := send.Call(2, uintptr(unsafe.Pointer(&pair[0])), unsafe.Sizeof(pair[0]))
		if n != 2 {
			t.Fatal("SendInput", n, err)
		}
	}
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		var m msg
		for {
			ok, _, _ := dictPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1)
			if ok == 0 {
				break
			}
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case in := <-h.inputs:
		t.Fatalf("injected input activated dictation: %+v", in)
	default:
	}
	if f5Received.Load() != 1 {
		t.Fatalf("ignored injected F5 did not reach original window: count=%d", f5Received.Load())
	}
	var text [32]uint16
	getText.Call(hwnd, uintptr(unsafe.Pointer(&text[0])), 32)
	if text[0] != 'a' {
		t.Fatalf("ordinary input was not preserved: %v", text)
	}
}

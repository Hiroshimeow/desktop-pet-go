//go:build windows

package main

import (
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestDictationNativeAnchoredUnicodeAndClipboardUntouched(t *testing.T) {
	if os.Getenv("DP_DICTATION_NATIVE_TEST") != "1" {
		t.Skip("explicit disposable Windows target test required")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	old, _, _ := dictForeground.Call()
	setForeground := user32.NewProc("SetForegroundWindow")
	setFocus := user32.NewProc("SetFocus")
	setText := user32.NewProc("SetWindowTextW")
	class, _ := syscall.UTF16PtrFromString("EDIT")
	hwnd, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, 0x10cf0004, 20, 20, 400, 160, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(err)
	}
	defer pDestroyWindow.Call(hwnd)
	defer setForeground.Call(old)
	setForeground.Call(hwnd)
	setFocus.Call(hwnd)
	if fg, _, _ := dictForeground.Call(); fg != hwnd {
		t.Fatal("disposable EDIT not foreground; no insertion")
	}
	sequence, _, _ := deliveryClipboardSequence.Call()
	reset := func(text string, start, end uintptr) dictationAnchor {
		wide, _ := syscall.UTF16PtrFromString(text)
		setText.Call(hwnd, uintptr(unsafe.Pointer(wide)))
		if _, err := deliveryMessage(hwnd, 0xb1, start, end); err != nil {
			t.Fatal(err)
		}
		a := captureDictationAnchor(captureDictationFocus())
		if !a.valid {
			t.Fatalf("anchor refused: %s", a.reason)
		}
		return a
	}
	check := func(want string) {
		text, err := nativeEditDocument(hwnd)
		if err != nil || syscall.UTF16ToString(text) != want {
			t.Fatalf("target did not match expected bounded test fixture (err=%v)", err)
		}
		seq, _, _ := deliveryClipboardSequence.Call()
		if seq != sequence {
			t.Fatal("insertion changed clipboard sequence")
		}
	}
	func(t *testing.T) {
		a := reset("left OLD right", 5, 8)
		if err := insertAnchoredNative(a, "Tiếng Việt API 😀", func() bool { return false }); err != nil {
			t.Fatal(err)
		}
		check("left Tiếng Việt API 😀 right")
	}(t)
	func(t *testing.T) {
		a := reset("original", 3, 3)
		deliveryMessage(hwnd, 0xb1, 5, 5)
		if err := insertAnchoredNative(a, "wrong", func() bool { return false }); err == nil {
			t.Fatal("moved caret accepted")
		}
		check("original")
	}(t)
	func(t *testing.T) {
		a := reset("original", 3, 3)
		wide, _ := syscall.UTF16PtrFromString("modified")
		setText.Call(hwnd, uintptr(unsafe.Pointer(wide)))
		deliveryMessage(hwnd, 0xb1, 3, 3)
		if err := insertAnchoredNative(a, "wrong", func() bool { return false }); err == nil {
			t.Fatal("changed document accepted")
		}
		check("modified")
	}(t)
	func(t *testing.T) {
		a := reset("original", 3, 3)
		if err := insertAnchoredNative(a, "wrong", func() bool { return true }); err == nil {
			t.Fatal("invalidated target accepted")
		}
		check("original")
	}(t)
	func(t *testing.T) {
		a := reset("original", 3, 3)
		other, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, 0x10cf0004, 40, 40, 200, 80, 0, 0, 0, 0)
		if other == 0 {
			t.Fatal(err)
		}
		defer pDestroyWindow.Call(other)
		setForeground.Call(other)
		setFocus.Call(other)
		if fg, _, _ := dictForeground.Call(); fg != other {
			t.Fatal("disposable second target not foreground")
		}
		if err := insertAnchoredNative(a, "wrong", func() bool { return false }); err == nil {
			t.Fatal("focus-switched target accepted")
		}
		text, err := nativeEditDocument(other)
		if err != nil || len(text) != 0 {
			t.Fatal("wrong window changed", err)
		}
		setForeground.Call(hwnd)
		setFocus.Call(hwnd)
		check("original")
	}(t)
	func(t *testing.T) {
		a := reset("original", 3, 3)
		style, _, _ := deliveryStyle.Call(hwnd, ^uintptr(15))
		setStyle := user32.NewProc("SetWindowLongPtrW")
		defer setStyle.Call(hwnd, ^uintptr(15), style)
		for _, bit := range []uintptr{0x20, 0x800} {
			setStyle.Call(hwnd, ^uintptr(15), style|bit)
			if err := insertAnchoredNative(a, "wrong", func() bool { return false }); err == nil {
				t.Fatal("secure/read-only target accepted")
			}
		}
		setStyle.Call(hwnd, ^uintptr(15), style)
		check("original")
	}(t)
	func(t *testing.T) {
		reset("original", 3, 3)
		a := &App{}
		a.beginDictationDelivery(captureDictationFocus())
		a.Dictation = dictationSession{sessionID: "native", recording: false}
		started := time.Now()
		a.handleDictationEvent(voiceEvent{Type: "final", SessionID: "native", ProtocolVersion: 1, Seq: 1, Text: " VI "})
		a.handleDictationEvent(voiceEvent{Type: "done", SessionID: "native", ProtocolVersion: 1, Seq: 2, Outcome: "final"})
		if time.Since(started) > 50*time.Millisecond {
			t.Fatal("final handling blocked UI")
		}
		deadline := time.Now().Add(time.Second)
		for a.DictationDelivery.worker != nil && time.Now().Before(deadline) {
			var m msg
			for {
				ok, _, _ := dictPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1)
				if ok == 0 {
					break
				}
				pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
				pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
			}
			a.processDictationDelivery()
			time.Sleep(time.Millisecond)
		}
		if a.DictationDelivery.worker != nil || a.DictationDelivery.text != "" {
			t.Fatalf("worker did not deliver: %s", a.DictationDelivery.reason)
		}
		if a.DictationDelivery.notice != 0 {
			pDestroyWindow.Call(a.DictationDelivery.notice)
		}
		a.handleDictationEvent(voiceEvent{Type: "final", SessionID: "native", ProtocolVersion: 1, Seq: 3, Text: "again"})
		check("ori VI ginal")
	}(t)
}

func TestDictationCopyValidationDoesNotTouchClipboard(t *testing.T) {
	sequence, _, _ := deliveryClipboardSequence.Call()
	for _, text := range []string{"valid", "contains\x00NUL"} {
		if err := copyDictationText(0, text); err == nil {
			t.Fatal("invalid copy owner accepted")
		}
	}
	seq, _, _ := deliveryClipboardSequence.Call()
	if sequence != seq {
		t.Fatal("failed validation mutated clipboard")
	}
}

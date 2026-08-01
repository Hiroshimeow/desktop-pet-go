//go:build windows

package main

import (
	"fmt"
	"log"
	"syscall"
	"unsafe"
)

const (
	voiceMenuReadClipboard = 1001
	voiceMenuPauseResume   = 1002
	voiceMenuSkip          = 1003
	voiceMenuStop          = 1004

	mfString       = 0x0000
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	cfUnicodeText  = 13
)

var (
	pCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	pAppendMenuW      = user32.NewProc("AppendMenuW")
	pTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	pDestroyMenu      = user32.NewProc("DestroyMenu")
	pOpenClipboard    = user32.NewProc("OpenClipboard")
	pCloseClipboard   = user32.NewProc("CloseClipboard")
	pGetClipboardData = user32.NewProc("GetClipboardData")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	pGlobalSize       = kernel32.NewProc("GlobalSize")
)

func (a *App) showVoiceMenu(hwnd uintptr) {
	menu, _, err := pCreatePopupMenu.Call()
	if menu == 0 {
		log.Printf("voice menu create failed: %v", err)
		return
	}
	defer pDestroyMenu.Call(menu)

	pauseLabel := "Pause"
	if a.Voice != nil && a.Voice.isPaused() {
		pauseLabel = "Resume"
	}
	for _, item := range []struct {
		id    uintptr
		label string
	}{
		{voiceMenuReadClipboard, "Read clipboard"},
		{voiceMenuPauseResume, pauseLabel},
		{voiceMenuSkip, "Skip"},
		{voiceMenuStop, "Stop"},
	} {
		if err := appendVoiceMenuItem(menu, item.id, item.label); err != nil {
			log.Printf("voice menu item failed label=%q: %v", item.label, err)
			return
		}
	}

	var pt point
	if r, _, err := pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r == 0 {
		log.Printf("voice menu cursor failed: %v", err)
		return
	}
	command, _, _ := pTrackPopupMenu.Call(
		menu,
		tpmRightButton|tpmReturnCmd,
		uintptr(int64(pt.X)),
		uintptr(int64(pt.Y)),
		0,
		hwnd,
		0,
	)
	switch command {
	case voiceMenuReadClipboard:
		a.readClipboardVoice(hwnd)
	case voiceMenuPauseResume:
		if a.Voice == nil || a.Voice.isStopped() {
			return
		}
		if a.Voice.isPaused() {
			a.Voice.resumePlayback()
		} else {
			a.Voice.pausePlayback()
		}
	case voiceMenuSkip:
		if a.Voice != nil && !a.Voice.isStopped() {
			a.Voice.skipCurrent()
		}
	case voiceMenuStop:
		if a.Voice != nil && !a.Voice.isStopped() {
			a.Voice.stopPlayback()
		}
	}
}

func appendVoiceMenuItem(menu, id uintptr, label string) error {
	text, err := syscall.UTF16PtrFromString(label)
	if err != nil {
		return err
	}
	if r, _, callErr := pAppendMenuW.Call(menu, mfString, id, uintptr(unsafe.Pointer(text))); r == 0 {
		return fmt.Errorf("AppendMenuW: %w", callErr)
	}
	return nil
}

func (a *App) readClipboardVoice(hwnd uintptr) {
	text, err := readClipboardText(hwnd)
	if err != nil {
		log.Printf("voice clipboard ignored: %v", err)
		return
	}
	requests, err := buildVoiceRequestsFromText(text, "auto")
	if err != nil {
		log.Printf("voice clipboard ignored: %v", err)
		return
	}
	if a.Voice == nil || a.Voice.isStopped() {
		a.startVoiceController(false, requests)
		return
	}
	a.Voice.enqueueRequests(requests)
}

func readClipboardText(hwnd uintptr) (string, error) {
	if r, _, err := pOpenClipboard.Call(hwnd); r == 0 {
		return "", fmt.Errorf("OpenClipboard: %w", err)
	}
	defer pCloseClipboard.Call()

	handle, _, err := pGetClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		return "", fmt.Errorf("clipboard has no Unicode text: %w", err)
	}
	size, _, err := pGlobalSize.Call(handle)
	if size < 2 {
		return "", fmt.Errorf("clipboard Unicode text is empty: %w", err)
	}
	ptr, _, err := pGlobalLock.Call(handle)
	if ptr == 0 {
		return "", fmt.Errorf("GlobalLock clipboard: %w", err)
	}
	defer pGlobalUnlock.Call(handle)

	text := syscall.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), int(size/2)))
	if text == "" {
		return "", fmt.Errorf("clipboard Unicode text is empty")
	}
	return text, nil
}

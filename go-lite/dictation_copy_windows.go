//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	deliveryAlloc             = kernel32.NewProc("GlobalAlloc")
	deliveryFree              = kernel32.NewProc("GlobalFree")
	deliveryEmpty             = user32.NewProc("EmptyClipboard")
	deliverySetData           = user32.NewProc("SetClipboardData")
	deliveryRegisterFormat    = user32.NewProc("RegisterClipboardFormatW")
	deliveryClipboardSequence = user32.NewProc("GetClipboardSequenceNumber")
)

func clipboardMemory(data []byte) (uintptr, error) {
	h, _, _ := deliveryAlloc.Call(2, uintptr(len(data)))
	if h == 0 {
		return 0, fmt.Errorf("clipboard allocation failed")
	}
	p, _, _ := pGlobalLock.Call(h)
	if p == 0 {
		deliveryFree.Call(h)
		return 0, fmt.Errorf("clipboard memory unavailable")
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(p)), len(data)), data)
	pGlobalUnlock.Call(h)
	return h, nil
}

// Explicit Copy deliberately replaces clipboard contents, never follows a
// failed automatic insertion implicitly. Preallocate before EmptyClipboard.
// No audio/transcript logging and all three history/cloud opt-out markers.
func copyDictationText(owner uintptr, text string) error {
	wide, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}
	if owner == 0 {
		return fmt.Errorf("clipboard owner required")
	}
	type item struct{ format, handle uintptr }
	items := []item{}
	defer func() {
		for _, i := range items {
			if i.handle != 0 {
				deliveryFree.Call(i.handle)
			}
		}
	}()
	h, err := clipboardMemory(unsafe.Slice((*byte)(unsafe.Pointer(&wide[0])), len(wide)*2))
	if err != nil {
		return err
	}
	items = append(items, item{13, h})
	for _, marker := range []struct {
		name  string
		value uint32
	}{
		{"ExcludeClipboardContentFromMonitorProcessing", 1},
		{"CanIncludeInClipboardHistory", 0},
		{"CanUploadToCloudClipboard", 0},
	} {
		name, _ := syscall.UTF16PtrFromString(marker.name)
		format, _, _ := deliveryRegisterFormat.Call(uintptr(unsafe.Pointer(name)))
		if format == 0 {
			return fmt.Errorf("clipboard privacy marker unavailable")
		}
		value := marker.value
		h, err := clipboardMemory(unsafe.Slice((*byte)(unsafe.Pointer(&value)), 4))
		if err != nil {
			return err
		}
		items = append(items, item{format, h})
	}
	ok, _, err := pOpenClipboard.Call(owner)
	if ok == 0 {
		return fmt.Errorf("clipboard busy; final retained: %w", err)
	}
	defer pCloseClipboard.Call()
	ok, _, err = deliveryEmpty.Call()
	if ok == 0 {
		return fmt.Errorf("clipboard unchanged: %w", err)
	}
	// Publish exclusion markers before transcript, while the lock prevents a
	// competing copy. SetData ownership transfers only on success.
	for _, index := range []int{1, 2, 3, 0} {
		i := &items[index]
		ok, _, err := deliverySetData.Call(i.format, i.handle)
		if ok == 0 {
			return fmt.Errorf("clipboard publication failed; final retained: %w", err)
		}
		i.handle = 0
	}
	return nil
}

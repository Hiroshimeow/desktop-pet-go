//go:build windows

package main

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const dictationMaxDocument = 1024 * 1024 // UTF-16 units; larger/inaccessible documents are copy-only.

var (
	deliveryClass          = user32.NewProc("GetClassNameW")
	deliveryStyle          = user32.NewProc("GetWindowLongPtrW")
	deliveryEnabled        = user32.NewProc("IsWindowEnabled")
	deliveryVisible        = user32.NewProc("IsWindowVisible")
	deliveryProcessTimes   = kernel32.NewProc("GetProcessTimes")
	deliveryOpenToken      = syscall.NewLazyDLL("advapi32.dll").NewProc("OpenProcessToken")
	deliveryTokenInfo      = syscall.NewLazyDLL("advapi32.dll").NewProc("GetTokenInformation")
	deliverySIDCount       = syscall.NewLazyDLL("advapi32.dll").NewProc("GetSidSubAuthorityCount")
	deliverySIDPart        = syscall.NewLazyDLL("advapi32.dll").NewProc("GetSidSubAuthority")
	deliveryCurrentProcess = kernel32.NewProc("GetCurrentProcess")
)

// Native EDIT/ RichEdit message contracts supply a concrete selection and
// synchronous consumption without a global Ctrl+V/clipboard race. Other
// providers need a separately verified UIA anchor; they are NOT guessed at.
type dictationAnchor struct {
	focus      dictationFocus
	created    uint64
	integrity  uint32
	start, end uint32
	document   [32]byte
	valid      bool
	reason     string
}

func deliveryMessage(hwnd, message, wp, lp uintptr) (uintptr, error) {
	var result uintptr
	ok, _, err := dictSendTimeout.Call(hwnd, message, wp, lp, 0x2|0x20, 100, uintptr(unsafe.Pointer(&result)))
	if ok == 0 {
		return 0, fmt.Errorf("target unavailable or timed out: %w", err)
	}
	return result, nil
}

func nativeDictationEdit(hwnd uintptr) bool {
	var name [128]uint16
	n, _, _ := deliveryClass.Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	class := strings.ToLower(syscall.UTF16ToString(name[:int(n)]))
	return class == "edit" || class == "richedit20w" || class == "richedit50w"
}

// This bounded query is made on the physical activation edge, before the
// input is queued or any UI is displayed. Never read text from a password.
func captureNativeSelection(f *dictationFocus) {
	if !f.valid || !nativeDictationEdit(f.focus) {
		return
	}
	style, _, _ := deliveryStyle.Call(f.focus, ^uintptr(15)) // GWL_STYLE = -16
	if style&(0x20|0x800) != 0 {
		return
	} // ES_PASSWORD / ES_READONLY
	var start, end uint32
	var result uintptr
	ok, _, _ := dictSendTimeout.Call(f.focus, 0xb0, uintptr(unsafe.Pointer(&start)), uintptr(unsafe.Pointer(&end)), 0x2|0x20, 10, uintptr(unsafe.Pointer(&result)))
	if ok != 0 {
		f.selectionStart, f.selectionEnd, f.selectionValid = start, end, true
	}
}

func processIntegrity(process uintptr) (uint32, error) {
	var token uintptr
	ok, _, err := deliveryOpenToken.Call(process, 8, uintptr(unsafe.Pointer(&token)))
	if ok == 0 {
		return 0, fmt.Errorf("target security unavailable: %w", err)
	}
	defer syscall.CloseHandle(syscall.Handle(token))
	var needed uint32
	deliveryTokenInfo.Call(token, 25, 0, 0, uintptr(unsafe.Pointer(&needed))) // TokenIntegrityLevel
	if needed == 0 || needed > 4096 {
		return 0, fmt.Errorf("target integrity unavailable")
	}
	buf := make([]byte, needed)
	ok, _, err = deliveryTokenInfo.Call(token, 25, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)))
	if ok == 0 {
		return 0, fmt.Errorf("target integrity unavailable: %w", err)
	}
	sid := *(*uintptr)(unsafe.Pointer(&buf[0]))
	count, _, _ := deliverySIDCount.Call(sid)
	if count == 0 || *(*byte)(unsafe.Pointer(count)) == 0 {
		return 0, fmt.Errorf("invalid integrity SID")
	}
	part, _, _ := deliverySIDPart.Call(sid, uintptr(*(*byte)(unsafe.Pointer(count))-1))
	if part == 0 {
		return 0, fmt.Errorf("invalid integrity SID")
	}
	return *(*uint32)(unsafe.Pointer(part)), nil
}

func nativeTargetIdentity(f dictationFocus) (uint64, uint32, error) {
	current := captureDictationFocus()
	if !f.valid || !current.valid || current.top != f.top || current.focus != f.focus || current.pid != f.pid {
		return 0, 0, fmt.Errorf("original target focus changed")
	}
	var focusPID uint32
	dictWindowThread.Call(f.focus, uintptr(unsafe.Pointer(&focusPID)))
	if focusPID != f.pid {
		return 0, 0, fmt.Errorf("target control owner changed")
	}
	if !nativeDictationEdit(f.focus) {
		return 0, 0, fmt.Errorf("target has no verified native caret anchor")
	}
	style, _, _ := deliveryStyle.Call(f.focus, ^uintptr(15))
	if style&0x20 != 0 {
		return 0, 0, fmt.Errorf("password field: copy only")
	}
	if style&0x800 != 0 {
		return 0, 0, fmt.Errorf("read-only field: copy only")
	}
	enabled, _, _ := deliveryEnabled.Call(f.focus)
	visible, _, _ := deliveryVisible.Call(f.focus)
	if enabled == 0 || visible == 0 {
		return 0, 0, fmt.Errorf("target not writable/visible")
	}
	process, _, err := openProcess.Call(0x1000, 0, uintptr(f.pid)) // PROCESS_QUERY_LIMITED_INFORMATION
	if process == 0 {
		return 0, 0, fmt.Errorf("target process unavailable: %w", err)
	}
	defer syscall.CloseHandle(syscall.Handle(process))
	var creation, exit, kernel, user syscall.Filetime
	ok, _, err := deliveryProcessTimes.Call(process, uintptr(unsafe.Pointer(&creation)), uintptr(unsafe.Pointer(&exit)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return 0, 0, fmt.Errorf("target process identity unavailable: %w", err)
	}
	level, err := processIntegrity(process)
	if err != nil {
		return 0, 0, err
	}
	self, _, _ := deliveryCurrentProcess.Call()
	own, err := processIntegrity(self)
	if err != nil || own != level {
		return 0, 0, fmt.Errorf("different target integrity: copy only")
	}
	// Elevated targets are copy-only even when the pet itself is elevated.
	if level >= 0x3000 {
		return 0, 0, fmt.Errorf("elevated target: copy only")
	}
	return uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime), level, nil
}

func nativeEditDocument(hwnd uintptr) ([]uint16, error) {
	n, err := deliveryMessage(hwnd, 0xe, 0, 0) // WM_GETTEXTLENGTH
	if err != nil {
		return nil, err
	}
	if n > dictationMaxDocument {
		return nil, fmt.Errorf("target document too large to verify")
	}
	buf := make([]uint16, int(n)+1)
	got, err := deliveryMessage(hwnd, 0xd, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0]))) // WM_GETTEXT
	if err != nil {
		return nil, err
	}
	if got != n {
		return nil, fmt.Errorf("target document changed during capture")
	}
	return buf[:int(n)], nil
}
func nativeEditSelection(hwnd uintptr) (uint32, uint32, error) {
	var start, end uint32
	_, err := deliveryMessage(hwnd, 0xb0, uintptr(unsafe.Pointer(&start)), uintptr(unsafe.Pointer(&end)))
	return start, end, err
}
func documentDigest(text []uint16) [32]byte {
	if len(text) == 0 {
		return sha256.Sum256(nil)
	}
	return sha256.Sum256(unsafe.Slice((*byte)(unsafe.Pointer(&text[0])), len(text)*2))
}
func captureDictationAnchor(f dictationFocus) dictationAnchor {
	a := dictationAnchor{focus: f}
	creation, level, err := nativeTargetIdentity(f)
	if err != nil {
		a.reason = err.Error()
		return a
	}
	if !f.selectionValid {
		a.reason = "original selection unavailable"
		return a
	}
	text, err := nativeEditDocument(f.focus)
	if err != nil {
		a.reason = err.Error()
		return a
	}
	start, end, err := nativeEditSelection(f.focus)
	if err != nil || start != f.selectionStart || end != f.selectionEnd || end < start || int(end) > len(text) {
		a.reason = "original caret/selection changed"
		return a
	}
	a.created, a.integrity, a.start, a.end, a.document, a.valid = creation, level, start, end, documentDigest(text), true
	return a
}

// Never sets focus, selects a range, sends keys, or retries an ambiguous write.
// The concrete control receives ONE bounded Unicode replacement at its
// unchanged selection. The complete document is checked before and after.
func insertAnchoredNative(a dictationAnchor, text string, invalid func() bool) error {
	if !a.valid {
		return fmt.Errorf("%s", a.reason)
	}
	wide, err := syscall.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("final contains an invalid NUL")
	}
	if text == "" || len(wide) > dictationMaxDocument {
		return fmt.Errorf("empty/oversized final")
	}
	created, level, err := nativeTargetIdentity(a.focus)
	if err != nil {
		return err
	}
	if created != a.created || level != a.integrity {
		return fmt.Errorf("target process/security changed")
	}
	before, err := nativeEditDocument(a.focus.focus)
	if err != nil {
		return err
	}
	start, end, err := nativeEditSelection(a.focus.focus)
	if err != nil {
		return err
	}
	if start != a.start || end != a.end || documentDigest(before) != a.document {
		return fmt.Errorf("original document/caret changed")
	}
	if invalid() {
		return fmt.Errorf("target changed or dictation cancelled")
	}
	// Recheck actual foreground immediately before addressed delivery.
	if _, _, err := nativeTargetIdentity(a.focus); err != nil {
		return err
	}
	if invalid() {
		return fmt.Errorf("target changed or dictation cancelled")
	}
	if _, err := deliveryMessage(a.focus.focus, 0xc2, 1, uintptr(unsafe.Pointer(&wide[0]))); err != nil {
		return fmt.Errorf("delivery uncertain; never automatically retried: %w", err)
	}
	expected := append(append(append([]uint16{}, before[:start]...), wide[:len(wide)-1]...), before[end:]...)
	after, err := nativeEditDocument(a.focus.focus)
	if err != nil || documentDigest(after) != documentDigest(expected) {
		return fmt.Errorf("delivery uncertain; verify target before explicit copy")
	}
	return nil
}

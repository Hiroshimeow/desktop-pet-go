//go:build windows

package main

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// An unresolved final stays in RAM. Never replace it with a later session,
// retry ambiguous insertion, log its contents, or implicitly copy it.
type dictationDelivery struct {
	text    string
	worker  *dictationDeliveryWorker
	notice  uintptr
	reason  string
	copying bool
}
type dictationDeliveryWorker struct {
	cancelled   atomic.Bool
	anchor      chan dictationAnchor
	result      chan error
	epoch       uint64
	cancelEpoch uint64
}

var deliveryCancelEpoch atomic.Uint64

func fenceDictationPhysicalAction(action dictationAction) {
	if action == dictationCancel {
		deliveryCancelEpoch.Add(1)
	}
}
func (w *dictationDeliveryWorker) invalid() bool {
	return w.cancelled.Load() || w.epoch != deliveryEpoch.Load() || w.cancelEpoch != deliveryCancelEpoch.Load()
}

var deliveryEpoch atomic.Uint64
var deliveryWinEvent = user32.NewProc("SetWinEventHook")
var deliveryUnhookEvent = user32.NewProc("UnhookWinEvent")
var deliveryEventCallback = syscall.NewCallback(func(hook uintptr, event uint32, hwnd uintptr, object, child int32, thread, at uint32) uintptr {
	// Track even switch-away-and-back, caret and document changes while
	// capture/finalization run. Nonactivating pet notices do not emit focus.
	if event == 3 || event == 0x8001 || event == 0x8005 || event == 0x8006 || event == 0x800a || event == 0x800e || event == 0x8014 || (event == 0x800b && object == -8) {
		deliveryEpoch.Add(1)
	}
	return 0
})

// Called on the existing pumped hotkey thread. No new service/window owner.
func watchDictationTarget() (func(), error) {
	a, _, err := deliveryWinEvent.Call(3, 3, 0, deliveryEventCallback, 0, 0, 0)
	if a == 0 {
		return nil, fmt.Errorf("foreground safety hook unavailable: %w", err)
	}
	b, _, err := deliveryWinEvent.Call(0x8000, 0x8014, 0, deliveryEventCallback, 0, 0, 0)
	if b == 0 {
		deliveryUnhookEvent.Call(a)
		return nil, fmt.Errorf("caret safety hook unavailable: %w", err)
	}
	return func() { deliveryUnhookEvent.Call(a); deliveryUnhookEvent.Call(b) }, nil
}

func (a *App) beginDictationDelivery(f dictationFocus) {
	if a.DictationDelivery.notice != 0 {
		pDestroyWindow.Call(a.DictationDelivery.notice)
	}
	w := &dictationDeliveryWorker{anchor: make(chan dictationAnchor, 1), result: make(chan error, 1), epoch: f.epoch, cancelEpoch: f.cancelEpoch}
	a.DictationDelivery = dictationDelivery{worker: w}
	go func() {
		anchor := captureDictationAnchor(f)
		if w.epoch != deliveryEpoch.Load() {
			anchor.valid = false
			anchor.reason = "target/caret changed after activation"
		}
		w.anchor <- anchor
	}()
}
func (a *App) cancelDictationDelivery() {
	if w := a.DictationDelivery.worker; w != nil {
		w.cancelled.Store(true)
	}
}
func (a *App) finishDictationDelivery(text string) {
	d := &a.DictationDelivery
	d.text = text
	if d.worker == nil {
		d.reason = "target anchor unavailable; explicit copy only"
		return
	}
	w := d.worker
	go func() {
		anchor := <-w.anchor
		err := insertAnchoredNative(anchor, text, w.invalid)
		w.result <- err
	}()
}
func (a *App) processDictationDelivery() {
	d := &a.DictationDelivery
	if d.worker == nil {
		return
	}
	select {
	case err := <-d.worker.result:
		d.worker = nil
		if err == nil {
			d.text = ""
			d.reason = "Final inserted once. Clipboard unchanged."
			if d.copying {
				d.reason = "Final copied by request. Clipboard replaced; no paste."
			}
		} else {
			d.reason = err.Error() + ". Not retried. Right-click pet: Copy final or Discard."
		}
		a.showDictationNotice(d.reason)
	default:
	}
}
func (a *App) showDictationNotice(reason string) {
	// Reuse the same nonactivating live panel for completion/failure; do not
	// create a second notification window over the original target.
	a.Preview.status = reason
	a.Preview.stable, a.Preview.unstable = "", ""
}
func (a *App) discardDictationFinal() {
	a.cancelDictationDelivery()
	if a.DictationDelivery.worker != nil {
		return
	} // Wait for single attempt to settle.
	if a.DictationDelivery.notice != 0 {
		pDestroyWindow.Call(a.DictationDelivery.notice)
	}
	a.DictationDelivery = dictationDelivery{}
}
func (a *App) copyDictationFinal() {
	if a.DictationDelivery.text == "" || a.DictationDelivery.worker != nil {
		return
	}
	text := a.DictationDelivery.text
	w := &dictationDeliveryWorker{result: make(chan error, 1)}
	a.DictationDelivery.worker = w
	a.DictationDelivery.copying = true
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		// Clipboard ownership requires a real HWND, never OpenClipboard(NULL)
		// followed by EmptyClipboard (which leaves no valid owner for SetData).
		class, _ := syscall.UTF16PtrFromString("STATIC")
		hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 0, 0, ^uintptr(2), 0, 0, 0) // HWND_MESSAGE
		if hwnd == 0 {
			w.result <- fmt.Errorf("copy owner unavailable; final retained")
			return
		}
		defer pDestroyWindow.Call(hwnd)
		w.result <- copyDictationText(hwnd, text)
	}()
}

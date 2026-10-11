//go:build windows

package main

import "testing"

func TestDictationAnchorInvalidDoesNotAttemptDelivery(t *testing.T) {
	sequence, _, _ := deliveryClipboardSequence.Call()
	if err := insertAnchoredNative(dictationAnchor{reason: "unverifiable target"}, "Tiếng Việt", func() bool { t.Fatal("invalid target reached write fence"); return false }); err == nil {
		t.Fatal("invalid target accepted")
	}
	seq, _, _ := deliveryClipboardSequence.Call()
	if seq != sequence {
		t.Fatal("invalid target changed clipboard")
	}
}

func TestDictationDeliveryPreservesUnresolvedFinalOnNewStart(t *testing.T) {
	a := &App{DictationDelivery: dictationDelivery{text: "retained"}}
	a.handleDictationInput(dictationInput{action: dictationStart})
	if a.DictationDelivery.text != "retained" {
		t.Fatal("unresolved final replaced")
	}
}

func TestDictationCopyNeverReportsInsertion(t *testing.T) {
	w := &dictationDeliveryWorker{result: make(chan error, 1)}
	w.result <- nil
	a := &App{DictationDelivery: dictationDelivery{text: "final", worker: w, copying: true}}
	a.processDictationDelivery()
	if a.DictationDelivery.reason != "Final copied by request. Clipboard replaced; no paste." {
		t.Fatalf("wrong copy status: %s", a.DictationDelivery.reason)
	}
}

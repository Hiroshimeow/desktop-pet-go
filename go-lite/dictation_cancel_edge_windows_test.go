//go:build windows

package main

import "testing"

func TestDictationPhysicalCancelFencesWorkerBeforeUIQueue(t *testing.T) {
	w := &dictationDeliveryWorker{epoch: deliveryEpoch.Load(), cancelEpoch: deliveryCancelEpoch.Load()}
	if w.invalid() {
		t.Fatal("fresh worker invalid")
	}
	fenceDictationPhysicalAction(dictationCancel)
	if !w.invalid() {
		t.Fatal("physical cancel waits for UI queue before fencing final delivery")
	}
}

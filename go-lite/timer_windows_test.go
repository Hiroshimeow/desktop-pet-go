//go:build windows

package main

import "testing"

func TestIsAppTimerMessageUsesReturnedTimerID(t *testing.T) {
	a := &App{TimerID: 42}
	if !a.isTimerMessage(msg{Hwnd: 0, Message: WM_TIMER, WParam: 42}) {
		t.Fatal("returned SetTimer ID must drive the UI tick")
	}
	if a.isTimerMessage(msg{Hwnd: 0, Message: WM_TIMER, WParam: appTimerID}) {
		t.Fatal("hard-coded requested ID must not match a different returned ID")
	}
	if a.isTimerMessage(msg{Hwnd: 1, Message: WM_TIMER, WParam: 42}) {
		t.Fatal("window timer must not be treated as the app thread timer")
	}
}

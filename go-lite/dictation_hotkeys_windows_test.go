//go:build windows

package main

import (
	"context"
	"testing"
	"time"
)

func TestDictationBindings(t *testing.T) {
	for _, tc := range []struct {
		toggle, hold string
		valid        bool
	}{
		{"F5", "F6", true}, {"Ctrl+Shift+D", "Ctrl+F8", true},
		{"", "F7", true}, {"F5", "", true}, {"", "", false},
		{"F5", "F5", false}, {"Ctrl+Alt+D", "F6", false},
		{"Win+D", "F6", false}, {"A", "F6", false},
		{"Ctrl+Space", "F6", false}, {"F12", "F6", false},
		{"Ctrl+Ctrl+D", "F6", false}, {"F25", "F6", false},
	} {
		_, err := parseDictationBindings(tc.toggle, tc.hold)
		if (err == nil) != tc.valid {
			t.Errorf("%q/%q: err=%v valid=%v", tc.toggle, tc.hold, err, tc.valid)
		}
	}
}

func TestDictationEdgesToggleRepeatEscAndOwnedRelease(t *testing.T) {
	b, _ := parseDictationBindings("F5", "F6")
	s := dictationKeyState{bindings: b}
	check := func(vk uint32, down bool, mods uint32, want dictationAction, consume bool) {
		t.Helper()
		action, blocked := s.key(vk, down, mods, false, true)
		if action != want || blocked != consume {
			t.Fatalf("key %x/%v: (%v,%v), want (%v,%v)", vk, down, action, blocked, want, consume)
		}
	}
	check(0x1b, true, 0, dictationNone, false)
	check(0x1b, false, 0, dictationNone, false)
	check(0x74, true, 0, dictationStart, true)
	check(0x74, true, 0, dictationNone, true)
	check(0x74, false, 0, dictationNone, true)
	check(0x74, true, 0, dictationStop, true)
	check(0x74, false, 0, dictationNone, true)
	check(0x74, true, 0, dictationNone, true) // Busy decode must not restart.
	check(0x74, false, 0, dictationNone, true)
	check(0x1b, true, 0, dictationCancel, true)
	check(0x1b, true, 0, dictationNone, true)
	s.finish()
	check(0x1b, false, 0, dictationNone, true) // Do not leak an owned key-up.
	check(0x1b, true, 0, dictationNone, false)
}

func TestDictationHoldReleaseAfterModifiersChange(t *testing.T) {
	b, _ := parseDictationBindings("F5", "Ctrl+Shift+D")
	s := dictationKeyState{bindings: b}
	a, c := s.key('D', true, dictModControl|dictModShift, false, true)
	if a != dictationStart || !c {
		t.Fatal(a, c)
	}
	a, c = s.key('D', false, 0, false, true)
	if a != dictationStop || !c {
		t.Fatal("release lost after modifiers changed", a, c)
	}
	s.finish()
	a, c = s.key('D', false, 0, false, true)
	if a != dictationNone || c {
		t.Fatal("duplicate release", a, c)
	}
}

func TestDictationDoesNotOwnModifiedIMEAltGrInjectedOrNotReadyInput(t *testing.T) {
	b, _ := parseDictationBindings("F5", "Ctrl+Shift+D")
	for _, tc := range []struct {
		vk, mods             uint32
		ime, ready, injected bool
	}{
		{0x74, dictModControl, false, true, false},
		{0x74, dictModAlt, false, true, false},
		{'D', dictModControl | dictModShift | dictModAltGr, false, true, false},
		{0x74, 0, true, true, false}, {0x74, 0, false, false, false},
		{0x74, 0, false, true, true},
	} {
		s := dictationKeyState{bindings: b}
		a, c := s.event(tc.vk, true, tc.mods, tc.ime, tc.ready, tc.injected)
		if a != dictationNone || c || s.busy {
			t.Fatalf("unrelated input owned: %+v action=%v consume=%v", tc, a, c)
		}
	}
}

func TestDictationPressThatStartedInIMEDoesNotActivateOnRepeat(t *testing.T) {
	b, _ := parseDictationBindings("F5", "F6")
	s := dictationKeyState{bindings: b}
	s.key(0x74, true, 0, true, true)
	a, c := s.key(0x74, true, 0, false, true)
	if a != dictationNone || c {
		t.Fatal("repeat after composition activated dictation", a, c)
	}
	s.key(0x74, false, 0, false, true)
	a, c = s.key(0x74, true, 0, false, true)
	if a != dictationStart || !c {
		t.Fatal(a, c)
	}
}

func TestDictationHoldLostReleaseCancelsNotFinalizes(t *testing.T) {
	b, _ := parseDictationBindings("F5", "F6")
	h := &dictationHotkeys{state: dictationKeyState{bindings: b, busy: true, recording: true, holding: true}, inputs: make(chan dictationInput, 2)}
	h.state.down[0x75] = true
	h.state.owned[0x75] = true
	h.reconcileHold(false)
	in := <-h.inputs
	if in.action != dictationCancel || h.state.recording || h.state.holding {
		t.Fatal("lost keyup did not cancel", in)
	}
	if !h.state.busy {
		t.Fatal("cancel must retain owner until done")
	}
}

func TestDictationShortHoldAlwaysDiscards(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	v := &VoiceController{ready: true, commands: make(chan voiceCommand, 8), processContext: ctx, cancel: cancel, done: make(chan struct{})}
	a := &App{Voice: v}
	at := time.Unix(1, 0)
	a.handleDictationInput(dictationInput{action: dictationStart, hold: true, at: at})
	<-v.commands
	a.handleDictationInput(dictationInput{action: dictationStop, at: at.Add(100 * time.Millisecond)})
	if c := <-v.commands; c.Type != "dictation_cancel" {
		t.Fatal("short hold must discard even if speech was detected", c)
	}
	if !a.Dictation.cancelled {
		t.Fatal("short hold did not fence late final")
	}
}

func TestDictationOwnerRetiresWhenSidecarCrashes(t *testing.T) {
	done := make(chan struct{})
	close(done)
	b, _ := parseDictationBindings("F5", "F6")
	a := &App{Voice: &VoiceController{ready: true, done: done}, Dictation: dictationSession{sessionID: "lost"}, DictationKeys: &dictationHotkeys{state: dictationKeyState{bindings: b, busy: true}, inputs: make(chan dictationInput, 2)}}
	a.processDictationInputs()
	if a.Dictation.sessionID != "" || a.DictationKeys.state.busy {
		t.Fatal("crash retained dictation input ownership")
	}
}

func TestDictationCommandsAreOrderedScopedAndCancelFencesFinal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	v := &VoiceController{ready: true, commands: make(chan voiceCommand, 8), processContext: ctx, cancel: cancel, done: make(chan struct{})}
	a := &App{Voice: v}
	a.handleDictationInput(dictationInput{action: dictationStart})
	start := <-v.commands
	if start.Type != "dictation_start" || start.ProtocolVersion != 1 || start.SessionID == "" {
		t.Fatal(start)
	}
	a.handleDictationInput(dictationInput{action: dictationStop})
	stop := <-v.commands
	if stop.Type != "dictation_stop" || stop.SessionID != start.SessionID {
		t.Fatal(stop)
	}
	a.handleDictationInput(dictationInput{action: dictationStart})
	if len(v.commands) != 0 {
		t.Fatal("restart during decode")
	}
	a.handleDictationInput(dictationInput{action: dictationCancel})
	c := <-v.commands
	if c.Type != "dictation_cancel" || c.SessionID != start.SessionID {
		t.Fatal(c)
	}
	a.handleDictationEvent(voiceEvent{Type: "final", SessionID: start.SessionID, ProtocolVersion: 1, Seq: 1, Text: "must not publish"})
	if a.Dictation.result != "" {
		t.Fatal("cancelled final published")
	}
	a.handleDictationEvent(voiceEvent{Type: "done", SessionID: "stale", ProtocolVersion: 1, Seq: 2})
	if a.Dictation.sessionID == "" {
		t.Fatal("stale done retired owner")
	}
	a.handleDictationEvent(voiceEvent{Type: "done", SessionID: start.SessionID, ProtocolVersion: 1, Seq: 2})
	if a.Dictation.sessionID != "" {
		t.Fatal("done did not retire owner")
	}
}

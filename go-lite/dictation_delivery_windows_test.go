//go:build windows

package main

import "testing"

func TestDictationDeliveryAcceptsOnlyOneStoppedSuccessfulFinal(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		events               []voiceEvent
		cancelled, recording bool
		want                 string
	}{
		{"partial", []voiceEvent{{Type: "partial", Seq: 1, Stable: "preview"}, {Type: "done", Seq: 2, Outcome: "final"}}, false, false, ""},
		{"cancel", []voiceEvent{{Type: "final", Seq: 1, Text: "discard"}, {Type: "done", Seq: 2, Outcome: "final"}}, true, false, ""},
		{"recording", []voiceEvent{{Type: "final", Seq: 1, Text: "early"}, {Type: "done", Seq: 2, Outcome: "final"}}, false, true, ""},
		{"error", []voiceEvent{{Type: "final", Seq: 1, Text: "discard"}, {Type: "error", Seq: 2}, {Type: "done", Seq: 3, Outcome: "error"}}, false, false, ""},
		{"duplicate", []voiceEvent{{Type: "final", Seq: 1, Text: "first"}, {Type: "final", Seq: 2, Text: "second"}, {Type: "done", Seq: 3, Outcome: "final"}}, false, false, ""},
		{"valid", []voiceEvent{{Type: "partial", Seq: 1, Stable: "preview"}, {Type: "final", Seq: 2, Text: "Tiếng Việt API v2"}, {Type: "done", Seq: 3, Outcome: "final"}, {Type: "done", Seq: 4, Outcome: "final"}}, false, false, "Tiếng Việt API v2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{Dictation: dictationSession{sessionID: "owned", cancelled: tc.cancelled, recording: tc.recording}}
			for _, ev := range tc.events {
				ev.SessionID = "owned"
				ev.ProtocolVersion = 1
				a.handleDictationEvent(ev)
			}
			if a.DictationDelivery.text != tc.want {
				t.Fatalf("retained final %q, want %q", a.DictationDelivery.text, tc.want)
			}
		})
	}
}

func TestDictationDeliveryRejectsLateStaleAndUnstoppedEvents(t *testing.T) {
	a := &App{Dictation: dictationSession{sessionID: "new", recording: false}}
	a.handleDictationEvent(voiceEvent{Type: "final", SessionID: "old", ProtocolVersion: 1, Seq: 100, Text: "wrong"})
	a.handleDictationEvent(voiceEvent{Type: "done", SessionID: "new", ProtocolVersion: 1, Seq: 1, Outcome: "final"})
	a.handleDictationEvent(voiceEvent{Type: "final", SessionID: "new", ProtocolVersion: 1, Seq: 2, Text: "late"})
	if a.DictationDelivery.text != "" {
		t.Fatal("stale/late final reached delivery")
	}
}

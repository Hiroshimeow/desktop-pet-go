//go:build windows

package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestVoiceMetricUsesFirstAudioField(t *testing.T) {
	var event voiceEvent
	if err := json.Unmarshal([]byte(`{"type":"turn_metrics","eos_ns":1000000000,"tts_done_ns":1500000000,"first_audio_ns":1750000000}`), &event); err != nil {
		t.Fatalf("unmarshal voice metric: %v", err)
	}
	if event.FirstAudioNS == 0 {
		t.Fatal("first_audio_ns must reach Go protocol event")
	}
	if got, want := voiceLatencyMS(event), 750.0; math.Abs(got-want) > 0.001 {
		t.Fatalf("voiceLatencyMS() = %.3f, want %.3f", got, want)
	}
}

//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"unicode/utf8"
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

type testWriteCloser struct {
	bytes.Buffer
}

func (w *testWriteCloser) Close() error { return nil }

func decodeVoiceCommands(t *testing.T, data []byte) []voiceCommand {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
	commands := make([]voiceCommand, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var command voiceCommand
		if err := json.Unmarshal(line, &command); err != nil {
			t.Fatalf("decode command: %v", err)
		}
		commands = append(commands, command)
	}
	return commands
}

// VOICE-REQ-033: skip cancels only the active chunk and a duplicate completion cannot advance twice.
func TestVoiceSkipCancelsCurrentAndAdvancesOnce(t *testing.T) {
	writer := &testWriteCloser{}
	controller := &VoiceController{stdin: writer, ready: true}
	controller.enqueueRequests([]voiceSpeakRequest{
		{Text: "one", Lang: "en"},
		{Text: "two", Lang: "en"},
		{Text: "three", Lang: "en"},
	})
	first := controller.activeTurn
	if first == "" {
		t.Fatal("first request was not dispatched")
	}
	controller.skipCurrent()
	controller.completeSpeak(first)
	second := controller.activeTurn
	if second == "" || second == first {
		t.Fatalf("skip did not advance exactly once: first=%q second=%q", first, second)
	}
	controller.completeSpeak(first)
	if controller.activeTurn != second {
		t.Fatalf("duplicate completion advanced again: got %q want %q", controller.activeTurn, second)
	}

	commands := decodeVoiceCommands(t, writer.Bytes())
	if len(commands) != 3 || commands[0].Type != "speak" || commands[1].Type != "skip" || commands[2].Type != "speak" {
		t.Fatalf("unexpected command sequence: %#v", commands)
	}
}

// VOICE-REQ-033: stop clears reader work and repeated stop remains harmless.
func TestVoiceStopClearsQueuedWorkAndIsIdempotent(t *testing.T) {
	writer := &testWriteCloser{}
	controller := &VoiceController{stdin: writer, ready: true}
	controller.enqueueRequests([]voiceSpeakRequest{
		{Text: "one", Lang: "en"},
		{Text: "two", Lang: "en"},
		{Text: "three", Lang: "en"},
	})
	first := controller.activeTurn
	controller.stopPlayback()
	controller.stopPlayback()
	controller.completeSpeak(first)

	if controller.activeTurn != "" || len(controller.queue) != 0 {
		t.Fatalf("stop left reader work: active=%q queued=%d", controller.activeTurn, len(controller.queue))
	}
	commands := decodeVoiceCommands(t, writer.Bytes())
	speakCount := 0
	for _, command := range commands {
		if command.Type == "speak" {
			speakCount++
		}
	}
	if speakCount != 1 {
		t.Fatalf("stop dispatched more reader chunks: %#v", commands)
	}
}

// VOICE-REQ-040/044: clipboard text uses the same deterministic VI/EN chunk path as file reading.
func TestClipboardTextUsesReaderChunkAndLanguagePath(t *testing.T) {
	requests, err := buildVoiceRequestsFromText(strings.Repeat("a", 360)+"\n\nXin chào", "auto")
	if err != nil {
		t.Fatalf("build clipboard requests: %v", err)
	}
	if len(requests) != 3 {
		t.Fatalf("request count = %d, want 3", len(requests))
	}
	for i, request := range requests {
		if utf8.RuneCountInString(request.Text) > 350 {
			t.Fatalf("request %d exceeds chunk cap: %d", i, utf8.RuneCountInString(request.Text))
		}
	}
	if requests[0].Lang != "en" || requests[1].Lang != "en" || requests[2].Lang != "vi" {
		t.Fatalf("unexpected language routing: %#v", requests)
	}
}

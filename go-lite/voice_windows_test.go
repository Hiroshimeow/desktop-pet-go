//go:build windows

package main

import (
	"bytes"
	"context"
	"desktop-pet-lite-go/internal/agent"
	"desktop-pet-lite-go/internal/voice"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalVoiceUsesZeroClawWithoutLocalChatLifecycle(t *testing.T) {
	controller := newVoiceController(make(chan voiceEvent, 1), true, false, nil)
	if !controller.needsZeroClaw() {
		t.Fatal("normal conversational voice must enable ZeroClaw")
	}
	if controller.agentChat == nil || controller.agentWatch == nil {
		t.Fatal("normal conversational voice must construct ZeroClaw chat and reminder watcher functions")
	}
}

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

func TestCommandModeListenOnceIsIdempotentUntilIdle(t *testing.T) {
	writer := &testWriteCloser{}
	controller := &VoiceController{stdin: writer, ready: true, commandMode: true}

	if !controller.armListenOnce() {
		t.Fatal("first command-mode arm was rejected")
	}
	if controller.armListenOnce() {
		t.Fatal("second command-mode arm overlapped the active turn")
	}
	commands := decodeVoiceCommands(t, writer.Bytes())
	if len(commands) != 1 || commands[0].Type != "listen_once" {
		t.Fatalf("arm commands = %#v, want exactly one listen_once", commands)
	}

	controller.handleCommandModeState("idle")
	if !controller.armListenOnce() {
		t.Fatal("command mode did not re-arm after idle")
	}
	commands = decodeVoiceCommands(t, writer.Bytes())
	if len(commands) != 2 || commands[1].Type != "listen_once" {
		t.Fatalf("re-arm commands = %#v, want second listen_once", commands)
	}
}

func TestCommandModeDoesNotEnableConversationRuntime(t *testing.T) {
	controller := newVoiceController(make(chan voiceEvent, 1), false, true, nil)
	if !controller.commandMode {
		t.Fatal("command mode was not retained")
	}
	if controller.listen {
		t.Fatal("command mode must not enable continuous conversational listening")
	}
	if controller.agentChat != nil || controller.agentWatch != nil || controller.needsZeroClaw() {
		t.Fatal("command mode must not construct or call ZeroClaw")
	}
}

func TestDeterministicVoiceCommandBypassesZeroClaw(t *testing.T) {
	writer := &testWriteCloser{}
	turns := 0
	controller := &VoiceController{
		session: voice.NewSession(5),
		listen:  true,
		stdin:   writer,
		agentChat: func(context.Context, string, func(agent.Event)) (string, error) {
			turns++
			return "should not be called", nil
		},
	}
	app := &App{Voice: controller}
	app.handleVoiceEvent(voiceEvent{Type: "utterance", TurnID: "turn-command", Text: "pet ơi, pause"})
	if turns != 0 {
		t.Fatalf("deterministic command created %d ZeroClaw turns", turns)
	}
	commands := decodeVoiceCommands(t, writer.Bytes())
	if len(commands) != 1 || commands[0].Type != "resume" {
		t.Fatalf("local command protocol = %#v, want one resume", commands)
	}
}

func TestZeroClawUnavailableFailsSoftAndResumes(t *testing.T) {
	writer := &testWriteCloser{}
	events := make(chan voiceEvent, 1)
	controller := &VoiceController{
		events: events,
		stdin:  writer,
		agentChat: func(context.Context, string, func(agent.Event)) (string, error) {
			return "", errors.New("gateway unavailable")
		},
	}
	controller.requestAgent("turn-agent", "xin chào")
	event := <-events
	if event.Type != "agent_error" || !strings.Contains(event.Detail, "gateway unavailable") {
		t.Fatalf("agent failure event = %#v", event)
	}
	app := &App{Voice: controller}
	app.handleVoiceEvent(event)
	commands := decodeVoiceCommands(t, writer.Bytes())
	if len(commands) != 1 || commands[0].Type != "resume" || commands[0].TurnID != "turn-agent" {
		t.Fatalf("fail-soft protocol = %#v, want resume for turn-agent", commands)
	}
}

func TestZeroClawLifecycleDrivesThinkingAndFinalLocalSpeech(t *testing.T) {
	writer := &testWriteCloser{}
	events := make(chan voiceEvent, 3)
	controller := &VoiceController{
		events: events,
		stdin:  writer,
		ready:  true,
		agentChat: func(_ context.Context, text string, onEvent func(agent.Event)) (string, error) {
			if text != "計画して" {
				t.Fatalf("agent text = %q", text)
			}
			onEvent(agent.Event{Kind: agent.EventThinking})
			onEvent(agent.Event{Kind: agent.EventWorking})
			return "計画です", nil
		},
	}
	controller.requestAgent("turn-plan", "計画して")

	thinking := <-events
	working := <-events
	reply := <-events
	if thinking.Type != "agent_state" || thinking.State != "thinking" {
		t.Fatalf("thinking event = %#v", thinking)
	}
	if working.Type != "agent_state" || working.State != "working" {
		t.Fatalf("working event = %#v", working)
	}
	if reply.Type != "agent_reply" || reply.Text != "計画です" {
		t.Fatalf("reply event = %#v", reply)
	}

	manifest := testManifest(t)
	store, err := LoadSpriteStore(manifest)
	if err != nil {
		t.Fatal(err)
	}
	p := NewPet("phase11-agent", "Phase 11 Agent", store.Manifest, store, 1000, 800, 128, 128)
	app := &App{Voice: controller, Pets: []*WindowPet{{Pet: p}}}
	app.handleVoiceEvent(thinking)
	app.handleVoiceEvent(working)
	if p.Animation == "" || store.Manifest.Animations[p.Animation].Locomotion {
		t.Fatalf("working lifecycle did not resolve a stationary semantic reaction: %q", p.Animation)
	}
	app.handleVoiceEvent(reply)
	commands := decodeVoiceCommands(t, writer.Bytes())
	if len(commands) != 1 || commands[0].Type != "speak" || commands[0].TurnID != "turn-plan" || commands[0].Text != "計画です" || commands[0].Lang != "ja" {
		t.Fatalf("final local speech command = %#v", commands)
	}
}

func TestZeroClawReminderUsesExistingSpeechQueueWithoutAgentTurn(t *testing.T) {
	writer := &testWriteCloser{}
	turns := 0
	controller := &VoiceController{
		stdin: writer,
		ready: true,
		agentChat: func(context.Context, string, func(agent.Event)) (string, error) {
			turns++
			return "unexpected", nil
		},
	}
	controller.enqueueRequests([]voiceSpeakRequest{{Text: "đang nói", Lang: "vi"}})
	firstTurn := controller.activeTurn
	app := &App{Voice: controller}
	app.handleVoiceEvent(voiceEvent{Type: "agent_reminder", Text: "Nhắc uống nước"})

	commands := decodeVoiceCommands(t, writer.Bytes())
	if len(commands) != 1 || commands[0].Type != "speak" || commands[0].Text != "đang nói" {
		t.Fatalf("reminder bypassed active local speech: %#v", commands)
	}
	if len(controller.queue) != 1 || controller.queue[0].Text != "Nhắc uống nước" || controller.queue[0].Lang != "vi" {
		t.Fatalf("queued reminder = %#v", controller.queue)
	}
	controller.completeSpeak(firstTurn)
	commands = decodeVoiceCommands(t, writer.Bytes())
	if len(commands) != 2 || commands[1].Type != "speak" || commands[1].Text != "Nhắc uống nước" || commands[1].Lang != "vi" {
		t.Fatalf("reminder local speech sequence = %#v", commands)
	}
	if turns != 0 {
		t.Fatalf("reminder created %d ZeroClaw chat turns", turns)
	}
}

func TestCommandLaunchTargetsAreFixedByEnum(t *testing.T) {
	tests := []struct {
		command voice.VoiceCommand
		want    string
	}{
		{voice.CommandOpenChrome, "chrome.exe"},
		{voice.CommandOpenFacebook, "https://www.facebook.com/"},
		{voice.CommandOpenYouTube, "https://www.youtube.com/"},
		{voice.CommandOpenCalendar, "https://calendar.google.com/"},
		{voice.CommandOpenNotepad, "notepad.exe"},
	}
	for _, tt := range tests {
		got, ok := commandLaunchTarget(tt.command)
		if !ok || got != tt.want {
			t.Fatalf("commandLaunchTarget(%q) = (%q, %v), want (%q, true)", tt.command, got, ok, tt.want)
		}
	}
	if got, ok := commandLaunchTarget(voice.CommandStatus); ok || got != "" {
		t.Fatalf("non-launch command target = (%q, %v), want empty/false", got, ok)
	}
}

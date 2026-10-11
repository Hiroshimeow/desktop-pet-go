//go:build windows

package main

import (
	"context"
	"desktop-pet-lite-go/internal/agent"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestPetChatSubmissionUsesExistingAgentWithoutAudioOrShell(t *testing.T) {
	events := make(chan voiceEvent, 8)
	called := make(chan string, 1)
	a := &App{VoiceEvents: events}
	a.Chat.request = func(ctx context.Context, text string, emit func(agent.Event)) (string, error) {
		called <- text
		return "Xin chào API", nil
	}
	a.submitPetChat("  Hello API  ", false)
	select {
	case text := <-called:
		if text != "Hello API" {
			t.Fatal(text)
		}
	case <-time.After(time.Second):
		t.Fatal("agent not called")
	}
	select {
	case ev := <-events:
		a.handleVoiceEvent(ev)
	case <-time.After(time.Second):
		t.Fatal("missing reply")
	}
	if a.Chat.reply != "Xin chào API" || a.Chat.busy || a.Voice != nil {
		t.Fatal("text reply incorrectly routed", a.Chat.reply)
	}
}

func TestPetClickReleaseFallbackDispatchesCommandExactlyOnce(t *testing.T) {
	writer := &testWriteCloser{}
	v := &VoiceController{stdin: writer, commandMode: true, ready: true}
	p := &WindowPet{Pet: &Pet{}, PendingLeft: true}
	a := &App{Voice: v}
	a.finishPetClick(p, "button_state_left_up")
	a.finishPetClick(p, "late_left_up")
	commands := decodeVoiceCommands(t, writer.Bytes())
	if len(commands) != 1 || commands[0].Type != "listen_once" || p.PendingLeft {
		t.Fatal("fallback lost or duplicated click", commands)
	}
}

func TestPetNativeChatControlsAndPreviewDoNotRetarget(t *testing.T) {
	if os.Getenv("DP_DICTATION_NATIVE_TEST") != "1" {
		t.Skip("explicit disposable Windows UI test required")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	prior := app
	defer func() { app = prior }()
	old, _, _ := dictForeground.Call()
	defer uiForeground.Call(old)
	a := &App{}
	app = a
	a.showPetChat()
	if a.Chat.hwnd == 0 || a.Chat.input == 0 || a.Chat.output == 0 || a.Chat.mic == 0 {
		t.Fatal("native chat controls missing")
	}
	defer a.closePetUI()
	uiText(a.Chat.input, "Xin chào API")
	text, err := nativeEditDocument(a.Chat.input)
	if err != nil || syscall.UTF16ToString(text) != "Xin chào API" {
		t.Fatal("Unicode chat input lost", err)
	}
	p := &WindowPet{Pet: &Pet{}, PendingLeft: true}
	a.Pets = []*WindowPet{p}
	a.Preview.status = "Recording"
	a.Preview.stable = "Tiếng Việt"
	a.Preview.unstable = " API"
	a.Preview.point = point{100, 100}
	before, _, _ := dictForeground.Call()
	a.updatePetUI()
	hwnd := a.Preview.hwnd
	if hwnd == 0 {
		t.Fatal("preview window missing")
	}
	style, _, _ := deliveryStyle.Call(hwnd, ^uintptr(19)) // GWL_EXSTYLE
	if style&0x080800a8 != 0x080800a8 {
		t.Fatalf("unsafe preview style: %x", style)
	}
	after, _, _ := dictForeground.Call()
	if before != after {
		t.Fatal("preview stole focus")
	}
	a.Preview.unstable = " corrected"
	a.updatePetUI()
	if a.Preview.hwnd != hwnd {
		t.Fatal("preview window recreated")
	}
	var buf [1024]uint16
	user32.NewProc("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if got := syscall.UTF16ToString(buf[:]); got != "Recording\r\nStable: Tiếng Việt\r\nPreview:  corrected" {
		t.Fatal("preview did not render Unicode state", got)
	}
}

func TestPetChatCancelFencesLateReply(t *testing.T) {
	a := &App{VoiceEvents: make(chan voiceEvent, 8)}
	a.Chat.busy = true
	a.Chat.turn = "pet-chat-1"
	a.cancelPetChat()
	a.handleVoiceEvent(voiceEvent{Type: "pet_chat_reply", TurnID: "pet-chat-1", Text: "late"})
	if a.Chat.reply != "" {
		t.Fatal("cancelled reply resurfaced")
	}
}

func TestPetMicFinalGoesToConversationNotInsertion(t *testing.T) {
	events := make(chan voiceEvent, 8)
	called := make(chan string, 1)
	a := &App{VoiceEvents: events, Dictation: dictationSession{sessionID: "mic", conversation: true}}
	a.Chat.request = func(ctx context.Context, text string, emit func(agent.Event)) (string, error) {
		called <- text
		return "reply", nil
	}
	a.handleDictationEvent(voiceEvent{Type: "partial", SessionID: "mic", ProtocolVersion: 1, Seq: 1, Stable: "preview"})
	if len(called) != 0 {
		t.Fatal("partial sent to agent")
	}
	a.handleDictationEvent(voiceEvent{Type: "final", SessionID: "mic", ProtocolVersion: 1, Seq: 2, Text: "Xin chào"})
	a.handleDictationEvent(voiceEvent{Type: "done", SessionID: "mic", ProtocolVersion: 1, Seq: 3, Outcome: "final"})
	select {
	case text := <-called:
		if text != "Xin chào" {
			t.Fatal(text)
		}
	case <-time.After(time.Second):
		t.Fatal("mic final did not reach agent")
	}
	if a.DictationDelivery.text != "" || a.DictationDelivery.worker != nil {
		t.Fatal("mic final reached insertion")
	}
}

func TestPetPreviewIsVisualOnlyAndCancelRejectsLatePartial(t *testing.T) {
	a := &App{Dictation: dictationSession{sessionID: "owned", recording: true}}
	a.handleDictationEvent(voiceEvent{Type: "partial", SessionID: "owned", ProtocolVersion: 1, Seq: 1, Stable: "Tiếng Việt", Unstable: " API"})
	if a.Preview.stable != "Tiếng Việt" || a.Preview.unstable != " API" || a.DictationDelivery.text != "" {
		t.Fatal("preview missing or inserted")
	}
	a.Dictation.cancelled = true
	a.handleDictationEvent(voiceEvent{Type: "partial", SessionID: "owned", ProtocolVersion: 1, Seq: 2, Stable: "late"})
	if a.Preview.stable == "late" {
		t.Fatal("cancelled preview resurfaced")
	}
}

func TestPetMicUnavailableIsVisibleAndDoesNotStartCapture(t *testing.T) {
	a := &App{Voice: &VoiceController{ready: false}}
	a.togglePetMic()
	if a.Preview.status == "" || a.Dictation.sessionID != "" {
		t.Fatal("mic unavailable is silent or activated")
	}
}

func TestPetPreviewEndsCaptureOnSidecarDurationLimit(t *testing.T) {
	a := &App{Dictation: dictationSession{sessionID: "owned", recording: true}}
	a.handleDictationEvent(voiceEvent{Type: "record_stopped", SessionID: "owned", ProtocolVersion: 1, Seq: 1})
	a.handleDictationEvent(voiceEvent{Type: "final", SessionID: "owned", ProtocolVersion: 1, Seq: 2, Text: "final"})
	a.handleDictationEvent(voiceEvent{Type: "done", SessionID: "owned", ProtocolVersion: 1, Seq: 3, Outcome: "final"})
	if a.DictationDelivery.text != "final" {
		t.Fatal("duration limit final rejected")
	}
}

func TestPetMicCrashWithoutHotkeysRetiresSession(t *testing.T) {
	done := make(chan struct{})
	close(done)
	a := &App{Voice: &VoiceController{done: done}, Dictation: dictationSession{sessionID: "mic", conversation: true, recording: true}}
	a.processDictationInputs()
	if a.Dictation.sessionID != "" {
		t.Fatal("mic crash retained session without hotkeys")
	}
}

func TestPetPreviewPlacementClampsNegativeAndSmallMonitors(t *testing.T) {
	r := uiRect{-1600, 0, 0, 900}
	x, y, w, h := previewPlacement(-5, 890, 560, 160, r)
	if x+w > 0 || y+h > 900 || x < -1600 {
		t.Fatalf("offscreen: %d %d %d %d", x, y, w, h)
	}
	x, y, w, h = previewPlacement(10, 10, 560, 160, uiRect{0, 0, 320, 100})
	if x != 0 || y != 0 || w != 320 || h != 100 {
		t.Fatal("small monitor not clamped")
	}
}

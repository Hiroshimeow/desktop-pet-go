//go:build windows

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	petbrain "desktop-pet-lite-go/internal/pet"
	"desktop-pet-lite-go/internal/voice"
)

type voiceEvent struct {
	Type         string `json:"type"`
	State        string `json:"state"`
	TurnID       string `json:"turn_id"`
	Text         string `json:"text"`
	Detail       string `json:"detail"`
	EOSNS        int64  `json:"eos_ns"`
	STTDoneNS    int64  `json:"stt_done_ns"`
	TTSDoneNS    int64  `json:"tts_done_ns"`
	FirstAudioNS int64  `json:"first_audio_ns"`
	UserText     string `json:"-"`
	FallbackText string `json:"-"`
}

type voiceCommand struct {
	Type   string `json:"type"`
	TurnID string `json:"turn_id,omitempty"`
	Text   string `json:"text,omitempty"`
	Lang   string `json:"lang,omitempty"`
}

type voiceSpeakRequest struct {
	Text string
	Lang string
}

type VoiceController struct {
	events      chan<- voiceEvent
	session     *voice.Session
	chat        *voice.ChatClient
	chatHistory []voice.ChatMessage
	listen      bool
	queue       []voiceSpeakRequest
	activeTurn  string
	nextTurn    int
	ready       bool
	paused      bool
	cancel      context.CancelFunc
	stdin       io.WriteCloser
	done        chan struct{}
	mu          sync.Mutex
}

const voiceChatPersona = "You are a small desktop pet. Reply naturally in the user's language, Vietnamese or English. Keep replies short, conversational, easy to speak aloud, and plain text only."

func (a *App) startVoiceAsync(listen bool, sayText, readFile, readLang string) {
	requests, err := buildStartupVoiceRequests(sayText, readFile, readLang)
	if err != nil {
		log.Printf("voice request ignored: %v", err)
	}
	if !listen && len(requests) == 0 {
		return
	}
	a.startVoiceController(listen, requests)
}

func (a *App) startVoiceController(listen bool, requests []voiceSpeakRequest) {
	if a.Voice != nil && !a.Voice.isStopped() {
		a.Voice.enqueueRequests(requests)
		return
	}
	controller := &VoiceController{
		events:  a.VoiceEvents,
		session: voice.NewSession(5 * time.Second),
		chat:    voice.NewLocalChatClient(),
		listen:  listen,
		queue:   append([]voiceSpeakRequest(nil), requests...),
		done:    make(chan struct{}),
	}
	a.Voice = controller
	go func() {
		if err := controller.run(); err != nil {
			log.Printf("voice disabled after sidecar failure: %v; run .\\scripts\\setup-voice.ps1", err)
			select {
			case a.VoiceEvents <- voiceEvent{Type: "state", State: "error", Detail: err.Error()}:
			default:
			}
		}
	}()
}

func (a *App) stopVoice() {
	if a == nil || a.Voice == nil {
		return
	}
	a.Voice.stop()
}

func (v *VoiceController) run() error {
	defer close(v.done)
	sidecarDir, err := locateVoiceSidecar()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.cancel = cancel
	script := filepath.Join(sidecarDir, "voice_sidecar.py")
	args := []string{"run", "--project", sidecarDir, "--frozen", "python", script}
	if !v.listen {
		args = append(args, "--no-listen")
	}
	cmd := exec.CommandContext(ctx, "uv", args...)
	cmd.Dir = sidecarDir
	cmd.Env = append(os.Environ(), "VIRTUAL_ENV=")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("voice stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("voice stderr pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("voice stdin pipe: %w", err)
	}
	v.stdin = stdin
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start voice sidecar with uv: %w", err)
	}
	log.Printf("voice sidecar started pid=%d dir=%s", cmd.Process.Pid, sidecarDir)
	go copyVoiceDiagnostics(stderr)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var event voiceEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			log.Printf("voice protocol ignored malformed event: %v", err)
			continue
		}
		log.Printf("voice protocol event type=%s state=%s turn=%s", event.Type, event.State, event.TurnID)
		select {
		case v.events <- event:
		default:
			log.Printf("voice event queue full; dropped type=%s state=%s", event.Type, event.State)
		}
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return nil
	}
	if scanErr != nil {
		return fmt.Errorf("read voice protocol: %w", scanErr)
	}
	if waitErr != nil {
		return fmt.Errorf("voice sidecar exited: %w", waitErr)
	}
	return nil
}

func copyVoiceDiagnostics(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		log.Printf("voice sidecar: %s", scanner.Text())
	}
}

func locateVoiceSidecar() (string, error) {
	cwd, _ := os.Getwd()
	exe, _ := os.Executable()
	candidates := []string{
		filepath.Join(cwd, "..", "voice-sidecar"),
		filepath.Join(cwd, "voice-sidecar"),
		filepath.Join(filepath.Dir(exe), "..", "voice-sidecar"),
		filepath.Join(filepath.Dir(exe), "voice-sidecar"),
	}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if info, err := os.Stat(filepath.Join(candidate, "voice_sidecar.py")); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("voice-sidecar not found near cwd=%s exe=%s", cwd, exe)
}

func (v *VoiceController) send(command voiceCommand) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.stdin == nil {
		return fmt.Errorf("voice sidecar is not ready")
	}
	return json.NewEncoder(v.stdin).Encode(command)
}

func (v *VoiceController) resume(turnID string) {
	if err := v.send(voiceCommand{Type: "resume", TurnID: turnID}); err != nil {
		log.Printf("voice resume failed: %v", err)
	}
}

func (v *VoiceController) speak(turnID, text, lang string) bool {
	if v.activeTurn != "" {
		log.Printf("voice speak ignored while turn=%s is active", v.activeTurn)
		return false
	}
	if err := v.send(voiceCommand{Type: "speak", TurnID: turnID, Text: text, Lang: lang}); err != nil {
		log.Printf("voice speak failed: %v", err)
		return false
	}
	v.activeTurn = turnID
	v.paused = false
	return true
}

func (v *VoiceController) enqueueRequests(requests []voiceSpeakRequest) {
	if len(requests) == 0 {
		return
	}
	v.queue = append(v.queue, requests...)
	v.dispatchNext()
}

func (v *VoiceController) dispatchNext() {
	if !v.ready || v.activeTurn != "" || len(v.queue) == 0 {
		return
	}
	request := v.queue[0]
	v.nextTurn++
	turnID := fmt.Sprintf("reader-%d", v.nextTurn)
	if !v.speak(turnID, request.Text, request.Lang) {
		return
	}
	v.queue = v.queue[1:]
}

func (v *VoiceController) completeSpeak(turnID string) {
	if turnID == "" || turnID != v.activeTurn {
		return
	}
	v.activeTurn = ""
	v.paused = false
	v.dispatchNext()
}

func (v *VoiceController) pausePlayback() {
	if v == nil || v.activeTurn == "" || v.paused {
		return
	}
	if err := v.send(voiceCommand{Type: "pause"}); err != nil {
		log.Printf("voice pause failed: %v", err)
		return
	}
	v.paused = true
}

func (v *VoiceController) resumePlayback() {
	if v == nil || v.activeTurn == "" || !v.paused {
		return
	}
	if err := v.send(voiceCommand{Type: "resume_playback"}); err != nil {
		log.Printf("voice playback resume failed: %v", err)
		return
	}
	v.paused = false
}

func (v *VoiceController) skipCurrent() {
	if v == nil || v.activeTurn == "" {
		return
	}
	v.paused = false
	if err := v.send(voiceCommand{Type: "skip"}); err != nil {
		log.Printf("voice skip failed: %v", err)
	}
}

func (v *VoiceController) stopPlayback() {
	if v == nil {
		return
	}
	v.queue = nil
	v.paused = false
	if v.activeTurn == "" {
		return
	}
	if err := v.send(voiceCommand{Type: "stop_playback"}); err != nil {
		log.Printf("voice stop failed: %v", err)
	}
}

func (v *VoiceController) isPaused() bool {
	return v != nil && v.paused
}

func (v *VoiceController) isStopped() bool {
	if v == nil {
		return true
	}
	if v.done == nil {
		return false
	}
	select {
	case <-v.done:
		return true
	default:
		return false
	}
}

func (v *VoiceController) stop() {
	_ = v.send(voiceCommand{Type: "shutdown"})
	if v.done == nil {
		return
	}
	select {
	case <-v.done:
		return
	case <-time.After(2 * time.Second):
		if v.cancel != nil {
			v.cancel()
		}
	}
}

func (a *App) processVoiceEvents() {
	for {
		select {
		case event := <-a.VoiceEvents:
			a.handleVoiceEvent(event)
		default:
			return
		}
	}
}

func (a *App) handleVoiceEvent(event voiceEvent) {
	if a.Voice == nil {
		return
	}
	switch event.Type {
	case "state":
		switch event.State {
		case "ready":
			log.Printf("voice ready")
			a.Voice.ready = true
			a.Voice.dispatchNext()
		case "listening":
			a.triggerVoiceIntent(petbrain.IntentVoiceListening)
		case "thinking":
			a.triggerVoiceIntent(petbrain.IntentVoiceThinking)
		case "speaking":
			a.triggerVoiceIntent(petbrain.IntentVoiceSpeaking)
		case "error":
			log.Printf("voice error: %s", event.Detail)
			a.triggerVoiceIntent(petbrain.IntentVoiceError)
		}
	case "utterance":
		log.Printf("voice transcript turn=%s text=%q", event.TurnID, event.Text)
		reply, ok := a.Voice.session.Handle(event.Text, time.Now())
		if !ok {
			a.Voice.resume(event.TurnID)
			return
		}
		route, command := voice.RouteTranscript(event.Text)
		if route == voice.RouteCommand {
			a.dispatchVoiceCommand(event, command)
			return
		}
		a.triggerVoiceIntent(petbrain.IntentVoiceUnknown)
		history := append([]voice.ChatMessage(nil), a.Voice.chatHistory...)
		go a.Voice.requestChat(event.TurnID, event.Text, reply.Text, history)
		return
	case "chat_reply":
		a.handleChatReply(event)
	case "chat_error":
		log.Printf("local chat failed: %s", event.Detail)
		if !a.Voice.speak(event.TurnID, event.FallbackText, "vi") {
			a.Voice.resume(event.TurnID)
		}
	case "speak_done":
		a.Voice.completeSpeak(event.TurnID)
	case "first_audio":
		log.Printf("voice first audio turn=%s first_audio_ns=%d", event.TurnID, event.FirstAudioNS)
	case "turn_metrics":
		log.Printf("voice metrics turn=%s latency_ms=%.1f eos_ns=%d stt_done_ns=%d tts_done_ns=%d first_audio_ns=%d", event.TurnID, voiceLatencyMS(event), event.EOSNS, event.STTDoneNS, event.TTSDoneNS, event.FirstAudioNS)
	}
}

func (v *VoiceController) requestChat(turnID, userText, fallbackText string, history []voice.ChatMessage) {
	reply, err := v.chat.Reply(context.Background(), voiceChatPersona, history, userText)
	event := voiceEvent{Type: "chat_reply", TurnID: turnID, Text: reply, UserText: userText, FallbackText: fallbackText}
	if err != nil {
		event.Type = "chat_error"
		event.Detail = err.Error()
	}
	v.events <- event
}

func (a *App) handleChatReply(event voiceEvent) {
	lang, err := voice.ResolveLanguage(event.Text, "auto")
	if err != nil {
		log.Printf("local chat reply language failed: %v", err)
		if !a.Voice.speak(event.TurnID, event.FallbackText, "vi") {
			a.Voice.resume(event.TurnID)
		}
		return
	}
	a.Voice.chatHistory = append(a.Voice.chatHistory,
		voice.ChatMessage{Role: "user", Content: event.UserText},
		voice.ChatMessage{Role: "assistant", Content: event.Text},
	)
	if len(a.Voice.chatHistory) > 6 {
		a.Voice.chatHistory = a.Voice.chatHistory[len(a.Voice.chatHistory)-6:]
	}
	if !a.Voice.speak(event.TurnID, event.Text, lang) {
		a.Voice.resume(event.TurnID)
	}
}

func (a *App) dispatchVoiceCommand(event voiceEvent, command voice.VoiceCommand) {
	switch command {
	case voice.CommandPause:
		a.Voice.pausePlayback()
		a.Voice.resume(event.TurnID)
	case voice.CommandResume:
		a.Voice.resumePlayback()
		a.Voice.resume(event.TurnID)
	case voice.CommandSkip:
		a.Voice.skipCurrent()
		a.Voice.resume(event.TurnID)
	case voice.CommandStop:
		a.Voice.stopPlayback()
		a.Voice.resume(event.TurnID)
	case voice.CommandReadClipboard:
		a.Voice.resume(event.TurnID)
		a.readClipboardVoice(0)
	case voice.CommandStatus:
		text, lang := "Mình đang sẵn sàng.", "vi"
		normalized := voice.Normalize(event.Text)
		if normalized == "status" || strings.HasSuffix(normalized, " status") {
			text, lang = "I'm ready.", "en"
		}
		if !a.Voice.speak(event.TurnID, text, lang) {
			a.Voice.resume(event.TurnID)
		}
	}
}

func buildStartupVoiceRequests(sayText, readFile, readLang string) ([]voiceSpeakRequest, error) {
	if sayText == "" && readFile == "" {
		return nil, nil
	}
	requests := make([]voiceSpeakRequest, 0, 8)
	if sayText != "" {
		lang, err := voice.ResolveLanguage(sayText, readLang)
		if err != nil {
			return nil, err
		}
		requests = append(requests, voiceSpeakRequest{Text: sayText, Lang: lang})
	}
	if readFile != "" {
		text, err := voice.ReadTextFile(readFile)
		if err != nil {
			return requests, err
		}
		readerRequests, err := buildVoiceRequestsFromText(text, readLang)
		if err != nil {
			return requests, fmt.Errorf("read %q: %w", readFile, err)
		}
		requests = append(requests, readerRequests...)
	}
	return requests, nil
}

func buildVoiceRequestsFromText(text, readLang string) ([]voiceSpeakRequest, error) {
	chunks := voice.ChunkText(text, 350)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("text contains no readable text")
	}
	requests := make([]voiceSpeakRequest, 0, len(chunks))
	for _, chunk := range chunks {
		lang, err := voice.ResolveLanguage(chunk, readLang)
		if err != nil {
			return nil, err
		}
		requests = append(requests, voiceSpeakRequest{Text: chunk, Lang: lang})
	}
	return requests, nil
}

func voiceLatencyMS(event voiceEvent) float64 {
	if event.EOSNS <= 0 || event.FirstAudioNS <= event.EOSNS {
		return 0
	}
	return float64(event.FirstAudioNS-event.EOSNS) / float64(time.Millisecond)
}

func (a *App) triggerVoiceIntent(intent petbrain.Intent) {
	for _, windowPet := range a.Pets {
		if windowPet != nil && windowPet.Pet != nil && !windowPet.Drag {
			windowPet.Pet.TriggerIntent(intent)
		}
	}
}

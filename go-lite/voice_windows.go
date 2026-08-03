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
	"unicode"
	"unsafe"

	"desktop-pet-lite-go/internal/agent"
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
	events         chan<- voiceEvent
	session        *voice.Session
	agentChat      func(context.Context, string, func(agent.Event)) (string, error)
	agentWatch     func(context.Context, func(agent.Event)) error
	listen         bool
	commandMode    bool
	listenOnceBusy bool
	queue          []voiceSpeakRequest
	activeTurn     string
	nextTurn       int
	ready          bool
	paused         bool
	cancel         context.CancelFunc
	agentCancel    context.CancelFunc
	agentWatchStop context.CancelFunc
	stdin          io.WriteCloser
	done           chan struct{}
	mu             sync.Mutex
	agentMu        sync.Mutex
}

func (a *App) startVoiceAsync(listen, commandMode bool, sayText, readFile, readLang string) {
	requests, err := buildStartupVoiceRequests(sayText, readFile, readLang)
	if err != nil {
		log.Printf("voice request ignored: %v", err)
	}
	if !listen && !commandMode && len(requests) == 0 {
		return
	}
	a.startVoiceControllerMode(listen, commandMode, requests)
}

func newVoiceController(events chan<- voiceEvent, listen, commandMode bool, requests []voiceSpeakRequest) *VoiceController {
	controller := &VoiceController{
		events:      events,
		session:     voice.NewSession(5 * time.Second),
		listen:      listen,
		commandMode: commandMode,
		queue:       append([]voiceSpeakRequest(nil), requests...),
		done:        make(chan struct{}),
	}
	if controller.needsZeroClaw() {
		if sidecarDir, err := locateVoiceSidecar(); err == nil {
			client := agent.NewZeroClaw(filepath.Dir(sidecarDir))
			controller.agentChat = client.ChatEvents
			controller.agentWatch = client.Watch
		}
	}
	return controller
}

func (a *App) startVoiceController(listen bool, requests []voiceSpeakRequest) {
	a.startVoiceControllerMode(listen, false, requests)
}

func (a *App) startVoiceControllerMode(listen, commandMode bool, requests []voiceSpeakRequest) {
	if a.Voice != nil && !a.Voice.isStopped() {
		a.Voice.enqueueRequests(requests)
		return
	}
	controller := newVoiceController(a.VoiceEvents, listen, commandMode, requests)
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

func (v *VoiceController) needsZeroClaw() bool {
	return v.listen && !v.commandMode
}

func (v *VoiceController) run() error {
	defer close(v.done)
	sidecarDir, err := locateVoiceSidecar()
	if err != nil {
		return err
	}
	if v.needsZeroClaw() && (v.agentChat == nil || v.agentWatch == nil) {
		client := agent.NewZeroClaw(filepath.Dir(sidecarDir))
		v.agentChat = client.ChatEvents
		v.agentWatch = client.Watch
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.cancel = cancel
	defer cancel()
	script := filepath.Join(sidecarDir, "voice_sidecar.py")
	args := []string{"run", "--project", sidecarDir, "--no-sync", "python", script}
	if v.commandMode {
		args = append(args, "--listen-on-command")
	} else if !v.listen {
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
	go copyProcessDiagnostics("voice sidecar", stderr)
	if v.needsZeroClaw() && v.agentWatch != nil {
		watchCtx, watchCancel := context.WithCancel(ctx)
		v.agentMu.Lock()
		v.agentWatchStop = watchCancel
		v.agentMu.Unlock()
		go v.watchAgent(watchCtx)
	}

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

func copyProcessDiagnostics(name string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		log.Printf("%s: %s", name, scanner.Text())
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

func (v *VoiceController) armListenOnce() bool {
	if v == nil || !v.commandMode || !v.ready || v.listenOnceBusy || v.activeTurn != "" {
		return false
	}
	v.listenOnceBusy = true
	if err := v.send(voiceCommand{Type: "listen_once"}); err != nil {
		v.listenOnceBusy = false
		log.Printf("voice listen-once failed: %v", err)
		return false
	}
	return true
}

func (v *VoiceController) handleCommandModeState(state string) {
	if v != nil && v.commandMode && state == "idle" {
		v.listenOnceBusy = false
	}
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
	v.cancelAgent()
	v.cancelAgentWatch()
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
		a.Voice.handleCommandModeState(event.State)
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
		if a.Voice.commandMode {
			command, ok := voice.ParseCommand(event.Text)
			if !ok {
				a.triggerVoiceIntent(petbrain.IntentVoiceUnknown)
				return
			}
			a.dispatchVoiceCommand(event, command)
			return
		}
		_, ok := a.Voice.session.Handle(event.Text, time.Now())
		if !ok {
			a.Voice.resume(event.TurnID)
			return
		}
		route, command := voice.RouteTranscript(event.Text)
		if route == voice.RouteCommand {
			a.dispatchVoiceCommand(event, command)
			return
		}
		a.triggerVoiceIntent(petbrain.IntentVoiceThinking)
		go a.Voice.requestAgent(event.TurnID, event.Text)
		return
	case "agent_state":
		if event.State == "thinking" || event.State == "working" {
			a.triggerVoiceIntent(petbrain.IntentVoiceThinking)
		}
	case "agent_reply":
		a.handleAgentReply(event)
	case "agent_reminder":
		lang, err := agentReplyLanguage(event.Text)
		if err != nil {
			log.Printf("ZeroClaw reminder language failed: %v", err)
			a.triggerVoiceIntent(petbrain.IntentVoiceError)
			return
		}
		a.Voice.enqueueRequests([]voiceSpeakRequest{{Text: event.Text, Lang: lang}})
	case "agent_watch_error":
		log.Printf("ZeroClaw reminder watcher failed: %s", event.Detail)
		a.triggerVoiceIntent(petbrain.IntentVoiceError)
	case "agent_error":
		log.Printf("ZeroClaw failed: %s", event.Detail)
		a.triggerVoiceIntent(petbrain.IntentVoiceError)
		a.Voice.resume(event.TurnID)
	case "speak_done":
		a.Voice.completeSpeak(event.TurnID)
	case "first_audio":
		log.Printf("voice first audio turn=%s first_audio_ns=%d", event.TurnID, event.FirstAudioNS)
	case "turn_metrics":
		log.Printf("voice metrics turn=%s latency_ms=%.1f eos_ns=%d stt_done_ns=%d tts_done_ns=%d first_audio_ns=%d", event.TurnID, voiceLatencyMS(event), event.EOSNS, event.STTDoneNS, event.TTSDoneNS, event.FirstAudioNS)
	}
}

func (v *VoiceController) requestAgent(turnID, userText string) {
	if v.agentChat == nil {
		v.events <- voiceEvent{Type: "agent_error", TurnID: turnID, Detail: "ZeroClaw client is unavailable"}
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.agentMu.Lock()
	if v.agentCancel != nil {
		v.agentCancel()
	}
	v.agentCancel = cancel
	v.agentMu.Unlock()

	reply, err := v.agentChat(ctx, userText, func(event agent.Event) {
		state := ""
		switch event.Kind {
		case agent.EventThinking:
			state = "thinking"
		case agent.EventWorking:
			state = "working"
		}
		if state != "" {
			select {
			case v.events <- voiceEvent{Type: "agent_state", TurnID: turnID, State: state}:
			case <-ctx.Done():
			}
		}
	})
	cancel()
	v.agentMu.Lock()
	v.agentCancel = nil
	v.agentMu.Unlock()

	event := voiceEvent{Type: "agent_reply", TurnID: turnID, Text: reply}
	if err != nil {
		event.Type = "agent_error"
		event.Detail = err.Error()
	}
	v.events <- event
}

func (v *VoiceController) cancelAgent() {
	v.agentMu.Lock()
	cancel := v.agentCancel
	v.agentCancel = nil
	v.agentMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (v *VoiceController) watchAgent(ctx context.Context) {
	err := v.agentWatch(ctx, func(event agent.Event) {
		var out voiceEvent
		switch event.Kind {
		case agent.EventReminder:
			if strings.TrimSpace(event.Text) == "" {
				return
			}
			out = voiceEvent{Type: "agent_reminder", Text: event.Text}
		case agent.EventError:
			out = voiceEvent{Type: "agent_watch_error", Detail: event.Text}
		default:
			return
		}
		select {
		case v.events <- out:
		case <-ctx.Done():
		}
	})
	if err != nil && ctx.Err() == nil {
		select {
		case v.events <- voiceEvent{Type: "agent_watch_error", Detail: err.Error()}:
		case <-ctx.Done():
		}
	}
}

func (v *VoiceController) cancelAgentWatch() {
	v.agentMu.Lock()
	cancel := v.agentWatchStop
	v.agentWatchStop = nil
	v.agentMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) handleAgentReply(event voiceEvent) {
	lang, err := agentReplyLanguage(event.Text)
	if err != nil {
		log.Printf("ZeroClaw reply language failed: %v", err)
		a.triggerVoiceIntent(petbrain.IntentVoiceError)
		a.Voice.resume(event.TurnID)
		return
	}
	if !a.Voice.speak(event.TurnID, event.Text, lang) {
		a.Voice.resume(event.TurnID)
	}
}

func agentReplyLanguage(text string) (string, error) {
	for _, r := range text {
		if unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han) {
			return "ja", nil
		}
	}
	return voice.ResolveLanguage(text, "auto")
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
	case voice.CommandOpenChrome, voice.CommandOpenFacebook, voice.CommandOpenYouTube, voice.CommandOpenCalendar, voice.CommandOpenNotepad:
		if err := launchVoiceCommand(command); err != nil {
			log.Printf("voice command launch failed command=%s: %v", command, err)
			a.triggerVoiceIntent(petbrain.IntentVoiceError)
		}
		a.Voice.resume(event.TurnID)
	}
}

func commandLaunchTarget(command voice.VoiceCommand) (string, bool) {
	switch command {
	case voice.CommandOpenChrome:
		return "chrome.exe", true
	case voice.CommandOpenFacebook:
		return "https://www.facebook.com/", true
	case voice.CommandOpenYouTube:
		return "https://www.youtube.com/", true
	case voice.CommandOpenCalendar:
		return "https://calendar.google.com/", true
	case voice.CommandOpenNotepad:
		return "notepad.exe", true
	default:
		return "", false
	}
}

var shellExecuteW = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")

func launchVoiceCommand(command voice.VoiceCommand) error {
	target, ok := commandLaunchTarget(command)
	if !ok {
		return fmt.Errorf("voice command has no fixed launch target: %s", command)
	}
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	result, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(targetPtr)),
		0,
		0,
		1,
	)
	if result <= 32 {
		return fmt.Errorf("ShellExecuteW failed for fixed target %q: code=%d", target, result)
	}
	return nil
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

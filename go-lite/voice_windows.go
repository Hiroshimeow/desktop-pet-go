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
}

type voiceCommand struct {
	Type   string `json:"type"`
	TurnID string `json:"turn_id,omitempty"`
	Text   string `json:"text,omitempty"`
}

type VoiceController struct {
	events  chan<- voiceEvent
	session *voice.Session
	cancel  context.CancelFunc
	stdin   io.WriteCloser
	done    chan struct{}
	mu      sync.Mutex
}

func (a *App) startVoiceAsync() {
	controller := &VoiceController{
		events:  a.VoiceEvents,
		session: voice.NewSession(5 * time.Second),
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
	cmd := exec.CommandContext(ctx, "uv", "run", "--project", sidecarDir, "--frozen", "python", script)
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

func (v *VoiceController) speak(turnID, text string) {
	if err := v.send(voiceCommand{Type: "speak", TurnID: turnID, Text: text}); err != nil {
		log.Printf("voice speak failed: %v", err)
	}
}

func (v *VoiceController) stop() {
	_ = v.send(voiceCommand{Type: "shutdown"})
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
		if reply.Intent == voice.IntentUnknown {
			a.triggerVoiceIntent(petbrain.IntentVoiceUnknown)
		}
		a.Voice.speak(event.TurnID, reply.Text)
	case "first_audio":
		log.Printf("voice first audio turn=%s first_audio_ns=%d", event.TurnID, event.FirstAudioNS)
	case "turn_metrics":
		log.Printf("voice metrics turn=%s latency_ms=%.1f eos_ns=%d stt_done_ns=%d tts_done_ns=%d first_audio_ns=%d", event.TurnID, voiceLatencyMS(event), event.EOSNS, event.STTDoneNS, event.TTSDoneNS, event.FirstAudioNS)
	}
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

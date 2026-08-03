package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/websocket"
)

const (
	defaultGatewayURL = "ws://127.0.0.1:42617/ws/chat"
	defaultAgent      = "pet"
	defaultSessionID  = "desktop-pet-voice"
	defaultClientName = "Desktop Pet"
)

type EventKind string

const (
	EventThinking EventKind = "thinking"
	EventWorking  EventKind = "working"
	EventReminder EventKind = "reminder"
	EventError    EventKind = "error"
)

type Event struct {
	Kind EventKind
	Text string
}

type gatewayEvent struct {
	Type         string          `json:"type"`
	FullResponse string          `json:"full_response"`
	Message      string          `json:"message"`
	Detail       string          `json:"detail"`
	Error        json.RawMessage `json:"error"`
	JobID        string          `json:"job_id"`
	Success      bool            `json:"success"`
	Output       string          `json:"output"`
}

type ZeroClaw struct {
	GatewayURL string
	Agent      string
	SessionID  string
	Name       string
	TokenPath  string
}

func NewZeroClaw(repoRoot string) *ZeroClaw {
	return &ZeroClaw{
		GatewayURL: defaultGatewayURL,
		Agent:      defaultAgent,
		SessionID:  defaultSessionID,
		Name:       defaultClientName,
		TokenPath:  filepath.Join(repoRoot, ".voice", "zeroclaw", "pet-token.txt"),
	}
}

func (z *ZeroClaw) Chat(ctx context.Context, content string) (string, error) {
	return z.ChatEvents(ctx, content, nil)
}

func (z *ZeroClaw) ChatEvents(ctx context.Context, content string, onEvent func(Event)) (string, error) {
	conn, cleanup, err := z.connect(ctx)
	if err != nil {
		return "", err
	}
	defer cleanup()

	if err := conn.WriteJSON(map[string]string{"type": "message", "content": content}); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("send ZeroClaw message: %w", err)
	}

	for {
		var event gatewayEvent
		if err := conn.ReadJSON(&event); err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", fmt.Errorf("read ZeroClaw event: %w", err)
		}
		switch event.Type {
		case "thinking":
			emitEvent(onEvent, Event{Kind: EventThinking})
		case "plan", "tool_call", "tool_result":
			emitEvent(onEvent, Event{Kind: EventWorking})
		case "done":
			return event.FullResponse, nil
		case "error", "aborted":
			return "", fmt.Errorf("ZeroClaw %s: %s", event.Type, gatewayErrorText(event.Message, event.Detail, event.Error))
		}
	}
}

func (z *ZeroClaw) Watch(ctx context.Context, onEvent func(Event)) error {
	conn, cleanup, err := z.connect(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	// ZeroClaw waits for one client frame before subscribing this socket to
	// session/global broadcasts. A connect handshake advances that lifecycle
	// without creating a user/model turn.
	if err := conn.WriteJSON(map[string]string{"type": "connect"}); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("start ZeroClaw watcher: %w", err)
	}

	for {
		var event gatewayEvent
		if err := conn.ReadJSON(&event); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read ZeroClaw event: %w", err)
		}
		switch event.Type {
		case "cron_result":
			if !event.Success {
				detail := strings.TrimSpace(event.Output)
				if detail == "" {
					detail = strings.TrimSpace(event.JobID)
				}
				if detail == "" {
					detail = "cron job failed"
				}
				emitEvent(onEvent, Event{Kind: EventError, Text: "ZeroClaw cron_result failed: " + detail})
				continue
			}
			if strings.TrimSpace(event.Output) != "" {
				emitEvent(onEvent, Event{Kind: EventReminder, Text: event.Output})
			}
		case "error", "aborted":
			return fmt.Errorf("ZeroClaw %s: %s", event.Type, gatewayErrorText(event.Message, event.Detail, event.Error))
		}
	}
}

func (z *ZeroClaw) connect(ctx context.Context) (*websocket.Conn, func(), error) {
	if z == nil {
		return nil, nil, fmt.Errorf("zeroclaw client is nil")
	}
	tokenBytes, err := os.ReadFile(z.TokenPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read ZeroClaw token: %w", err)
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return nil, nil, fmt.Errorf("read ZeroClaw token: token is empty")
	}

	endpoint, err := url.Parse(z.GatewayURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ZeroClaw Gateway URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("agent", z.Agent)
	query.Set("session_id", z.SessionID)
	query.Set("name", z.Name)
	endpoint.RawQuery = query.Encode()

	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	conn, response, err := websocket.DefaultDialer.DialContext(ctx, endpoint.String(), header)
	if err != nil {
		if response != nil {
			return nil, nil, fmt.Errorf("connect ZeroClaw Gateway: HTTP %s: %w", response.Status, err)
		}
		return nil, nil, fmt.Errorf("connect ZeroClaw Gateway: %w", err)
	}

	finished := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-finished:
		}
	}()
	cleanup := func() {
		close(finished)
		_ = conn.Close()
	}
	return conn, cleanup, nil
}

func emitEvent(handler func(Event), event Event) {
	if handler != nil {
		handler(event)
	}
}

func gatewayErrorText(message, detail string, raw json.RawMessage) string {
	if strings.TrimSpace(message) != "" {
		return message
	}
	if strings.TrimSpace(detail) != "" {
		return detail
	}
	if len(raw) != 0 && string(raw) != "null" {
		var text string
		if json.Unmarshal(raw, &text) == nil && strings.TrimSpace(text) != "" {
			return text
		}
		return string(raw)
	}
	return "gateway error"
}

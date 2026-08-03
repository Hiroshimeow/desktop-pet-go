package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func testZeroClaw(t *testing.T, serverURL string) *ZeroClaw {
	t.Helper()
	tokenPath := filepath.Join(t.TempDir(), "pet-token.txt")
	if err := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/ws/chat"
	return &ZeroClaw{
		GatewayURL: wsURL,
		Agent:      "pet",
		SessionID:  "desktop-pet-voice",
		Name:       "Desktop Pet",
		TokenPath:  tokenPath,
	}
}

func TestZeroClawChatUsesBearerAgentSessionAndFinalReply(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		query := r.URL.Query()
		if query.Get("agent") != "pet" || query.Get("session_id") != "desktop-pet-voice" || query.Get("name") != "Desktop Pet" {
			t.Errorf("query = %v", query)
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		var request map[string]any
		if err := conn.ReadJSON(&request); err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		if request["type"] != "message" || request["content"] != "xin chào" {
			t.Errorf("request = %#v", request)
		}
		for _, event := range []any{
			map[string]any{"type": "session_start"},
			map[string]any{"type": "thinking"},
			map[string]any{"type": "plan"},
			map[string]any{"type": "tool_call"},
			map[string]any{"type": "tool_result"},
			map[string]any{"type": "cron_result", "success": true, "output": "must be watcher-owned"},
			map[string]any{"type": "chunk", "content": "Chào "},
			map[string]any{"type": "done", "full_response": "Chào bạn."},
		} {
			if err := conn.WriteJSON(event); err != nil {
				t.Errorf("write event: %v", err)
				return
			}
		}
	}))
	defer server.Close()

	client := testZeroClaw(t, server.URL)
	var lifecycle []EventKind
	got, err := client.ChatEvents(context.Background(), "xin chào", func(event Event) {
		lifecycle = append(lifecycle, event.Kind)
	})
	if err != nil {
		t.Fatalf("ChatEvents() error: %v", err)
	}
	if got != "Chào bạn." {
		t.Fatalf("ChatEvents() = %q, want final done.full_response", got)
	}
	want := []EventKind{EventThinking, EventWorking, EventWorking, EventWorking}
	if len(lifecycle) != len(want) {
		t.Fatalf("lifecycle = %#v, want %#v", lifecycle, want)
	}
	for i := range want {
		if lifecycle[i] != want[i] {
			t.Fatalf("lifecycle = %#v, want %#v", lifecycle, want)
		}
	}
}

func TestZeroClawWatchEmitsSuccessfulCronAndReportsFailedCron(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		var connect map[string]any
		if err := conn.ReadJSON(&connect); err != nil {
			t.Errorf("read watcher connect: %v", err)
			return
		}
		if connect["type"] != "connect" {
			t.Errorf("watcher first frame = %#v, want connect handshake", connect)
			return
		}
		for _, event := range []any{
			map[string]any{"type": "session_start"},
			map[string]any{"type": "cron_result", "job_id": "job-ok", "success": true, "output": "Nhắc uống nước"},
			map[string]any{"type": "cron_result", "job_id": "job-fail", "success": false, "output": "cron execution failed"},
		} {
			if err := conn.WriteJSON(event); err != nil {
				t.Errorf("write event: %v", err)
				return
			}
		}
	}))
	defer server.Close()

	client := testZeroClaw(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var events []Event
	err := client.Watch(ctx, func(event Event) {
		events = append(events, event)
		if event.Kind == EventError {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Watch() error = %v, want context.Canceled after test callback", err)
	}
	if len(events) != 2 || events[0].Kind != EventReminder || events[0].Text != "Nhắc uống nước" || events[1].Kind != EventError || !strings.Contains(events[1].Text, "cron execution failed") {
		t.Fatalf("watch events = %#v, want successful reminder then failed-cron error", events)
	}
}

func TestZeroClawWatchCancelsPromptly(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	connected := make(chan struct{})
	connectionClosed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		var connect map[string]any
		if err := conn.ReadJSON(&connect); err != nil {
			t.Errorf("read watcher connect: %v", err)
			return
		}
		if connect["type"] != "connect" {
			t.Errorf("watcher first frame = %#v, want connect handshake", connect)
			return
		}
		close(connected)
		if _, _, err := conn.ReadMessage(); err == nil {
			t.Error("expected watcher cancellation to close websocket")
		}
		close(connectionClosed)
	}))
	defer server.Close()

	client := testZeroClaw(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Watch(ctx, func(Event) {}) }()
	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("watcher did not connect")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Watch() cancellation error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Watch() did not return promptly after cancellation")
	}
	select {
	case <-connectionClosed:
	case <-time.After(time.Second):
		t.Fatal("server did not observe watcher websocket close")
	}
}

func TestZeroClawChatCancelsInflightTurn(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	requestSeen := make(chan struct{})
	connectionClosed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		var request map[string]any
		if err := conn.ReadJSON(&request); err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		close(requestSeen)
		if _, _, err := conn.ReadMessage(); err == nil {
			t.Error("expected client cancellation to close websocket")
		}
		close(connectionClosed)
	}))
	defer server.Close()

	client := testZeroClaw(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.Chat(ctx, "đợi một chút")
		done <- err
	}()

	select {
	case <-requestSeen:
	case <-time.After(time.Second):
		t.Fatal("server did not receive chat request")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Chat() cancellation error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Chat() did not return promptly after cancellation")
	}
	select {
	case <-connectionClosed:
	case <-time.After(time.Second):
		t.Fatal("server did not observe websocket close after cancellation")
	}
}

func TestZeroClawChatReturnsGatewayError(t *testing.T) {
	client := &ZeroClaw{
		GatewayURL: "ws://127.0.0.1:1/ws/chat",
		Agent:      "pet",
		SessionID:  "desktop-pet-voice",
		Name:       "Desktop Pet",
		TokenPath:  filepath.Join(t.TempDir(), "missing-token.txt"),
	}
	if _, err := client.Chat(context.Background(), "hello"); err == nil {
		t.Fatal("Chat() with missing token unexpectedly succeeded")
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		var request map[string]any
		if err := conn.ReadJSON(&request); err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		_ = conn.WriteJSON(map[string]any{"type": "error", "message": "provider unavailable"})
	}))
	defer server.Close()

	client = testZeroClaw(t, server.URL)
	if _, err := client.Chat(context.Background(), "hello"); err == nil || !strings.Contains(err.Error(), "provider unavailable") {
		t.Fatalf("Chat() gateway error = %v", err)
	}
}

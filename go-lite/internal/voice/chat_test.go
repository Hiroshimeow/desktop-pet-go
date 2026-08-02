package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatClientReplyBoundsRequestAndReturnsPlainText(t *testing.T) {
	var got struct {
		Model     string        `json:"model"`
		Messages  []ChatMessage `json:"messages"`
		MaxTokens int           `json:"max_tokens"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  Xin chào!  "}}]}`))
	}))
	defer server.Close()

	history := []ChatMessage{
		{Role: "user", Content: "drop-1"},
		{Role: "assistant", Content: "drop-2"},
		{Role: "user", Content: strings.Repeat("á", 601)},
		{Role: "assistant", Content: "h2"},
		{Role: "user", Content: "h3"},
		{Role: "assistant", Content: "h4"},
		{Role: "user", Content: "h5"},
		{Role: "assistant", Content: "h6"},
	}
	client := &ChatClient{endpoint: server.URL + "/v1/chat/completions", client: server.Client()}
	reply, err := client.Reply(context.Background(), "pet persona", history, strings.Repeat("x", 601))
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if reply != "Xin chào!" {
		t.Fatalf("reply = %q", reply)
	}
	if got.Model != "desktop-pet" || got.MaxTokens != 96 {
		t.Fatalf("model/max_tokens = %q/%d", got.Model, got.MaxTokens)
	}
	if len(got.Messages) != 8 {
		t.Fatalf("message count = %d, want 8", len(got.Messages))
	}
	if got.Messages[0] != (ChatMessage{Role: "system", Content: "pet persona"}) {
		t.Fatalf("system message = %#v", got.Messages[0])
	}
	if got.Messages[1].Content == "drop-1" || got.Messages[1].Content == "drop-2" {
		t.Fatalf("old history was not dropped: %#v", got.Messages)
	}
	if len([]rune(got.Messages[1].Content)) != 600 || len([]rune(got.Messages[7].Content)) != 600 {
		t.Fatalf("history/user bounds = %d/%d", len([]rune(got.Messages[1].Content)), len([]rune(got.Messages[7].Content)))
	}
}

func TestChatClientReplyRejectsBadResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "http error", status: http.StatusServiceUnavailable, body: `busy`},
		{name: "malformed", status: http.StatusOK, body: `{`},
		{name: "empty", status: http.StatusOK, body: `{"choices":[{"message":{"content":"   "}}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := &ChatClient{endpoint: server.URL, client: server.Client()}
			if _, err := client.Reply(context.Background(), "pet", nil, "hello"); err == nil {
				t.Fatal("Reply error = nil")
			}
		})
	}
}

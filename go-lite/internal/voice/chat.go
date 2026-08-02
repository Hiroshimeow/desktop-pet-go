package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	localChatEndpoint = "http://127.0.0.1:8080/v1/chat/completions"
	localChatModel    = "desktop-pet"
	chatHistoryLimit  = 6
	chatMessageRunes  = 600
	chatMaxTokens     = 96
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatClient struct {
	endpoint string
	client   *http.Client
}

func NewLocalChatClient() *ChatClient {
	return &ChatClient{
		endpoint: localChatEndpoint,
		client:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *ChatClient) Reply(ctx context.Context, persona string, history []ChatMessage, user string) (string, error) {
	start := len(history) - chatHistoryLimit
	if start < 0 {
		start = 0
	}
	messages := make([]ChatMessage, 0, 2+len(history)-start)
	messages = append(messages, ChatMessage{Role: "system", Content: persona})
	for _, message := range history[start:] {
		message.Content = boundRunes(message.Content, chatMessageRunes)
		messages = append(messages, message)
	}
	messages = append(messages, ChatMessage{Role: "user", Content: boundRunes(user, chatMessageRunes)})

	body, err := json.Marshal(struct {
		Model     string        `json:"model"`
		Messages  []ChatMessage `json:"messages"`
		MaxTokens int           `json:"max_tokens"`
	}{
		Model:     localChatModel,
		Messages:  messages,
		MaxTokens: chatMaxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("encode local chat request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create local chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("local chat request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("local chat response: %s", resp.Status)
	}
	var result struct {
		Choices []struct {
			Message ChatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode local chat response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("local chat response has no choices")
	}
	reply := strings.TrimSpace(result.Choices[0].Message.Content)
	if reply == "" {
		return "", fmt.Errorf("local chat response is empty")
	}
	return reply, nil
}

func boundRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

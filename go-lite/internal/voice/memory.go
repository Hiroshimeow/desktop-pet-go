package voice

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultPersona  = "You are a small desktop pet. Reply naturally in the user's language, Vietnamese or English. Keep replies short, conversational, easy to speak aloud, and plain text only."
	memoryTurnLimit = 3
	personaFileName = "persona.txt"
	historyFileName = "history.json"
)

type memoryTurn struct {
	User      string `json:"user"`
	Assistant string `json:"assistant"`
}

type Memory struct {
	dir     string
	persona string
	turns   []memoryTurn
}

func LoadMemory(dir string) (*Memory, error) {
	memory := &Memory{dir: dir, persona: DefaultPersona}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return memory, fmt.Errorf("create voice memory directory: %w", err)
	}

	personaPath := filepath.Join(dir, personaFileName)
	persona, err := os.ReadFile(personaPath)
	switch {
	case err == nil:
		memory.persona = string(persona)
	case os.IsNotExist(err):
		if err := os.WriteFile(personaPath, []byte(DefaultPersona), 0o644); err != nil {
			return memory, fmt.Errorf("create default voice persona: %w", err)
		}
	default:
		return memory, fmt.Errorf("read voice persona: %w", err)
	}

	historyPath := filepath.Join(dir, historyFileName)
	history, err := os.ReadFile(historyPath)
	if os.IsNotExist(err) {
		return memory, nil
	}
	if err != nil {
		return memory, fmt.Errorf("read voice history: %w", err)
	}
	if err := json.Unmarshal(history, &memory.turns); err != nil {
		memory.turns = nil
		return memory, fmt.Errorf("decode voice history: %w", err)
	}
	memory.trim()
	return memory, nil
}

func (m *Memory) Snapshot() (string, []ChatMessage) {
	if m == nil {
		return DefaultPersona, nil
	}
	history := make([]ChatMessage, 0, len(m.turns)*2)
	for _, turn := range m.turns {
		history = append(history,
			ChatMessage{Role: "user", Content: turn.User},
			ChatMessage{Role: "assistant", Content: turn.Assistant},
		)
	}
	return m.persona, history
}

func (m *Memory) Append(user, assistant string) error {
	m.turns = append(m.turns, memoryTurn{User: user, Assistant: assistant})
	m.trim()

	data, err := json.Marshal(m.turns)
	if err != nil {
		return fmt.Errorf("encode voice history: %w", err)
	}
	if err := os.WriteFile(filepath.Join(m.dir, historyFileName), data, 0o644); err != nil {
		return fmt.Errorf("write voice history: %w", err)
	}
	return nil
}

func (m *Memory) trim() {
	if len(m.turns) <= memoryTurnLimit {
		return
	}
	m.turns = append([]memoryTurn(nil), m.turns[len(m.turns)-memoryTurnLimit:]...)
}

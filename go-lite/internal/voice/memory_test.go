package voice

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMemoryRoundTripAndTrimOldestTurns(t *testing.T) {
	dir := t.TempDir()
	memory, err := LoadMemory(dir)
	if err != nil {
		t.Fatalf("LoadMemory default: %v", err)
	}
	persona, history := memory.Snapshot()
	if persona != DefaultPersona {
		t.Fatalf("default persona = %q", persona)
	}
	if len(history) != 0 {
		t.Fatalf("default history = %#v", history)
	}
	personaPath := filepath.Join(dir, "persona.txt")
	if got, err := os.ReadFile(personaPath); err != nil {
		t.Fatalf("read default persona: %v", err)
	} else if string(got) != DefaultPersona {
		t.Fatalf("persona file = %q", got)
	}

	const customPersona = "You are Miu. Reply briefly in Vietnamese or English."
	if err := os.WriteFile(personaPath, []byte(customPersona), 0o644); err != nil {
		t.Fatalf("edit persona: %v", err)
	}
	memory, err = LoadMemory(dir)
	if err != nil {
		t.Fatalf("reload custom persona: %v", err)
	}
	for i, turn := range [][2]string{{"u1", "a1"}, {"u2", "a2"}, {"u3", "a3"}, {"u4", "a4"}} {
		if err := memory.Append(turn[0], turn[1]); err != nil {
			t.Fatalf("Append turn %d: %v", i+1, err)
		}
	}

	wantHistory := []ChatMessage{
		{Role: "user", Content: "u2"},
		{Role: "assistant", Content: "a2"},
		{Role: "user", Content: "u3"},
		{Role: "assistant", Content: "a3"},
		{Role: "user", Content: "u4"},
		{Role: "assistant", Content: "a4"},
	}
	persona, history = memory.Snapshot()
	if persona != customPersona {
		t.Fatalf("custom persona = %q", persona)
	}
	if !reflect.DeepEqual(history, wantHistory) {
		t.Fatalf("trimmed history = %#v, want %#v", history, wantHistory)
	}

	reloaded, err := LoadMemory(dir)
	if err != nil {
		t.Fatalf("reload persisted memory: %v", err)
	}
	persona, history = reloaded.Snapshot()
	if persona != customPersona || !reflect.DeepEqual(history, wantHistory) {
		t.Fatalf("reloaded persona/history = %q/%#v", persona, history)
	}
}

func TestMemoryMissingOrMalformedHistoryStartsClean(t *testing.T) {
	t.Run("missing history", func(t *testing.T) {
		memory, err := LoadMemory(t.TempDir())
		if err != nil {
			t.Fatalf("LoadMemory missing history: %v", err)
		}
		_, history := memory.Snapshot()
		if len(history) != 0 {
			t.Fatalf("history = %#v", history)
		}
	})

	t.Run("malformed history", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "persona.txt"), []byte("pet"), 0o644); err != nil {
			t.Fatalf("write persona: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "history.json"), []byte("{"), 0o644); err != nil {
			t.Fatalf("write malformed history: %v", err)
		}
		memory, err := LoadMemory(dir)
		if memory == nil || err == nil {
			t.Fatalf("LoadMemory malformed = %#v, %v; want usable memory plus warning", memory, err)
		}
		persona, history := memory.Snapshot()
		if persona != "pet" || len(history) != 0 {
			t.Fatalf("malformed recovery persona/history = %q/%#v", persona, history)
		}
	})

	t.Run("write failure keeps in-memory turn", func(t *testing.T) {
		dir := t.TempDir()
		memory, err := LoadMemory(dir)
		if err != nil {
			t.Fatalf("LoadMemory: %v", err)
		}
		if err := os.Mkdir(filepath.Join(dir, "history.json"), 0o755); err != nil {
			t.Fatalf("block history path: %v", err)
		}
		if err := memory.Append("u", "a"); err == nil {
			t.Fatal("Append write error = nil")
		}
		_, history := memory.Snapshot()
		want := []ChatMessage{{Role: "user", Content: "u"}, {Role: "assistant", Content: "a"}}
		if !reflect.DeepEqual(history, want) {
			t.Fatalf("in-memory history after write failure = %#v", history)
		}
	})
}

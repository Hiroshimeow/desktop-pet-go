package voice

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveLanguageExplicitAndAuto(t *testing.T) {
	cases := []struct {
		name string
		text string
		hint string
		want string
	}{
		{name: "auto Vietnamese", text: "Xin chào bạn", hint: "auto", want: "vi"},
		{name: "auto English", text: "Hello friend", hint: "auto", want: "en"},
		{name: "explicit Vietnamese wins", text: "Hello friend", hint: "vi", want: "vi"},
		{name: "explicit English wins", text: "Xin chào bạn", hint: "en", want: "en"},
	}
	for _, tc := range cases {
		got, err := ResolveLanguage(tc.text, tc.hint)
		if err != nil {
			t.Fatalf("%s: ResolveLanguage() error = %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: ResolveLanguage() = %q, want %q", tc.name, got, tc.want)
		}
	}
	if _, err := ResolveLanguage("hello", "ja"); err == nil {
		t.Fatal("ResolveLanguage() must reject unsupported language hints")
	}
	if _, err := ResolveLanguage("こんにちは", "auto"); err == nil {
		t.Fatal("ResolveLanguage() auto must reject text outside supported VI/EN scripts")
	}
}

func TestReadTextFileAcceptsUTF8TXTAndMDOnly(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"note.txt", "note.md"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("Xin chào\n\nHello"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := ReadTextFile(path)
		if err != nil {
			t.Fatalf("ReadTextFile(%q) error = %v", name, err)
		}
		if got != "Xin chào\n\nHello" {
			t.Fatalf("ReadTextFile(%q) = %q", name, got)
		}
	}

	badExt := filepath.Join(root, "note.pdf")
	if err := os.WriteFile(badExt, []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTextFile(badExt); err == nil {
		t.Fatal("ReadTextFile() must reject non-TXT/MD extensions")
	}

	badUTF8 := filepath.Join(root, "bad.txt")
	if err := os.WriteFile(badUTF8, []byte{0xff, 0xfe, 0xfd}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTextFile(badUTF8); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("ReadTextFile() invalid UTF-8 error = %v", err)
	}
}

func TestChunkTextParagraphSentenceThenHardCapDeterministic(t *testing.T) {
	input := "One short paragraph.\n\nSentence one. Sentence two is long.\n\nabcdefghijklmnopqrstuvwxyz"
	want := []string{
		"One short paragraph.",
		"Sentence one.",
		"Sentence two is long.",
		"abcdefghijklmnopqrstuvwx",
		"yz",
	}
	first := ChunkText(input, 24)
	second := ChunkText(input, 24)
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("ChunkText() = %#v, want %#v", first, want)
	}
	if !reflect.DeepEqual(second, first) {
		t.Fatalf("ChunkText() is not deterministic: first=%#v second=%#v", first, second)
	}
	for _, chunk := range first {
		if len([]rune(chunk)) > 24 {
			t.Fatalf("chunk exceeds hard cap: %q", chunk)
		}
	}
}

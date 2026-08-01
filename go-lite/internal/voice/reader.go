package voice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const vietnameseDiacritics = "ăâđêôơưáàảãạắằẳẵặấầẩẫậéèẻẽẹếềểễệíìỉĩịóòỏõọốồổỗộớờởỡợúùủũụứừửữựýỳỷỹỵ"

func ResolveLanguage(text, hint string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(hint)) {
	case "", "auto":
		lower := strings.ToLower(text)
		for _, r := range lower {
			if strings.ContainsRune(vietnameseDiacritics, r) {
				return "vi", nil
			}
		}
		for _, r := range lower {
			if unicode.Is(unicode.Latin, r) {
				return "en", nil
			}
		}
		return "", fmt.Errorf("cannot detect Vietnamese or English text")
	case "vi":
		return "vi", nil
	case "en":
		return "en", nil
	default:
		return "", fmt.Errorf("unsupported language %q; use auto, vi, or en", hint)
	}
}

func ReadTextFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".txt" && ext != ".md" {
		return "", fmt.Errorf("unsupported reader file %q: only .txt and .md are supported", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("read %q: file must be UTF-8", path)
	}
	return string(data), nil
}

func ChunkText(text string, maxChars int) []string {
	if maxChars <= 0 {
		return nil
	}
	paragraphs := splitParagraphs(text)
	chunks := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if runeLen(paragraph) <= maxChars {
			chunks = append(chunks, paragraph)
			continue
		}
		for _, sentence := range splitSentences(paragraph) {
			if runeLen(sentence) <= maxChars {
				chunks = append(chunks, sentence)
				continue
			}
			chunks = append(chunks, hardCap(sentence, maxChars)...)
		}
	}
	return chunks
}

func splitParagraphs(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var paragraphs []string
	var lines []string
	flush := func() {
		if len(lines) == 0 {
			return
		}
		paragraph := strings.Join(lines, " ")
		if paragraph = strings.TrimSpace(paragraph); paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
		lines = lines[:0]
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		lines = append(lines, line)
	}
	flush()
	return paragraphs
}

func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder
	flush := func() {
		sentence := strings.TrimSpace(current.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
		current.Reset()
	}
	for _, r := range text {
		current.WriteRune(r)
		switch r {
		case '.', '!', '?', '…':
			flush()
		}
	}
	flush()
	return sentences
}

func hardCap(text string, maxChars int) []string {
	runes := []rune(strings.TrimSpace(text))
	chunks := make([]string, 0, (len(runes)+maxChars-1)/maxChars)
	for len(runes) > 0 {
		end := maxChars
		if len(runes) < end {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		runes = runes[end:]
	}
	return chunks
}

func runeLen(text string) int {
	return utf8.RuneCountInString(text)
}

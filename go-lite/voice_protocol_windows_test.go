//go:build windows

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVoiceProtocolOversizeFailsWithoutPublishingSuffix(t *testing.T) {
	input := "{\"type\":\"state\"}\n" + strings.Repeat("A", maxVoiceEventBytes+1) + "{\"type\":\"final\"}\n"
	count := 0
	err := readVoiceProtocolLines(strings.NewReader(input), func([]byte) bool { count++; return true })
	if err == nil || count != 1 {
		t.Fatalf("oversize did not fail closed: err=%v events=%d", err, count)
	}
}

func TestVoiceProtocolPreservesUnicodeAndEOFEvent(t *testing.T) {
	input := "{\"text\":\"Xin chào ✓\"}\r\n{\"type\":\"done\"}"
	var received []string
	if err := readVoiceProtocolLines(bytes.NewBufferString(input), func(line []byte) bool {
		received = append(received, string(line))
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[0] != "{\"text\":\"Xin chào ✓\"}" || received[1] != "{\"type\":\"done\"}" {
		t.Fatalf("protocol changed event: %q", received)
	}
}

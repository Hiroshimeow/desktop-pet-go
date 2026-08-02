package voice

import "testing"

func TestParseCommandExactBilingualPhrases(t *testing.T) {
	// VOICE-REQ-050: STT text maps only to the fixed VoiceCommand enum.
	tests := []struct {
		input string
		want  VoiceCommand
	}{
		{"tạm dừng", CommandPause},
		{"pause", CommandPause},
		{"tiếp tục", CommandResume},
		{"resume", CommandResume},
		{"bỏ qua", CommandSkip},
		{"đoạn tiếp", CommandSkip},
		{"skip", CommandSkip},
		{"next", CommandSkip},
		{"dừng đọc", CommandStop},
		{"dừng lại", CommandStop},
		{"stop reading", CommandStop},
		{"stop", CommandStop},
		{"đọc clipboard", CommandReadClipboard},
		{"read clipboard", CommandReadClipboard},
		{"trạng thái", CommandStatus},
		{"status", CommandStatus},
	}
	for _, tt := range tests {
		got, ok := ParseCommand(tt.input)
		if !ok || got != tt.want {
			t.Fatalf("ParseCommand(%q) = (%q, %v), want (%q, true)", tt.input, got, ok, tt.want)
		}
	}
}

func TestParseCommandNormalizationWakeAndUnknown(t *testing.T) {
	// VOICE-REQ-051: unknown or near-match text must remain a non-command.
	matches := []struct {
		input string
		want  VoiceCommand
	}{
		{"  PET ƠI,   TẠM DỪNG!!! ", CommandPause},
		{"pet ơi, pause", CommandPause},
		{"STATUS!!!", CommandStatus},
	}
	for _, tt := range matches {
		got, ok := ParseCommand(tt.input)
		if !ok || got != tt.want {
			t.Fatalf("ParseCommand(%q) = (%q, %v), want (%q, true)", tt.input, got, ok, tt.want)
		}
	}

	for _, input := range []string{"pause please", "please stop", "đọc clipboard ngay", "hôm nay thế nào"} {
		if got, ok := ParseCommand(input); ok {
			t.Fatalf("ParseCommand(%q) = (%q, true), want non-command", input, got)
		}
	}
}

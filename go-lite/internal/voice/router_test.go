package voice

import "testing"

func TestRouteTranscriptCommands(t *testing.T) {
	tests := []struct {
		input string
		want  VoiceCommand
	}{
		{"pause", CommandPause},
		{"pet ơi, pause", CommandPause},
		{"tạm dừng", CommandPause},
		{"tiếp tục", CommandResume},
		{"skip", CommandSkip},
		{"dừng đọc", CommandStop},
		{"read clipboard", CommandReadClipboard},
		{"trạng thái", CommandStatus},
	}
	for _, tt := range tests {
		kind, command := RouteTranscript(tt.input)
		if kind != RouteCommand || command != tt.want {
			t.Fatalf("RouteTranscript(%q) = (%v, %q), want (%v, %q)", tt.input, kind, command, RouteCommand, tt.want)
		}
	}
}

func TestRouteTranscriptConversation(t *testing.T) {
	for _, input := range []string{
		"hôm nay bạn thế nào",
		"what are you doing today",
		"xin chào",
		"hello",
		"em là ai",
		"bye",
		"pause please",
	} {
		kind, command := RouteTranscript(input)
		if kind != RouteConversation || command != "" {
			t.Fatalf("RouteTranscript(%q) = (%v, %q), want (%v, empty command)", input, kind, command, RouteConversation)
		}
	}
}

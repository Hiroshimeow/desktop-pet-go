package voice

import "testing"

func TestMatchFixedVietnameseIntents(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		intent Intent
	}{
		{"greeting", "Xin chào em!", IntentGreeting},
		{"identity", "em là ai", IntentIdentity},
		{"identity unaccented", "ten em la gi", IntentIdentity},
		{"status", "em khỏe không", IntentStatus},
		{"goodbye", "tạm biệt nhé", IntentGoodbye},
		{"unknown", "hôm nay trời có mưa không", IntentUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Match(tt.input); got.Intent != tt.intent {
				t.Fatalf("Match(%q) intent = %q, want %q", tt.input, got.Intent, tt.intent)
			}
		})
	}
}

func TestNormalizeVietnamese(t *testing.T) {
	if got, want := Normalize("  PET ƠI,   Em LÀ ai? "), "pet ơi em là ai"; got != want {
		t.Fatalf("Normalize() = %q, want %q", got, want)
	}
}

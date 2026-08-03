package voice

import (
	"testing"
	"time"
)

func TestSessionWakeAndQuestionSameUtterance(t *testing.T) {
	now := time.Unix(100, 0)
	for _, text := range []string{
		"pet ơi, em là ai",
		"pét ơi, em là ai",
		"bết ơi, em là ai",
		"mèo ơi, em là ai",
	} {
		s := NewSession(5 * time.Second)
		got, ok := s.Handle(text, now)
		if !ok || got.Intent != IntentIdentity {
			t.Fatalf("Handle(%q) = (%+v, %v), want identity reply", text, got, ok)
		}
	}
	for _, text := range []string{
		"ペット、今日はどう？",
		"ねえ ペット、今日はどう？",
	} {
		s := NewSession(5 * time.Second)
		if _, ok := s.Handle(text, now); !ok {
			t.Fatalf("Handle(%q) did not accept Japanese wake + question", text)
		}
	}
}

func TestSessionJapaneseWakeRejectsNearMatch(t *testing.T) {
	now := time.Unix(100, 0)
	for _, text := range []string{
		"ペットボトル、今日はどう？",
		"ねえ ペットボトル、今日はどう？",
	} {
		s := NewSession(5 * time.Second)
		if _, ok := s.Handle(text, now); ok {
			t.Fatalf("Japanese near-match %q must not wake the pet", text)
		}
	}
}

func TestSessionJapaneseWakeAllowsNaturalNoSpaceSentence(t *testing.T) {
	now := time.Unix(100, 0)
	for _, text := range []string{
		"ペット今日はどう？",
		"ペット元気ですか",
		"ねえ ペット今日はどう？",
	} {
		s := NewSession(5 * time.Second)
		if _, ok := s.Handle(text, now); !ok {
			t.Fatalf("Japanese no-space wake %q was rejected", text)
		}
	}
}

func TestSessionWakeThenFollowUp(t *testing.T) {
	now := time.Unix(100, 0)
	s := NewSession(5 * time.Second)
	if _, ok := s.Handle("pet ơi", now); ok {
		t.Fatal("wake-only utterance must not reply")
	}
	got, ok := s.Handle("em khỏe không", now.Add(3*time.Second))
	if !ok || got.Intent != IntentStatus {
		t.Fatalf("follow-up = (%+v, %v), want status reply", got, ok)
	}
}

func TestSessionIgnoresAmbientAndExpiredFollowUp(t *testing.T) {
	now := time.Unix(100, 0)
	s := NewSession(5 * time.Second)
	if _, ok := s.Handle("em là ai", now); ok {
		t.Fatal("ambient non-wake utterance must be ignored")
	}
	s.Handle("pet ơi", now)
	if _, ok := s.Handle("em là ai", now.Add(6*time.Second)); ok {
		t.Fatal("expired follow-up must be ignored")
	}
}

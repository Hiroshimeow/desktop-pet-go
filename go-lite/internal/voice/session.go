package voice

import (
	"strings"
	"time"
)

type Session struct {
	followUp time.Duration
	awakeTo  time.Time
}

func NewSession(followUp time.Duration) *Session {
	if followUp <= 0 {
		followUp = 5 * time.Second
	}
	return &Session{followUp: followUp}
}

func (s *Session) Handle(text string, now time.Time) (Reply, bool) {
	n := Normalize(text)
	rest, woke := stripWake(n)
	if woke {
		if rest == "" {
			s.awakeTo = now.Add(s.followUp)
			return Reply{}, false
		}
		s.awakeTo = time.Time{}
		return Match(rest), true
	}
	if s.awakeTo.IsZero() || now.After(s.awakeTo) {
		s.awakeTo = time.Time{}
		return Reply{}, false
	}
	s.awakeTo = time.Time{}
	return Match(n), true
}

func stripWake(text string) (string, bool) {
	for _, wake := range []string{
		"pet ơi", "pet oi", "pét ơi",
		"bét ơi", "bết ơi", "bet oi",
		"mèo ơi", "meo oi",
	} {
		if text == wake {
			return "", true
		}
		if strings.HasPrefix(text, wake+" ") {
			return strings.TrimSpace(strings.TrimPrefix(text, wake)), true
		}
	}
	return text, false
}

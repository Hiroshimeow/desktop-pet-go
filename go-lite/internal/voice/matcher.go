package voice

import (
	"strings"
	"unicode"
)

type Intent string

const (
	IntentGreeting Intent = "greeting"
	IntentIdentity Intent = "identity"
	IntentStatus   Intent = "status"
	IntentGoodbye  Intent = "goodbye"
	IntentUnknown  Intent = "unknown"
)

type Reply struct {
	Intent Intent
	Text   string
}

var replies = map[Intent]string{
	IntentGreeting: "Chào bạn, mình đang nghe đây.",
	IntentIdentity: "Mình là pet nhỏ trên màn hình của bạn.",
	IntentStatus:   "Mình khỏe và vẫn đang ở đây với bạn.",
	IntentGoodbye:  "Tạm biệt bạn, gọi mình khi cần nhé.",
	IntentUnknown:  "Mình chưa hiểu câu đó, bạn thử nói cách khác nhé.",
}

func Normalize(text string) string {
	text = strings.ToLower(text)
	var b strings.Builder
	space := true
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func Match(text string) Reply {
	n := Normalize(text)
	intent := IntentUnknown
	switch {
	case containsAny(n, "em là ai", "bạn là ai", "pet là ai", "tên em là gì", "ten em la gi", "em tên gì", "em ten gi"):
		intent = IntentIdentity
	case containsAny(n, "em khỏe không", "em khoẻ không", "khoe khong", "khỏe không", "khoẻ không", "thế nào rồi", "sao rồi"):
		intent = IntentStatus
	case containsAny(n, "tạm biệt", "tam biet", "bye", "ngủ ngon", "ngu ngon"):
		intent = IntentGoodbye
	case containsAny(n, "xin chào", "xin chao", "chào em", "chao em", "hello", "alo"):
		intent = IntentGreeting
	}
	return Reply{Intent: intent, Text: replies[intent]}
}

func containsAny(text string, aliases ...string) bool {
	for _, alias := range aliases {
		if strings.Contains(text, alias) {
			return true
		}
	}
	return false
}

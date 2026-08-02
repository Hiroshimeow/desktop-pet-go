package voice

type VoiceCommand string

const (
	CommandPause         VoiceCommand = "pause"
	CommandResume        VoiceCommand = "resume"
	CommandSkip          VoiceCommand = "skip"
	CommandStop          VoiceCommand = "stop"
	CommandReadClipboard VoiceCommand = "read_clipboard"
	CommandStatus        VoiceCommand = "status"
)

func ParseCommand(text string) (VoiceCommand, bool) {
	n := Normalize(text)
	if rest, woke := stripWake(n); woke {
		n = rest
	}

	switch n {
	case "tạm dừng", "pause":
		return CommandPause, true
	case "tiếp tục", "resume":
		return CommandResume, true
	case "bỏ qua", "đoạn tiếp", "skip", "next":
		return CommandSkip, true
	case "dừng đọc", "dừng lại", "stop reading", "stop":
		return CommandStop, true
	case "đọc clipboard", "read clipboard":
		return CommandReadClipboard, true
	case "trạng thái", "status":
		return CommandStatus, true
	default:
		return "", false
	}
}

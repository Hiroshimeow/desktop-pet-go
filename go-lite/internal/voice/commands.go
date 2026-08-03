package voice

type VoiceCommand string

const (
	CommandPause         VoiceCommand = "pause"
	CommandResume        VoiceCommand = "resume"
	CommandSkip          VoiceCommand = "skip"
	CommandStop          VoiceCommand = "stop"
	CommandReadClipboard VoiceCommand = "read_clipboard"
	CommandStatus        VoiceCommand = "status"
	CommandOpenChrome    VoiceCommand = "open_chrome"
	CommandOpenFacebook  VoiceCommand = "open_facebook"
	CommandOpenYouTube   VoiceCommand = "open_youtube"
	CommandOpenCalendar  VoiceCommand = "open_calendar"
	CommandOpenNotepad   VoiceCommand = "open_notepad"
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
	case "mở chrome", "open chrome":
		return CommandOpenChrome, true
	case "mở facebook", "open facebook":
		return CommandOpenFacebook, true
	case "mở youtube", "open youtube":
		return CommandOpenYouTube, true
	case "mở lịch", "open calendar":
		return CommandOpenCalendar, true
	case "mở notepad", "mở ghi chú", "open notepad", "open note":
		return CommandOpenNotepad, true
	default:
		return "", false
	}
}

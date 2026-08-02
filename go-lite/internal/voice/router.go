package voice

type RouteKind uint8

const (
	RouteConversation RouteKind = iota
	RouteCommand
)

func RouteTranscript(text string) (RouteKind, VoiceCommand) {
	if command, ok := ParseCommand(text); ok {
		return RouteCommand, command
	}
	return RouteConversation, ""
}

package bot

type MessageType int

const (
	TextMessage MessageType = iota
	LinkMessage
	MarkdownMessage
	ActionCardMessage
	FeedCardMessage
)

type Message interface {
	Marshal() ([]byte, error)
}

func NewMessage(t MessageType, opt ...Option) Message {
	switch t {
	case TextMessage:
		m := &textMessage{
			message: message{
				Msgtype: "text",
			},
			Text: struct {
				Content string "json:\"content\""
			}{
				Content: "",
			},
			At: struct {
				AtMobiles []string "json:\"atMobiles\""
				AtUserIds []string "json:\"atUserIds\""
				IsAtAll   bool     "json:\"isAtAll\""
			}{
				AtMobiles: []string{},
				AtUserIds: []string{},
				IsAtAll:   false,
			},
		}
		for _, optFunc := range opt {
			optFunc(m)
		}
		return m
	default:
		return nil
	}
}

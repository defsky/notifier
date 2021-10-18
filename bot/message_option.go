package bot

type Option func(Message)

func WithText(s string) Option {
	return func(m Message) {
		switch m.(type) {
		case *textMessage:
			m.(*textMessage).Text.Content = s
		}
	}
}

func WithAtMobiles(phones []string) Option {
	return func(m Message) {
		switch m.(type) {
		case *textMessage:
			m.(*textMessage).At.AtMobiles =
				append(m.(*textMessage).At.AtMobiles, phones...)
		}
	}
}
func WithAtAll(atAll bool) Option {
	return func(m Message) {
		switch m.(type) {
		case *textMessage:
			m.(*textMessage).At.IsAtAll = atAll
		}
	}
}

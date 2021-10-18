package bot

import "encoding/json"

type message struct {
	Msgtype string `json:"msgtype"`
}
type textMessage struct {
	message
	Text struct {
		Content string `json:"content"`
	} `json:"text"`
	At struct {
		AtMobiles []string `json:"atMobiles"`
		AtUserIds []string `json:"atUserIds"`
		IsAtAll   bool     `json:"isAtAll"`
	} `json:"at"`
}

func (msg *textMessage) Marshal() ([]byte, error) {
	return json.Marshal(msg)
}

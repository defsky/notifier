package bot

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

type Bot interface {
	SendMessage(Message) int
}

type bot struct {
	webhook string
	client  *http.Client
}

type HookResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (bot *bot) SendMessage(msg Message) int {
	data, err := msg.Marshal()
	if err != nil {
		log.Println("Marshal message error: ", err)
		return 1
	}
	req, err := http.NewRequest("POST", bot.webhook, bytes.NewBuffer(data))
	if err != nil {
		log.Println("Init request error: ", err)
		return 1
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := bot.client.Do(req)
	if err != nil {
		log.Println("Do http request error: ", err)
		return 1
	}
	defer resp.Body.Close()

	hookResp := &HookResponse{}
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("read resp.Body error: ", err)
		return 1
	}
	err = json.Unmarshal(respData, hookResp)
	if err != nil {
		log.Println("Unmarshal respData error: ", err)
		return 1
	}
	if hookResp.ErrCode != 0 {
		log.Println("Hook error: ", hookResp.ErrMsg)
		return hookResp.ErrCode
	}
	return 0
}

func NewBot(hook string) Bot {
	return &bot{
		webhook: hook,
		client:  &http.Client{},
	}
}

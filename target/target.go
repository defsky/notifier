package target

import (
	"io/ioutil"
	"net/http"
	"notifier/bot"
	"reflect"
	"sync"

	"github.com/spf13/viper"
)

type BotMessage struct {
	TargetName string
	Message    bot.Message
}
type Target interface {
	Start()
	SetConfig(*viper.Viper, chan BotMessage, chan struct{}, *sync.WaitGroup) (Target, error)
}

func NewTarget(tn string) Target {
	if typeTable[tn] != nil {
		t := reflect.ValueOf(typeTable[tn]).Type()
		v := reflect.New(t).Elem()
		v.FieldByName("Name").SetString(tn)
		return v.Interface().(Target)
	}
	return nil
}

type baseTarget struct {
	client *http.Client
	Config *viper.Viper
	M      chan BotMessage
	Kill   chan struct{}
	Name   string
	wg     *sync.WaitGroup
}

func (t baseTarget) Get(url string) ([]byte, error) {
	resp, err := t.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

var typeTable map[string]Target

func init() {
	typeTable = make(map[string]Target)
	typeTable["baddocTarget"] = baddocTarget{}
}

package target

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"notifier/bot"
	"notifier/cache"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/viper"
)

type acTarget struct {
	baseTarget
	api           string
	checkDelay    int
	isAlive       bool
	realThreshold float64
	realDelay     int64
	avgThreshold  float64
	avgDelay      int64
	receiver      string
	memo          string
}

func (t acTarget) Start() {
	t.wg.Add(1)
	go t.worker()
}

func (t acTarget) SetConfig(cfg *viper.Viper, ch chan BotMessage, stop chan struct{}, wg *sync.WaitGroup) (Target, error) {
	t.Config = cfg
	t.client = &http.Client{}
	t.wg = wg

	t.memo = t.Config.GetString("memo")

	if t.M = ch; t.M == nil {
		return nil, errors.New(t.Name + ", message channel is nil")
	}

	if t.Kill = stop; t.Kill == nil {
		return nil, errors.New(t.Name + ", stop channel is nil")
	}

	if t.api = t.Config.GetString("api"); len(t.api) == 0 {
		return nil, errors.New("need string value for config key 'api'")
	}

	if t.checkDelay = t.Config.GetInt("check-delay"); t.checkDelay <= 0 {
		return nil, errors.New("need positive int value for config key 'check-delay'")
	}
	if t.checkDelay <= 30 {
		t.checkDelay = 30
	}

	if t.realThreshold = t.Config.GetFloat64("real-threshold"); t.realThreshold <= 0 {
		return nil, errors.New("need positive int value for config key 'real-threshold'")
	}

	if t.realDelay = t.Config.GetInt64("real-delay") * 60; t.realDelay <= 0 {
		return nil, errors.New("need positive int value for config key 'real-delay'")
	}

	if t.avgThreshold = t.Config.GetFloat64("avg-threshold"); t.avgThreshold <= 0 {
		return nil, errors.New("need positive int value for config key 'avg-threshold'")
	}

	if t.avgDelay = t.Config.GetInt64("avg-delay") * 60; t.avgDelay <= 0 {
		return nil, errors.New("need positive int value for config key 'avg-delay'")
	}

	return t, nil
}

func (t acTarget) worker() {
	log.Println("target worker started: ", t.Name)
	t.isAlive = true
	ticker := time.NewTicker(time.Second * time.Duration(t.checkDelay))
	defer ticker.Stop()
	processACStatus(t)
DONE:
	for {
		select {
		case _, ok := <-t.Kill:
			if !ok {
				log.Println(t.Name, "target worker stopping ...")
				break DONE
			}
		case <-ticker.C:
			processACStatus(t)
		}
		log.Println(t.Name, fmt.Sprintf("target worker sleep %d seconds ...", t.checkDelay))
	}
	log.Println(t.Name, "target worker stopped.")
	t.wg.Done()
}

func processACStatus(t acTarget) {
	log.Println("query target status:", t.Name)
	resp, err := t.Get(t.api)
	if err != nil {
		log.Println(t.Name, "Get api data error: ", err)
		return
	}
	status := &acStatus{}
	if err = json.Unmarshal(resp, status); err != nil {
		log.Println(t.Name, "unmarshal status data error: ", err)
		return
	}
	if err = status.FormatData(); err != nil {
		log.Println(t.Name, err)
		return
	}
	fmtstr := "%s，%s。 实时温度：%f，平均温度：%f"
	if status.AvgTemp >= t.avgThreshold || status.RealTemp >= t.realThreshold {
		if cache.GetCache().IsExpired("ac-temp-reporter", t.avgDelay) {
			msg := fmt.Sprintf(fmtstr, t.memo, "温度超过报警阈值", status.RealTemp, status.AvgTemp)
			t.M <- BotMessage{
				TargetName: t.Name,
				Message: bot.NewMessage(bot.TextMessage,
					bot.WithText(msg),
					bot.WithAtMobiles([]string{t.receiver})),
			}
		}
	}
}

type acStatus struct {
	Real     string `json:"real"`
	Avg      string `json:"avg"`
	Alive    string `json:"alive"`
	RealTemp float64
	AvgTemp  float64
	IsAlive  bool
}

func (s *acStatus) FormatData() error {
	var err error
	if s.RealTemp, err = strconv.ParseFloat(s.Real, 32); err != nil {
		return errors.New("format data error: " + err.Error())
	}
	if s.AvgTemp, err = strconv.ParseFloat(s.Avg, 32); err != nil {
		return errors.New("format data error: " + err.Error())
	}
	if s.IsAlive, err = strconv.ParseBool(s.Alive); err != nil {
		return errors.New("format data error: " + err.Error())
	}
	return nil
}

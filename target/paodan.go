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

type paodanTarget struct {
	baseTarget
	api               string
	checkDelay        int
	isAlive           bool
	notAliveDelay     int64
	queueLenThreshold int
	queueDelay        int64
	receiver          string
	memo              string
}

func (t paodanTarget) Start() {
	t.wg.Add(1)
	go t.worker()
}

func (t paodanTarget) SetConfig(cfg *viper.Viper, ch chan BotMessage, stop chan struct{}, wg *sync.WaitGroup) (Target, error) {
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
	if t.notAliveDelay = t.Config.GetInt64("not-alive-delay") * 60; t.notAliveDelay <= 0 {
		return nil, errors.New("need positive int value for config key 'not-alive-delay'")
	}

	if t.queueLenThreshold = t.Config.GetInt("queue-len-threshold"); t.queueLenThreshold <= 0 {
		return nil, errors.New("need positive int value for config key 'queue-len-threshold'")
	}

	if t.queueDelay = t.Config.GetInt64("queue-len-delay") * 60; t.queueDelay <= 0 {
		return nil, errors.New("need positive int value for config key 'queue-len-delay'")
	}

	if t.receiver = t.Config.GetString("receiver"); len(t.receiver) <= 0 {
		return nil, errors.New("need string value for config key 'receiver'")
	}

	return t, nil
}

func (t paodanTarget) worker() {
	log.Println("target worker started: ", t.Name)
	t.isAlive = true
	ticker := time.NewTicker(time.Second * time.Duration(t.checkDelay))
	defer ticker.Stop()
	processPaodanStatus(&t)
DONE:
	for {
		select {
		case _, ok := <-t.Kill:
			if !ok {
				log.Println(t.Name, "target worker stopping ...")
				break DONE
			}
		case <-ticker.C:
			processPaodanStatus(&t)
		}
		log.Println(t.Name, fmt.Sprintf("target worker sleep %d seconds ...", t.checkDelay))
	}
	log.Println(t.Name, "target worker stopped.")
	t.wg.Done()
}

func processPaodanStatus(t *paodanTarget) {
	log.Println("query target status:", t.Name)
	resp, err := t.Get(t.api)
	if err != nil {
		log.Println(t.Name, "Get api data error: ", err)
		return
	}
	status := &paodanStatus{}
	if err = json.Unmarshal(resp, status); err != nil {
		log.Println(t.Name, "Unmarshal status data error: ", err)
		return
	}
	if err = status.FormatData(); err != nil {
		log.Println("format pandan status data error: ", err)
		return
	}
	if status.IsAlive {
		// 状态变为正常
		if !t.isAlive {
			t.isAlive = true
			x := fmt.Sprintf("%s，恢复正常。待同步订单: %d", t.memo, status.QueueLen)
			// log.Println(x)
			t.M <- BotMessage{
				TargetName: t.Name,
				Message: bot.NewMessage(bot.TextMessage,
					bot.WithText(x),
					bot.WithAtMobiles([]string{t.receiver})),
			}
			return
		}
	} else {
		// 状态变为异常
		msg := fmt.Sprintf("%s，停止工作。待同步订单: %d", t.memo, status.QueueLen)
		if t.isAlive {
			t.isAlive = false
			// log.Println("status-fail-first", msg)
			t.M <- BotMessage{
				TargetName: t.Name,
				Message: bot.NewMessage(bot.TextMessage,
					bot.WithText(msg),
					bot.WithAtMobiles([]string{t.receiver})),
			}
		} else {
			// 状态持续异常
			if cache.GetCache().IsExpired("paodan-status-fail", t.notAliveDelay) {
				// log.Println("status-fail-lasts", msg)
				t.M <- BotMessage{
					TargetName: t.Name,
					Message: bot.NewMessage(bot.TextMessage,
						bot.WithText(msg),
						bot.WithAtMobiles([]string{t.receiver})),
				}
			}
		}
		return
	}
	if status.QueueLen >= t.queueLenThreshold {
		if cache.GetCache().IsExpired("paodan-status-queue-len-report", t.queueDelay) {
			t.M <- BotMessage{
				TargetName: t.Name,
				Message: bot.NewMessage(bot.TextMessage,
					bot.WithText(fmt.Sprintf("%s，同步队列长度超过报警阈值。待同步订单：%d", t.memo, status.QueueLen)),
					bot.WithAtMobiles([]string{t.receiver})),
			}
		}
	}
}

type paodanStatus struct {
	Paodan   string `json:"paodan"`
	Qlen     string `json:"qlen"`
	IsAlive  bool
	QueueLen int
}

func (d *paodanStatus) FormatData() error {
	switch d.Paodan {
	case "0":
		d.IsAlive = false
	case "1":
		d.IsAlive = true
	default:
		return errors.New("unexpected data in paodanStatus.paodan")
	}
	var err error
	d.QueueLen, err = strconv.Atoi(d.Qlen)
	if err != nil {
		return errors.New("unexpected data in paodanStatus.qlen," + err.Error())
	}

	return nil
}

package target

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"notifier/bot"
	"notifier/cache"
	"sync"
	"time"

	"github.com/spf13/viper"
)

type baddocTarget struct {
	baseTarget
	api                  string
	checkDelay           int
	rmaDocMemo           string
	rmaDocDelay          int64
	rmaDocReceiver       string
	unprovedDocTTL       int64
	unprovedDocThreshold int
	unprovedDocReceiver  string
}

func (t baddocTarget) Start() {
	t.wg.Add(1)
	go t.worker()
}

func (t baddocTarget) SetConfig(cfg *viper.Viper, ch chan BotMessage, stop chan struct{}, wg *sync.WaitGroup) (Target, error) {
	t.Config = cfg
	t.client = &http.Client{}
	t.wg = wg

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
	t.rmaDocMemo = t.Config.GetString("rma-doc-memo")

	if t.rmaDocDelay = int64(t.Config.GetInt("rma-doc-delay") * 60); t.rmaDocDelay <= 0 {
		return nil, errors.New("need positive int value for target key 'rma-doc-delay'")
	}

	if t.rmaDocReceiver = t.Config.GetString(("rma-doc-receiver")); len(t.rmaDocReceiver) == 0 {
		return nil, errors.New("need string value for config key 'rma-doc-receiver'")
	}

	if t.unprovedDocTTL = int64(t.Config.GetInt("unproved-doc-ttl") * 60); t.unprovedDocTTL <= 0 {
		return nil, errors.New("need positive int value for target key 'unproved-doc-ttl'")
	}

	if t.unprovedDocThreshold = t.Config.GetInt("unproved-doc-threshold"); t.unprovedDocThreshold <= 0 {
		return nil, errors.New("need positive int value for target key 'unproved-doc-threshold'")
	}

	if t.unprovedDocReceiver = t.Config.GetString("unproved-doc-receiver"); len(t.unprovedDocReceiver) == 0 {
		return nil, errors.New("need string value for target key 'unproved-doc-receiver'")
	}

	return t, nil
}

func (t baddocTarget) worker() {
	log.Println("target worker started: ", t.Name)
	ticker := time.NewTicker(time.Second * time.Duration(t.checkDelay))
	defer ticker.Stop()
	processBadDocStatus(t)
DONE:
	for {
		select {
		case _, ok := <-t.Kill:
			if !ok {
				log.Println(t.Name, "target worker stopping ...")
				break DONE
			}
		case <-ticker.C:
			processBadDocStatus(t)
		}
		log.Println(t.Name, fmt.Sprintf("target worker sleep %d seconds ...", t.checkDelay))
	}
	log.Println(t.Name, "target worker stopped.")
	t.wg.Done()
}

type baddoc struct {
	Name     string `json:"name"`
	Value    int    `json:"value"`
	DrillKey string `json:"drillkey"`
}
type baddocList []baddoc

func processBadDocStatus(t baddocTarget) {
	log.Println("query target status:", t.Name)
	querystr := "?key="
	data, err := t.Get(t.api + querystr + "dashboard:baddoc")
	if err != nil {
		log.Println("baddoc get error: ", err)
		return
	}
	baddoc := &baddocList{}
	if err = json.Unmarshal(data, baddoc); err != nil {
		log.Println("baddoc: Unmarshal data error, ", err)
		return
	}

	needToNotify := false

	baddocText := ""
	for _, row := range *baddoc {
		if row.Value > 0 {
			switch row.Name {
			case "退回处理":
				m, err := processRMA(t, querystr, row)
				if err != nil {
					log.Println(err)
				}
				if m != nil {
					t.M <- *m
				}
			case "未审核单":
				if row.Value < t.unprovedDocThreshold {
					break
				}
				m, err := processUnproved(row, t, querystr)
				if err != nil {
					log.Println(err)
				}
				if m != nil {
					t.M <- *m
				}
			default:
				baddocText = baddocSummary(baddocText, row)
			}
		}
	}
	if needToNotify {
		if cache.GetCache().IsExpired("BadDocSummary", t.unprovedDocTTL) {
			t.M <- BotMessage{
				TargetName: t.Name,
				Message: bot.NewMessage(bot.TextMessage,
					bot.WithText(baddocText),
					bot.WithAtMobiles([]string{"18384264312"}))}
		}
	}
}

func processUnproved(row baddoc, t baddocTarget, querystr string) (*BotMessage, error) {

	detail, err := t.Get(t.api + querystr + row.DrillKey)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s %s %s %s", t.Name, row.Name, "Get unproved detail data error: ", err))
	}
	detailData := &DetailData{}
	if err = json.Unmarshal(detail, detailData); err != nil {
		return nil, errors.New(fmt.Sprintf("%s %s %s %s", t.Name, row.Name, "Unmarshal unproved detail data error: ", err))
	}
	msg := fmt.Sprintf("%s合计: %d", row.Name, row.Value)
	for _, row := range detailData.Data {
		msg = msg + "，"
		msg = msg + fmt.Sprintf("%s: %s", row[0], row[1])
	}

	if cache.GetCache().IsExpired("BadDocSummary-unproved-doc", t.unprovedDocTTL) {
		return &BotMessage{
			TargetName: t.Name,
			Message: bot.NewMessage(bot.TextMessage,
				bot.WithText(msg),
				bot.WithAtMobiles([]string{t.unprovedDocReceiver})),
		}, nil
	}
	return nil, nil
}

func baddocSummary(baddocText string, row baddoc) string {
	if len(baddocText) > 0 {
		baddocText = baddocText + "，"
	}
	baddocText = baddocText + fmt.Sprintf("%s:%d", row.Name, row.Value)
	return baddocText
}

func processRMA(t baddocTarget, querystr string, row baddoc) (*BotMessage, error) {
	resp, err := t.Get(t.api + querystr + row.DrillKey)
	if err == nil {
		rmadata := &DetailData{}
		if err = json.Unmarshal(resp, rmadata); err != nil {
			return nil, errors.New(fmt.Sprintln("Unmarshal RMA data error: ", err))
		}
		docnos := rmadata.GetDocNo(t.rmaDocDelay)
		msg := ""
		if len(docnos) > 0 {
			for _, v := range docnos {
				if len(msg) > 0 {
					msg = msg + ","
				}
				msg = msg + v
			}
			msg = msg + " " + t.rmaDocMemo

			return &BotMessage{
				TargetName: t.Name,
				Message: bot.NewMessage(bot.TextMessage,
					bot.WithText(msg),
					bot.WithAtMobiles([]string{t.rmaDocReceiver}))}, nil
		}
		return nil, nil
	} else {
		return nil, errors.New(fmt.Sprintln("Get RMA data error: ", err))
	}
}

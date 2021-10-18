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
	memo                 string
	rmaDocDelay          int64
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

	if ch == nil {
		return nil, errors.New(t.Name + ", message channel is nil")
	}
	t.M = ch
	if stop == nil {
		return nil, errors.New(t.Name + ", stop channel is nil")
	}
	t.Kill = stop

	t.api = t.Config.GetString("api")
	if len(t.api) == 0 {
		return nil, errors.New("need string value for config key 'api'")
	}

	t.memo = t.Config.GetString("memo")

	t.rmaDocDelay = int64(t.Config.GetInt("rma-doc-delay") * 60)
	if t.rmaDocDelay <= 0 {
		return nil, errors.New("need positive int value for target key 'rma-doc-delay'")
	}

	t.unprovedDocTTL = int64(t.Config.GetInt("unproved-doc-ttl") * 60)
	if t.unprovedDocTTL <= 0 {
		return nil, errors.New("need positive int value for target key 'unproved-doc-ttl'")
	}

	t.unprovedDocThreshold = t.Config.GetInt("unproved-doc-threshold")
	if t.unprovedDocThreshold <= 0 {
		return nil, errors.New("need positive int value for target key 'unproved-doc-threshold'")
	}

	t.unprovedDocReceiver = t.Config.GetString("unproved-doc-receiver")
	if len(t.unprovedDocReceiver) == 0 {
		return nil, errors.New("need string value for target key 'unproved-doc-receiver'")
	}

	return t, nil
}

func (t baddocTarget) worker() {
	log.Println("target worker started: ", t.Name)
DONE:
	for {
		select {
		case _, ok := <-t.Kill:
			if !ok {
				log.Println(t.Name, "worker stopping ...")
				break DONE
			}
		default:
			processTargetStatus(t)
		}
		log.Println(t.Name, "worker sleep 15 seconds ...")
		time.Sleep(time.Second * 15)
	}
	log.Println(t.Name, "worker stopped.")
	t.wg.Done()
}

type baddoc struct {
	Name     string `json:"name"`
	Value    int    `json:"value"`
	DrillKey string `json:"drillkey"`
}
type baddocList []baddoc

func processTargetStatus(t baddocTarget) {
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
					m, err := processUnproved(row, t, querystr)
					if err != nil {
						log.Println(err)
					}
					if m != nil {
						t.M <- *m
					}
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
		return nil, errors.New(fmt.Sprintf("%s %s %s %s", t.Name, row.Name, "Get detail data error: ", err))
	}
	detailData := &DetailData{}
	if err = json.Unmarshal(detail, detailData); err != nil {
		return nil, errors.New(fmt.Sprintf("%s %s %s %s", t.Name, row.Name, "Unmarshal detail data error: ", err))
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
			msg = msg + " " + t.memo

			return &BotMessage{
				TargetName: t.Name,
				Message: bot.NewMessage(bot.TextMessage,
					bot.WithText(msg),
					bot.WithAtMobiles([]string{"18384264312"}))}, nil
		}
		return nil, nil
	} else {
		return nil, errors.New(fmt.Sprintln("Get RMA data error: ", err))
	}
}

type DataColumnHeader struct {
	Name  string `json:"name"`
	Width int    `json:"width"`
}
type DetailData struct {
	ColNames []DataColumnHeader `json:"colNames"`
	Data     [][]string         `json:"data"`
}

func (d *DetailData) GetDocNo(ttl int64) []string {
	data := []string{}

	cache := cache.GetCache()
	d1 := make(map[string]bool)
	for _, v := range d.Data {
		docno := v[0]
		if _, ok := d1[docno]; !ok {
			d1[docno] = true
			if cache.IsExpired(docno, ttl) {
				data = append(data, docno)
			}
		}
	}

	return data
}

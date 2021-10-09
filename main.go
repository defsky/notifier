package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/spf13/viper"
)

var config *viper.Viper

const configFileName string = "config"
const configFileType string = "yaml"

func main() {
	log.Println("load config " + configFileName + "." + configFileType + " ...")
	config = viper.New()
	config.SetConfigName(configFileName)
	config.SetConfigType(configFileType)
	config.AddConfigPath("./")
	if err := config.ReadInConfig(); err != nil {
		log.Fatalln(err)
	}
	redisAddress := config.GetString("redis-server")
	redisKey := config.GetString("redis-key")
	cqurl := config.GetString("CQ-URL")
	recvid := config.GetString("receiver-id")

	log.Println("CQ URL: " + cqurl)
	log.Println("Receiver QQ: " + recvid)
	log.Println("Redis Address: " + redisAddress)
	log.Println("Redis Key: " + redisKey)

	conn, err := redis.Dial("tcp", redisAddress)
	if err != nil {
		log.Fatalln("redis dial err =", err)
	}
	defer conn.Close() // 关闭redis数据库
	log.Println("Redis server connected.")

	cache := GetCache()
	defer cache.Destroy()

	// 2.通过go向redis中写入数据 string[key-val]
	// _, err = conn.Do("Set", "name", "tom and jerry")
	// if err!=nil{
	// 	log.Println("set err =", err)
	// 	return
	// }

	// 通过go向redis中读取数据 string[key-val]

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

DONE:
	for {
		select {
		case <-c:
			log.Println("Interrupt signal detected")
			break DONE
		case <-ticker.C:
			r, err := redis.Bytes(conn.Do("Get", redisKey))
			if err != nil {
				log.Println("redis Get error = ", err)
				break
			}
			data := &RMAData{}
			err = json.Unmarshal(r, data)
			if err != nil {
				log.Println("Unmarshal data error: ", err)
				break
			}

			msg := ""
			needNotifyDocNo := data.GetDocNo()
			if len(needNotifyDocNo) <= 0 {
				log.Println("no data need to notify")
				break
			}
			for _, v := range needNotifyDocNo {
				if len(msg) > 0 {
					msg = msg + ","
				}
				msg = msg + v
			}
			msg = msg + " 霞姐 估价金额"

			escapedMsg := url.QueryEscape(msg)
			reqURL := cqurl + "/send_private_msg?user_id=" + recvid + "&message=" + escapedMsg

			client := &http.Client{}
			log.Println("Get: ", reqURL)
			resp, err := client.Get(reqURL)
			if err != nil {
				log.Println("send message error: ", err.Error())
				break
			}
			respData, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				log.Println("Read response data error: ", err.Error())
				break
			}
			log.Println("Response: ", string(respData))
		}
		log.Println("Sleep 15s ...")
	}
}

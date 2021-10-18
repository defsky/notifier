package main

import (
	"log"
	"notifier/bot"
	"notifier/db"
	"notifier/target"
	"os"
	"os/signal"
	"sync"

	"github.com/spf13/viper"
)

var config *viper.Viper

const configFileName string = "config"
const configFileType string = "yaml"

var messageChannel chan target.BotMessage
var stopSignalChannel chan struct{}

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

	db.InitRedis(redisAddress)
	log.Println("Redis server connected.")
	defer db.RedisConn.Close()

	messageChannel = make(chan target.BotMessage, 10)
	stopSignalChannel = make(chan struct{})

	wg := &sync.WaitGroup{}
	wg1 := &sync.WaitGroup{}
	// message sender routine
	wg1.Add(1)
	go func() {
		dingBot := bot.NewBot(config.GetString("dingding-webhook"))
		for m := range messageChannel {
			errcode := dingBot.SendMessage(m.Message)
			if errcode != 0 {
				log.Printf("SendMessage error: target(%s) errcode(%d)\n", m.TargetName, errcode)
			}
		}
		for _, ok := <-messageChannel; ok; {
		}
		log.Println("Message sender stopped.")
		wg1.Done()
	}()

	targets := config.GetStringMap("targets")

	for k, _ := range targets {
		targetConfig := config.Sub("targets." + k)
		typeName := targetConfig.GetString("type")
		if len(typeName) == 0 {
			log.Println(k, "need non-null string for config key 'type'")
			continue
		}
		if t := target.NewTarget(typeName); t != nil {
			if tt, err := t.SetConfig(targetConfig, messageChannel, stopSignalChannel, wg); err != nil {
				log.Println("start target worker failed: ", err)
			} else {
				tt.Start()
			}
		} else {
			log.Println("Undefined target type name: ", typeName)
		}
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)

	// wait system interrupt, ctrl+c, kill
	<-c
	log.Println("Stopping service ...")

	// close target workers
	close(stopSignalChannel)
	wg.Wait()
	log.Println("All workers stopped.")

	// close message sender
	close(messageChannel)
	wg1.Wait()

	log.Println("Main routine stopped.")
}

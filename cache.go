package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

const ExpireHours float32 = 8
const CleanDelaySeconds float32 = 10
const CacheFileName string = "cache.db"

type Cache interface {
	Destroy()
	IsExpired(string) bool
	Persist()
}

var cacheInstance Cache

func GetCache() Cache {
	if cacheInstance != nil {
		return cacheInstance
	}

	c := &cache{
		chDesdroy: make(chan struct{}),
		Data:      make(map[string]int64),
	}
	cacheInstance = c

	var err1 error
	if checkFileIsExist(CacheFileName) { //如果文件存在
		c.cachedb, err1 = os.OpenFile(CacheFileName, os.O_RDONLY, 0666) //打开文件
		if err1 != nil {
			log.Fatalln(err1)
		}
		data, err := ioutil.ReadAll(c.cachedb)
		if err != nil {
			log.Fatalln(err)
		}
		c.cachedb.Close()

		c.lock.Lock()
		err = json.Unmarshal(data, &c.Data)
		c.lock.Unlock()

		if err != nil {
			log.Fatalln(err)
		}
	} else {
		c.Persist()
	}

	go c.cleaner()

	log.Println("cachedb initiated.")
	return c
}

type cache struct {
	lock      sync.Mutex
	cachedb   *os.File
	chDesdroy chan struct{}
	Data      map[string]int64 `json:"data"`
}

func (c *cache) Persist() {
	c.lock.Lock()
	data, err := json.Marshal(c.Data)
	c.lock.Unlock()

	if err != nil {
		log.Fatalln(err)
	}
	err = ioutil.WriteFile(CacheFileName, data, 0666)
	if err != nil {
		log.Fatalln(err)
	}
}

func (c *cache) Destroy() {
	c.chDesdroy <- struct{}{}
	close(c.chDesdroy)
}
func (c *cache) IsExpired(s string) bool {
	c.lock.Lock()
	v, ok := c.Data[s]
	c.lock.Unlock()

	if ok {
		ktime := time.Unix(v, 0)
		now := time.Now()
		exitsTime := now.Sub(ktime).Seconds()

		if exitsTime >= float64(ExpireHours*60*60) {
			c.lock.Lock()
			c.Data[s] = now.Unix()
			c.lock.Unlock()

			c.Persist()

			return true
		} else {
			return false
		}
	}

	c.lock.Lock()
	c.Data[s] = time.Now().Unix()
	c.lock.Unlock()

	return true
}

func (c *cache) cleaner() {
	timeCounter := time.NewTicker(time.Second * 10)

DONE:
	for {
		select {
		case _, ok := <-c.chDesdroy:
			if ok {
				break DONE
			}
		case <-timeCounter.C:
			c.lock.Lock()
			for k, v := range c.Data {
				ktime := time.Unix(v, 0)
				existsTime := time.Now().Sub(ktime).Seconds()
				if existsTime >= float64(ExpireHours*60*60+CleanDelaySeconds) {
					delete(c.Data, k)
				}
			}
			c.lock.Unlock()

			c.Persist()
		}
	}
	timeCounter.Stop()
}

func checkFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

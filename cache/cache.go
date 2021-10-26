package cache

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
	// destroy the cache instance
	Destroy()

	// check if the 'key' in cache is expired for 'ttl'
	IsExpired(key string, ttl int64) bool

	// persist cache data into file
	Persist()
}

var cacheInstance Cache
var lck sync.Mutex

type TTL struct {
	CreateTime int64 `json:"createtime"`
	TimeToLive int64 `json:"ttl"`
}
type cache struct {
	lock      sync.Mutex
	cachedb   *os.File
	chDesdroy chan struct{}
	Data      map[string]TTL `json:"data"`
}

func (c *cache) Persist() {
	c.lock.Lock()
	defer c.lock.Unlock()
	data, err := json.Marshal(c.Data)

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

func (c *cache) IsExpired(s string, ttl int64) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.Data[s]; ok {
		return false
	}

	c.Data[s] = TTL{
		CreateTime: time.Now().Unix(),
		TimeToLive: ttl,
	}

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
			cleanCacheData(c)

			c.Persist()
		}
	}
	timeCounter.Stop()
}

func cleanCacheData(c *cache) {
	c.lock.Lock()
	for k, v := range c.Data {
		ktime := time.Unix(v.CreateTime, 0)
		existsTime := time.Now().Sub(ktime).Seconds()
		if existsTime >= float64(v.TimeToLive) {
			delete(c.Data, k)
		}
	}
	c.lock.Unlock()
}

func GetCache() Cache {
	lck.Lock()
	defer lck.Unlock()

	if cacheInstance != nil {
		return cacheInstance
	}

	c := &cache{
		lock:      sync.Mutex{},
		cachedb:   &os.File{},
		chDesdroy: make(chan struct{}),
		Data:      make(map[string]TTL),
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

func checkFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

func Init() {
	lck = sync.Mutex{}
	c := GetCache()
	cleanCacheData(c.(*cache))
}

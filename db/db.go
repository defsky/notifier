package db

import (
	"log"

	"github.com/gomodule/redigo/redis"
)

var RedisConn redis.Conn

func InitRedis(address string) {
	conn, err := redis.Dial("tcp", address)
	if err != nil {
		log.Fatalln("redis dial err =", err)
	}
	RedisConn = conn
}

// 2.通过go向redis中写入数据 string[key-val]
// _, err = conn.Do("Set", "name", "tom and jerry")
// if err!=nil{
// 	log.Println("set err =", err)
// 	return
// }

// 通过go向redis中读取数据 string[key-val]

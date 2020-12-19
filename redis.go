package main

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
)

func (ks *KiteServer) connectRedis() {
	ks.rdb = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", ks.conf.RedisServer,"6379"),
		Password: ks.conf.RedisPassword,
		DB: 0,
	})

}

func (ks *KiteServer) logToRedis(message string) {
	if err := ks.rdb.Set(ks.ctx, "hello", message, 0).Err(); err != nil {
		log.Printf("Error logging message to redis --> %s", err)
	}
}

func (ks *KiteServer) getLogFromRedis() ([]string, error) {
	return nil,nil
}



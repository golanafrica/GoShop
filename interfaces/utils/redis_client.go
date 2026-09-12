package utils

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

var Rdb *redis.Client

// InitRedis initialise le client Redis à partir de REDIS_HOST/REDIS_PORT
// (les mêmes variables que celles utilisées par config.Config), avec un
// fallback sur REDIS_ADDR pour compatibilité, puis sur localhost:6379.
func InitRedis() error {
	addr := os.Getenv("REDIS_ADDR")

	if addr == "" {
		host := os.Getenv("REDIS_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("REDIS_PORT")
		if port == "" {
			port = "6379"
		}
		addr = fmt.Sprintf("%s:%s", host, port)
	}

	Rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	_, err := Rdb.Ping(context.Background()).Result()
	return err
}

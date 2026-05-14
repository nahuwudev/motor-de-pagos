package db

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisConnectionPool(addr string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,

		PoolSize:     100,
		MinIdleConns: 10,
		PoolTimeout:  30 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return rdb, nil
}

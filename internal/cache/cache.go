package cache

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
)

// RedisClient global redisClient
var RedisClient *redis.Client
var ctx = context.Background()

// InitRedis init redis database
func InitRedis(addr, pass string, db int) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       db,
	})
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Fail to connect to redis")
	}
	RedisClient = client
}

// GetKeyOrSetCache get key or set cache
func GetKeyOrSetCache(key string, cb func() (string, error)) (string, error) {
	data, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		freshData, err := cb()
		if err != nil {
			return "", err
		}
		SetCache(key, freshData)
		return freshData, nil
	}
	if err != nil {
		return "", err
	}
	return data, nil
}

// SetCache update cache
func SetCache(key, data string) error {
	return RedisClient.Set(ctx, key, data, 0).Err()
}

// GetCache get cache
func GetCache(key string) (string, error) {
	return RedisClient.Get(ctx, key).Result()
}

// DeleteCache delete cache
func DeleteCache(key string) error {
	_, err := RedisClient.Del(ctx, key).Result()
	return err
}

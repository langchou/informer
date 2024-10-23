package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	mylog "github.com/langchou/informer/pkg/log"
)

var redisClient *redis.Client

func InitRedis(addr string, password string, db int) error {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}

	mylog.Info("Successfully connected to Redis")
	return nil
}

func GetClient() *redis.Client {
	return redisClient
}

func SetProxies(proxies []string, expiration time.Duration) error {
	ctx := context.Background()
	for _, proxy := range proxies {
		err := redisClient.Set(ctx, "proxy:"+proxy, "1", expiration).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func GetProxies() ([]string, error) {
	ctx := context.Background()
	keys, err := redisClient.Keys(ctx, "proxy:*").Result()
	if err != nil {
		return nil, err
	}

	var proxies []string
	for _, key := range keys {
		proxies = append(proxies, key[6:]) // Remove "proxy:" prefix
	}
	return proxies, nil
}

func RemoveProxy(proxy string) error {
	ctx := context.Background()
	return redisClient.Del(ctx, "proxy:"+proxy).Err()
}

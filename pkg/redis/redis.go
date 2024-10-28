package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	mylog "github.com/langchou/informer/pkg/log"
)

const (
	ProxySetKey = "proxy:list" // 使用普通set存储代理
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

// ReplaceProxies 替换所有代理
func ReplaceProxies(proxies []string) error {
	ctx := context.Background()

	// 使用管道执行命令
	pipe := redisClient.Pipeline()

	// 删除旧的代理集合
	pipe.Del(ctx, ProxySetKey)

	// 如果有新代理，则添加
	if len(proxies) > 0 {
		pipe.SAdd(ctx, ProxySetKey, convertToInterface(proxies)...)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetRandomProxy 随机获取一个代理
func GetRandomProxy() (string, error) {
	ctx := context.Background()
	return redisClient.SRandMember(ctx, ProxySetKey).Result()
}

// GetProxyCount 获取当前代理数量
func GetProxyCount() (int64, error) {
	ctx := context.Background()
	return redisClient.SCard(ctx, ProxySetKey).Result()
}

// 辅助函数：将字符串切片转换为空接口切片
func convertToInterface(strs []string) []interface{} {
	interfaces := make([]interface{}, len(strs))
	for i, s := range strs {
		interfaces[i] = s
	}
	return interfaces
}

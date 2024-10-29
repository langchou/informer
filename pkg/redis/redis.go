package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	mylog "github.com/langchou/informer/pkg/log"
)

const (
	ProxySetKey     = "proxy:list"      // 使用普通set存储代理
	PreferredSetKey = "proxy:preferred" // 存储优选代理的有序集合
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

	// 初始化时清理优选代理 key
	if err := ClearPreferredProxies(); err != nil {
		mylog.Error(fmt.Sprintf("清理优选代理失败: %v", err))
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

// RemoveProxy 从代理池中删除指定代理
func RemoveProxy(proxy string) error {
	ctx := context.Background()
	return redisClient.SRem(ctx, ProxySetKey, proxy).Err()
}

// 辅助函数：将字符串切片转换为空接口切片
func convertToInterface(strs []string) []interface{} {
	interfaces := make([]interface{}, len(strs))
	for i, s := range strs {
		interfaces[i] = s
	}
	return interfaces
}

// AddPreferredProxy 添加到优选代理列表，使用响应时间作为分数
func AddPreferredProxy(proxy string, responseTime float64) error {
	ctx := context.Background()

	// 首先检查 key 的类型
	keyType, err := redisClient.Type(ctx, PreferredSetKey).Result()
	if err != nil {
		return fmt.Errorf("检查key类型失败: %v", err)
	}

	// 如果 key 存在但类型不是 zset，则删除它
	if keyType != "zset" && keyType != "none" {
		if err := redisClient.Del(ctx, PreferredSetKey).Err(); err != nil {
			return fmt.Errorf("删除旧key失败: %v", err)
		}
	}

	// 使用 ZADD 命令添加到有序集合，分数为响应时间（越小越好）
	err = redisClient.ZAdd(ctx, PreferredSetKey, &redis.Z{
		Score:  responseTime,
		Member: proxy,
	}).Err()

	if err == nil {
		// 使用Debug级别记录日志，避免刷屏
		mylog.Debug(fmt.Sprintf("添加新的优选代理: %s, 响应时间: %.2fms", proxy, responseTime))
	}

	return err
}

// GetPreferredProxy 获取响应时间最快的优选代理
func GetPreferredProxy() (string, error) {
	ctx := context.Background()
	// 使用 ZRANGE 获取分数最低（响应最快）的代理
	result, err := redisClient.ZRange(ctx, PreferredSetKey, 0, 0).Result()
	if err != nil {
		return "", err
	}
	if len(result) == 0 {
		return "", redis.Nil
	}
	return result[0], nil
}

// GetPreferredProxies 获取所有优选代理，按响应时间排序
func GetPreferredProxies() ([]string, error) {
	ctx := context.Background()
	return redisClient.ZRange(ctx, PreferredSetKey, 0, -1).Result()
}

// GetPreferredProxyCount 获取优选代理数量
func GetPreferredProxyCount() (int64, error) {
	ctx := context.Background()
	return redisClient.ZCard(ctx, PreferredSetKey).Result()
}

// RemovePreferredProxy 从优选代理列表中删除
func RemovePreferredProxy(proxy string) error {
	ctx := context.Background()
	return redisClient.ZRem(ctx, PreferredSetKey, proxy).Err()
}

// ClearPreferredProxies 清空优选代理列表
func ClearPreferredProxies() error {
	ctx := context.Background()
	return redisClient.Del(ctx, PreferredSetKey).Err()
}

// GetAllProxies 获取所有代理
func GetAllProxies() ([]string, error) {
	ctx := context.Background()
	return redisClient.SMembers(ctx, ProxySetKey).Result()
}

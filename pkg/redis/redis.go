package redis

import (
	"context"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	mylog "github.com/langchou/informer/pkg/log"
)

const (
	ProxyScoreKey  = "proxy:scores" // Redis sorted set key for proxy scores
	ProxyMinScore  = 0.0            // 最低分数
	ProxyMaxScore  = 100.0          // 最高分数
	ProxyInitScore = 50.0           // 初始分数
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

// SetProxy 添加代理并设置初始分数
func SetProxy(proxy string) error {
	ctx := context.Background()
	return redisClient.ZAdd(ctx, ProxyScoreKey, &redis.Z{
		Score:  ProxyInitScore,
		Member: proxy,
	}).Err()
}

// SetProxies 批量添加代理
func SetProxies(proxies []string) error {
	ctx := context.Background()
	var zs []*redis.Z
	for _, proxy := range proxies {
		zs = append(zs, &redis.Z{
			Score:  ProxyInitScore,
			Member: proxy,
		})
	}
	return redisClient.ZAdd(ctx, ProxyScoreKey, zs...).Err()
}

// GetProxies 获取所有代理，按分数从高到低排序
func GetProxies() ([]string, error) {
	ctx := context.Background()
	result, err := redisClient.ZRevRangeByScore(ctx, ProxyScoreKey, &redis.ZRangeBy{
		Min: strconv.FormatFloat(ProxyMinScore, 'f', 1, 64),
		Max: strconv.FormatFloat(ProxyMaxScore, 'f', 1, 64),
	}).Result()
	return result, err
}

// GetTopProxy 获取得分最高的代理
func GetTopProxy() (string, error) {
	ctx := context.Background()
	result, err := redisClient.ZRevRangeWithScores(ctx, ProxyScoreKey, 0, 0).Result()
	if err != nil {
		return "", err
	}
	if len(result) == 0 {
		return "", nil
	}
	return result[0].Member.(string), nil
}

// UpdateProxyScore 更新代理分数
func UpdateProxyScore(proxy string, delta float64) error {
	ctx := context.Background()

	// 获取当前分数
	score, err := redisClient.ZScore(ctx, ProxyScoreKey, proxy).Result()
	if err != nil {
		return err
	}

	// 计算新分数
	newScore := score + delta
	if newScore < ProxyMinScore {
		// 如果分数低于最低分，删除该代理
		return redisClient.ZRem(ctx, ProxyScoreKey, proxy).Err()
	}
	if newScore > ProxyMaxScore {
		newScore = ProxyMaxScore
	}

	// 更新分数
	return redisClient.ZAdd(ctx, ProxyScoreKey, &redis.Z{
		Score:  newScore,
		Member: proxy,
	}).Err()
}

// GetProxyCount 获取当前代理池中的代理数量
func GetProxyCount() (int64, error) {
	ctx := context.Background()
	return redisClient.ZCard(ctx, ProxyScoreKey).Result()
}

// RemoveLowScoreProxies 删除低分代理
func RemoveLowScoreProxies(threshold float64) error {
	ctx := context.Background()
	return redisClient.ZRemRangeByScore(ctx, ProxyScoreKey,
		strconv.FormatFloat(ProxyMinScore, 'f', 1, 64),
		strconv.FormatFloat(threshold, 'f', 1, 64)).Err()
}

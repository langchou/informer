package proxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/langchou/informer/pkg/checker"
	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/redis"
)

const (
	proxyExpiration = 30 * time.Minute
	updateInterval  = 5 * time.Minute
	lastUpdateKey   = "proxy:last_update"
	checkInterval   = 1 * time.Minute
	maxConcurrent   = 10 // 最大并发数
)

var ProxyAPI string

// SetProxyAPI 设置代理API URL
func SetProxyAPI(url string) {
	ProxyAPI = url
	go periodicCheck()
}

// periodicCheck 定期检查代理IP的有效性
func periodicCheck() {
	ticker := time.NewTicker(checkInterval)
	for range ticker.C {
		checkAllProxies()
	}
}

// checkAllProxies 并行检查所有代理IP的有效性
func checkAllProxies() {
	proxies, err := redis.GetProxies()
	if err != nil {
		mylog.Error("获取代理列表失败: %v", err)
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrent)

	for _, proxy := range proxies {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(p string) {
			defer wg.Done()
			defer func() { <-semaphore }()
			if !checker.CheckIP(p) {
				err := redis.RemoveProxy(p)
				if err != nil {
					mylog.Error("移除无效代理失败: %v", err)
				} else {
					mylog.Info("移除无效代理: %s", p)
				}
			}
		}(proxy)
	}

	wg.Wait()
}

func FetchProxies() error {
	if ProxyAPI == "" {
		return fmt.Errorf("ProxyAPI URL not set")
	}

	client := redis.GetClient()
	ctx := client.Context()

	// 检查是否需要更新代理列表
	lastUpdate, err := client.Get(ctx, lastUpdateKey).Time()
	if err == nil && time.Since(lastUpdate) < updateInterval {
		return nil
	}

	mylog.Info("Updating proxy list...")

	resp, err := http.Get(ProxyAPI)
	if err != nil {
		return fmt.Errorf("获取代理池失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("代理池请求返回无效状态码: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取代理池响应失败: %v", err)
	}

	proxies := strings.Split(strings.TrimSpace(string(body)), ",")

	// 并行检查代理有效性
	var validProxies []string
	var mutex sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrent)

	for _, proxy := range proxies {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(p string) {
			defer wg.Done()
			defer func() { <-semaphore }()
			if checker.CheckIP(p) {
				mutex.Lock()
				validProxies = append(validProxies, p)
				mutex.Unlock()
				mylog.Info("验证代理有效: %s", p)
			} else {
				mylog.Info("代理无效，跳过: %s", p)
			}
		}(proxy)
	}

	wg.Wait()

	if len(validProxies) == 0 {
		return fmt.Errorf("没有找到有效的代理")
	}

	// 获取最佳代理
	bestProxies := checker.GetBestProxies(len(validProxies))

	// 只保存最佳代理
	err = redis.SetProxies(bestProxies, proxyExpiration)
	if err != nil {
		return fmt.Errorf("缓存代理到Redis失败: %v", err)
	}

	// 更新最后更新时间
	err = client.Set(ctx, lastUpdateKey, time.Now(), proxyExpiration).Err()
	if err != nil {
		return fmt.Errorf("更新最后更新时间失败: %v", err)
	}

	mylog.Info("成功更新并缓存代理，有效代理数量: %d", len(validProxies))
	return nil
}

func GetProxies() ([]string, error) {
	proxies, err := redis.GetProxies()
	if err != nil || len(proxies) == 0 {
		// 如果 Redis 中没有代理或出错，尝试重新获取
		if err := FetchProxies(); err != nil {
			return nil, err
		}
		proxies, err = redis.GetProxies()
		if err != nil {
			return nil, err
		}
	}

	// 使用 GetBestProxies 函数获取最佳代理
	bestProxies := checker.GetBestProxies(5) // 获取前5个最佳代理
	if len(bestProxies) > 0 {
		return bestProxies, nil
	}

	return proxies, nil
}

func RemoveProxy(proxy string) error {
	return redis.RemoveProxy(proxy)
}

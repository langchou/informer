package proxy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/redis"
)

const (
	UpdateInterval = 5 * time.Minute // 改为5分钟更新一次
)

var ProxyAPI string

// SetProxyAPI 设置代理API URL
func SetProxyAPI(url string) {
	ProxyAPI = url
}

// UpdateProxyPool 更新代理池
func UpdateProxyPool() error {
	if ProxyAPI == "" {
		return fmt.Errorf("ProxyAPI URL not set")
	}

	// 获取新代理
	newProxies, err := fetchNewProxies()
	if err != nil {
		return fmt.Errorf("获取新代理失败: %v", err)
	}

	// 清空旧代理并添加新代理
	if err := redis.ReplaceProxies(newProxies); err != nil {
		return fmt.Errorf("更新代理失败: %v", err)
	}

	// 获取当前代理数量用于日志
	count, err := redis.GetProxyCount()
	if err != nil {
		return fmt.Errorf("获取代理数量失败: %v", err)
	}

	mylog.Info(fmt.Sprintf("代理池更新完成，当前代理数量: %d", count))
	return nil
}

// 获取新代理的辅助函数
func fetchNewProxies() ([]string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", ProxyAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	proxies := strings.Split(strings.TrimSpace(string(body)), ",")
	var cleanProxies []string
	for _, p := range proxies {
		proxy := strings.TrimSpace(p)
		if proxy != "" {
			if !strings.HasPrefix(proxy, "socks5://") {
				proxy = "socks5://" + proxy
			}
			cleanProxies = append(cleanProxies, proxy)
		}
	}

	return cleanProxies, nil
}

// GetProxy 获取一个代理
func GetProxy() (string, error) {
	return redis.GetRandomProxy()
}

// StartProxyPoolManager 启动代理池管理器
func StartProxyPoolManager(ctx context.Context) {
	ticker := time.NewTicker(UpdateInterval) // 每5分钟更新一次
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := UpdateProxyPool(); err != nil {
				mylog.Error(fmt.Sprintf("更新代理池失败: %v", err))
			}
		}
	}
}

// GetProxyCount 获取当前代理池中的代理数量
func GetProxyCount() (int64, error) {
	return redis.GetProxyCount()
}

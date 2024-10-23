package proxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/redis"
)

const (
	proxyExpiration = 30 * time.Minute
	updateInterval  = 5 * time.Minute
	lastUpdateKey   = "proxy:last_update"
)

var ProxyAPI string

// SetProxyAPI 设置代理API URL
func SetProxyAPI(url string) {
	ProxyAPI = url
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
	err = redis.SetProxies(proxies, proxyExpiration)
	if err != nil {
		return fmt.Errorf("缓存代理到Redis失败: %v", err)
	}

	// 更新最后更新时间
	err = client.Set(ctx, lastUpdateKey, time.Now(), proxyExpiration).Err()
	if err != nil {
		return fmt.Errorf("更新最后更新时间失败: %v", err)
	}

	mylog.Info("成功更新并缓存代理")
	return nil
}

func GetProxies() ([]string, error) {
	proxies, err := redis.GetProxies()
	if err != nil || len(proxies) == 0 {
		// 如果 Redis 中没有代理或出错，尝试重新获取
		if err := FetchProxies(); err != nil {
			return nil, err
		}
		return redis.GetProxies()
	}
	return proxies, nil
}

func RemoveProxy(proxy string) error {
	return redis.RemoveProxy(proxy)
}

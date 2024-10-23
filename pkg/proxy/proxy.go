package proxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/langchou/informer/pkg/checker"  // 使用 checker 包替代 fetch
	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/redis"
)

const (
	proxyExpiration = 30 * time.Minute
	updateInterval  = 5 * time.Minute
	lastUpdateKey   = "proxy:last_update"
	checkInterval   = 1 * time.Minute // 定期检查间隔
)

var ProxyAPI string

// SetProxyAPI 设置代理API URL
func SetProxyAPI(url string) {
	ProxyAPI = url
	// 启动定期检查
	go periodicCheck()
}

// periodicCheck 定期检查代理IP的有效性
func periodicCheck() {
	ticker := time.NewTicker(checkInterval)
	for range ticker.C {
		checkAllProxies()
	}
}

// checkAllProxies 检查所有代理IP的有效性
func checkAllProxies() {
	proxies, err := redis.GetProxies()
	if err != nil {
		mylog.Error("获取代理列表失败: %v", err)
		return
	}

	for _, proxy := range proxies {
		if !checker.CheckIP(proxy) {  // 使用 checker.CheckIP
			err := redis.RemoveProxy(proxy)
			if err != nil {
				mylog.Error("移除无效代理失败: %v", err)
			} else {
				mylog.Info("移除无效代理: %s", proxy)
			}
		}
	}
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
	
	// 检查并只保存有效的代理
	var validProxies []string
	for _, proxy := range proxies {
		if checker.CheckIP(proxy) {  // 使用 checker.CheckIP
			validProxies = append(validProxies, proxy)
			mylog.Info("验证代理有效: %s", proxy)
		} else {
			mylog.Info("代理无效，跳过: %s", proxy)
		}
	}

	if len(validProxies) == 0 {
		return fmt.Errorf("没有找到有效的代理")
	}

	// 只保存有效的代理
	err = redis.SetProxies(validProxies, proxyExpiration)
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
		return redis.GetProxies()
	}
	return proxies, nil
}

func RemoveProxy(proxy string) error {
	return redis.RemoveProxy(proxy)
}

package proxy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/langchou/informer/pkg/checker"
	mylog "github.com/langchou/informer/pkg/log"
)

const (
	UpdateInterval = 5 * time.Minute // 代理池更新间隔
	CheckInterval  = 5 * time.Minute // 改为10分钟检测一次
)

var ProxyAPI string

// ProxyPool 代理池结构
type ProxyPool struct {
	sync.RWMutex
	proxies         map[string]bool      // 普通代理池
	preferredProxies map[string]float64  // 优选代理池，值为响应时间
}

var proxyPool = &ProxyPool{
	proxies:         make(map[string]bool),
	preferredProxies: make(map[string]float64),
}

// SetProxyAPI 设置代理API URL
func SetProxyAPI(url string) {
	ProxyAPI = url
}

// StartIPChecker 启动IP检测器
func StartIPChecker(ctx context.Context) {
	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkAllProxies()
		}
	}
}

// checkAllProxies 检查所有代理的可用性
func checkAllProxies() {
	proxyPool.RLock()
	preferredCount := len(proxyPool.preferredProxies)
	proxyPool.RUnlock()

	if preferredCount > 50 {
		mylog.Info(fmt.Sprintf("当前优选代理数量充足: %d，跳过检测", preferredCount))
		return
	}

	proxyPool.RLock()
	proxies := make([]string, 0, len(proxyPool.proxies))
	for proxy := range proxyPool.proxies {
		proxies = append(proxies, proxy)
	}
	proxyPool.RUnlock()

	mylog.Info(fmt.Sprintf("开始检测代理池中的IP，共 %d 个代理", len(proxies)))

	checkedCount := 0
	for _, proxyIP := range proxies {
		valid, responseTime := checker.CheckIP(proxyIP)
		if valid {
			proxyPool.Lock()
			proxyPool.preferredProxies[proxyIP] = responseTime
			proxyPool.Unlock()
			mylog.Debug(fmt.Sprintf("添加新的优选代理: %s, 响应时间: %.2fms", proxyIP, responseTime))
			checkedCount++
		}

		if checkedCount >= 10 {
			mylog.Info("已找到足够的优选代理，停止检测")
			break
		}
	}

	proxyPool.RLock()
	count := len(proxyPool.preferredProxies)
	proxyPool.RUnlock()
	mylog.Info(fmt.Sprintf("IP检测完成，当前优选代理数量: %d", count))
}

// UpdateProxyPool 更新代理池
func UpdateProxyPool() error {
	if ProxyAPI == "" {
		return fmt.Errorf("ProxyAPI URL not set")
	}

	newProxies, err := fetchNewProxies()
	if err != nil {
		return fmt.Errorf("获取新代理失败: %v", err)
	}

	proxyPool.Lock()
	// 清空现有代理
	proxyPool.proxies = make(map[string]bool)
	// 添加新代理
	for _, proxy := range newProxies {
		proxyPool.proxies[proxy] = true
	}
	proxyPool.Unlock()

	count := len(newProxies)
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

	proxies := strings.Split(strings.TrimSpace(string(body)), "\n")
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
	// 首先尝试从优选代理池中获取
	proxyPool.RLock()
	if len(proxyPool.preferredProxies) > 0 {
		// 获取响应时间最短的代理
		var bestProxy string
		var bestTime float64 = float64(^uint64(0) >> 1) // 最大float64值
		for proxy, time := range proxyPool.preferredProxies {
			if time < bestTime {
				bestProxy = proxy
				bestTime = time
			}
		}
		proxyPool.RUnlock()
		return bestProxy, nil
	}
	proxyPool.RUnlock()

	// 如果没有优选代理，从普通代理池中随机获取一个
	proxyPool.RLock()
	defer proxyPool.RUnlock()
	
	if len(proxyPool.proxies) == 0 {
		return "", fmt.Errorf("代理池为空")
	}

	// 随机选择一个代理
	for proxy := range proxyPool.proxies {
		return proxy, nil // 返回第一个找到的代理
	}

	return "", fmt.Errorf("无法获取代理")
}

// GetProxyCount 获取当前代理池中的代理数量
func GetProxyCount() (int64, error) {
	proxyPool.RLock()
	count := len(proxyPool.proxies)
	proxyPool.RUnlock()
	return int64(count), nil
}

// RemoveProxy 从代理池中删除指定代理
func RemoveProxy(proxy string) {
	proxyPool.Lock()
	delete(proxyPool.proxies, proxy)
	delete(proxyPool.preferredProxies, proxy)
	proxyPool.Unlock()
}

// StartProxyPoolManager 启动代理池管理器
func StartProxyPoolManager(ctx context.Context) {
	ticker := time.NewTicker(UpdateInterval)
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

// GetPreferredProxyCount 获取优选代理数量
func GetPreferredProxyCount() int {
	proxyPool.RLock()
	count := len(proxyPool.preferredProxies)
	proxyPool.RUnlock()
	return count
}

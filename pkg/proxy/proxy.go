package proxy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/langchou/informer/pkg/checker"
	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/redis"
)

const (
	UpdateInterval = 5 * time.Minute // 代理池更新间隔
	CheckInterval  = 5 * time.Minute // 改为10分钟检测一次
)

var ProxyAPI string

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
	// 获取当前优选代理数量
	preferredCount, err := redis.GetPreferredProxyCount()
	if err == nil && preferredCount > 50 {
		mylog.Info(fmt.Sprintf("当前优选代理数量充足: %d，跳过检测", preferredCount))
		return
	}

	proxies, err := redis.GetAllProxies()
	if err != nil {
		mylog.Error(fmt.Sprintf("获取代理列表失败: %v", err))
		return
	}

	mylog.Info(fmt.Sprintf("开始检测代理池中的IP，共 %d 个代理", len(proxies)))

	// 检查每个代理
	checkedCount := 0
	for _, proxyIP := range proxies {
		valid, responseTime := checker.CheckIP(proxyIP)
		if valid {
			// 如果代理可用，添加到优选列表
			if err := redis.AddPreferredProxy(proxyIP, responseTime); err != nil {
				mylog.Error(fmt.Sprintf("添加优选代理失败: %v", err))
			} else {
				mylog.Debug(fmt.Sprintf("添加新的优选代理: %s, 响应时间: %.2fms", proxyIP, responseTime))
				checkedCount++
			}
		}

		// 如果已经找到足够的优选代理，就停止检测
		if checkedCount >= 10 {
			mylog.Info("已找到足够的优选代理，停止检测")
			break
		}
	}

	// 获取优选代理数量并记录日志
	count, _ := redis.GetPreferredProxyCount()
	mylog.Info(fmt.Sprintf("IP检测完成，当前优选代理数量: %d", count))
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

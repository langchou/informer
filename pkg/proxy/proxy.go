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
	MinProxyCount  = 50 // 最小代理数量
	UpdateInterval = 10 * time.Minute
	ScoreThreshold = 20.0 // 低于此分数的代理将被删除

	// 分数调整值
	ScoreDecrease = -10.0 // 代理失败时降低的分数
	ScoreIncrease = 5.0   // 代理成功时提升的分数
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

	// 删除低分代理
	if err := redis.RemoveLowScoreProxies(ScoreThreshold); err != nil {
		mylog.Error(fmt.Sprintf("删除低分代理失败: %v", err))
	}

	// 添加新代理
	if err := redis.SetProxies(newProxies); err != nil {
		return fmt.Errorf("添加新代理失败: %v", err)
	}

	// 检查代理池大小
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

// ReportProxySuccess 报告代理使用成功
func ReportProxySuccess(proxy string) {
	err := redis.UpdateProxyScore(proxy, ScoreIncrease)
	if err != nil {
		mylog.Error(fmt.Sprintf("更新代理分数失败: %v", err))
	}
}

// ReportProxyFailure 报告代理使用失败
func ReportProxyFailure(proxy string) {
	err := redis.UpdateProxyScore(proxy, ScoreDecrease)
	if err != nil {
		mylog.Error(fmt.Sprintf("更新代理分数失败: %v", err))
	}
}

// GetBestProxy 获取最佳代���
func GetBestProxy() (string, error) {
	return redis.GetTopProxy()
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

// GetProxyCount 获取当前代理池中的代理数量
func GetProxyCount() (int64, error) {
	return redis.GetProxyCount()
}

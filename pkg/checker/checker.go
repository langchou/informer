package checker

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	mylog "github.com/langchou/informer/pkg/log"
	customproxy "golang.org/x/net/proxy"
)

const (
	maxRetries  = 3
	baseTimeout = 10 * time.Second
)

type ProxyScore struct {
	Address   string
	Score     int
	LastCheck time.Time
}

var (
	proxyScores = make(map[string]*ProxyScore)
	scoreMutex  sync.RWMutex
)

func CheckIP(proxyIP string) bool {
	scoreMutex.RLock()
	score, exists := proxyScores[proxyIP]
	scoreMutex.RUnlock()

	if exists && time.Since(score.LastCheck) < 5*time.Minute {
		return score.Score > 0
	}

	for retry := 0; retry < maxRetries; retry++ {
		if checkIPOnce(proxyIP, baseTimeout*(1<<uint(retry))) {
			updateProxyScore(proxyIP, 1)
			return true
		}
	}

	updateProxyScore(proxyIP, -1)
	return false
}

func checkIPOnce(proxyIP string, timeout time.Duration) bool {
	ProcessedProxyIP := strings.Replace(proxyIP, "socks5://", "", 1)
	pollURL := "http://ipinfo.io"
	begin := time.Now()

	dialer, err := customproxy.SOCKS5("tcp", ProcessedProxyIP, nil, customproxy.Direct)
	if err != nil {
		mylog.Debug("代理 %s 创建SOCKS5拨号器失败: %v", proxyIP, err)
		return false
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.Dial(network, addr)
			if err != nil {
				mylog.Debug("代理 %s 拨号失败: %v", proxyIP, err)
			}
			return conn, err
		},
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConnsPerHost: 50,
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	request, err := http.NewRequest("GET", pollURL, nil)
	if err != nil {
		mylog.Debug("代理 %s 创建请求失败: %v", proxyIP, err)
		return false
	}
	request.Header.Add("accept", "text/plain")

	resp, err := client.Do(request)
	if err != nil {
		mylog.Debug("代理 %s 执行请求失败: %v", proxyIP, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		duration := time.Since(begin).Milliseconds()
		mylog.Info("代理 %s 有效, 响应时间: %d ms", ProcessedProxyIP, duration)
		return true
	}

	mylog.Debug("代理 %s 无效: 响应状态码 %d", proxyIP, resp.StatusCode)
	return false
}

func updateProxyScore(proxyIP string, delta int) {
	scoreMutex.Lock()
	defer scoreMutex.Unlock()

	score, exists := proxyScores[proxyIP]
	if !exists {
		score = &ProxyScore{Address: proxyIP}
		proxyScores[proxyIP] = score
	}

	score.Score += delta
	score.LastCheck = time.Now()

	if score.Score < -5 {
		delete(proxyScores, proxyIP)
	}
}

func GetBestProxies(n int) []string {
	scoreMutex.RLock()
	defer scoreMutex.RUnlock()

	var bestProxies []ProxyScore
	for _, score := range proxyScores {
		if score.Score > 0 {
			bestProxies = append(bestProxies, *score)
		}
	}

	// Sort bestProxies by score (implement this part)

	result := make([]string, 0, n)
	for i := 0; i < n && i < len(bestProxies); i++ {
		result = append(result, bestProxies[i].Address)
	}

	return result
}

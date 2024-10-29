package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	mylog "github.com/langchou/informer/pkg/log"
	customproxy "golang.org/x/net/proxy"
)

func CheckIP(proxyIP string) (bool, float64) {
	ProcessedProxyIP := strings.Replace(proxyIP, "socks5://", "", 1)
	pollURL := "http://ipinfo.io"
	begin := time.Now()

	// 解析代理URL
	proxyURL, err := url.Parse(proxyIP)
	if err != nil {
		mylog.Debug(fmt.Sprintf("解析代理URL失败: %v", err))
		return false, 0
	}

	// 创建 SOCKS5 拨号器
	dialer, err := customproxy.SOCKS5("tcp", proxyURL.Host, nil, customproxy.Direct)
	if err != nil {
		mylog.Debug(fmt.Sprintf("创建SOCKS5拨号器失败: %v", err))
		return false, 0
	}

	// 减少超时时间
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			MaxIdleConnsPerHost: 10,
		},
	}

	// 添加请求上下文超时控制
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, "GET", pollURL, nil)
	if err != nil {
		mylog.Debug(fmt.Sprintf("创建请求失败: %v", err))
		return false, 0
	}
	request.Header.Add("accept", "text/plain")

	resp, err := client.Do(request)
	if err != nil {
		mylog.Debug(fmt.Sprintf("请求失败: %v", err))
		return false, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		duration := time.Since(begin).Milliseconds()
		mylog.Debug(fmt.Sprintf("代理 %s 可用, 响应时间: %d ms", ProcessedProxyIP, duration))
		return true, float64(duration)
	}

	mylog.Debug(fmt.Sprintf("代理 %s 响应状态码异常: %d", ProcessedProxyIP, resp.StatusCode))
	return false, 0
}

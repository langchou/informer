// fetch/fetch.go

package fetch

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	mylog "github.com/langchou/informer/pkg/log"

	"github.com/PuerkitoBio/goquery"
	"github.com/langchou/informer/pkg/proxy"
	customproxy "golang.org/x/net/proxy"
)

func CheckIP(proxyIP string) bool {
	// 去掉前缀 "socks5://"
	ProcessedProxyIP := strings.Replace(proxyIP, "socks5://", "", 1)

	// 打印处理后的 IP，确保没有前缀
	// fmt.Printf("Processed proxy IP: %s\n", ProcessedProxyIP)

	// 选择 ipinfo.io 测试 URL
	pollURL := "http://ipinfo.io"

	// 打印正在测试的 IP
	// fmt.Printf("Testing proxy IP: %s\n", ProcessedProxyIP)
	begin := time.Now() // 记录开始时间

	// 创建 SOCKS5 Dialer
	dialer, err := customproxy.SOCKS5("tcp", ProcessedProxyIP, nil, customproxy.Direct)
	if err != nil {
		// fmt.Printf("Failed to create SOCKS5 dialer: %v\n", err)
		return false
	}

	// 创建自定义 HTTP Transport，使用 SOCKS5 Dialer
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // 忽略证书验证
		MaxIdleConnsPerHost: 50,
	}

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout:   20 * time.Second, // 设置请求超时时间
		Transport: transport,
	}

	// 构建 HTTP 请求
	request, err := http.NewRequest("GET", pollURL, nil)
	if err != nil {
		// fmt.Printf("Failed to create request: %v\n", err)
		return false
	}
	// 添加请求头
	request.Header.Add("accept", "text/plain")

	// 发送 HTTP 请求
	resp, err := client.Do(request)
	if err != nil {
		// fmt.Printf("Proxy test failed for IP %s: %v\n", ProcessedProxyIP, err)
		return false
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode == http.StatusOK {
		// 计算代理响应速度，单位为毫秒
		duration := time.Since(begin).Milliseconds()
		mylog.Info("Proxy %s is valid, response time: %d ms", ProcessedProxyIP, duration)
		return true
	}

	// fmt.Printf("Proxy %s returned non-OK status: %d\n", ProcessedProxyIP, resp.StatusCode)
	return false
}

func FetchWithProxy(proxyIP string, targetURL string, headers map[string]string) (string, error) {
	if !CheckIP(proxyIP) {
		return "", fmt.Errorf("代理 IP %s 无效", proxyIP)
	}

	proxyURL, err := proxy.ParseProxyURL(proxyIP)
	if err != nil {
		return "", fmt.Errorf("解析代理 URL 失败: %v", err)
	}

	// 自定义 DialContext 以通过代理发送 DNS 查询
	dialer, err := proxy.SOCKSDialer(proxyURL) // 使用 SOCKS5 代理
	if err != nil {
		return "", fmt.Errorf("创建 SOCKS 代理失败: %v", err)
	}

	// 自定义 Transport 来确保所有流量通过代理
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL), // 设置代理
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr) // 手动包装 DialContext
		},
	}

	client := &http.Client{
		Transport: transport,
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("无效的响应状态码: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("解析 HTML 失败: %v", err)
	}

	html, _ := doc.Html()
	return html, nil
}

func FetchWithProxies(proxies []string, url string, headers map[string]string) (string, error) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background()) // 用于取消其他 goroutines 的上下文
	defer cancel()

	resultCh := make(chan string, 1)        // 用于接收第一个有效响应
	errCh := make(chan error, len(proxies)) // 用于接收所有错误

	// 并发请求所有代理 IP
	for _, proxyIP := range proxies {
		wg.Add(1)
		go func(proxyIP string) {
			defer wg.Done()

			// 检查上下文是否已经取消，如果已取消则退出
			select {
			case <-ctx.Done():
				return
			default:
			}

			content, err := FetchWithProxy(proxyIP, url, headers) // 使用 FetchWithProxy
			if err != nil {
				errCh <- err
				return
			}

			// 第一个有效结果，取消其他请求
			select {
			case resultCh <- content:
				cancel() // 成功获取结果，取消其他请求
			case <-ctx.Done():
				// 上下文已经取消，不需要处理
			}
		}(proxyIP)
	}

	// 等待所有 goroutines 完成
	go func() {
		wg.Wait()
		close(resultCh)
		close(errCh)
	}()

	// 返回第一个成功的结果，或全部失败时
	select {
	case content := <-resultCh:
		return content, nil
	case <-time.After(30 * time.Second): // 超时时间为 30 秒，避免所有请求都失败时挂起
		return "", fmt.Errorf("所有代理请求超时或失败")
	}
}

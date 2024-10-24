// fetch/fetch.go

package fetch

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	mylog "github.com/langchou/informer/pkg/log"

	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/langchou/informer/pkg/proxy"
	customproxy "golang.org/x/net/proxy"
)

func CheckIP(proxyIP string) bool {
	ProcessedProxyIP := strings.Replace(proxyIP, "socks5://", "", 1)
	pollURL := "http://ipinfo.io"
	begin := time.Now()

	// 减少超时时间
	client := &http.Client{
		Timeout: 10 * time.Second, // 从20秒改为10秒
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// 添加连接超时
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, network, addr)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			// 减少空闲连接数
			MaxIdleConnsPerHost: 10,
		},
	}

	// 添加请求上下文超时控制
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, "GET", pollURL, nil)
	if err != nil {
		return false
	}
	request.Header.Add("accept", "text/plain")

	resp, err := client.Do(request)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		duration := time.Since(begin).Milliseconds()
		mylog.Info(fmt.Sprintf("Proxy %s is valid, response time: %d ms", ProcessedProxyIP, duration))
		return true
	}

	return false
}

func FetchWithProxies(targetURL string, headers map[string]string) (string, error) {
	proxies, err := proxy.GetProxies()
	if err != nil {
		return "", fmt.Errorf("获取代理失败: %v", err)
	}

	for _, proxyIP := range proxies {
		if CheckIP(proxyIP) {
			content, err := FetchWithProxy(proxyIP, targetURL, headers)
			if err == nil {
				return content, nil
			}
			mylog.Error("使用代理 %s 请求失败: %v", proxyIP, err)
		} else {
			proxy.RemoveProxy(proxyIP)
		}
	}

	return "", fmt.Errorf("所有代理请求失败")
}

func FetchWithProxy(proxyIP string, targetURL string, headers map[string]string) (string, error) {
	// 清理代理IP字符串
	proxyIP = strings.TrimSpace(proxyIP)

	// 解析代理URL
	proxyURL, err := ParseProxyURL(proxyIP)
	if err != nil {
		return "", fmt.Errorf("解析代理 URL 失败: %v", err)
	}

	// 创建 SOCKS5 拨号器
	dialer, err := customproxy.SOCKS5("tcp", proxyURL.Host, nil, customproxy.Direct)
	if err != nil {
		return "", fmt.Errorf("创建 SOCKS 代理失败: %v", err)
	}

	transport := &http.Transport{
		// 移除 Proxy 字段，因为我们使用 SOCKS5 拨号器
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: transport,
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

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

func ParseProxyURL(proxyIP string) (*url.URL, error) {
	// 清理代理IP字符串中的空白字符
	proxyIP = strings.TrimSpace(proxyIP)

	if strings.HasPrefix(proxyIP, "socks5://") {
		// 如果已经是 socks5:// 开头，直接解析
		return url.Parse(proxyIP)
	}
	// 如果不是 socks5:// 开头，添加前缀
	return url.Parse("socks5://" + proxyIP)
}

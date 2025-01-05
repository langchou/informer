// fetch/fetch.go

package fetch

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
	"github.com/langchou/informer/pkg/proxy"
	customproxy "golang.org/x/net/proxy"

	"github.com/PuerkitoBio/goquery"
)

func FetchWithProxies(targetURL string, headers map[string]string) (string, error) {
	// 检查代理池数量
	proxyCount, err := proxy.GetProxyCount()
	if err != nil {
		return "", fmt.Errorf("获取代理数量失败: %v", err)
	}

	// 如果代理池为空，等待一段时间
	if proxyCount == 0 {
		mylog.Warn("代理池为空，等待30秒后重试")
		time.Sleep(30 * time.Second)
		return "", fmt.Errorf("代理池为空，请稍后重试")
	}

	// 首先尝试使用优选代理
	preferredCount := proxy.GetPreferredProxyCount()
	if preferredCount > 0 {
		proxyIP, err := proxy.GetProxy()
		if err == nil {
			content, err := FetchWithProxy(proxyIP, targetURL, headers)
			if err == nil {
				mylog.Debug(fmt.Sprintf("使用优选代理 %s 请求成功", proxyIP))
				return content, nil
			} else {
				mylog.Warn(fmt.Sprintf("使用优选代理 %s 请求失败: %v", proxyIP, err))
				proxy.RemoveProxy(proxyIP)
			}
		}
	}

	// 如果优选代理都失败了，使用普通代理
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		proxyIP, err := proxy.GetProxy()
		if err != nil {
			return "", fmt.Errorf("获取代理失败: %v", err)
		}

		content, err := FetchWithProxy(proxyIP, targetURL, headers)
		if err == nil {
			return content, nil
		} else {
			mylog.Warn(fmt.Sprintf("使用代理 %s 请求失败: %v", proxyIP, err))
			proxy.RemoveProxy(proxyIP)
		}
	}

	return "", fmt.Errorf("所有重试都失败")
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
		Timeout:   5 * time.Second,
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

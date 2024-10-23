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

	"github.com/PuerkitoBio/goquery"
	"github.com/langchou/informer/pkg/proxy"
	customproxy "golang.org/x/net/proxy"
	"net/url"
)

func CheckIP(proxyIP string) bool {
	ProcessedProxyIP := strings.Replace(proxyIP, "socks5://", "", 1)
	pollURL := "http://ipinfo.io"
	begin := time.Now()

	dialer, err := customproxy.SOCKS5("tcp", ProcessedProxyIP, nil, customproxy.Direct)
	if err != nil {
		return false
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConnsPerHost: 50,
	}

	client := &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
	}

	request, err := http.NewRequest("GET", pollURL, nil)
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
		mylog.Info("Proxy %s is valid, response time: %d ms", ProcessedProxyIP, duration)
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
	proxyURL, err := ParseProxyURL(proxyIP)
	if err != nil {
		return "", fmt.Errorf("解析代理 URL 失败: %v", err)
	}

	dialer, err := customproxy.SOCKS5("tcp", proxyURL.Host, nil, customproxy.Direct)
	if err != nil {
		return "", fmt.Errorf("创建 SOCKS 代理失败: %v", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
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
	if strings.HasPrefix(proxyIP, "socks5://") {
		return url.Parse(proxyIP)
	}
	return url.Parse(fmt.Sprintf("http://%s", proxyIP))
}

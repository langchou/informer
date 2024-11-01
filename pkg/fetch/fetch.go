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
	"github.com/langchou/informer/pkg/proxy"

	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/langchou/informer/pkg/checker"
	"github.com/langchou/informer/pkg/redis"
	customproxy "golang.org/x/net/proxy"
)

func FetchWithProxies(targetURL string, headers map[string]string) (string, error) {
	// 检查代理池数量
	proxyCount, err := redis.GetProxyCount()
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
	preferredProxies, err := redis.GetPreferredProxies() // 获取所有优选代理
	if err == nil && len(preferredProxies) > 0 {
		// 依次尝试每个优选代理
		for _, proxyIP := range preferredProxies {
			content, err := FetchWithProxy(proxyIP, targetURL, headers)
			if err == nil {
				mylog.Debug(fmt.Sprintf("使用优选代理 %s 请求成功, 可用优选代理数: %d", proxyIP, len(preferredProxies)))
				return content, nil
			} else {
				mylog.Warn(fmt.Sprintf("使用优选代理 %s 请求失败: %v", proxyIP, err))
				// 从优选列表和普通列表中删除失效的代理
				if err := redis.RemovePreferredProxy(proxyIP); err != nil {
					mylog.Error(fmt.Sprintf("从优选列表删除代理失败: %v", err))
				}
				if err := redis.RemoveProxy(proxyIP); err != nil {
					mylog.Error(fmt.Sprintf("从代理池删除代理失败: %v", err))
				}
				// 继续尝试下一个优选代理
				continue
			}
		}
	}

	// 如果所有优选代理都失败了，使用普通代理
	maxRetries, err := proxy.GetProxyCount()
	if err != nil {
		return "", fmt.Errorf("获取代理数量失败: %v", err)
	}

	for i := 0; i < int(maxRetries); i++ {
		proxyIP, err := redis.GetRandomProxy()
		if err != nil {
			return "", fmt.Errorf("获取代理失败: %v", err)
		}

		if proxyIP == "" {
			time.Sleep(time.Second)
			continue
		}

		// 修复：正确处理 CheckIP 的两个返回值
		valid, responseTime := checker.CheckIP(proxyIP)
		if valid {
			content, err := FetchWithProxy(proxyIP, targetURL, headers)
			if err == nil {
				// 请求成功，将该代理添加到优选列表，传入响应时间
				if err := redis.AddPreferredProxy(proxyIP, responseTime); err != nil {
					mylog.Error(fmt.Sprintf("添加优选代理失败: %v", err))
				} else {
					mylog.Debug(fmt.Sprintf("添加新的优选代理: %s, 响应时间: %.2fms", proxyIP, responseTime))
				}
				return content, nil
			} else {
				mylog.Warn(fmt.Sprintf("使用代理 %s 请求失败: %v", proxyIP, err))
				if err := redis.RemoveProxy(proxyIP); err != nil {
					mylog.Error(fmt.Sprintf("删除失效代理失败: %v", err))
				} else {
					mylog.Info(fmt.Sprintf("已删除失效代理: %s, 剩余代理数量: %d", proxyIP, maxRetries-int64(i)-1))
				}
			}
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

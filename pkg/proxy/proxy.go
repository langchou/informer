package proxy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	mylog "github.com/langchou/informer/pkg/log"
	"golang.org/x/net/proxy"
)

var cachedProxies []string             // 缓存的代理列表
var lastFetchTime time.Time            // 上次请求代理池的时间
const cacheDuration = 10 * time.Minute // 缓存时间为10分钟

// SOCKSDialer 创建一个通过 SOCKS5 代理的 Dialer
func SOCKSDialer(proxyURL *url.URL) (proxy.Dialer, error) {
	// 使用 SOCKS5 代理创建 Dialer
	dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, nil, proxy.Direct)
	if err != nil {
		return nil, err
	}
	return dialer, nil
}

// CreateTransport 根据代理 URL 创建 http.Transport
func CreateTransport(proxyURL *url.URL) (*http.Transport, error) {
	// 默认的 http.Transport 结构体
	transport := &http.Transport{}

	if proxyURL.Scheme == "socks5" {
		// 如果代理是 SOCKS5，创建 SOCKS5 的 Dialer
		dialer, err := SOCKSDialer(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("创建 SOCKS5 Dialer 失败: %v", err)
		}
		// 使用 SOCKS5 代理的 Dialer
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
	} else {
		// HTTP 代理
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	// 禁用系统的 DNS 解析，确保所有 DNS 查询都通过代理进行
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		// 通过代理地址解析 DNS
		return net.Dial(network, addr)
	}

	return transport, nil
}

func FetchProxies(ProxyAPI string) ([]string, error) {
	now := time.Now()
	if len(cachedProxies) == 0 || now.Sub(lastFetchTime) > cacheDuration {
		// 如果缓存为空或缓存过期，重新获取代理
		resp, err := http.Get(ProxyAPI)
		if err != nil {
			return nil, fmt.Errorf("获取代理池失败: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("代理池请求返回无效状态码: %d", resp.StatusCode)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("读取代理池响应失败: %v", err)
		}

		// 将响应的内容按逗号分隔，去掉首尾的空白字符
		cachedProxies = strings.Split(strings.TrimSpace(string(body)), ",")
		lastFetchTime = now // 更新上次请求时间
		mylog.Info("Cached proxies: %s", strings.Join(cachedProxies, ", "))
	}

	return cachedProxies, nil
}

// ParseProxyURL 解析代理 IP 为 URL 格式
func ParseProxyURL(proxyIP string) (*url.URL, error) {
	var proxyURL *url.URL
	var err error

	if strings.HasPrefix(proxyIP, "socks5://") {
		// 如果代理是 SOCKS5
		proxyURL, err = url.Parse(proxyIP)
		if err != nil {
			return nil, fmt.Errorf("解析 SOCKS5 代理失败: %v", err)
		}
	} else {
		// 默认为 HTTP 代理
		proxyURL, err = url.Parse(fmt.Sprintf("http://%s", proxyIP))
		if err != nil {
			return nil, fmt.Errorf("解析 HTTP 代理失败: %v", err)
		}
	}

	return proxyURL, nil
}

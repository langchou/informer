package proxy

import (
	"fmt"
	"golang.org/x/exp/rand"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const ProxyAPI = "https://269900.xyz/fetch_http_all?region=cn"

var cachedProxies []string             // 缓存的代理列表
var lastFetchTime time.Time            // 上次请求代理池的时间
const cacheDuration = 10 * time.Minute // 缓存时间为10分钟

func FetchProxies() ([]string, error) {
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
	}

	// 随机从缓存的代理列表中取出三分之一
	randomProxies := getRandomSubset(cachedProxies, len(cachedProxies)/3)

	return randomProxies, nil
}

func getRandomSubset(proxies []string, count int) []string {
	rand.Seed(uint64(time.Now().UnixNano()))
	rand.Shuffle(len(proxies), func(i, j int) {
		proxies[i], proxies[j] = proxies[j], proxies[i]
	})

	if count > len(proxies) {
		count = len(proxies)
	}
	return proxies[:count]
}

// ParseProxyURL 解析代理 IP 为 URL 格式
func ParseProxyURL(proxyIP string) *url.URL {
	proxyURL, err := url.Parse(fmt.Sprintf("http://%s", proxyIP))
	if err != nil {
		return nil
	}
	return proxyURL
}

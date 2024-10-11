package proxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const ProxyAPI = "https://269900.xyz/fetch_http_all?region=cn&count=10"

func FetchProxies() ([]string, error) {
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
	proxies := strings.Split(strings.TrimSpace(string(body)), ",")

	return proxies, nil
}

// ParseProxyURL 解析代理 IP 为 URL 格式
func ParseProxyURL(proxyIP string) *url.URL {
	proxyURL, err := url.Parse(fmt.Sprintf("http://%s", proxyIP))
	if err != nil {
		return nil
	}
	return proxyURL
}

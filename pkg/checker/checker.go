package checker

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	mylog "github.com/langchou/informer/pkg/log"
	customproxy "golang.org/x/net/proxy"
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

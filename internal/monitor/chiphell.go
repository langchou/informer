package monitor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/langchou/informer/db"
	"github.com/langchou/informer/pkg/fetch"
	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/notifier"
	"github.com/langchou/informer/pkg/proxy"
	"github.com/langchou/informer/pkg/util"
	"golang.org/x/exp/rand"
)

var _ ForumMonitor = &ChiphellMonitor{}

type ChiphellMonitor struct {
	ForumName     string
	Cookies       string
	UserKeywords  map[string][]string // 手机号及其关键词的映射
	Notifier      *notifier.DingTalkNotifier
	Database      *db.Database
	Logger        *mylog.Logger
	MessageQueue  chan NotificationMessage
	WaitTimeRange struct {
		Min int
		Max int
	}
	ProxyAPI string
}

type NotificationMessage struct {
	Title         string
	Message       string
	AtPhoneNumber []string
}

func NewChiphellMonitor(forumName string, cookies string, userKeywords map[string][]string, notifier *notifier.DingTalkNotifier, database *db.Database, waitTimeRange struct{ Min, Max int }, proxyAPI string) *ChiphellMonitor {
	monitor := &ChiphellMonitor{
		ForumName:     forumName,
		Cookies:       cookies,
		UserKeywords:  userKeywords,
		Notifier:      notifier,
		Database:      database,
		MessageQueue:  make(chan NotificationMessage, 100), // 创建消息队列，容量100
		WaitTimeRange: waitTimeRange,
		ProxyAPI:      proxyAPI,
	}
	// 启动 goroutine 处理消息队列
	go monitor.processMessageQueue()

	return monitor
}

// 消息队列的处理函数，每3秒发送一条消息
func (c *ChiphellMonitor) processMessageQueue() {
	for {
		select {
		case msg := <-c.MessageQueue:
			// 发送消息
			err := c.Notifier.SendNotification(msg.Title, msg.Message, msg.AtPhoneNumber)
			if err != nil {
				mylog.Error(fmt.Sprintf("发送钉钉通知失败: %v", err))
			} else {
				mylog.Info(fmt.Sprintf("成功发送消息: %s", msg.Message))
			}
			time.Sleep(3 * time.Second) // 控制发送频率，每3秒发送一条
		}
	}
}

// 将通知消息放入队列
func (c *ChiphellMonitor) enqueueNotification(title, message string, atPhoneNumbers []string) {
	notification := NotificationMessage{
		Title:         title,
		Message:       message,
		AtPhoneNumber: atPhoneNumbers,
	}

	c.MessageQueue <- notification
}

// 获取页面内容
// FetchPageContent 使用代理池并发请求访问论坛页面
func (c *ChiphellMonitor) FetchPageContent() (string, error) {
	if c.ProxyAPI != "" {
		// 使用代理池
		proxies, err := proxy.FetchProxies(c.ProxyAPI) // 获取代理池中的代理
		if err != nil {
			return "", fmt.Errorf("获取代理池失败: %v", err)
		}

		headers := map[string]string{
			"Cookie":     c.Cookies,
			"User-Agent": "Mozilla/5.0",
		}

		// 使用新的 fetch 包的 FetchWithProxies 方法
		content, err := fetch.FetchWithProxies(proxies, "https://www.chiphell.com/forum-26-1.html", headers)
		if err != nil {
			return "", err
		}
		return content, nil
	} else {
		// 不使用代理，直接请求 Chiphell
		return c.fetchWithoutProxy()
	}
}

func (c *ChiphellMonitor) fetchWithProxy(proxyIP string) (string, error) {
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

	req, err := http.NewRequest("GET", "https://www.chiphell.com/forum-26-1.html", nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Cookie", c.Cookies)
	// randomUserAgent := userAgents[rand.Intn(len(userAgents))]
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

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

	// 记录使用的代理 IP
	mylog.Info(fmt.Sprintf("成功使用代理 IP: %s", proxyIP))

	html, _ := doc.Html()
	return html, nil
}

func (c *ChiphellMonitor) fetchWithoutProxy() (string, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", "https://www.chiphell.com/forum-26-1.html", nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Cookie", c.Cookies)
	req.Header.Set("User-Agent", "Mozilla/5.0")

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

func (c *ChiphellMonitor) ParseContent(content string) ([]Post, error) {
	var posts []Post

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %v", err)
	}

	doc.Find("tbody[id^='normalthread_']").Each(func(i int, s *goquery.Selection) {
		postLink := s.Find("a.s.xst")
		postTitle := postLink.Text()

		postHref, exists := postLink.Attr("href")
		if exists {
			posts = append(posts, Post{
				Title: postTitle,
				Link:  "https://www.chiphell.com/" + postHref,
			})
		}
	})
	return posts, nil
}

func (c *ChiphellMonitor) FetchPostMainContent(postURL string, proxies []string) (string, string, string, string, string, error) {
	headers := map[string]string{
		"Cookie":     c.Cookies,
		"User-Agent": "Mozilla/5.0", // 使用固定的 User-Agent
	}

	// 使用代理并发请求
	content, err := fetch.FetchWithProxies(proxies, postURL, headers)
	if err != nil {
		mylog.Error(fmt.Sprintf("请求失败: %v", err))
		return "", "", "", "", "", err
	}

	if len(content) == 0 {
		return "", "", "", "", "", fmt.Errorf("获取内容失败: 返回内容为空")
	}

	mylog.Debug(fmt.Sprintf("Fetched content size: %d", len(content)))

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("解析 HTML 失败: %v", err)
	}

	// 提取信息
	var qq, price, tradeRange, address, phone string

	// Adjusting the selector to directly target the rows in the table
	doc.Find(".typeoption tbody tr").Each(func(i int, tr *goquery.Selection) {
		th := strings.TrimSpace(tr.Find("th").Text())
		td := strings.TrimSpace(tr.Find("td").Text())

		// Log to see what we are finding
		mylog.Debug(fmt.Sprintf("Found th: '%s', td: '%s'", th, td))

		switch th {
		case "所在地:":
			address = td
		case "电话:":
			phone = td
		case "QQ:":
			qq = td
		case "价格:":
			price = td
		case "交易范围:":
			tradeRange = td
		}
	})

	mylog.Debug(fmt.Sprintf("Parsed qq: %s, price: %s, tradeRange: %s, address: %s, phone: %s", qq, price, tradeRange, address, phone))

	return qq, price, tradeRange, address, phone, nil
}

func (c *ChiphellMonitor) ProcessPosts(posts []Post) error {
	// proxies, err := proxy.FetchProxies(c.ProxyAPI)
	// if err != nil {
	// 	mylog.Error("获取代理池失败: %v", err)
	// }
	for _, post := range posts {
		postHash := util.HashString(post.Title)

		if c.Database.IsNewPost(c.ForumName, postHash) {
			c.Database.StorePostHash(c.ForumName, postHash)
			mylog.Info(fmt.Sprintf("检测到新帖子: 标题: %s 链接: %s", post.Title, post.Link))

			// 获取主楼内容
			// qq, price, tradeRange, address, phone, err := c.FetchPostMainContent(post.Link, proxies)
			// if err != nil {
			// mylog.Error(fmt.Sprintf("获取主楼内容失败: %v", err))
			// continue
			// }
			// message := fmt.Sprintf("标题: %s\n链接: %s\nqq:%s\n电话: %s\n价格: %s\n所在地: %s\n交易范围: %s\n", post.Title, post.Link, qq, phone, price, address, tradeRange)
			message := fmt.Sprintf("标题: %s\n链接: %s\n", post.Title, post.Link)

			// 收集所有关注该帖子的手机号
			var phoneNumbers []string

			// 遍历用户的关键词进行匹配
			for phoneNumber, keywords := range c.UserKeywords {
				for _, keyword := range keywords {
					lowerKeyword := strings.ToLower(keyword)

					if strings.Contains(strings.ToLower(post.Title), lowerKeyword) {
						// 如果用户的关键词匹配，则添加手机号到列表
						phoneNumbers = append(phoneNumbers, phoneNumber)
						break
					}
				}
			}

			// 如果有匹配的手机号，发送包含所有手机号的通知
			if len(phoneNumbers) > 0 {
				c.enqueueNotification(post.Title, message, phoneNumbers)
			} else {
				c.enqueueNotification(post.Title, message, nil)
			}
		}
	}
	return nil
}

func (c *ChiphellMonitor) MonitorPage() {
	failedAttempts := 0

	for {
		content, err := c.FetchPageContent()
		if err != nil {
			failedAttempts++
			mylog.Error("获取页面内容失败", "error", err)
			continue
		}

		// 请求成功，重置失败计数
		failedAttempts = 0

		posts, err := c.ParseContent(content)
		if err != nil {
			mylog.Error("解析页面内容失败", "error", err)
			c.Notifier.ReportError("解析页面内容失败", err.Error())
			continue
		}

		err = c.ProcessPosts(posts)
		if err != nil {
			mylog.Error("处理帖子失败", "error", err)
			c.Notifier.ReportError("处理帖子失败", err.Error())
		}

		// 正常处理完毕，等待一段时间后再进行下一次监控
		waitTime := time.Duration(c.WaitTimeRange.Min+rand.Intn(c.WaitTimeRange.Max-c.WaitTimeRange.Min+1)) * time.Second
		time.Sleep(waitTime)

		// 定期清理数据库中过期的帖子
		// c.Database.CleanUpOldPosts(720 * time.Hour)
	}
}

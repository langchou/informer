package monitor

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/langchou/informer/db"
	"github.com/langchou/informer/pkg/fetch"
	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/notifier"
	"github.com/langchou/informer/pkg/proxy"
	"github.com/langchou/informer/pkg/redis"
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

	// 设置 ProxyAPI
	proxy.SetProxyAPI(proxyAPI)

	// 启动 goroutine 处理消息队列
	go monitor.processMessageQueue()

	return monitor
}

// 修改消息队列的处理函数，批量处理消息
func (c *ChiphellMonitor) processMessageQueue() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	var messages []NotificationMessage
	for {
		select {
		case msg := <-c.MessageQueue:
			messages = append(messages, msg)
		case <-ticker.C:
			if len(messages) > 0 {
				// 批量发送所有积累的消息
				var combinedMessage strings.Builder
				var allPhoneNumbers []string
				phoneNumbersMap := make(map[string]bool)

				for i, msg := range messages {
					// 添加分隔线
					if i > 0 {
						combinedMessage.WriteString("\n----------------------------------------\n")
					}
					combinedMessage.WriteString(fmt.Sprintf("%s\n%s", msg.Title, msg.Message))

					// 收集所有需要@的手机号，去重
					for _, phone := range msg.AtPhoneNumber {
						if !phoneNumbersMap[phone] {
							phoneNumbersMap[phone] = true
							allPhoneNumbers = append(allPhoneNumbers, phone)
						}
					}
				}

				// 发送合并后的消息
				err := c.Notifier.SendNotification(
					fmt.Sprintf("新帖子通知 (共%d条)", len(messages)),
					combinedMessage.String(),
					allPhoneNumbers,
				)

				if err != nil {
					mylog.Error(fmt.Sprintf("发送钉钉通知失败: %v", err))
				} else {
					mylog.Debug(fmt.Sprintf("成功发送%d条合并消息", len(messages)))
				}

				// 清空消息列表
				messages = nil
			}
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
		headers := map[string]string{
			"Cookie":     c.Cookies,
			"User-Agent": "Mozilla/5.0",
		}

		content, err := fetch.FetchWithProxies("https://www.chiphell.com/forum-26-1.html", headers)
		if err != nil {
			return "", err
		}
		return content, nil
	} else {
		return c.fetchWithoutProxy()
	}
}

func (c *ChiphellMonitor) fetchWithProxy(proxyIP string) (string, error) {
	proxyURL, err := fetch.ParseProxyURL(proxyIP)
	if err != nil {
		return "", fmt.Errorf("解析代理 URL 失败: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	req, err := http.NewRequest("GET", "https://www.chiphell.com/forum-26-1.html", nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Cookie", c.Cookies)
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
	mylog.Debug(fmt.Sprintf("成功使用代理 IP: %s", proxyIP))

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

func (c *ChiphellMonitor) FetchPostMainContent(postURL string) (string, string, string, string, string, error) {
	headers := map[string]string{
		"Cookie":     c.Cookies,
		"User-Agent": "Mozilla/5.0",
	}

	// 使用代理池获取主楼内容
	content, err := fetch.FetchWithProxies(postURL, headers)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("获取主楼内容失败: %v", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("解析 HTML 失败: %v", err)
	}

	// 提取信息
	var qq, price, tradeRange, address, phone string

	// 调整选择器以直接定位表格中的行
	doc.Find(".typeoption tbody tr").Each(func(i int, tr *goquery.Selection) {
		th := strings.TrimSpace(tr.Find("th").Text())
		td := strings.TrimSpace(tr.Find("td").Text())

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

	return qq, price, tradeRange, address, phone, nil
}

func (c *ChiphellMonitor) ProcessPosts(posts []Post) error {
	for _, post := range posts {
		// 从帖子链接中提取ID
		postID := extractPostID(post.Link)

		if c.Database.IsNewPost(c.ForumName, postID) {
			c.Database.StorePostID(c.ForumName, postID)
			mylog.Info(fmt.Sprintf("检测到新帖子: 标题: %s 链接: %s", post.Title, post.Link))

			// 获取主楼内容
			qq, price, tradeRange, address, phone, err := c.FetchPostMainContent(post.Link)
			if err != nil {
				mylog.Error(fmt.Sprintf("获取主楼内容失败: %v", err))
				// 即使获取主楼失败，也继续处理其他帖子
				message := fmt.Sprintf("标题: %s\n: %s\n", post.Title, post.Link)
				c.processNotification(post.Title, message)
				continue
			}

			// 构建完整消息
			message := fmt.Sprintf("标题: %s\n链接: %s\nQQ: %s\n电话: %s\n价格: %s\n所在地: %s\n交易范围: %s",
				post.Title, post.Link, qq, phone, price, address, tradeRange)

			c.processNotification(post.Title, message)
		}
	}
	return nil
}

// 辅助函数：从链接中提取帖子ID
func extractPostID(link string) string {
	// 假设链接格式为 https://www.chiphell.com/thread-2646639-1-1.html
	parts := strings.Split(link, "-")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// 处理通知的辅助方法
func (c *ChiphellMonitor) processNotification(title, message string) {
	// 获取当前可用代理数量
	count, err := proxy.GetProxyCount()
	proxyCount := 0
	if err == nil {
		proxyCount = int(count)
	}

	// 获取优选代理数量
	preferredCount, err := redis.GetPreferredProxyCount()
	preferredProxyCount := 0
	if err == nil {
		preferredProxyCount = int(preferredCount)
	}

	// 获取当前系统时间
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// 在消息末尾添加系统信息
	messageWithInfo := fmt.Sprintf("%s\n系统信息:\n当前时间: %s\n可用代理数: %d\n优选代理数: %d\n",
		message,
		currentTime,
		proxyCount,
		preferredProxyCount,
	)

	// 收集所有关注该帖子的手机号
	var phoneNumbers []string

	// 遍历用户的关键词进行匹配
	for phoneNumber, keywords := range c.UserKeywords {
		for _, keyword := range keywords {
			lowerKeyword := strings.ToLower(keyword)
			if strings.Contains(strings.ToLower(title), lowerKeyword) {
				phoneNumbers = append(phoneNumbers, phoneNumber)
				break
			}
		}
	}

	// 发送通知
	if len(phoneNumbers) > 0 {
		c.enqueueNotification(title, messageWithInfo, phoneNumbers)
	} else {
		c.enqueueNotification(title, messageWithInfo, nil)
	}
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

		// 正常处理完毕，等待一段时间后再进行一次监控
		waitTime := time.Duration(c.WaitTimeRange.Min+rand.Intn(c.WaitTimeRange.Max-c.WaitTimeRange.Min+1)) * time.Second
		time.Sleep(waitTime)

		// 定期清理数据库中过期的帖子
		// c.Database.CleanUpOldPosts(720 * time.Hour)
	}
}

package monitor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/langchou/informer/db"
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
}

type NotificationMessage struct {
	Title         string
	Message       string
	AtPhoneNumber []string
}

func NewChiphellMonitor(forumName string, cookies string, userKeywords map[string][]string, notifier *notifier.DingTalkNotifier, database *db.Database, logger *mylog.Logger, waitTimeRange struct{ Min, Max int }) *ChiphellMonitor {
	monitor := &ChiphellMonitor{
		ForumName:     forumName,
		Cookies:       cookies,
		UserKeywords:  userKeywords,
		Notifier:      notifier,
		Database:      database,
		Logger:        logger,
		MessageQueue:  make(chan NotificationMessage, 100), // 创建消息队列，容量100
		WaitTimeRange: waitTimeRange,
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
				c.Logger.Error(fmt.Sprintf("发送钉钉通知失败: %v", err))
			} else {
				c.Logger.Info(fmt.Sprintf("成功发送消息: %s", msg.Message))
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
	proxies, err := proxy.FetchProxies() // 获取代理池中的代理
	if err != nil {
		return "", fmt.Errorf("获取代理池失败: %v", err)
	}

	// 使用 WaitGroup 等待所有并发请求完成
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background()) // 用于取消其他goroutines的上下文
	defer cancel()

	resultCh := make(chan string, 1)        // 用于接收第一个有效响应
	errCh := make(chan error, len(proxies)) // 用于接收所有错误

	// 并发请求所有代理 IP
	for _, proxyIP := range proxies {
		wg.Add(1)
		go func(proxyIP string) {
			defer wg.Done()
			// 检查上下文是否已经取消，如果已取消则退出
			select {
			case <-ctx.Done():
				return
			default:
			}

			content, err := c.fetchWithProxy(proxyIP)
			if err != nil {
				errCh <- err
				return
			}

			// 第一个有效结果，取消其他请求
			select {
			case resultCh <- content:
				cancel() // 成功获取结果，取消其他请求
			case <-ctx.Done():
				// 上下文已经取消，不需要处理
			}
		}(proxyIP)
	}

	// 等待所有 goroutines 完成
	go func() {
		wg.Wait()
		close(resultCh)
		close(errCh)
	}()

	// 返回第一个成功的结果，或全部失败时重新获取代理池
	select {
	case content := <-resultCh:
		return content, nil
	case <-time.After(30 * time.Second): // 超时时间为 30 秒，避免所有请求都失败时挂起
		return "", errors.New("所有代理请求超时或失败")
	}
}

func (c *ChiphellMonitor) fetchWithProxy(proxyIP string) (string, error) {
	proxyURL := proxy.ParseProxyURL(proxyIP)
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
	//c.Logger.Info(fmt.Sprintf("成功使用代理 IP: %s", proxyIP))

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
	client := &http.Client{}
	req, err := http.NewRequest("GET", postURL, nil)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Cookie", c.Cookies)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", "", fmt.Errorf("无效的响应状态码: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("解析 HTML 失败: %v", err)
	}

	// 提取信息
	var qq, price, tradeRange, address, phone string

	doc.Find(".typeoption tbody").Each(func(i int, s *goquery.Selection) {
		s.Find("tr").Each(func(i int, tr *goquery.Selection) {
			th := tr.Find("th").Text()
			td := tr.Find("td").Text()

			switch th {
			case "所在地":
				address = td
			case "电话":
				phone = td
			case "QQ:":
				qq = td
			case "价格:":
				price = td
			case "交易范围:":
				tradeRange = td
			}
		})
	})

	return qq, price, tradeRange, address, phone, nil
}

func (c *ChiphellMonitor) ProcessPosts(posts []Post) error {
	for _, post := range posts {
		postHash := util.HashString(post.Title)

		if c.Database.IsNewPost(c.ForumName, postHash) {
			c.Database.StorePostHash(c.ForumName, postHash)
			c.Logger.Info(fmt.Sprintf("检测到新帖子: 标题: %s 链接: %s", post.Title, post.Link))

			// 获取主楼内容
			qq, price, tradeRange, address, phone, err := c.FetchPostMainContent(post.Link)
			if err != nil {
				c.Logger.Error(fmt.Sprintf("获取主楼内容失败: %v", err))
				continue
			}

			message := fmt.Sprintf("标题: %s\n链接: %s\nqq:%s\n电话: %s\n价格: %s\n所在地: %s\n交易范围: %s\n", post.Title, post.Link, qq, phone, price, address, tradeRange)

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
	maxFailedAttempts := 3
	normalWaitTime := 10 * time.Second
	retryWaitTime := 10 * time.Minute

	for {
		content, err := c.FetchPageContent()
		if err != nil {
			failedAttempts++
			c.Logger.Error("获取页面内容失败", "error", err)
			c.Notifier.ReportError("获取页面内容失败", err.Error())

			// 达到最大失败次数时等待较长时间重试
			if failedAttempts >= maxFailedAttempts {
				c.Logger.Info(fmt.Sprintf("连续请求失败 %d 次，等待 %v 后重新尝试", failedAttempts, retryWaitTime))
				time.Sleep(retryWaitTime)
			} else {
				c.Logger.Info(fmt.Sprintf("请求失败 %d 次，等待 %v 后重试", failedAttempts, normalWaitTime))
				time.Sleep(normalWaitTime)
			}
			continue
		}

		// 请求成功，重置失败计数
		failedAttempts = 0

		posts, err := c.ParseContent(content)
		if err != nil {
			c.Logger.Error("解析页面内容失败", "error", err)
			c.Notifier.ReportError("解析页面内容失败", err.Error())
			continue
		}

		err = c.ProcessPosts(posts)
		if err != nil {
			c.Logger.Error("处理帖子失败", "error", err)
			c.Notifier.ReportError("处理帖子失败", err.Error())
		}

		// 正常处理完毕，等待一段时间后再进行下一次监控
		waitTime := time.Duration(c.WaitTimeRange.Min+rand.Intn(c.WaitTimeRange.Max-c.WaitTimeRange.Min+1)) * time.Second
		time.Sleep(waitTime)

		// 定期清理数据库中过期的帖子
		// c.Database.CleanUpOldPosts(720 * time.Hour)
	}
}

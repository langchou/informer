package monitor

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/langchou/informer/db"
	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/notifier"
	"github.com/langchou/informer/pkg/util"
	"golang.org/x/exp/rand"
)

type ChiphellMonitor struct {
	Cookies       string
	Categories    []string
	UserKeywords  map[string][]string // 手机号及其关键词的映射
	Notifier      *notifier.DingTalkNotifier
	Database      *db.Database
	Logger        *mylog.Logger
	WaitTimeRange struct {
		Min int
		Max int
	}
}

func (c *ChiphellMonitor) FetchPageContent() (string, error) {
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
		category := s.Find("em > a").Text()
		postLink := s.Find("a.s.xst")
		postTitle := postLink.Text()

		postHref, exists := postLink.Attr("href")
		if exists && c.shouldMonitorCategory(category) {
			posts = append(posts, Post{
				Title:    postTitle,
				Link:     "https://www.chiphell.com/" + postHref,
				Category: category,
			})
		}
	})
	return posts, nil
}

func (c *ChiphellMonitor) ProcessPosts(posts []Post) error {
	for _, post := range posts {
		postHash := util.HashString(post.Title)

		if c.Database.IsNewPost(postHash) {
			c.Database.StorePostHash(postHash)
			message := fmt.Sprintf("类别: %s\n标题: %s\n链接: %s", post.Category, post.Title, post.Link)

			// 遍历用户的关键词进行匹配
			for phoneNumber, keywords := range c.UserKeywords {
				for _, keyword := range keywords {
					lowerKeyword := strings.ToLower(keyword)

					if strings.Contains(strings.ToLower(post.Title), lowerKeyword) {
						c.Notifier.SendNotification(post.Title, message, phoneNumber)
						time.Sleep(3 * time.Second) // 添加3秒的等待以控制发送频率
						break
					}
				}
			}

			c.Logger.Info(fmt.Sprintf("检测到新帖子: 类别: %s 标题: %s 链接: %s", post.Category, post.Title, post.Link))
		}
	}
	return nil
}

func (c *ChiphellMonitor) shouldMonitorCategory(category string) bool {
	for _, monitoredCategory := range c.Categories {
		if strings.Contains(category, monitoredCategory) {
			return true
		}
	}
	return false
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
		c.Logger.Info("等待 %v 后开始下一轮监控", waitTime)
		time.Sleep(waitTime)

		// 定期清理数据库中过期的帖子
		c.Database.CleanUpOldPosts(720 * time.Hour)
	}
}

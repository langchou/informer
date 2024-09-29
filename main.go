package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/langchou/informer/db"
	"github.com/langchou/informer/pkg/config"
	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/notifier"

	"github.com/PuerkitoBio/goquery"

	_ "github.com/mattn/go-sqlite3" // SQLite 驱动
	"github.com/spf13/viper"
)

type Message struct {
	title   string
	content string
}

var (
	messageQueue      = make(chan Message, 100) // 消息队列，缓冲大小为100
	failedAttempts    = 0                       // 连续请求失败次数
	retryWaitTime     = 10 * time.Minute        // 失败后等待时间
	normalWaitTime    = 10 * time.Second        // 正常监控的等待时间
	maxFailedAttempts = 3                       // 最大失败次数，超过后才会等待较长时间
	applog            *mylog.Logger             // 全局日志实例
)

const dbFile = "data/posts.db"

func main() {
	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	applog = mylog.InitLogger(
		cfg.LogConfig.File,
		cfg.LogConfig.MaxSize,
		cfg.LogConfig.MaxBackups,
		cfg.LogConfig.MaxAge,
		cfg.LogConfig.Compress,
	)
	defer applog.Sync()

	// 日志输出示例
	applog.Info("读取的配置",
		"Cookies", cfg.Cookies,
		"钉钉 Token", cfg.DingTalkToken,
		"钉钉 Secret", cfg.DingTalkSecret,
		"Monitored Categories", cfg.MonitoredCategories,
		"User Keywords", cfg.UserKeywords,
	)

	db, err := db.InitDB(dbFile, applog)
	if err != nil {
		applog.Error("无法初始化数据库", "error", err)
		return
	}
	defer db.DB.Close()

	err = db.CreateTableIfNotExists()
	if err != nil {
		applog.Error("无法创建表 posts", "error", err)
		return
	}

	// 初始化 DingTalk 客户端
	dingNotifier := notifier.NewDingTalkNotifier(cfg.DingTalkToken, cfg.DingTalkSecret, applog)
	// 启动一个 goroutine 用于处理消息队列
	go handleMessages(dingNotifier)

	for {
		monitorPage(db, cfg.Cookies, dingNotifier, cfg.MonitoredCategories)

		// 根据请求是否成功调整等待时间
		if failedAttempts == 0 {
			waitTime := time.Duration(10+rand.Intn(5)) * time.Second
			time.Sleep(waitTime)
		} else {
			applog.Info("请求失败次数 %d 次，等待 %v 后重新尝试", failedAttempts, retryWaitTime)
			time.Sleep(retryWaitTime)
		}

		// 定期清理数据库中过期的帖子记录
		db.CleanUpOldPosts(720 * time.Hour)
	}
}

func monitorPage(database *db.Database, cookies string, dingNotifier *notifier.DingTalkNotifier, monitoredCategories []string) {
	client := &http.Client{}
	url := "https://www.chiphell.com/forum-26-1.html"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		dingNotifier.ReportError("创建请求失败", err.Error())
		return
	}

	req.Header.Set("Cookie", cookies)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		handleRequestFailure(dingNotifier, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		handleRequestFailure(dingNotifier, fmt.Sprintf("状态码: %d", resp.StatusCode))
		return
	}

	// 重置失败计数器
	failedAttempts = 0

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		dingNotifier.ReportError("解析 HTML 失败", err.Error())
		return
	}

	// 读取用户关键词监控配置
	userKeywords := viper.GetStringMapStringSlice("user_keywords")

	// 遍历帖子
	doc.Find("tbody[id^='normalthread_']").Each(func(i int, s *goquery.Selection) {
		category := s.Find("em > a").Text()
		postLink := s.Find("a.s.xst")
		postTitle := postLink.Text()

		// 标题转换成小写
		postTitle = strings.ToLower(postTitle)

		postHref, exists := postLink.Attr("href")
		if exists && shouldMonitorCategory(category, monitoredCategories) {
			postHash := hashString(postTitle)
			if database.IsNewPost(postHash) {
				database.StorePostHash(postHash)
				message := fmt.Sprintf("类别: %s\n标题: %s\n链接: https://www.chiphell.com/%s", category, postTitle, postHref)
				applog.Info("检测到新帖子:", message)

				// 遍历每个手机号及其关键词
				for phoneNumber, keywords := range userKeywords {
					for _, keyword := range keywords {

						// 转换关键词为小写，进行匹配
						lowerKeyword := strings.ToLower(keyword)

						if strings.Contains(postTitle, lowerKeyword) {
							// 如果标题中包含关键词，则 @ 对应的手机号用户
							dingNotifier.SendNotification(postTitle, message, phoneNumber)
							break // 每个关键词匹配一次即可
						}
					}
				}

				// 将消息添加到队列，处理其他不包含关键词的情况
				messageQueue <- Message{title: postTitle, content: message}
			}
		}
	})
}

func handleMessages(dingNotifier *notifier.DingTalkNotifier) {
	ticker := time.NewTicker(time.Minute / 20) // 每 3 秒发送一条消息
	defer ticker.Stop()

	for {
		select {
		case msg := <-messageQueue:
			<-ticker.C // 等待限速
			dingNotifier.SendNotification(msg.title, msg.content)
		}
	}
}

func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", hash)
}

// 过滤帖子类别
func shouldMonitorCategory(category string, monitoredCategories []string) bool {
	for _, monitoredCategory := range monitoredCategories {
		if strings.Contains(category, monitoredCategory) {
			return true
		}
	}
	return false
}

func handleRequestFailure(dingNotifier *notifier.DingTalkNotifier, errorMsg string) {
	failedAttempts++

	if failedAttempts <= maxFailedAttempts {
		// 第一次失败时立即发送通知
		dingNotifier.ReportError("请求失败", errorMsg)
	} else {
		// 失败超过一次后，减少发送频率
		applog.Error("连续请求失败，等待下次重试。")
	}
}

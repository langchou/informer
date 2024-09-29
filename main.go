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

	"github.com/PuerkitoBio/goquery"
	"github.com/blinkbean/dingtalk"

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
)

const dbFile = "data/posts.db"
const chiphellDB = "chiphell"

func main() {
	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	mylog.InitLogger(cfg.LogConfig)

	db, err := db.InitDB(dbFile)
	if err != nil {
		log.Fatalf("无法初始化数据库: %v", err)
	}
	defer db.DB.Close()

	err = db.CreateTableIfNotExists()
	if err != nil {
		log.Fatalf("无法创建表 posts %v", err)
	}

	fmt.Println("读取的配置:")
	fmt.Println("Cookies:", cfg.Cookies)
	fmt.Println("钉钉 Token:", cfg.DingTalkToken)
	fmt.Println("钉钉 Secret:", cfg.DingTalkSecret)
	fmt.Println("Monitored Categories:", cfg.MonitoredCategories)
	fmt.Println("UserKeywords:", cfg.UserKeywords)

	// 初始化 DingTalk 客户端
	dingTalkClient := dingtalk.InitDingTalkWithSecret(cfg.DingTalkToken, cfg.DingTalkSecret)

	// 启动一个 goroutine 用于处理消息队列
	go handleMessages(dingTalkClient)

	for {
		monitorPage(db, cfg.Cookies, dingTalkClient, cfg.MonitoredCategories)

		// 根据请求是否成功调整等待时间
		if failedAttempts == 0 {
			waitTime := time.Duration(10+rand.Intn(5)) * time.Second
			time.Sleep(waitTime)
		} else {
			log.Printf("请求失败次数 %d 次，等待 %v 后重新尝试", failedAttempts, retryWaitTime)
			time.Sleep(retryWaitTime)
		}

		// 定期清理数据库中过期的帖子记录
		db.CleanUpOldPosts(720 * time.Hour)
	}
}

func monitorPage(database *db.Database, cookies string, dingTalkClient *dingtalk.DingTalk, monitoredCategories []string) {
	client := &http.Client{}
	url := "https://www.chiphell.com/forum-26-1.html"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		reportError(dingTalkClient, "创建请求失败", err.Error())
		return
	}

	req.Header.Set("Cookie", cookies)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		handleRequestFailure(dingTalkClient, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		handleRequestFailure(dingTalkClient, fmt.Sprintf("状态码: %d", resp.StatusCode))
		return
	}

	// 重置失败计数器
	failedAttempts = 0

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		reportError(dingTalkClient, "解析 HTML 失败", err.Error())
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
				log.Println("检测到新帖子:", message)

				// 遍历每个手机号及其关键词
				for phoneNumber, keywords := range userKeywords {
					for _, keyword := range keywords {

						// 转换关键词为小写，进行匹配
						lowerKeyword := strings.ToLower(keyword)

						if strings.Contains(postTitle, lowerKeyword) {
							// 如果标题中包含关键词，则 @ 对应的手机号用户
							sendDingTalkNotification(dingTalkClient, postTitle, message, phoneNumber)
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

func handleMessages(dingTalkClient *dingtalk.DingTalk) {
	ticker := time.NewTicker(time.Minute / 20) // 每 3 秒发送一条消息
	defer ticker.Stop()

	for {
		select {
		case msg := <-messageQueue:
			<-ticker.C // 等待限速
			sendDingTalkNotification(dingTalkClient, msg.title, msg.content)
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

func sendDingTalkNotification(dingTalkClient *dingtalk.DingTalk, title, content string, phoneNumber ...string) {
	msg := fmt.Sprintf("%s\n%s", title, content)
	var err error
	if len(phoneNumber) > 0 && phoneNumber[0] != "" {
		// @ 对应手机号用户
		err = dingTalkClient.SendTextMessage(msg, dingtalk.WithAtMobiles([]string{phoneNumber[0]}))
	} else {
		// 发送普通通知
		err = dingTalkClient.SendTextMessage(msg)
	}

	if err != nil {
		log.Println("发送钉钉通知失败:", err)
	} else {
		log.Println("钉钉通知发送成功")
	}
}

func sendDingTalkNotificationforSomeOne(dingTalkClient *dingtalk.DingTalk, title, content string, phoneNumber string) {
	msg := fmt.Sprintf("%s\n%s", title, content)
	err := dingTalkClient.SendTextMessage(msg, dingtalk.WithAtMobiles([]string{phoneNumber}))

	if err != nil {
		log.Println("发送钉钉通知失败:", err)
	} else {
		log.Println("钉钉通知发送成功")
	}
}

func handleRequestFailure(dingTalkClient *dingtalk.DingTalk, errorMsg string) {
	failedAttempts++

	if failedAttempts <= maxFailedAttempts {
		// 第一次失败时立即发送通知
		reportError(dingTalkClient, "请求失败", errorMsg)
	} else {
		// 失败超过一次后，减少发送频率
		log.Println("连续请求失败，等待下次重试。")
	}
}

func reportError(dingTalkClient *dingtalk.DingTalk, title, content string) {
	log.Printf("错误: %s - %s\n", title, content)
	// 将错误信息也添加到消息队列
	messageQueue <- Message{title: "监控程序错误: " + title, content: content}
}

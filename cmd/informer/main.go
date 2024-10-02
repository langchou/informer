package main

import (
	"log"
	"time"

	"github.com/langchou/informer/db"
	"github.com/langchou/informer/internal/monitor"
	"github.com/langchou/informer/pkg/config"
	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/notifier"
	"golang.org/x/exp/rand"

	_ "github.com/mattn/go-sqlite3" // SQLite 驱动
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
	applog.Info(
		"读取的配置\nCookies: %s\n钉钉 Token: %s\n钉钉 Secret: %s\nMonitored Categories: %v\nUser Keywords: %v",
		cfg.Chiphell.Cookies,
		cfg.DingTalk.Token,
		cfg.DingTalk.Secret,
		cfg.Chiphell.MonitoredCategories,
		cfg.Chiphell.UserKeywords,
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
	dingNotifier := notifier.NewDingTalkNotifier(cfg.DingTalk.Token, cfg.DingTalk.Secret, applog)

	chiphellMonitor := monitor.NewChiphellMonitor(
		cfg.Chiphell.Cookies,
		cfg.Chiphell.MonitoredCategories,
		cfg.Chiphell.UserKeywords,
		dingNotifier,
		db,
		applog,
		cfg.Chiphell.WaitTimeRange, // 从配置中读取等待时间范围
	)

	for {
		chiphellMonitor.MonitorPage()
		// 正常监控的等待时间，随机在 10 到 15 秒之间
		waitTime := time.Duration(10+rand.Intn(5)) * time.Second
		time.Sleep(waitTime)
	}
}

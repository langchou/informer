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

var applog *mylog.Logger

const dbFile = "data/posts.db"

func main() {
	cfg, err := config.InitConfig()
	if err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	mylog.InitLogger(
		cfg.LogConfig.File,
		cfg.LogConfig.MaxSize,
		cfg.LogConfig.MaxBackups,
		cfg.LogConfig.MaxAge,
		cfg.LogConfig.Compress,
	)

	defer mylog.Sync()

	db, err := db.InitDB(dbFile)
	if err != nil {
		mylog.Error("无法初始化数据库", "error", err)
		return
	}
	defer db.DB.Close()

	// 创建每个论坛的表
	for _, forum := range []string{"chiphell"} {
		err := db.CreateTableIfNotExists(forum)
		if err != nil {
			mylog.Error("无法创建论坛表", "forum", forum, "error", err)
			return
		}
	}

	// 初始化 DingTalk 客户端
	dingNotifier := notifier.NewDingTalkNotifier(cfg.DingTalk.Token, cfg.DingTalk.Secret, applog)

	for _, forum := range []string{"chiphell"} {
		go func(forum string) {
			forumConfig, ok := cfg.Forums[forum]
			if !ok {
				mylog.Error("没有找到 %s 论坛的配置", forum)
				return
			}

			monitor := monitor.NewMonitor(
				forum,
				forumConfig.Cookies,
				forumConfig.UserKeywords,
				dingNotifier,
				db,
				struct{ Min, Max int }{
					Min: forumConfig.WaitTimeRange.Min,
					Max: forumConfig.WaitTimeRange.Max,
				},
				cfg.ProxyPoolAPI,
			)

			if monitor != nil {
				for {
					monitor.MonitorPage()
					waitTime := time.Duration(10+rand.Intn(5)) * time.Second
					time.Sleep(waitTime)
				}
			}
		}(forum)
	}

	select {}
}

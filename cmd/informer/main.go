package main

import (
	"context"
	"log"
	"time"

	"github.com/langchou/informer/db"
	"github.com/langchou/informer/internal/monitor"
	"github.com/langchou/informer/pkg/config"
	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/notifier"
	"github.com/langchou/informer/pkg/proxy"
	"github.com/langchou/informer/pkg/redis"
	"golang.org/x/exp/rand"

	_ "github.com/mattn/go-sqlite3"
)

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
		cfg.LogConfig.Level,
	)

	defer mylog.Sync()

	db, err := db.InitDB(dbFile)
	if err != nil {
		mylog.Error("无法初始化数据库", "error", err)
		return
	}
	defer db.DB.Close()

	// 创建chiphell表
	err = db.CreateTableIfNotExists("chiphell")
	if err != nil {
		mylog.Error("无法创建数据表", "error", err)
		return
	}

	// 初始化 DingTalk 客户端
	dingNotifier := notifier.NewDingTalkNotifier(cfg.DingTalk.Token, cfg.DingTalk.Secret)

	// 初始化Redis连接
	err = redis.InitRedis(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		mylog.Error("无法初始化Redis连接", "error", err)
		return
	}

	// 设置 ProxyAPI
	proxy.SetProxyAPI(cfg.ProxyPoolAPI)

	// 初始化时更新一次代理池
	if err := proxy.UpdateProxyPool(); err != nil {
		mylog.Error("初始化代理池失败", "error", err)
	}

	// 启动代理池管理器
	ctx := context.Background()
	go proxy.StartProxyPoolManager(ctx)

	// 启动IP检测器
	go proxy.StartIPChecker(ctx)

	// 创建并启动监控器
	monitor := monitor.NewMonitor(
		cfg.Cookies,
		cfg.UserKeyWords,
		dingNotifier,
		db,
		cfg.WaitTimeRange,
		cfg.ProxyPoolAPI,
	)

	// 主循环
	for {
		monitor.MonitorPage()
		// 添加随机等待时间
		waitTime := time.Duration(cfg.WaitTimeRange.Min+rand.Intn(cfg.WaitTimeRange.Max-cfg.WaitTimeRange.Min)) * time.Second
		mylog.Debug("等待 %v 后继续监控", waitTime)
		time.Sleep(waitTime)
	}
}

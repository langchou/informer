package config

import (
	"log"

	"github.com/spf13/viper"
)

type LogConfig struct {
	File       string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
}

type DingTalkConfig struct {
	Token  string
	Secret string
}

type ChiphellConfig struct {
	Cookies             string
	MonitoredCategories []string
	UserKeywords        map[string][]string
	WaitTimeRange       struct {
		Min int
		Max int
	}
}

type Config struct {
	LogConfig LogConfig
	DingTalk  DingTalkConfig
	Chiphell  ChiphellConfig
}

func InitConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("data") // 配置文件所在路径

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("无法读取配置文件: %v", err)
		return nil, err
	}

	dingTalkConfig := DingTalkConfig{
		Token:  viper.GetString("dingtalk.token"),
		Secret: viper.GetString("dingtalk.secret"),
	}

	chiphellConfig := ChiphellConfig{
		Cookies:             viper.GetString("chiphell.cookies"),
		MonitoredCategories: viper.GetStringSlice("chiphell.monitored_categories"),
		UserKeywords:        viper.GetStringMapStringSlice("chiphell.user_keywords"),
		WaitTimeRange: struct {
			Min int
			Max int
		}{
			Min: viper.GetInt("chiphell.wait_time_range.min"),
			Max: viper.GetInt("chiphell.wait_time_range.max"),
		},
	}

	viper.SetDefault("chiphell.monitored_categories", []string{"显卡", "处理器主板内存", "笔记本/平板", "手机通讯", "影音娱乐", "游戏设备", "网络设备", "外设"})
	viper.SetDefault("chiphell.user_keywords", map[string][]string{})
	viper.SetDefault("chiphell.wait_time_range.min", 5)
	viper.SetDefault("chiphell.wait_time_range.max", 10)

	logConfig := LogConfig{
		File:       viper.GetString("log.file"),
		MaxSize:    viper.GetInt("log.max_size"),
		MaxBackups: viper.GetInt("log.max_backups"),
		MaxAge:     viper.GetInt("log.max_age"),
		Compress:   viper.GetBool("log.compress"),
	}

	// 构造最终的 Config 对象
	config := &Config{
		LogConfig: logConfig,
		DingTalk:  dingTalkConfig,
		Chiphell:  chiphellConfig,
	}

	return config, nil
}

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

type Config struct {
	Cookies             string
	DingTalkToken       string
	DingTalkSecret      string
	MonitoredCategories []string
	UserKeywords        map[string][]string
	LogConfig           LogConfig
}

func InitConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("data")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("无法读取配置文件: %v", err)
		return nil, err
	}

	viper.SetEnvPrefix("app")

	viper.BindEnv("cookies")
	viper.BindEnv("dingtalk_token")
	viper.BindEnv("dingtalk_secret")
	viper.BindEnv("monitored_categories")
	viper.BindEnv("user_keywords")

	viper.SetDefault("monitored_categories", []string{"显卡", "处理器主板内存", "笔记本/平板", "手机通讯", "影音娱乐", "游戏设备", "网络设备", "外设"})
	viper.SetDefault("user_keywords", map[string][]string{})

	config := &Config{
		Cookies:             viper.GetString("cookies"),
		DingTalkToken:       viper.GetString("dingtalk_token"),
		DingTalkSecret:      viper.GetString("dingtalk_secret"),
		MonitoredCategories: viper.GetStringSlice("monitored_categories"),
		UserKeywords:        viper.GetStringMapStringSlice("user_keywords"),
		LogConfig: LogConfig{
			File:       viper.GetString("log.file"),
			MaxSize:    viper.GetInt("log.max_size"),
			MaxBackups: viper.GetInt("log.max_backups"),
			MaxAge:     viper.GetInt("log.max_age"),
			Compress:   viper.GetBool("log.compress"),
		},
	}

	return config, nil
}

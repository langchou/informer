package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type LogConfig struct {
	File       string `yaml:"file"`
	MaxSize    int    `yaml:"maxSize"`
	MaxBackups int    `yaml:"maxBackups"`
	MaxAge     int    `yaml:"maxAge"`
	Compress   bool   `yaml:"compress"`
	Level      string `yaml:"level"`
}

type DingTalkConfig struct {
	Token  string `yaml:"token"`
	Secret string `yaml:"secret"`
}

type ForumConfig struct {
	Cookies       string              `yaml:"cookies"`
	UserKeywords  map[string][]string `yaml:"userKeywords"`
	WaitTimeRange struct {
		Min int `yaml:"min"`
		Max int `yaml:"max"`
	} `yaml:"waitTimeRange"`
}

type Config struct {
	LogConfig    LogConfig              `yaml:"log"`
	DingTalk     DingTalkConfig         `yaml:"dingtalk"`
	Forums       map[string]ForumConfig `yaml:"forums"`
	ProxyPoolAPI string                 `yaml:"proxyPoolAPI"`
	Redis        RedisConfig            `yaml:"redis"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
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
	viper.SetDefault("logconfig.level", "info")
	viper.SetDefault("chiphell.wait_time_range.min", 5)
	viper.SetDefault("chiphell.wait_time_range.max", 10)

	// 定义配置对象
	var config Config

	// 将viper解析到配置结构体中
	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatalf("无法解析配置文件到结构体: %v", err)
		return nil, err
	}

	fmt.Printf("%+v\n", config)

	return &config, nil
}

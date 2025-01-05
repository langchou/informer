package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	LogConfig struct {
		File       string `yaml:"file"`
		MaxSize    int    `yaml:"maxSize"`
		MaxBackups int    `yaml:"maxBackups"`
		MaxAge     int    `yaml:"maxAge"`
		Compress   bool   `yaml:"compress"`
		Level      string `yaml:"level"`
	} `yaml:"logconfig"`

	DingTalk struct {
		Token  string `yaml:"token"`
		Secret string `yaml:"secret"`
	} `yaml:"dingtalk"`

	ProxyPoolAPI string `yaml:"proxyPoolAPI"`
	Cookies      string `yaml:"cookies"`

	UserKeyWords map[string][]string `yaml:"userKeyWords"`

	WaitTimeRange struct {
		Min int `yaml:"min"`
		Max int `yaml:"max"`
	} `yaml:"waitTimeRange"`
}

func InitConfig() (*Config, error) {
	configFile := "data/config.yaml"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("配置文件不存在: %s", configFile)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return &config, nil
}

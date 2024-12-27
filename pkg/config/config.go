package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	LogConfig struct {
		File       string
		MaxSize    int
		MaxBackups int
		MaxAge     int
		Compress   bool
		Level      string
	}
	DingTalk struct {
		Token  string
		Secret string
	}
	ProxyPoolAPI  string
	Cookies       string
	UserKeyWords  map[string][]string
	WaitTimeRange struct {
		Min int
		Max int
	}
	Redis struct {
		Addr     string
		Password string
		DB       int
	}
}

func InitConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("data")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"strings"
)

func doesPathExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	fmt.Println("检查路径时发生错误:", err)
	return false
}

func NewConfig(p string) *viper.Viper {
	envConf := os.Getenv("APP_CONF")
	if envConf == "" {
		envConf = p
	}
	fmt.Println("load conf file:", envConf)
	return getConfig(envConf, ".")
}

func setDefaults(conf *viper.Viper) {
	// HTTP settings
	conf.SetDefault("http.host", "0.0.0.0")
	conf.SetDefault("http.port", 9000)
	conf.SetDefault("http.title", "GPT Mirror")
	conf.SetDefault("http.rate", 100)

	// Database settings
	conf.SetDefault("database.driver", "sqlite")
	conf.SetDefault("database.dsn", "./data/data.db")

	// Automatic account/share maintenance is disabled until the provider layer
	// owns credential refresh and remote share/session behavior.
	conf.SetDefault("account.refresh.enabled", false)
	conf.SetDefault("account.refresh.cron", "")
	conf.SetDefault("share.refresh.enabled", false)

	// Share settings
	conf.SetDefault("share.random", true)
	conf.SetDefault("share.custom", true)

	// One API integration settings
	conf.SetDefault("oneapi.token", "")
	conf.SetDefault("oneapi.domain", "")

	// Log settings
	conf.SetDefault("log.level", "info")
	conf.SetDefault("log.encoding", "console")
	conf.SetDefault("log.output", "console")
	conf.SetDefault("log.log_file_name", "./logs/server.log")
	conf.SetDefault("log.max_backups", 30)
	conf.SetDefault("log.max_age", 7)
	conf.SetDefault("log.max_size", 1024)
	conf.SetDefault("log.compress", true)
}

func getConfig(path ...string) *viper.Viper {
	conf := viper.New()
	conf.SetConfigName("config")
	for _, p := range path {
		if !doesPathExist(p) {
			continue
		}
		conf.AddConfigPath(p)
	}
	if err := conf.ReadInConfig(); err != nil {
		panic(err)
	}
	setDefaults(conf)
	if err := conf.BindEnv("security.admin_password", "ADMIN_PASSWORD"); err != nil {
		return nil
	}
	conf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	conf.AutomaticEnv()
	conf.WatchConfig()
	return conf
}

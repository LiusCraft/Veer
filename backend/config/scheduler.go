package config

import (
	"github.com/spf13/viper"
)

// SchedulerConfig Scheduler 服务专有配置
type SchedulerConfig struct {
	Service         ServiceConfig  `mapstructure:"service"`
	Database        DatabaseConfig `mapstructure:"database"`
	RefreshInterval int            `mapstructure:"refresh_interval" default:"10"`
}

// LoadSchedulerConfig 加载 Scheduler 配置（config-scheduler.yaml）
func LoadSchedulerConfig() (*SchedulerConfig, error) {
	viper.SetConfigName("config-scheduler")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("CDNC")
	viper.AutomaticEnv()

	bindEnv(
		"service.host", "CDNC_SERVICE_HOST",
		"service.port", "CDNC_SERVICE_PORT",
		"database.path", "CDNC_DATABASE_PATH",
		"refresh_interval", "CDNC_REFRESH_INTERVAL",
	)

	setSchedulerDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg SchedulerConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setSchedulerDefaults() {
	viper.SetDefault("service.host", "0.0.0.0")
	viper.SetDefault("service.port", 8081)
	viper.SetDefault("database.path", "./veer.db")
	viper.SetDefault("refresh_interval", 10)
}

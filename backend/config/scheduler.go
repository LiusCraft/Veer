package config

import (
	"github.com/spf13/viper"
)

type GeoIPConfig struct {
	Enabled bool   `mapstructure:"enabled" default:"false"`
	DBPath  string `mapstructure:"db_path" default:"./data/ip2region.xdb"`
}

type SchedulingCostConfig struct {
	Distance DistanceCostConfig `mapstructure:"distance"`
}

// SchedulerConfig Scheduler 服务专有配置
type SchedulerConfig struct {
	Service         ServiceConfig        `mapstructure:"service"`
	Database        DatabaseConfig       `mapstructure:"database"`
	RefreshInterval int                  `mapstructure:"refresh_interval" default:"10"`
	GeoIP           GeoIPConfig          `mapstructure:"geoip"`
	SchedulingCost  SchedulingCostConfig `mapstructure:"scheduling"`
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
		"geoip.enabled", "CDNC_GEOIP_ENABLED",
		"geoip.db_path", "CDNC_GEOIP_DB_PATH",
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
	viper.SetDefault("geoip.enabled", false)
	viper.SetDefault("geoip.db_path", "./data/ip2region.xdb")
	viper.SetDefault("scheduling.cost.distance.same_province", 1.0)
	viper.SetDefault("scheduling.cost.distance.same_region", 1.2)
	viper.SetDefault("scheduling.cost.distance.cross_region", 1.5)
}

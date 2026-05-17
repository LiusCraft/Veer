// Package config provides configuration structures and loading functionality for the Veer backend.
//
// 本模块使用 Viper 实现配置管理，支持从 config.yaml 加载配置并支持环境变量覆盖。
// 环境变量需以 CDNC_ 为前缀，例如 CDNC_SERVER_PORT 可覆盖 server.port 配置。
package config

import (
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DatabaseConfig 数据库配置（Manager 与 Scheduler 共享）
type DatabaseConfig struct {
	Path string `mapstructure:"path" default:"./veer.db"`
}

// Config 应用程序根配置结构
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Scheduler   SchedulerConfig   `mapstructure:"scheduler"`
	Edge        EdgeConfig        `mapstructure:"edge"`
	Database    DatabaseConfig    `mapstructure:"database"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	Admin       AdminConfig       `mapstructure:"admin"`
	HealthCheck HealthCheckConfig `mapstructure:"health_check"`
	RateLimit   RateLimitConfig   `mapstructure:"rate_limit"`
	Cache       CacheConfig       `mapstructure:"cache"`
}

// InitDB initializes and returns a SQLite database connection with the specified database path.
func InitDB(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// LoadConfig 从配置文件和环境变量加载配置
//
// 使用优先级: 环境变量 > 配置文件 > 代码默认值
// 环境变量需要以 CDNC_ 为前缀，例如 CDNC_SERVER_PORT
//
// configName 指定配置文件名（不含扩展名），例如 "config-manager" 会读取 config-manager.yaml
func LoadConfig(configName string) (*Config, error) {
	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("CDNC")
	viper.AutomaticEnv()

	_ = viper.BindEnv("server.host", "CDNC_SERVER_HOST")
	_ = viper.BindEnv("server.port", "CDNC_SERVER_PORT")
	_ = viper.BindEnv("database.path", "CDNC_DATABASE_PATH")
	_ = viper.BindEnv("jwt.secret", "CDNC_JWT_SECRET")
	_ = viper.BindEnv("jwt.expiry_hours", "CDNC_JWT_EXPIRY_HOURS")
	_ = viper.BindEnv("admin.username", "CDNC_ADMIN_USERNAME")
	_ = viper.BindEnv("admin.password", "CDNC_ADMIN_PASSWORD")
	_ = viper.BindEnv("health_check.enabled", "CDNC_HEALTH_CHECK_ENABLED")
	_ = viper.BindEnv("rate_limit.enabled", "CDNC_RATE_LIMIT_ENABLED")
	_ = viper.BindEnv("rate_limit.requests_per_minute", "CDNC_RATE_LIMIT_REQUESTS_PER_MINUTE")
	_ = viper.BindEnv("scheduler.host", "CDNC_SCHEDULER_HOST")
	_ = viper.BindEnv("scheduler.port", "CDNC_SCHEDULER_PORT")
	_ = viper.BindEnv("scheduler.refresh_interval", "CDNC_SCHEDULER_REFRESH_INTERVAL")

	_ = viper.BindEnv("edge.host", "CDNC_EDGE_HOST")
	_ = viper.BindEnv("edge.port", "CDNC_EDGE_PORT")
	_ = viper.BindEnv("edge.name", "CDNC_EDGE_NAME")
	_ = viper.BindEnv("edge.region", "CDNC_EDGE_REGION")
	_ = viper.BindEnv("edge.public_url", "CDNC_EDGE_PUBLIC_URL")
	_ = viper.BindEnv("edge.manager.url", "CDNC_EDGE_MANAGER_URL")
	_ = viper.BindEnv("edge.manager.secret", "CDNC_EDGE_MANAGER_SECRET")
	_ = viper.BindEnv("edge.origin_base_url", "CDNC_EDGE_ORIGIN_BASE_URL")
	_ = viper.BindEnv("edge.cache.ttl_seconds", "CDNC_EDGE_CACHE_TTL_SECONDS")
	_ = viper.BindEnv("edge.cache.max_size_mb", "CDNC_EDGE_CACHE_MAX_SIZE_MB")
	_ = viper.BindEnv("edge.cache.max_l1_mb", "CDNC_EDGE_CACHE_MAX_L1_MB")
	_ = viper.BindEnv("edge.cache.disk.enabled", "CDNC_EDGE_CACHE_DISK_ENABLED")
	_ = viper.BindEnv("edge.cache.disk.path", "CDNC_EDGE_CACHE_DISK_PATH")
	_ = viper.BindEnv("edge.cache.disk.max_size_gb", "CDNC_EDGE_CACHE_DISK_MAX_SIZE_GB")
	_ = viper.BindEnv("edge.cache.disk.segment_size_mb", "CDNC_EDGE_CACHE_DISK_SEGMENT_SIZE_MB")
	_ = viper.BindEnv("edge.cache.disk.write_buffer_kb", "CDNC_EDGE_CACHE_DISK_WRITE_BUFFER_KB")
	_ = viper.BindEnv("edge.cache.disk.flush_interval_ms", "CDNC_EDGE_CACHE_DISK_FLUSH_INTERVAL_MS")
	_ = viper.BindEnv("edge.cache.disk.debug", "CDNC_EDGE_CACHE_DISK_DEBUG")
	_ = viper.BindEnv("edge.cache.disk.compaction.enabled", "CDNC_EDGE_CACHE_DISK_COMPACTION_ENABLED")
	_ = viper.BindEnv("edge.cache.disk.compaction.watermark", "CDNC_EDGE_CACHE_DISK_COMPACTION_WATERMARK")
	_ = viper.BindEnv("edge.cache.disk.compaction.interval_minutes", "CDNC_EDGE_CACHE_DISK_COMPACTION_INTERVAL_MINUTES")
	_ = viper.BindEnv("edge.cache.disk.compaction.max_segments", "CDNC_EDGE_CACHE_DISK_COMPACTION_MAX_SEGMENTS")
	_ = viper.BindEnv("edge.cache.disk.index.bloom_bits_per_entry", "CDNC_EDGE_CACHE_DISK_INDEX_BLOOM_BITS_PER_ENTRY")
	_ = viper.BindEnv("edge.cache.disk.index.sparse_max_entries", "CDNC_EDGE_CACHE_DISK_INDEX_SPARSE_MAX_ENTRIES")

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// setDefaults 设置所有配置项的默认值
func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)

	viper.SetDefault("database.path", "./veer.db")

	viper.SetDefault("jwt.secret", "change-me-in-production-256bits!")
	viper.SetDefault("jwt.expiry_hours", 24)

	viper.SetDefault("admin.username", "admin")
	viper.SetDefault("admin.password", "admin123")

	viper.SetDefault("health_check.enabled", true)
	viper.SetDefault("health_check.interval_seconds", 30)
	viper.SetDefault("health_check.fail_threshold", 3)
	viper.SetDefault("health_check.timeout_seconds", 5)

	viper.SetDefault("rate_limit.enabled", true)
	viper.SetDefault("rate_limit.requests_per_minute", 60)
	viper.SetDefault("rate_limit.whitelist", []string{"127.0.0.1", "::1"})

	viper.SetDefault("cache.max_age_seconds", 300)

	viper.SetDefault("scheduler.host", "0.0.0.0")
	viper.SetDefault("scheduler.port", 8081)
	viper.SetDefault("scheduler.refresh_interval", 10)

	viper.SetDefault("edge.host", "0.0.0.0")
	viper.SetDefault("edge.port", 8082)
	viper.SetDefault("edge.name", "edge-1")
	viper.SetDefault("edge.region", "default")
	viper.SetDefault("edge.public_url", "http://localhost:8082")
	viper.SetDefault("edge.manager.url", "")
	viper.SetDefault("edge.manager.secret", "veer-edge-secret")
	viper.SetDefault("edge.origin_base_url", "")
	viper.SetDefault("edge.cache.ttl_seconds", 300)
	viper.SetDefault("edge.cache.max_size_mb", 512)
	viper.SetDefault("edge.cache.max_l1_mb", 4096)
	viper.SetDefault("edge.cache.disk.enabled", false)
	viper.SetDefault("edge.cache.disk.path", "./cache")
	viper.SetDefault("edge.cache.disk.max_size_gb", 500)
	viper.SetDefault("edge.cache.disk.segment_size_mb", 512)
	viper.SetDefault("edge.cache.disk.write_buffer_kb", 4096)
	viper.SetDefault("edge.cache.disk.flush_interval_ms", 100)
	viper.SetDefault("edge.cache.disk.debug", false)
	viper.SetDefault("edge.cache.disk.compaction.enabled", true)
	viper.SetDefault("edge.cache.disk.compaction.watermark", 0.85)
	viper.SetDefault("edge.cache.disk.compaction.interval_minutes", 30)
	viper.SetDefault("edge.cache.disk.compaction.max_segments", 200)
	viper.SetDefault("edge.cache.disk.index.bloom_bits_per_entry", 16)
	viper.SetDefault("edge.cache.disk.index.sparse_max_entries", 10000000)
}

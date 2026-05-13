// Package config provides configuration structures and loading functionality for the Veer backend.
//
// 本模块使用 Viper 实现配置管理，支持从 config.yaml 加载配置并支持环境变量覆盖。
// 环境变量需以 CDNC_ 为前缀，例如 CDNC_SERVER_PORT 可覆盖 server.port 配置。
package config

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

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

// ServerConfig Manager 服务配置
type ServerConfig struct {
	Host string `mapstructure:"host" default:"0.0.0.0"`
	Port int    `mapstructure:"port" default:"8080"`
}

// SchedulerConfig 调度服务配置
type SchedulerConfig struct {
	Host            string `mapstructure:"host" default:"0.0.0.0"`
	Port            int    `mapstructure:"port" default:"8081"`
	RefreshInterval int    `mapstructure:"refresh_interval" default:"10"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Path string `mapstructure:"path" default:"./veer.db"`
}

// JWTConfig JWT 认证配置
type JWTConfig struct {
	Secret      string `mapstructure:"secret" default:"change-me-in-production-256bits!"`
	ExpiryHours int    `mapstructure:"expiry_hours" default:"24"`
}

// AdminConfig 管理员账户配置
type AdminConfig struct {
	Username string `mapstructure:"username" default:"admin"`
	Password string `mapstructure:"password" default:"admin123"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled         bool `mapstructure:"enabled" default:"true"`
	IntervalSeconds int  `mapstructure:"interval_seconds" default:"30"`
	FailThreshold   int  `mapstructure:"fail_threshold" default:"3"`
	TimeoutSeconds  int  `mapstructure:"timeout_seconds" default:"5"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled           bool     `mapstructure:"enabled" default:"true"`
	RequestsPerMinute int      `mapstructure:"requests_per_minute" default:"60"`
	Whitelist         []string `mapstructure:"whitelist"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	MaxAgeSeconds int `mapstructure:"max_age_seconds" default:"300"`
}

// EdgeConfig 边缘节点服务配置
type EdgeConfig struct {
	Host          string            `mapstructure:"host" default:"0.0.0.0"`
	Port          int               `mapstructure:"port" default:"8082"`
	Name          string            `mapstructure:"name" default:"edge-1"`
	Region        string            `mapstructure:"region" default:"default"`
	PublicURL     string            `mapstructure:"public_url" default:"http://localhost:8082"` // 对外地址（Manager 用此做 302 跳转）
	Manager       EdgeManagerConfig `mapstructure:"manager"`                                    // Manager 连接配置
	OriginBaseURL string            `mapstructure:"origin_base_url" default:"http://origin:80"` // 回源地址（未从 Manager 拉取时用）
	Cache         EdgeCacheConfig   `mapstructure:"cache"`
	NodeID        uint              // 注册后由 Manager 分配，运行时值不来自配置文件
}

// EdgeManagerConfig 边缘节点连接 Manager 的配置
type EdgeManagerConfig struct {
	URL    string `mapstructure:"url" default:"http://localhost:8080"`
	Secret string `mapstructure:"secret" default:"veer-edge-secret"`
}

// EdgeCacheConfig 边缘节点缓存配置
type EdgeCacheConfig struct {
	TTLSeconds int                 `mapstructure:"ttl_seconds" default:"300"`
	MaxSizeMB  int                 `mapstructure:"max_size_mb" default:"512"`
	MaxL1MB    int                 `mapstructure:"max_l1_mb" default:"4096"`
	Disk       EdgeDiskCacheConfig `mapstructure:"disk"`
}

// EdgeDiskCacheConfig 磁盘缓存层配置
type EdgeDiskCacheConfig struct {
	Enabled         bool                     `mapstructure:"enabled" default:"false"`
	Path            string                   `mapstructure:"path" default:"./cache"`
	MaxSizeGB       int                      `mapstructure:"max_size_gb" default:"500"`
	SegmentSizeMB   int                      `mapstructure:"segment_size_mb" default:"512"`
	WriteBufferKB   int                      `mapstructure:"write_buffer_kb" default:"4096"`
	FlushIntervalMS int                      `mapstructure:"flush_interval_ms" default:"100"`
	Debug           bool                     `mapstructure:"debug" default:"false"`
	Compaction      EdgeDiskCompactionConfig `mapstructure:"compaction"`
	Index           EdgeDiskIndexConfig      `mapstructure:"index"`
}

// EdgeDiskCompactionConfig Compaction 配置
type EdgeDiskCompactionConfig struct {
	Enabled         bool    `mapstructure:"enabled" default:"true"`
	Watermark       float64 `mapstructure:"watermark" default:"0.85"`
	IntervalMinutes int     `mapstructure:"interval_minutes" default:"30"`
	MaxSegments     int     `mapstructure:"max_segments" default:"200"`
}

// EdgeDiskIndexConfig 索引配置
type EdgeDiskIndexConfig struct {
	BloomBitsPerEntry int `mapstructure:"bloom_bits_per_entry" default:"16"`
	SparseMaxEntries  int `mapstructure:"sparse_max_entries" default:"10000000"`
}

// JWTClaims JWT 令牌中的声明结构
type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// LoadConfig 从配置文件和环境变量加载配置
//
// 使用优先级: 环境变量 > 配置文件 > 代码默认值
// 环境变量需要以 CDNC_ 为前缀，例如 CDNC_SERVER_PORT
//
// configName 指定配置文件名（不含扩展名），例如 "config-manager" 会读取 config-manager.yaml
//
// 返回:
//   - *Config: 加载成功的配置对象
//   - error: 加载失败时的错误信息
func LoadConfig(configName string) (*Config, error) {
	// 设置配置文件的路径和名称
	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// 设置环境变量前缀
	viper.SetEnvPrefix("CDNC")
	viper.AutomaticEnv()

	// 绑定环境变量到结构体字段
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

	// Edge env bindings
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

	// 设置默认值
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		// 如果配置文件不存在，使用默认值继续运行
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// 解码配置到结构体
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// setDefaults 设置所有配置项的默认值
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)

	// Database defaults
	viper.SetDefault("database.path", "./veer.db")

	// JWT defaults
	viper.SetDefault("jwt.secret", "change-me-in-production-256bits!")
	viper.SetDefault("jwt.expiry_hours", 24)

	// Admin defaults
	viper.SetDefault("admin.username", "admin")
	viper.SetDefault("admin.password", "admin123")

	// HealthCheck defaults
	viper.SetDefault("health_check.enabled", true)
	viper.SetDefault("health_check.interval_seconds", 30)
	viper.SetDefault("health_check.fail_threshold", 3)
	viper.SetDefault("health_check.timeout_seconds", 5)

	// RateLimit defaults
	viper.SetDefault("rate_limit.enabled", true)
	viper.SetDefault("rate_limit.requests_per_minute", 60)
	viper.SetDefault("rate_limit.whitelist", []string{"127.0.0.1", "::1"})

	// Cache defaults
	viper.SetDefault("cache.max_age_seconds", 300)

	// Scheduler defaults
	viper.SetDefault("scheduler.host", "0.0.0.0")
	viper.SetDefault("scheduler.port", 8081)
	viper.SetDefault("scheduler.refresh_interval", 10)

	// Edge defaults
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

// GetExpiryDuration 返回 JWT 过期时长（作为 time.Duration）
func (c *JWTConfig) GetExpiryDuration() time.Duration {
	if c.ExpiryHours <= 0 {
		c.ExpiryHours = 24
	}
	return time.Duration(c.ExpiryHours) * time.Hour
}

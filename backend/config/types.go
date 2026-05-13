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
	Database    DatabaseConfig    `mapstructure:"database"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	Admin       AdminConfig       `mapstructure:"admin"`
	HealthCheck HealthCheckConfig `mapstructure:"health_check"`
	RateLimit   RateLimitConfig   `mapstructure:"rate_limit"`
	Cache       CacheConfig       `mapstructure:"cache"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `mapstructure:"host" default:"0.0.0.0"`
	Port int    `mapstructure:"port" default:"8080"`
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

// JWTClaims JWT 令牌中的声明结构
type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// LoadConfig 从配置文件和环境变量加载配置
//
// 使用优先级: 环境变量 > config.yaml > 代码默认值
// 环境变量需要以 CDNC_ 为前缀，例如 CDNC_SERVER_PORT
//
// 返回:
//   - *Config: 加载成功的配置对象
//   - error: 加载失败时的错误信息
func LoadConfig() (*Config, error) {
	// 设置配置文件的路径和名称
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

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
}

// GetExpiryDuration 返回 JWT 过期时长（作为 time.Duration）
func (c *JWTConfig) GetExpiryDuration() time.Duration {
	if c.ExpiryHours <= 0 {
		c.ExpiryHours = 24
	}
	return time.Duration(c.ExpiryHours) * time.Hour
}

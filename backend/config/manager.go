package config

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ServerConfig Manager 服务配置
type ServerConfig struct {
	Host string `mapstructure:"host" default:"0.0.0.0"`
	Port int    `mapstructure:"port" default:"8080"`
}

// AdminConfig 管理员账户配置
type AdminConfig struct {
	Username string `mapstructure:"username" default:"admin"`
	Password string `mapstructure:"password" default:"admin123"`
}

// JWTConfig JWT 认证配置
type JWTConfig struct {
	Secret      string `mapstructure:"secret" default:"change-me-in-production-256bits!"`
	ExpiryHours int    `mapstructure:"expiry_hours" default:"24"`
}

// GetExpiryDuration 返回 JWT 过期时长（作为 time.Duration）
func (c *JWTConfig) GetExpiryDuration() time.Duration {
	if c.ExpiryHours <= 0 {
		c.ExpiryHours = 24
	}
	return time.Duration(c.ExpiryHours) * time.Hour
}

// JWTClaims JWT 令牌中的声明结构
type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
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

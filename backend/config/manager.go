package config

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

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

// ManagerEdgeCacheConfig Manager 为 Edge 节点提供的默认缓存参数
type ManagerEdgeCacheConfig struct {
	TTLSeconds int `mapstructure:"ttl_seconds" default:"300"`
	MaxSizeMB  int `mapstructure:"max_size_mb" default:"512"`
}

// ManagerEdgeConfig Manager 侧管理的 Edge 节点相关配置（共享密钥、默认值）
type ManagerEdgeConfig struct {
	// Secret 是 Edge 节点注册和心跳时使用的共享密钥
	Secret        string                 `mapstructure:"secret" default:"veer-edge-secret"`
	OriginBaseURL string                 `mapstructure:"origin_base_url" default:""`
	Cache         ManagerEdgeCacheConfig `mapstructure:"cache"`
}

// ManagerConfig Manager 服务专有配置
type ManagerConfig struct {
	Service     ServiceConfig     `mapstructure:"service"`
	Database    DatabaseConfig    `mapstructure:"database"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	Admin       AdminConfig       `mapstructure:"admin"`
	HealthCheck HealthCheckConfig `mapstructure:"health_check"`
	RateLimit   RateLimitConfig   `mapstructure:"rate_limit"`
	Cache       CacheConfig       `mapstructure:"cache"`
	Edge        ManagerEdgeConfig `mapstructure:"edge"`
}

// LoadManagerConfig 加载 Manager 配置（config-manager.yaml）
func LoadManagerConfig() (*ManagerConfig, error) {
	viper.SetConfigName("config-manager")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("CDNC")
	viper.AutomaticEnv()

	bindEnv(
		"service.host", "CDNC_SERVICE_HOST",
		"service.port", "CDNC_SERVICE_PORT",
		"database.path", "CDNC_DATABASE_PATH",
		"jwt.secret", "CDNC_JWT_SECRET",
		"jwt.expiry_hours", "CDNC_JWT_EXPIRY_HOURS",
		"admin.username", "CDNC_ADMIN_USERNAME",
		"admin.password", "CDNC_ADMIN_PASSWORD",
		"health_check.enabled", "CDNC_HEALTH_CHECK_ENABLED",
		"rate_limit.enabled", "CDNC_RATE_LIMIT_ENABLED",
		"rate_limit.requests_per_minute", "CDNC_RATE_LIMIT_REQUESTS_PER_MINUTE",
		"edge.secret", "CDNC_EDGE_MANAGER_SECRET",
		"edge.origin_base_url", "CDNC_EDGE_ORIGIN_BASE_URL",
		"edge.cache.ttl_seconds", "CDNC_EDGE_CACHE_TTL_SECONDS",
		"edge.cache.max_size_mb", "CDNC_EDGE_CACHE_MAX_SIZE_MB",
	)

	setManagerDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg ManagerConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setManagerDefaults() {
	viper.SetDefault("service.host", "0.0.0.0")
	viper.SetDefault("service.port", 8080)
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
	viper.SetDefault("edge.secret", "veer-edge-secret")
	viper.SetDefault("edge.origin_base_url", "")
	viper.SetDefault("edge.cache.ttl_seconds", 300)
	viper.SetDefault("edge.cache.max_size_mb", 512)
}

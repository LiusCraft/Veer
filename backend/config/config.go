// Package config provides configuration structures and loading functionality for the Veer backend.
package config

import (
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ServiceConfig 基础服务网络配置（所有服务共用）
type ServiceConfig struct {
	Host string `mapstructure:"host" default:"0.0.0.0"`
	Port int    `mapstructure:"port" default:"8080"`
}

// DatabaseConfig 数据库配置（Manager 与 Scheduler 共享）
type DatabaseConfig struct {
	Path string `mapstructure:"path" default:"./veer.db"`
}

// InitDB initializes and returns a SQLite database connection with the specified database path.
// Uses WAL mode and busy timeout for concurrent access from manager + scheduler.
func InitDB(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// bindEnv is a helper to register explicit env var bindings for known config keys.
// Though AutomaticEnv handles CDNC_PREFIX → key.lowercase automatically, explicit
// BindEnv is kept here as a whitelist for discoverability and backward compat.
func bindEnv(input ...string) {
	for i := 0; i < len(input); i += 2 {
		_ = viper.BindEnv(input[i], input[i+1])
	}
}

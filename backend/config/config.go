// Package config provides database initialization.
package config

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB initializes and returns a SQLite database connection with the specified database path.
//
// 参数:
//   - dbPath: 数据库文件路径，例如 "./veer.db"
//
// 返回:
//   - *gorm.DB: 数据库连接对象
//   - error: 初始化失败时的错误信息
func InitDB(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

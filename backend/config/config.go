// Package config provides database initialization and seed data functionality.
package config

import (
	"encoding/json"
	"log"

	"veer/models"

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

// SeedData inserts initial demo data if tables are empty.
//
// 如果数据库中尚无 CDN 节点和重定向规则，则插入示例数据。
// 示例数据包括:
//   - 3 个 CDN 节点（阿里云、腾讯云、AWS）
//   - 2 个重定向规则（视频分发、静态资源分发）
//
// 参数:
//   - db: 已初始化的数据库连接对象
func SeedData(db *gorm.DB) {
	// Seed CDN nodes
	var nodeCount int64
	db.Model(&models.CdnNode{}).Count(&nodeCount)
	if nodeCount == 0 {
		nodes := []models.CdnNode{
			{Name: "阿里云-华东", URL: "https://cdn-east.aliyun-demo.com", Weight: 3, Region: "华东", Status: "active", Latency: 0},
			{Name: "腾讯云-华南", URL: "https://cdn-south.tencent-demo.com", Weight: 2, Region: "华南", Status: "active", Latency: 0},
			{Name: "AWS-美国西部", URL: "https://cdn-us-west.aws-demo.com", Weight: 1, Region: "美国西部", Status: "active", Latency: 0},
		}
		if result := db.Create(&nodes); result.Error != nil {
			log.Printf("Failed to seed nodes: %v", result.Error)
			return
		}
		log.Println("Seeded 3 CDN nodes")

		// Seed redirect rules
		nodeIDs12, _ := json.Marshal([]uint{nodes[0].ID, nodes[1].ID})
		nodeIDsAll, _ := json.Marshal([]uint{nodes[0].ID, nodes[1].ID, nodes[2].ID})
		rules := []models.RedirectRule{
			{
				Key:         "video",
				Description: "视频资源分发",
				Strategy:    "weighted",
				NodeIDs:     string(nodeIDs12),
				HitCount:    0,
			},
			{
				Key:         "static",
				Description: "静态资源分发",
				Strategy:    "round-robin",
				NodeIDs:     string(nodeIDsAll),
				HitCount:    0,
			},
		}
		if result := db.Create(&rules); result.Error != nil {
			log.Printf("Failed to seed rules: %v", result.Error)
			return
		}
		log.Println("Seeded 2 redirect rules")
	}
}

// Package models defines the data models for the Veer system.
package models

import "time"

// CdnNode represents a CDN node in the system.
type CdnNode struct {
	ID               uint      `json:"id" gorm:"primarykey"`
	Name             string    `json:"name" binding:"required"`
	URL              string    `json:"url" binding:"required"` // 302 跳转目标（public_url）
	Weight           int       `json:"weight" gorm:"default:1"`
	Region           string    `json:"region"`
	Status           string    `json:"status" gorm:"default:'active'"`             // active/inactive
	Latency          int       `json:"latency"`                                    // ms, last health check latency
	ConsecutiveFails int       `json:"consecutive_fails" gorm:"default:0"`         // 连续失败次数，健康检查用
	OriginBaseURL    string    `json:"origin_base_url" gorm:"size:512;default:''"` // 边缘节点回源地址（从 Manager 下发）
	CacheTTL         int       `json:"cache_ttl" gorm:"default:300"`               // 边缘节点缓存 TTL（秒）
	CreatedAt        time.Time `json:"created_at"`
}

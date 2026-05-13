// Package models defines the data models for the Veer system.
package models

import "time"

// CdnNode represents a CDN node in the system.
type CdnNode struct {
	ID               uint      `json:"id" gorm:"primarykey"`
	Name             string    `json:"name" binding:"required"`
	URL              string    `json:"url" binding:"required"`
	Weight           int       `json:"weight" gorm:"default:1"`
	Region           string    `json:"region"`
	Status           string    `json:"status" gorm:"default:'active'"`     // active/inactive
	Latency          int       `json:"latency"`                            // ms, last health check latency
	ConsecutiveFails int       `json:"consecutive_fails" gorm:"default:0"` // 连续失败次数，健康检查用
	CreatedAt        time.Time `json:"created_at"`
}

// Package models defines the data models for the Veer system.
package models

import "time"

// AccessLog records each redirect request for analytics.
type AccessLog struct {
	ID             uint      `json:"id" gorm:"primarykey"`
	Domain         string    `json:"domain"` // 请求的域名
	Path           string    `json:"path"`   // 请求的路径
	NodeID         uint      `json:"node_id"`
	NodeName       string    `json:"node_name"`
	TargetURL      string    `json:"target_url"`
	ClientIP       string    `json:"client_ip"`
	UserAgent      string    `json:"user_agent"`
	StatusCode     int       `json:"status_code"`
	ResponseTimeMs int       `json:"response_time_ms" gorm:"default:0"` // 节点响应时间（边缘节点上报后回填）
	CreatedAt      time.Time `json:"created_at"`
}

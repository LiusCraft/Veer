// Package models defines the data models for the Veer system.
package models

import "time"

// AccessLog records each redirect request for analytics.
// 记录每次重定向请求，用于统计分析
type AccessLog struct {
	ID         uint      `json:"id" gorm:"primarykey"`
	RuleKey    string    `json:"rule_key"`
	Domain     string    `json:"domain"` // 请求的域名
	Path       string    `json:"path"`   // 请求的路径
	NodeID     uint      `json:"node_id"`
	NodeName   string    `json:"node_name"`
	TargetURL  string    `json:"target_url"`
	ClientIP   string    `json:"client_ip"`
	UserAgent  string    `json:"user_agent"`
	StatusCode int       `json:"status_code"`
	CreatedAt  time.Time `json:"created_at"`
}

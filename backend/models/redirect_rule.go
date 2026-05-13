// Package models defines the data models for the Veer system.
package models

import "time"

// RedirectRule represents a URL redirect rule with associated CDN nodes.
// Domain 是主查找键，请求的 Host 匹配 Domain 后执行 302 重定向
type RedirectRule struct {
	ID            uint      `json:"id" gorm:"primarykey"`
	Domain        string    `json:"domain" gorm:"uniqueIndex;not null"` // 唯一域名，调度器通过 Host 匹配此字段
	Description   string    `json:"description"`
	Strategy      string    `json:"strategy" gorm:"default:'round-robin'"`      // round-robin/weighted/random
	NodeIDs       string    `json:"node_ids"`                                   // JSON array string, e.g. "[1,2,3]"
	OriginBaseURL string    `json:"origin_base_url" gorm:"size:512;default:''"` // 回源地址，Edge 节点缓存未命中时使用
	HitCount      int64     `json:"hit_count"`
	CreatedAt     time.Time `json:"created_at"`
}

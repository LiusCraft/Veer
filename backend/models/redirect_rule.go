// Package models defines the data models for the Veer system.
package models

import "time"

// RedirectRule represents a URL redirect rule with associated CDN nodes.
// Key + Domain 构成联合唯一索引，支持按域名分流
type RedirectRule struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	Key         string    `json:"key" gorm:"uniqueIndex:idx_rule_key_domain"`                                         // 联合唯一索引，与 Domain 一起
	Domain      string    `json:"domain" gorm:"uniqueIndex:idx_rule_key_domain;default:'';index:idx_rule_key_domain"` // 目标域名，空表示通用规则
	Description string    `json:"description"`
	Strategy    string    `json:"strategy" gorm:"default:'round-robin'"` // round-robin/weighted/random
	NodeIDs     string    `json:"node_ids"`                              // JSON array string, e.g. "[1,2,3]"
	HitCount    int64     `json:"hit_count"`
	CreatedAt   time.Time `json:"created_at"`
}

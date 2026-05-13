// Package models defines the data models for the Veer system.
package models

import "time"

// AdminUser 管理员用户模型
// 用于 JWT 认证系统的用户管理
type AdminUser struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:64;not null"`
	PasswordHash string    `json:"-" gorm:"size:255;not null"` // json:"-" 不返回密码哈希
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

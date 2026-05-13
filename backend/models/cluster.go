package models

import "time"

type Cluster struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	Name        string    `json:"name" gorm:"size:64;not null;uniqueIndex"`
	Description string    `json:"description" gorm:"size:256"`
	Strategy    string    `json:"strategy" gorm:"size:16;default:'round-robin'"`
	Region      string    `json:"region" gorm:"size:32;index"`
	ISP         string    `json:"isp" gorm:"size:32;index"`
	Provider    string    `json:"provider" gorm:"size:32"`
	Status      string    `json:"status" gorm:"size:16;default:'active'"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

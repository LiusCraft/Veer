package models

import "time"

type CdnNode struct {
	ID        uint `json:"id" gorm:"primarykey"`
	ClusterID uint `json:"cluster_id" gorm:"index;not null;default:0"`

	Name string `json:"name" binding:"required"`
	URL  string `json:"url" binding:"required"`

	IP             string `json:"ip" gorm:"size:45"`
	Region         string `json:"region" gorm:"size:32"`
	ISP            string `json:"isp" gorm:"size:32"`
	Provider       string `json:"provider" gorm:"size:32"`
	NodeType       string `json:"node_type" gorm:"size:16;default:'edge'"`
	Weight         int    `json:"weight" gorm:"default:1"`
	BandwidthMbps  int    `json:"bandwidth_mbps" gorm:"default:1000"`
	MaxConnections int    `json:"max_connections" gorm:"default:10000"`

	CPUUsage      float64   `json:"cpu_usage" gorm:"default:0"`
	MemUsage      float64   `json:"mem_usage" gorm:"default:0"`
	DiskUsage     float64   `json:"disk_usage" gorm:"default:0"`
	LoadAvg       float64   `json:"load_avg" gorm:"default:0"`
	AgentVersion  string    `json:"agent_version" gorm:"size:32"`
	LastHeartbeat time.Time `json:"last_heartbeat"`

	Status           string `json:"status" gorm:"size:16;default:'active'"`
	Latency          int    `json:"latency"`
	ConsecutiveFails int    `json:"consecutive_fails" gorm:"default:0"`

	OriginBaseURL string `json:"origin_base_url" gorm:"size:512;default:''"`
	CacheTTL      int    `json:"cache_ttl" gorm:"default:300"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

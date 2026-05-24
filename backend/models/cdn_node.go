package models

import "time"

type CdnNode struct {
	ID   uint   `json:"id" gorm:"primarykey"`
	Name string `json:"name" binding:"required"`
	URL  string `json:"url" binding:"required"`

	InternalURL string `json:"internal_url" gorm:"size:512;default:''"`

	IP             string `json:"ip" gorm:"size:45"`
	Region         string `json:"region" gorm:"size:32"`
	ISP            string `json:"isp" gorm:"size:32"`
	Provider       string `json:"provider" gorm:"size:32"`
	NodeType       string `json:"node_type" gorm:"size:16;default:'edge'"`
	Weight         int    `json:"weight" gorm:"default:1"`
	BandwidthMbps  int    `json:"bandwidth_mbps" gorm:"default:1000"`
	UplinkMbps     int    `json:"uplink_mbps" gorm:"default:1000"`
	DownlinkMbps   int    `json:"downlink_mbps" gorm:"default:1000"`
	MaxConnections int    `json:"max_connections" gorm:"default:10000"`

	CPUCores   int   `json:"cpu_cores" gorm:"default:4"`
	MemoryMB   int64 `json:"memory_mb" gorm:"default:8192"`
	DiskSizeMB int64 `json:"disk_size_mb" gorm:"default:102400"`

	TxBytes1m int64 `json:"tx_bytes_1m" gorm:"default:0"`
	RxBytes1m int64 `json:"rx_bytes_1m" gorm:"default:0"`

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

package models

import "time"

type ClusterMetric struct {
	ID             uint      `json:"id" gorm:"primarykey"`
	ClusterID      uint      `json:"cluster_id" gorm:"index"`
	RequestCount   int64     `json:"request_count"`
	BandwidthBytes int64     `json:"bandwidth_bytes"`
	AvgLatencyMs   float64   `json:"avg_latency_ms"`
	PeriodMinutes  int       `json:"period_minutes"`
	RecordedAt     time.Time `json:"recorded_at"`
}

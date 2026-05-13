package handlers

import (
	"math"
	"net/http"
	"time"

	"veer/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NodeBrief struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	IP            string    `json:"ip"`
	Status        string    `json:"status"`
	Latency       int       `json:"latency"`
	CPUUsage      float64   `json:"cpu_usage"`
	MemUsage      float64   `json:"mem_usage"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

type ClusterTopology struct {
	models.Cluster
	Stats map[string]interface{} `json:"stats"`
	Nodes []NodeBrief            `json:"nodes"`
}

func computeHealthStatus(online, total int, avgCPU float64) string {
	if total == 0 {
		return "down"
	}
	rate := float64(online) / float64(total)
	if online == 0 || rate <= 0.3 {
		return "down"
	}
	if rate < 0.7 {
		return "degraded"
	}
	if avgCPU >= 85 {
		return "overload"
	}
	if avgCPU >= 70 {
		return "overload"
	}
	return "healthy"
}

func GetTopology(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var clusters []models.Cluster
		db.Order("name asc").Find(&clusters)

		result := make([]ClusterTopology, 0, len(clusters))
		for _, cl := range clusters {
			var nodes []models.CdnNode
			db.Joins("JOIN node_clusters ON node_clusters.node_id = cdn_nodes.id").
				Where("node_clusters.cluster_id = ?", cl.ID).
				Order("cdn_nodes.name asc").
				Find(&nodes)

			var stats struct {
				Online         int     `json:"online"`
				Total          int     `json:"total"`
				AvgCPU         float64 `json:"avg_cpu"`
				AvgMem         float64 `json:"avg_mem"`
				AvgLatency     float64 `json:"avg_latency"`
				TotalBandwidth int     `json:"total_bandwidth_mbps"`
			}
			db.Table("cdn_nodes").
				Joins("JOIN node_clusters ON node_clusters.node_id = cdn_nodes.id").
				Where("node_clusters.cluster_id = ?", cl.ID).
				Select("COUNT(*) as total").Scan(&stats)
			db.Table("cdn_nodes").
				Joins("JOIN node_clusters ON node_clusters.node_id = cdn_nodes.id").
				Where("node_clusters.cluster_id = ? AND cdn_nodes.status = 'active'", cl.ID).
				Select("COUNT(*) as online, COALESCE(AVG(cdn_nodes.cpu_usage),0) as avg_cpu, COALESCE(AVG(cdn_nodes.mem_usage),0) as avg_mem, COALESCE(AVG(cdn_nodes.latency),0) as avg_latency, COALESCE(SUM(cdn_nodes.bandwidth_mbps),0) as total_bandwidth").
				Scan(&stats)

			nodeBriefs := make([]NodeBrief, 0, len(nodes))
			for _, n := range nodes {
				nodeBriefs = append(nodeBriefs, NodeBrief{
					ID: n.ID, Name: n.Name, IP: n.IP, Status: n.Status,
					Latency: n.Latency, CPUUsage: n.CPUUsage, MemUsage: n.MemUsage,
					LastHeartbeat: n.LastHeartbeat,
				})
			}

			result = append(result, ClusterTopology{
				Cluster: cl,
				Stats: map[string]interface{}{
					"online": stats.Online, "total": stats.Total,
					"avg_cpu": stats.AvgCPU, "avg_mem": stats.AvgMem,
					"avg_latency": stats.AvgLatency, "total_bandwidth_mbps": stats.TotalBandwidth,
				},
				Nodes: nodeBriefs,
			})
		}

		c.JSON(http.StatusOK, gin.H{"data": gin.H{"clusters": result}})
	}
}

func GetHealthMatrix(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var clusters []models.Cluster
		db.Order("name asc").Find(&clusters)

		type ClusterHealth struct {
			ClusterID          uint     `json:"cluster_id"`
			ClusterName        string   `json:"cluster_name"`
			Region             []string `json:"region"`
			OnlineNodes        int      `json:"online_nodes"`
			TotalNodes         int      `json:"total_nodes"`
			OnlineRate         float64  `json:"online_rate"`
			AvgCPU             float64  `json:"avg_cpu"`
			AvgMem             float64  `json:"avg_mem"`
			AvgDisk            float64  `json:"avg_disk"`
			AvgLatency         float64  `json:"avg_latency"`
			TotalBandwidthMbps int      `json:"total_bandwidth_mbps"`
			HealthStatus       string   `json:"health_status"`
		}

		result := make([]ClusterHealth, 0, len(clusters))
		for _, cl := range clusters {
			var stats struct {
				Online         int     `json:"online"`
				Total          int     `json:"total"`
				AvgCPU         float64 `json:"avg_cpu"`
				AvgMem         float64 `json:"avg_mem"`
				AvgDisk        float64 `json:"avg_disk"`
				AvgLatency     float64 `json:"avg_latency"`
				TotalBandwidth int     `json:"total_bandwidth_mbps"`
			}
			db.Table("cdn_nodes").
				Joins("JOIN node_clusters ON node_clusters.node_id = cdn_nodes.id").
				Where("node_clusters.cluster_id = ?", cl.ID).
				Select("COUNT(*) as total").Scan(&stats)
			db.Table("cdn_nodes").
				Joins("JOIN node_clusters ON node_clusters.node_id = cdn_nodes.id").
				Where("node_clusters.cluster_id = ? AND cdn_nodes.status = 'active'", cl.ID).
				Select("COUNT(*) as online, COALESCE(AVG(cdn_nodes.cpu_usage),0) as avg_cpu, COALESCE(AVG(cdn_nodes.mem_usage),0) as avg_mem, COALESCE(AVG(cdn_nodes.disk_usage),0) as avg_disk, COALESCE(AVG(cdn_nodes.latency),0) as avg_latency, COALESCE(SUM(cdn_nodes.bandwidth_mbps),0) as total_bandwidth").
				Scan(&stats)

			onlineRate := float64(0)
			if stats.Total > 0 {
				onlineRate = float64(stats.Online) / float64(stats.Total)
			}

			healthStatus := computeHealthStatus(stats.Online, stats.Total, stats.AvgCPU)

			result = append(result, ClusterHealth{
				ClusterID:          cl.ID,
				ClusterName:        cl.Name,
				Region:             cl.Region,
				OnlineNodes:        stats.Online,
				TotalNodes:         stats.Total,
				OnlineRate:         math.Round(onlineRate*100) / 100,
				AvgCPU:             math.Round(stats.AvgCPU*100) / 100,
				AvgMem:             math.Round(stats.AvgMem*100) / 100,
				AvgDisk:            math.Round(stats.AvgDisk*100) / 100,
				AvgLatency:         math.Round(stats.AvgLatency*100) / 100,
				TotalBandwidthMbps: stats.TotalBandwidth,
				HealthStatus:       healthStatus,
			})
		}

		c.JSON(http.StatusOK, gin.H{"data": result})
	}
}

func GetTrafficDistribution(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var metrics []models.ClusterMetric
		db.Where("recorded_at > ?", time.Now().Add(-10*time.Minute)).Find(&metrics)

		type ClusterTraffic struct {
			ClusterID   uint    `json:"cluster_id"`
			ClusterName string  `json:"cluster_name"`
			Requests    int64   `json:"requests"`
			Bandwidth   float64 `json:"bandwidth_gbps"`
		}

		result := make([]ClusterTraffic, 0)
		totalRequests := int64(0)
		totalBandwidth := float64(0)

		if len(metrics) == 0 {
			var clusters []models.Cluster
			db.Order("name asc").Find(&clusters)
			for _, cl := range clusters {
				result = append(result, ClusterTraffic{
					ClusterID: cl.ID, ClusterName: cl.Name,
					Requests: 0, Bandwidth: 0,
				})
			}
		} else {
			metricMap := make(map[uint]models.ClusterMetric)
			for _, m := range metrics {
				metricMap[m.ClusterID] = m
			}
			var clusters []models.Cluster
			db.Find(&clusters)
			for _, cl := range clusters {
				m, ok := metricMap[cl.ID]
				req := int64(0)
				bw := float64(0)
				if ok {
					req = m.RequestCount
					bw = float64(m.BandwidthBytes) / (1024 * 1024 * 1024) * 8
				}
				result = append(result, ClusterTraffic{
					ClusterID: cl.ID, ClusterName: cl.Name,
					Requests: req, Bandwidth: math.Round(bw*100) / 100,
				})
				totalRequests += req
				totalBandwidth += bw
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"period_minutes":       5,
				"clusters":             result,
				"total_requests":       totalRequests,
				"total_bandwidth_gbps": math.Round(totalBandwidth*100) / 100,
			},
		})
	}
}

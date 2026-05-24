package manager

import (
	"net/http"
	"strconv"
	"time"

	"veer/config"
	"veer/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NodeHeartbeatHandler(db *gorm.DB, cfg *config.ManagerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := c.GetHeader("X-Edge-Secret")
		if secret == "" || secret != cfg.Edge.Secret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid secret"})
			return
		}

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node id"})
			return
		}

		var body struct {
			CPUUsage         float64 `json:"cpu_usage"`
			MemUsage         float64 `json:"mem_usage"`
			DiskUsage        float64 `json:"disk_usage"`
			LoadAvg          float64 `json:"load_avg"`
			RequestCount1m   int64   `json:"request_count_1m"`
			BandwidthBytes1m int64   `json:"bandwidth_bytes_1m"`
			TxBytes1m        int64   `json:"tx_bytes_1m"`
			RxBytes1m        int64   `json:"rx_bytes_1m"`
			CacheHits1m      int64   `json:"cache_hits_1m"`
			CacheMisses1m    int64   `json:"cache_misses_1m"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		var node models.CdnNode
		if err := db.First(&node, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
			return
		}

		now := time.Now()
		cacheHitRate := 0.0
		if body.CacheHits1m+body.CacheMisses1m > 0 {
			cacheHitRate = float64(body.CacheHits1m) / float64(body.CacheHits1m+body.CacheMisses1m) * 100
		}
		updates := map[string]interface{}{
			"cpu_usage":      body.CPUUsage,
			"mem_usage":      body.MemUsage,
			"disk_usage":     body.DiskUsage,
			"load_avg":       body.LoadAvg,
			"tx_bytes1m":     body.TxBytes1m,
			"rx_bytes1m":     body.RxBytes1m,
			"cache_hit_rate": cacheHitRate,
			"last_heartbeat": now,
		}

		if err := db.Model(&node).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update heartbeat"})
			return
		}

		if body.RequestCount1m > 0 {
			var clusterIDs []uint
			db.Model(&models.NodeCluster{}).Where("node_id = ?", node.ID).Pluck("cluster_id", &clusterIDs)
			for _, cid := range clusterIDs {
				var metric models.ClusterMetric
				result := db.Where("cluster_id = ? AND recorded_at > ?", cid, now.Add(-5*time.Minute)).First(&metric)
				if result.Error != nil {
					metric = models.ClusterMetric{
						ClusterID:      cid,
						RequestCount:   body.RequestCount1m,
						BandwidthBytes: body.BandwidthBytes1m,
						PeriodMinutes:  5,
						RecordedAt:     now,
					}
					db.Create(&metric)
				} else {
					db.Model(&metric).Updates(map[string]interface{}{
						"request_count":   gorm.Expr("request_count + ?", body.RequestCount1m),
						"bandwidth_bytes": gorm.Expr("bandwidth_bytes + ?", body.BandwidthBytes1m),
					})
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "heartbeat received"})
	}
}

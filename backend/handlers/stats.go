// Package handlers provides HTTP request handlers for the Veer system.
package handlers

import (
	"net/http"
	"strconv"
	"time"
	"veer/models"
	"veer/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// OverviewStats represents the summary statistics for the dashboard.
type OverviewStats struct {
	TotalRedirects    int64  `json:"total_redirects"`
	ActiveNodes       int64  `json:"active_nodes"`
	TotalRules        int64  `json:"total_rules"`
	TodayRequests     int64  `json:"today_requests"`
	HealthCheckStatus string `json:"health_check_status"`
}

// TrafficPoint represents a single day's traffic data.
type TrafficPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// GetOverview handles GET /api/stats/overview — returns dashboard summary stats.
func GetOverview(db *gorm.DB, hcm *services.HealthCheckManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var stats OverviewStats

		// Total redirects = sum of all hit counts
		db.Model(&models.RedirectRule{}).Select("COALESCE(SUM(hit_count), 0)").Scan(&stats.TotalRedirects)

		// Active nodes count
		db.Model(&models.CdnNode{}).Where("status = ?", "active").Count(&stats.ActiveNodes)

		// Total rules count
		db.Model(&models.RedirectRule{}).Count(&stats.TotalRules)

		// Today's requests from access_log
		today := time.Now().Truncate(24 * time.Hour)
		db.Model(&models.AccessLog{}).Where("created_at >= ?", today).Count(&stats.TodayRequests)

		// 健康检测状态
		if hcm != nil && hcm.IsRunning() {
			stats.HealthCheckStatus = "running"
		} else {
			stats.HealthCheckStatus = "stopped"
		}

		c.JSON(http.StatusOK, gin.H{"data": stats})
	}
}

// GetLogs handles GET /api/stats/logs — returns paginated access logs.
func GetLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "20")
		startTime := c.Query("start_time")
		endTime := c.Query("end_time")

		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}

		offset := (page - 1) * pageSize

		query := db.Model(&models.AccessLog{}).Order("created_at desc")
		if startTime != "" {
			query = query.Where("created_at >= ?", startTime)
		}
		if endTime != "" {
			query = query.Where("created_at <= ?", endTime)
		}

		var total int64
		query.Count(&total)

		var logs []models.AccessLog
		if err := query.Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data":      logs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// GetTraffic handles GET /api/stats/traffic — returns last 7 days daily traffic.
func GetTraffic(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		points := make([]TrafficPoint, 7)

		for i := 6; i >= 0; i-- {
			day := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
			nextDay := day.Add(24 * time.Hour)
			dateStr := day.Format("01-02")

			var count int64
			db.Model(&models.AccessLog{}).
				Where("created_at >= ? AND created_at < ?", day, nextDay).
				Count(&count)

			points[6-i] = TrafficPoint{
				Date:  dateStr,
				Count: count,
			}
		}

		c.JSON(http.StatusOK, gin.H{"data": points})
	}
}

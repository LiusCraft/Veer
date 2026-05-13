// Package handlers provides HTTP request handlers for the Veer system.
package handlers

import (
	"net/http"
	"veer/services"

	"github.com/gin-gonic/gin"
)

// GetHealthCheckStatus 处理 GET /api/config/health-check — 获取健康检测状态
//
// 返回当前健康检测管理器的运行状态。
func GetHealthCheckStatus(hcm *services.HealthCheckManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"running": hcm.IsRunning(),
			},
		})
	}
}

// TriggerHealthCheck 处理 POST /api/config/health-check/toggle — 手动触发健康检测
//
// 触发一次全量节点健康检测，立即返回触发成功响应。
func TriggerHealthCheck(hcm *services.HealthCheckManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		hcm.CheckNow()
		c.JSON(http.StatusOK, gin.H{
			"message": "health check triggered",
		})
	}
}

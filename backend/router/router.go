// Package router sets up the HTTP routers for the Veer system.
// Manager: serves admin UI + API (control plane)
// Scheduler: serves domain-based 302 redirects (data plane)
package router

import (
	"veer/config"
	"veer/handlers"
	"veer/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupRouter is the legacy manager router setup (backward compat)
func SetupRouter(db *gorm.DB, cfg *config.Config, hcm *services.HealthCheckManager) *gin.Engine {
	return SetupManagerRouter(db, cfg, hcm)
}

// SetupSchedulerRouter 配置调度服务路由
//
// 调度服务只有两个端点:
//   - GET /health — 健康检查
//   - /*path — 域名匹配 302 重定向
func SetupSchedulerRouter(cache *handlers.RuleCache) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.NoRoute(handlers.SchedulerHandler(cache))
	r.GET("/", handlers.SchedulerHandler(cache))

	return r
}

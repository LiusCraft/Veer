// Package router sets up the HTTP router for the Veer service.
package router

import (
	"veer/config"
	"veer/handlers"
	"veer/middleware"
	"veer/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupRouter 配置所有路由并返回 Gin 引擎
//
// 参数:
//   - db: 数据库连接对象
//   - cfg: 应用程序配置对象
//
// 返回:
//   - *gin.Engine: 配置好的 Gin 引擎
func SetupRouter(db *gorm.DB, cfg *config.Config, hcm *services.HealthCheckManager) *gin.Engine {
	r := gin.Default()

	// 应用全局 CORS 中间件
	r.Use(middleware.CORS())

	// 应用限流中间件（如果启用）
	if cfg.RateLimit.Enabled {
		requestsPerMinute := cfg.RateLimit.RequestsPerMinute
		if requestsPerMinute <= 0 {
			requestsPerMinute = 60
		}
		r.Use(middleware.RateLimitMiddlewareWithConfig(requestsPerMinute, cfg.RateLimit.Whitelist))
	}

	// 静态文件服务（Go embed 嵌入的前端管理页面）
	r.StaticFS("/admin", StaticFiles())
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/admin/")
	})

	// 302 重定向端点（公开访问，不需要认证）
	// 使用 /*path 通配符支持路径透传
	r.GET("/r/:ruleKey/*path", handlers.RedirectHandler(db))

	// API 路由组（需要 JWT 认证）
	api := r.Group("/api")
	api.Use(middleware.JWTAuthMiddleware(cfg.JWT.Secret))
	{
		// 认证相关路由
		// 注意：/api/auth/login 不在受保护范围内，在下面单独处理
		auth := api.Group("/auth")
		{
			// logout 和 me 需要 JWT 认证（在 api 组内）
			auth.POST("/logout", handlers.LogoutHandler())
			auth.GET("/me", handlers.GetCurrentUserHandler())
		}

		// CDN 节点管理路由
		nodes := api.Group("/nodes")
		{
			nodes.GET("", handlers.ListNodes(db))
			nodes.POST("", handlers.CreateNode(db))
			nodes.PUT("/:id", handlers.UpdateNode(db))
			nodes.DELETE("/:id", handlers.DeleteNode(db))
			nodes.POST("/:id/test", handlers.TestNode(db))
			nodes.DELETE("/batch", handlers.BatchDeleteNodes(db))
			nodes.PUT("/batch/status", handlers.BatchUpdateNodeStatus(db))
		}

		// 重定向规则管理路由
		rules := api.Group("/rules")
		{
			rules.GET("", handlers.ListRules(db))
			rules.POST("", handlers.CreateRule(db))
			rules.PUT("/:id", handlers.UpdateRule(db))
			rules.DELETE("/:id", handlers.DeleteRule(db))
			rules.DELETE("/batch", handlers.BatchDeleteRules(db))
		}

		// 统计信息路由
		stats := api.Group("/stats")
		{
			stats.GET("/overview", handlers.GetOverview(db, hcm))
			stats.GET("/logs", handlers.GetLogs(db))
			stats.GET("/traffic", handlers.GetTraffic(db))
		}
	}

	// 登录路由单独处理（不需要 JWT 认证）
	r.POST("/api/auth/login", handlers.LoginHandler(db, cfg))

	return r
}

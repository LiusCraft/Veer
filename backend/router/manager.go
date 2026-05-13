package router

import (
	"veer/config"
	"veer/handlers"
	"veer/middleware"
	"veer/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupManagerRouter(db *gorm.DB, cfg *config.Config, hcm *services.HealthCheckManager) *gin.Engine {
	r := gin.Default()

	r.Use(middleware.CORS())

	if cfg.RateLimit.Enabled {
		requestsPerMinute := cfg.RateLimit.RequestsPerMinute
		if requestsPerMinute <= 0 {
			requestsPerMinute = 60
		}
		r.Use(middleware.RateLimitMiddlewareWithConfig(requestsPerMinute, cfg.RateLimit.Whitelist))
	}

	r.StaticFS("/admin", StaticFiles())
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/admin/")
	})

	api := r.Group("/api")
	api.Use(middleware.JWTAuthMiddleware(cfg.JWT.Secret))
	{
		auth := api.Group("/auth")
		{
			auth.POST("/logout", handlers.LogoutHandler())
			auth.GET("/me", handlers.GetCurrentUserHandler())
		}

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

		rules := api.Group("/rules")
		{
			rules.GET("", handlers.ListRules(db))
			rules.POST("", handlers.CreateRule(db))
			rules.PUT("/:id", handlers.UpdateRule(db))
			rules.DELETE("/:id", handlers.DeleteRule(db))
			rules.DELETE("/batch", handlers.BatchDeleteRules(db))
		}

		stats := api.Group("/stats")
		{
			stats.GET("/overview", handlers.GetOverview(db, hcm))
			stats.GET("/logs", handlers.GetLogs(db))
			stats.GET("/traffic", handlers.GetTraffic(db))
		}
	}

	r.POST("/api/auth/login", handlers.LoginHandler(db, cfg))

	edgeAPI := r.Group("/api/edge")
	{
		edgeAPI.POST("/register", handlers.RegisterEdgeHandler(db, cfg))
		edgeAPI.GET("/rules", handlers.ListEdgeRulesHandler(db, cfg))
	}

	return r
}

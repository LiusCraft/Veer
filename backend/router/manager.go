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
		r.Use(middleware.RateLimitMiddlewareWithConfig(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Whitelist))
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
			nodes.GET("/:id", handlers.GetNode(db))
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
			rules.GET("/:id", handlers.GetRule(db))
			rules.POST("", handlers.CreateRule(db))
			rules.PUT("/:id", handlers.UpdateRule(db))
			rules.DELETE("/:id", handlers.DeleteRule(db))
			rules.DELETE("/batch", handlers.BatchDeleteRules(db))
			rules.PUT("/batch/toggle", handlers.BatchToggleRules(db))
			rules.PUT("/reorder", handlers.ReorderRules(db))
		}

		clusters := api.Group("/clusters")
		{
			clusters.GET("", handlers.ListClusters(db))
			clusters.POST("", handlers.CreateCluster(db))
			clusters.GET("/:id", handlers.GetCluster(db))
			clusters.PUT("/:id", handlers.UpdateCluster(db))
			clusters.DELETE("/:id", handlers.DeleteCluster(db))
			clusters.GET("/:id/nodes", handlers.ListClusterNodes(db))
			clusters.PUT("/:id/nodes", handlers.SetClusterNodes(db))
			clusters.GET("/:id/rules", handlers.ListClusterRules(db))
			clusters.GET("/:id/stats", handlers.GetClusterStats(db))
		}

		stats := api.Group("/stats")
		{
			stats.GET("/overview", handlers.GetOverview(db, hcm))
			stats.GET("/logs", handlers.GetLogs(db))
			stats.GET("/traffic", handlers.GetTraffic(db))
		}
		views := api.Group("/views")
		{
			views.GET("/topology", handlers.GetTopology(db))
			views.GET("/health-matrix", handlers.GetHealthMatrix(db))
			views.GET("/traffic-distribution", handlers.GetTrafficDistribution(db))
		}
	}

	r.POST("/api/auth/login", handlers.LoginHandler(db, cfg))

	edgeAPI := r.Group("/api/edge")
	{
		edgeAPI.POST("/register", handlers.RegisterEdgeHandler(db, cfg))
		edgeAPI.GET("/rules", handlers.ListEdgeRulesHandler(db, cfg))
	}

	// Node heartbeat (authenticated via X-Edge-Secret, not JWT)
	r.POST("/api/nodes/:id/heartbeat", handlers.NodeHeartbeatHandler(db, cfg))

	return r
}

package manager

import (
	"veer/config"
	"veer/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupManagerRouter(db *gorm.DB, cfg *config.ManagerConfig, hcm *HealthCheckManager) *gin.Engine {
	r := gin.Default()

	r.Use(middleware.CORS())

	if cfg.RateLimit.Enabled {
		r.Use(middleware.RateLimitMiddlewareWithConfig(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Whitelist))
	}

	api := r.Group("/api")
	api.Use(middleware.JWTAuthMiddleware(cfg.JWT.Secret))
	{
		auth := api.Group("/auth")
		{
			auth.POST("/logout", LogoutHandler())
			auth.GET("/me", GetCurrentUserHandler())
		}

		nodes := api.Group("/nodes")
		{
			nodes.GET("", ListNodes(db))
			nodes.GET("/:id", GetNode(db))
			nodes.POST("", CreateNode(db))
			nodes.PUT("/:id", UpdateNode(db))
			nodes.DELETE("/:id", DeleteNode(db))
			nodes.POST("/:id/test", TestNode(db))
			nodes.DELETE("/batch", BatchDeleteNodes(db))
			nodes.PUT("/batch/status", BatchUpdateNodeStatus(db))
		}

		rules := api.Group("/rules")
		{
			rules.GET("", ListRules(db))
			rules.GET("/:id", GetRule(db))
			rules.POST("", CreateRule(db))
			rules.PUT("/:id", UpdateRule(db))
			rules.DELETE("/:id", DeleteRule(db))
			rules.DELETE("/batch", BatchDeleteRules(db))
			rules.PUT("/batch/toggle", BatchToggleRules(db))
			rules.PUT("/reorder", ReorderRules(db))
		}

		clusters := api.Group("/clusters")
		{
			clusters.GET("", ListClusters(db))
			clusters.POST("", CreateCluster(db))
			clusters.GET("/:id", GetCluster(db))
			clusters.PUT("/:id", UpdateCluster(db))
			clusters.DELETE("/:id", DeleteCluster(db))
			clusters.GET("/:id/nodes", ListClusterNodes(db))
			clusters.PUT("/:id/nodes", SetClusterNodes(db))
			clusters.GET("/:id/rules", ListClusterRules(db))
			clusters.GET("/:id/stats", GetClusterStats(db))
		}

		stats := api.Group("/stats")
		{
			stats.GET("/overview", GetOverview(db, hcm))
			stats.GET("/logs", GetLogs(db))
			stats.GET("/traffic", GetTraffic(db))
		}
		views := api.Group("/views")
		{
			views.GET("/topology", GetTopology(db))
			views.GET("/health-matrix", GetHealthMatrix(db))
			views.GET("/traffic-distribution", GetTrafficDistribution(db))
		}
	}

	r.POST("/api/auth/login", LoginHandler(db, cfg))

	edgeAPI := r.Group("/api/edge")
	{
		edgeAPI.POST("/register", RegisterEdgeHandler(db, cfg))
		edgeAPI.GET("/rules", ListEdgeRulesHandler(db, cfg))
	}

	r.POST("/api/nodes/:id/heartbeat", NodeHeartbeatHandler(db, cfg))

	return r
}

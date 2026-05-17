package scheduler

import (
	"github.com/gin-gonic/gin"
)

func SetupSchedulerRouter(cache *RuleCache) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.NoRoute(SchedulerHandler(cache))
	r.GET("/", SchedulerHandler(cache))

	return r
}

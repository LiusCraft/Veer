// Package main is the entry point for the Veer backend service.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"veer/config"
	"veer/models"
	"veer/router"
	"veer/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

//go:embed all:dist
var embeddedFrontend embed.FS

func main() {
	// 加载配置文件
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded successfully")
	log.Printf("Server starting on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// 使用配置中的数据库路径初始化数据库
	db, err := config.InitDB(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Printf("Database initialized: %s", cfg.Database.Path)

	// Auto migrate 所有模型（包括新增的 AdminUser）
	if err := db.AutoMigrate(
		&models.AdminUser{},
		&models.CdnNode{},
		&models.RedirectRule{},
		&models.AccessLog{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migration completed")

	// 首次启动时创建默认管理员
	var userCount int64
	db.Model(&models.AdminUser{}).Count(&userCount)
	if userCount == 0 {
		// 使用 bcrypt 哈希密码
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Admin.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}
		adminUser := models.AdminUser{
			Username:     cfg.Admin.Username,
			PasswordHash: string(hash),
		}
		if err := db.Create(&adminUser).Error; err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		log.Printf("Created default admin user: %s", cfg.Admin.Username)
	}

	// Seed 初始演示数据
	config.SeedData(db)

	// 启动健康检查后台任务（如果启用）
	var hcm *services.HealthCheckManager
	if cfg.HealthCheck.Enabled {
		hcm = services.NewHealthCheckManager(db, &cfg.HealthCheck)
		hcm.Start()
		log.Printf("Health checker started (interval=%ds, threshold=%d)",
			cfg.HealthCheck.IntervalSeconds, cfg.HealthCheck.FailThreshold)
	}

	// 设置并启动路由
	r := router.SetupRouter(db, cfg, hcm)

	// 嵌入前端静态文件
	frontendDist, err := fs.Sub(embeddedFrontend, "dist")
	if err != nil {
		log.Printf("Warning: failed to load embedded frontend: %v", err)
	} else {
		// 静态资源
		r.StaticFS("/assets", http.FS(frontendDist))

		// SPA fallback
		indexHTML, err := fs.ReadFile(frontendDist, "index.html")
		if err != nil {
			log.Printf("Warning: failed to read embedded index.html: %v", err)
		} else {
			r.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/r/") {
					c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
					return
				}
				c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
			})
		}
	}

	// 使用配置中的地址启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Veer server starting on %s", addr)
	log.Println("Admin panel available at http://" + addr + "/")
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// 优雅关闭时停止健康检查
	if hcm != nil {
		hcm.Stop()
	}
}

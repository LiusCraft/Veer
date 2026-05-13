package main

import (
	"fmt"
	"log"

	"veer/config"
	"veer/models"
	"veer/router"
	"veer/services"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg, err := config.LoadConfig("config-manager")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Manager service starting on %s:%d", cfg.Server.Host, cfg.Server.Port)

	db, err := config.InitDB(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Printf("Database initialized: %s", cfg.Database.Path)

	if err := db.AutoMigrate(
		&models.AdminUser{},
		&models.CdnNode{},
		&models.RedirectRule{},
		&models.AccessLog{},
		&models.Cluster{},
		&models.RuleCluster{},
		&models.ClusterMetric{},
		&models.NodeCluster{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migration completed")

	var userCount int64
	db.Model(&models.AdminUser{}).Count(&userCount)
	if userCount == 0 {
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

	config.SeedData(db)

	var hcm *services.HealthCheckManager
	if cfg.HealthCheck.Enabled {
		hcm = services.NewHealthCheckManager(db, &cfg.HealthCheck, cfg.Edge.Manager.Secret)
		hcm.Start()
		log.Printf("Health checker started (interval=%ds, threshold=%d)",
			cfg.HealthCheck.IntervalSeconds, cfg.HealthCheck.FailThreshold)
	}

	r := router.SetupManagerRouter(db, cfg, hcm)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Veer manager service starting on %s", addr)
	log.Println("API available at http://" + addr + "/api/")
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	if hcm != nil {
		hcm.Stop()
	}
}

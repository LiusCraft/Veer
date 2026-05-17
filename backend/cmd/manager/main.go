package main

import (
	"fmt"
	"log"

	"veer/config"
	"veer/manager"
	"veer/models"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg, err := config.LoadManagerConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Manager service starting on %s:%d", cfg.Service.Host, cfg.Service.Port)

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

	var hcm *manager.HealthCheckManager
	if cfg.HealthCheck.Enabled {
		hcm = manager.NewHealthCheckManager(db, &cfg.HealthCheck, cfg.Edge.Secret)
		hcm.Start()
		log.Printf("Health checker started (interval=%ds, threshold=%d)",
			cfg.HealthCheck.IntervalSeconds, cfg.HealthCheck.FailThreshold)
	}

	r := manager.SetupManagerRouter(db, cfg, hcm)

	addr := fmt.Sprintf("%s:%d", cfg.Service.Host, cfg.Service.Port)
	log.Printf("Veer manager service starting on %s", addr)
	log.Println("API available at http://" + addr + "/api/")
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	if hcm != nil {
		hcm.Stop()
	}
}

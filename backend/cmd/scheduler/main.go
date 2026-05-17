package main

import (
	"fmt"
	"log"

	"veer/config"
	"veer/models"
	"veer/scheduler"
)

func main() {
	cfg, err := config.LoadSchedulerConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Scheduler service starting on %s:%d", cfg.Service.Host, cfg.Service.Port)

	db, err := config.InitDB(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	if err := db.AutoMigrate(
		&models.CdnNode{},
		&models.RedirectRule{},
		&models.AccessLog{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	cache := scheduler.NewRuleCache(db, cfg.RefreshInterval)

	r := scheduler.SetupSchedulerRouter(cache)

	addr := fmt.Sprintf("%s:%d", cfg.Service.Host, cfg.Service.Port)
	log.Printf("Veer scheduler service starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
}

package main

import (
	"fmt"
	"log"

	"veer/config"
	"veer/models"
	"veer/scheduler"
)

func main() {
	cfg, err := config.LoadConfig("config-scheduler")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Scheduler service starting on %s:%d", cfg.Scheduler.Host, cfg.Scheduler.Port)

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

	cache := scheduler.NewRuleCache(db, cfg.Scheduler.RefreshInterval)

	r := scheduler.SetupSchedulerRouter(cache)

	addr := fmt.Sprintf("%s:%d", cfg.Scheduler.Host, cfg.Scheduler.Port)
	log.Printf("Veer scheduler service starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
}

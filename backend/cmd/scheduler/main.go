package main

import (
	"fmt"
	"log"
	"time"

	"veer/config"
	"veer/geoip"
	"veer/models"
	"veer/scheduler"
)

func main() {
	cfg, err := config.LoadSchedulerConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Scheduler service starting on %s:%d", cfg.Service.Host, cfg.Service.Port)

	if cfg.GeoIP.Enabled {
		geoip.InitGlobalGeoIP(cfg.GeoIP.DBPath)
	} else {
		log.Println("[geoip] GeoIP is disabled, skipping IP lookup")
	}

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

	nodeURLs := make(map[uint]string)
	var nodes []models.CdnNode
	if err := db.Where("status = ?", "active").Find(&nodes).Error; err == nil {
		for _, n := range nodes {
			url := n.URL
			if n.InternalURL != "" {
				url = n.InternalURL
			}
			nodeURLs[n.ID] = url
		}
	}
	if len(nodeURLs) > 0 {
		scheduler.StartRTTProber(nodeURLs, 60*time.Second)
	}

	r := scheduler.SetupSchedulerRouter(cache)

	addr := fmt.Sprintf("%s:%d", cfg.Service.Host, cfg.Service.Port)
	log.Printf("Veer scheduler service starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
}

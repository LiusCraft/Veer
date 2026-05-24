package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"veer/config"
	"veer/edge"
)

func main() {
	cfg, err := config.LoadEdgeConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	diskCacheStatus := "disabled"
	if cfg.Cache.Disk.Enabled {
		diskCacheStatus = fmt.Sprintf("enabled (path=%s, max_size=%dGB)", cfg.Cache.Disk.Path, cfg.Cache.Disk.MaxSizeGB)
	}
	log.Printf("Edge node %q starting (listen: %s:%d, public: %s, disk_cache: %s)",
		cfg.Name, cfg.Service.Host, cfg.Service.Port, cfg.PublicURL, diskCacheStatus)

	if cfg.Manager.URL != "" {
		if err := edge.RegisterWithManager(cfg); err != nil {
			log.Printf("[edge] WARNING: manager registration failed (running with local config): %v", err)
		}
	} else {
		log.Println("[edge] no manager URL configured, using local config")
	}

	server := edge.NewEdgeServer(cfg)

	if cfg.Manager.URL != "" && cfg.NodeID != 0 {
		go edge.StartHeartbeatLoop(server)
	}

	if cfg.Manager.URL != "" {
		if err := edge.SyncRules(server); err != nil {
			log.Printf("[edge] WARNING: failed to sync rules from manager: %v", err)
		}
		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if err := edge.SyncRules(server); err != nil {
					log.Printf("[edge] WARNING: failed to sync rules: %v", err)
				}
			}
		}()
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[edge] shutting down...")
		server.Stop()
		os.Exit(0)
	}()

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start edge server: %v", err)
	}
}

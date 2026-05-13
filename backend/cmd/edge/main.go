package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"veer/config"
	"veer/edge"
)

func main() {
	cfg, err := config.LoadConfig("config-edge")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Edge node %q starting (listen: %s:%d, public: %s)",
		cfg.Edge.Name, cfg.Edge.Host, cfg.Edge.Port, cfg.Edge.PublicURL)

	if cfg.Edge.Manager.URL != "" {
		if err := edge.RegisterWithManager(&cfg.Edge); err != nil {
			log.Printf("[edge] WARNING: manager registration failed (running with local config): %v", err)
		}
	} else {
		log.Println("[edge] no manager URL configured, using local config")
	}

	server := edge.NewEdgeServer(&cfg.Edge)

	if cfg.Edge.Manager.URL != "" {
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

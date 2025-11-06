package main

import (
	"log"
	"os"

	"github.com/linus/recal/internal/config"
	"github.com/linus/recal/internal/server"
)

func main() {
	// Determine config file path
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded from %s", configPath)
	log.Printf("Server port: %d", cfg.Server.Port)
	log.Printf("Upstream default: %s", cfg.Upstream.DefaultURL)
	log.Printf("Cache max size: %d", cfg.Cache.MaxSize)
	log.Printf("Cache min output: %v", cfg.Cache.MinOutputCache)

	// Create and start server
	srv := server.New(cfg)

	log.Printf("Starting ReCal server...")
	log.Printf("Endpoints:")
	log.Printf("  - /filter  - Filter upstream iCal feed")
	log.Printf("  - /health  - Health check")

	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

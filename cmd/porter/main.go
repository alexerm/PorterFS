package main

import (
	"flag"
	"log"
	"os"

	"github.com/alexerm/porterfs/internal/config"
	"github.com/alexerm/porterfs/internal/server"
)

func main() {
	configFile := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	server, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	addr := cfg.Server.Address
	if port := os.Getenv("PORT"); port != "" {
		addr = "0.0.0.0:" + port
	}

	log.Printf("Starting PorterFS server on %s", addr)
	log.Fatal(server.ListenAndServe(addr))
}

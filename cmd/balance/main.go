package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/therealutkarshpriyadarshi/balance/pkg/config"
	"github.com/therealutkarshpriyadarshi/balance/pkg/proxy"
)

var (
	// Version information (set during build)
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Show version and exit
	if *showVersion {
		fmt.Printf("Balance %s\n", Version)
		fmt.Printf("Git commit: %s\n", GitCommit)
		fmt.Printf("Build time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Starting Balance proxy (version: %s)", Version)
	log.Printf("Loaded configuration from: %s", *configPath)

	// Create proxy server based on configuration
	var server *proxy.Server
	switch cfg.Mode {
	case "tcp":
		server, err = proxy.NewTCPServer(cfg)
	case "http":
		server, err = proxy.NewHTTPServer(cfg)
	default:
		log.Fatalf("Unsupported mode: %s (supported: tcp, http)", cfg.Mode)
	}

	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	// Start the server
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("Proxy listening on %s (mode: %s)", cfg.Listen, cfg.Mode)

	// Wait for shutdown signal
	waitForShutdown(server)
}

// waitForShutdown waits for interrupt signal and gracefully shuts down the server
func waitForShutdown(server *proxy.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, gracefully shutting down...")

	if err := server.Shutdown(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped")
}

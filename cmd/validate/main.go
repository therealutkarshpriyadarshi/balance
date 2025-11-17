package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/therealutkarshpriyadarshi/balance/pkg/config"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	verbose := flag.Bool("verbose", false, "Show verbose output")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Balance Config Validator %s\n", Version)
		fmt.Printf("Git commit: %s\n", GitCommit)
		fmt.Printf("Build time: %s\n", BuildTime)
		os.Exit(0)
	}

	if *verbose {
		fmt.Printf("Validating configuration file: %s\n", *configPath)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("✓ Configuration file loaded successfully\n")
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	// Additional validation checks
	errors := []string{}

	// Check mode
	if cfg.Mode != "tcp" && cfg.Mode != "http" {
		errors = append(errors, fmt.Sprintf("invalid mode '%s' (must be 'tcp' or 'http')", cfg.Mode))
	}

	// Check backends
	if len(cfg.Backends) == 0 {
		errors = append(errors, "no backends configured")
	}

	// Check load balancer algorithm
	validAlgorithms := map[string]bool{
		"round-robin":       true,
		"least-connections": true,
		"weighted-round-robin": true,
		"weighted-least-connections": true,
		"consistent-hash":   true,
		"bounded-load":      true,
	}
	if cfg.LoadBalancer.Algorithm != "" && !validAlgorithms[cfg.LoadBalancer.Algorithm] {
		errors = append(errors, fmt.Sprintf("invalid load balancer algorithm '%s'", cfg.LoadBalancer.Algorithm))
	}

	// Check TLS configuration
	if cfg.TLS != nil && cfg.TLS.Enabled {
		if cfg.TLS.CertFile == "" {
			errors = append(errors, "TLS enabled but no certificate file specified")
		}
		if cfg.TLS.KeyFile == "" {
			errors = append(errors, "TLS enabled but no key file specified")
		}
	}

	// Check timeouts
	if cfg.Timeouts != nil {
		if cfg.Timeouts.Connect <= 0 {
			errors = append(errors, "invalid connect timeout (must be positive)")
		}
		if cfg.Timeouts.Read <= 0 {
			errors = append(errors, "invalid read timeout (must be positive)")
		}
		if cfg.Timeouts.Write <= 0 {
			errors = append(errors, "invalid write timeout (must be positive)")
		}
	}

	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "❌ Configuration validation failed with %d error(s):\n", len(errors))
		for i, err := range errors {
			fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, err)
		}
		os.Exit(1)
	}

	// Success
	fmt.Printf("✅ Configuration is valid\n")
	if *verbose {
		fmt.Printf("\nConfiguration summary:\n")
		fmt.Printf("  Mode: %s\n", cfg.Mode)
		fmt.Printf("  Listen: %s\n", cfg.Listen)
		fmt.Printf("  Backends: %d\n", len(cfg.Backends))
		if cfg.LoadBalancer.Algorithm != "" {
			fmt.Printf("  Load Balancer: %s\n", cfg.LoadBalancer.Algorithm)
		}
		if cfg.TLS != nil && cfg.TLS.Enabled {
			fmt.Printf("  TLS: enabled\n")
		}
		if cfg.HealthCheck != nil && cfg.HealthCheck.Enabled {
			fmt.Printf("  Health Checks: enabled\n")
		}
		if cfg.Admin != nil && cfg.Admin.Enabled {
			fmt.Printf("  Admin API: enabled on %s\n", cfg.Admin.Listen)
		}
	}
}

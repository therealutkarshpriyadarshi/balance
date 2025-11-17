package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	// Mode can be "tcp" or "http"
	Mode string `yaml:"mode"`

	// Listen address (e.g., ":8080" or "0.0.0.0:8080")
	Listen string `yaml:"listen"`

	// Backends configuration
	Backends []Backend `yaml:"backends"`

	// LoadBalancer configuration
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer"`

	// TLS configuration (optional)
	TLS *TLSConfig `yaml:"tls,omitempty"`

	// Health check configuration (optional)
	HealthCheck *HealthCheckConfig `yaml:"health_check,omitempty"`

	// Timeouts configuration
	Timeouts TimeoutConfig `yaml:"timeouts"`

	// Metrics configuration
	Metrics MetricsConfig `yaml:"metrics"`
}

// Backend represents a backend server configuration
type Backend struct {
	// Name of the backend
	Name string `yaml:"name"`

	// Address of the backend (host:port)
	Address string `yaml:"address"`

	// Weight for weighted load balancing (default: 1)
	Weight int `yaml:"weight"`

	// MaxConnections limits concurrent connections to this backend (0 = unlimited)
	MaxConnections int `yaml:"max_connections"`
}

// LoadBalancerConfig represents load balancer settings
type LoadBalancerConfig struct {
	// Algorithm: "round-robin", "least-connections", "consistent-hash", "weighted-round-robin"
	Algorithm string `yaml:"algorithm"`

	// HashKey for consistent hashing (e.g., "source-ip", "header:X-User-ID")
	HashKey string `yaml:"hash_key,omitempty"`
}

// TLSConfig represents TLS/SSL configuration
type TLSConfig struct {
	// Enabled enables TLS termination
	Enabled bool `yaml:"enabled"`

	// CertFile path to certificate file
	CertFile string `yaml:"cert_file"`

	// KeyFile path to private key file
	KeyFile string `yaml:"key_file"`

	// MinVersion minimum TLS version (e.g., "1.2", "1.3")
	MinVersion string `yaml:"min_version"`
}

// HealthCheckConfig represents health check settings
type HealthCheckConfig struct {
	// Enabled enables health checking
	Enabled bool `yaml:"enabled"`

	// Interval between health checks
	Interval time.Duration `yaml:"interval"`

	// Timeout for health check requests
	Timeout time.Duration `yaml:"timeout"`

	// UnhealthyThreshold number of failures before marking unhealthy
	UnhealthyThreshold int `yaml:"unhealthy_threshold"`

	// HealthyThreshold number of successes before marking healthy
	HealthyThreshold int `yaml:"healthy_threshold"`

	// Path for HTTP health checks (e.g., "/health")
	Path string `yaml:"path,omitempty"`
}

// TimeoutConfig represents timeout settings
type TimeoutConfig struct {
	// Connect timeout for connecting to backends
	Connect time.Duration `yaml:"connect"`

	// Read timeout for reading from connections
	Read time.Duration `yaml:"read"`

	// Write timeout for writing to connections
	Write time.Duration `yaml:"write"`

	// Idle timeout for idle connections
	Idle time.Duration `yaml:"idle"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	// Enabled enables Prometheus metrics
	Enabled bool `yaml:"enabled"`

	// Listen address for metrics endpoint (e.g., ":9090")
	Listen string `yaml:"listen"`

	// Path for metrics endpoint (default: "/metrics")
	Path string `yaml:"path"`
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	cfg.setDefaults()

	return &cfg, nil
}

// setDefaults sets default values for optional configuration
func (c *Config) setDefaults() {
	// Default mode
	if c.Mode == "" {
		c.Mode = "tcp"
	}

	// Default listen address
	if c.Listen == "" {
		c.Listen = ":8080"
	}

	// Default load balancer algorithm
	if c.LoadBalancer.Algorithm == "" {
		c.LoadBalancer.Algorithm = "round-robin"
	}

	// Default backend weights
	for i := range c.Backends {
		if c.Backends[i].Weight == 0 {
			c.Backends[i].Weight = 1
		}
	}

	// Default timeouts
	if c.Timeouts.Connect == 0 {
		c.Timeouts.Connect = 5 * time.Second
	}
	if c.Timeouts.Read == 0 {
		c.Timeouts.Read = 30 * time.Second
	}
	if c.Timeouts.Write == 0 {
		c.Timeouts.Write = 30 * time.Second
	}
	if c.Timeouts.Idle == 0 {
		c.Timeouts.Idle = 60 * time.Second
	}

	// Default health check settings
	if c.HealthCheck != nil && c.HealthCheck.Enabled {
		if c.HealthCheck.Interval == 0 {
			c.HealthCheck.Interval = 10 * time.Second
		}
		if c.HealthCheck.Timeout == 0 {
			c.HealthCheck.Timeout = 3 * time.Second
		}
		if c.HealthCheck.UnhealthyThreshold == 0 {
			c.HealthCheck.UnhealthyThreshold = 3
		}
		if c.HealthCheck.HealthyThreshold == 0 {
			c.HealthCheck.HealthyThreshold = 2
		}
	}

	// Default metrics settings
	if c.Metrics.Enabled && c.Metrics.Path == "" {
		c.Metrics.Path = "/metrics"
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate mode
	if c.Mode != "tcp" && c.Mode != "http" {
		return fmt.Errorf("invalid mode: %s (must be 'tcp' or 'http')", c.Mode)
	}

	// Validate backends
	if len(c.Backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}

	for i, backend := range c.Backends {
		if backend.Address == "" {
			return fmt.Errorf("backend %d: address is required", i)
		}
		if backend.Weight < 0 {
			return fmt.Errorf("backend %d: weight must be non-negative", i)
		}
	}

	// Validate load balancer algorithm
	validAlgorithms := map[string]bool{
		"round-robin":           true,
		"least-connections":     true,
		"consistent-hash":       true,
		"weighted-round-robin":  true,
	}
	if !validAlgorithms[c.LoadBalancer.Algorithm] {
		return fmt.Errorf("invalid load balancer algorithm: %s", c.LoadBalancer.Algorithm)
	}

	// Validate TLS configuration
	if c.TLS != nil && c.TLS.Enabled {
		if c.TLS.CertFile == "" {
			return fmt.Errorf("TLS cert_file is required when TLS is enabled")
		}
		if c.TLS.KeyFile == "" {
			return fmt.Errorf("TLS key_file is required when TLS is enabled")
		}
	}

	return nil
}

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

	// HTTP configuration (for HTTP mode)
	HTTP *HTTPConfig `yaml:"http,omitempty"`

	// TLS configuration (optional)
	TLS *TLSConfig `yaml:"tls,omitempty"`

	// Health check configuration (optional)
	HealthCheck *HealthCheckConfig `yaml:"health_check,omitempty"`

	// Resilience configuration (optional)
	Resilience *ResilienceConfig `yaml:"resilience,omitempty"`

	// Timeouts configuration
	Timeouts TimeoutConfig `yaml:"timeouts"`

	// Metrics configuration
	Metrics MetricsConfig `yaml:"metrics"`

	// Security configuration
	Security *SecurityConfig `yaml:"security,omitempty"`

	// ConnectionPool configuration (Phase 6)
	ConnectionPool *ConnectionPoolConfig `yaml:"connection_pool,omitempty"`

	// Transform configuration (Phase 6)
	Transform *TransformConfig `yaml:"transform,omitempty"`

	// Tracing configuration (Phase 6)
	Tracing *TracingConfig `yaml:"tracing,omitempty"`

	// Logging configuration (Phase 6)
	Logging *LoggingConfig `yaml:"logging,omitempty"`
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

	// Certificates is a list of certificate configurations for multi-domain support
	Certificates []CertificateConfig `yaml:"certificates,omitempty"`

	// CertFile path to certificate file (deprecated - use Certificates instead)
	CertFile string `yaml:"cert_file,omitempty"`

	// KeyFile path to private key file (deprecated - use Certificates instead)
	KeyFile string `yaml:"key_file,omitempty"`

	// MinVersion minimum TLS version (e.g., "1.0", "1.1", "1.2", "1.3")
	MinVersion string `yaml:"min_version,omitempty"`

	// MaxVersion maximum TLS version (e.g., "1.3")
	MaxVersion string `yaml:"max_version,omitempty"`

	// CipherSuites is a list of enabled cipher suites (empty = use secure defaults)
	CipherSuites []string `yaml:"cipher_suites,omitempty"`

	// PreferServerCipherSuites controls whether server cipher suite preferences are used
	PreferServerCipherSuites bool `yaml:"prefer_server_cipher_suites"`

	// SessionTicketsDisabled disables session ticket (resumption) support
	SessionTicketsDisabled bool `yaml:"session_tickets_disabled"`

	// ClientAuth determines the server's policy for client authentication
	// Options: "none", "request", "require", "verify", "require-and-verify"
	ClientAuth string `yaml:"client_auth,omitempty"`

	// ClientCAFile path to client CA certificate file for client authentication
	ClientCAFile string `yaml:"client_ca_file,omitempty"`

	// ALPN protocols (e.g., ["h2", "http/1.1"])
	ALPNProtocols []string `yaml:"alpn_protocols,omitempty"`

	// Backend TLS configuration
	Backend *BackendTLSConfig `yaml:"backend,omitempty"`

	// SNI configuration
	SNI *SNIConfig `yaml:"sni,omitempty"`
}

// CertificateConfig represents a single certificate configuration
type CertificateConfig struct {
	// CertFile path to certificate file
	CertFile string `yaml:"cert_file"`

	// KeyFile path to private key file
	KeyFile string `yaml:"key_file"`

	// Domains is a list of domains this certificate is valid for (optional, auto-detected from cert)
	Domains []string `yaml:"domains,omitempty"`

	// Default indicates this is the default certificate
	Default bool `yaml:"default,omitempty"`
}

// BackendTLSConfig represents TLS configuration for backend connections
type BackendTLSConfig struct {
	// Enabled enables TLS for backend connections
	Enabled bool `yaml:"enabled"`

	// InsecureSkipVerify controls whether to verify backend certificates (for testing only)
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`

	// CAFile path to CA certificate file for backend verification
	CAFile string `yaml:"ca_file,omitempty"`

	// ClientCertFile path to client certificate file for mTLS
	ClientCertFile string `yaml:"client_cert_file,omitempty"`

	// ClientKeyFile path to client private key file for mTLS
	ClientKeyFile string `yaml:"client_key_file,omitempty"`
}

// SNIConfig represents SNI routing configuration
type SNIConfig struct {
	// Routes maps SNI hostnames to backend names
	Routes map[string][]string `yaml:"routes,omitempty"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	// RateLimit configuration
	RateLimit *RateLimitConfig `yaml:"rate_limit,omitempty"`

	// ConnectionProtection configuration
	ConnectionProtection *ConnectionProtectionConfig `yaml:"connection_protection,omitempty"`

	// IPBlocklist configuration
	IPBlocklist *IPBlocklistConfig `yaml:"ip_blocklist,omitempty"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	// Enabled enables rate limiting
	Enabled bool `yaml:"enabled"`

	// Type: "token-bucket" or "sliding-window"
	Type string `yaml:"type"`

	// RequestsPerSecond for token bucket rate limiting
	RequestsPerSecond float64 `yaml:"requests_per_second,omitempty"`

	// BurstSize for token bucket (max tokens)
	BurstSize int64 `yaml:"burst_size,omitempty"`

	// WindowSize for sliding window rate limiting (e.g., "1m", "1h")
	WindowSize string `yaml:"window_size,omitempty"`

	// MaxRequests for sliding window rate limiting
	MaxRequests int64 `yaml:"max_requests,omitempty"`
}

// ConnectionProtectionConfig represents connection protection configuration
type ConnectionProtectionConfig struct {
	// MaxConnectionsPerIP limits concurrent connections per IP
	MaxConnectionsPerIP int `yaml:"max_connections_per_ip"`

	// MaxConnectionRate limits new connections per second per IP
	MaxConnectionRate float64 `yaml:"max_connection_rate"`

	// ReadTimeout for reading request headers (Slowloris protection)
	ReadTimeout string `yaml:"read_timeout"`

	// MaxRequestSize limits the maximum request size in bytes
	MaxRequestSize int64 `yaml:"max_request_size"`

	// MaxHeaderSize limits the maximum header size in bytes
	MaxHeaderSize int64 `yaml:"max_header_size"`
}

// IPBlocklistConfig represents IP blocklist configuration
type IPBlocklistConfig struct {
	// BlockedIPs is a list of permanently blocked IPs
	BlockedIPs []string `yaml:"blocked_ips,omitempty"`

	// BlockedCIDRs is a list of blocked CIDR ranges
	BlockedCIDRs []string `yaml:"blocked_cidrs,omitempty"`
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

	// Type of health check: "tcp", "http", or "https"
	Type string `yaml:"type,omitempty"`

	// Path for HTTP health checks (e.g., "/health")
	Path string `yaml:"path,omitempty"`

	// PassiveChecks enables passive health checking
	PassiveChecks *PassiveHealthCheckConfig `yaml:"passive_checks,omitempty"`
}

// PassiveHealthCheckConfig represents passive health check settings
type PassiveHealthCheckConfig struct {
	// Enabled enables passive health checking
	Enabled bool `yaml:"enabled"`

	// ErrorRateThreshold is the error rate (0.0-1.0) that triggers unhealthy
	ErrorRateThreshold float64 `yaml:"error_rate_threshold,omitempty"`

	// ConsecutiveFailures is the number of consecutive failures to mark unhealthy
	ConsecutiveFailures int `yaml:"consecutive_failures,omitempty"`

	// Window is the time window for tracking failures
	Window time.Duration `yaml:"window,omitempty"`
}

// ResilienceConfig represents circuit breaker and retry configuration
type ResilienceConfig struct {
	// CircuitBreaker configuration
	CircuitBreaker *CircuitBreakerConfig `yaml:"circuit_breaker,omitempty"`

	// Retry configuration
	Retry *RetryConfig `yaml:"retry,omitempty"`
}

// CircuitBreakerConfig represents circuit breaker settings
type CircuitBreakerConfig struct {
	// Enabled enables circuit breaker
	Enabled bool `yaml:"enabled"`

	// MaxFailures before opening the circuit
	MaxFailures int `yaml:"max_failures,omitempty"`

	// Timeout before attempting recovery (half-open state)
	Timeout time.Duration `yaml:"timeout,omitempty"`

	// MaxConcurrentRequests in half-open state
	MaxConcurrentRequests int `yaml:"max_concurrent_requests,omitempty"`
}

// RetryConfig represents retry policy configuration
type RetryConfig struct {
	// Enabled enables retry logic
	Enabled bool `yaml:"enabled"`

	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int `yaml:"max_attempts,omitempty"`

	// InitialDelay is the initial backoff delay
	InitialDelay time.Duration `yaml:"initial_delay,omitempty"`

	// MaxDelay is the maximum backoff delay
	MaxDelay time.Duration `yaml:"max_delay,omitempty"`

	// Multiplier is the backoff multiplier
	Multiplier float64 `yaml:"multiplier,omitempty"`

	// Jitter adds randomness to backoff (0.0-1.0)
	Jitter float64 `yaml:"jitter,omitempty"`
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

// HTTPConfig represents HTTP-specific configuration
type HTTPConfig struct {
	// Routes for HTTP routing (optional, if empty uses default backend pool)
	Routes []Route `yaml:"routes,omitempty"`

	// EnableWebSocket enables WebSocket proxying
	EnableWebSocket bool `yaml:"enable_websocket"`

	// EnableHTTP2 enables HTTP/2 support
	EnableHTTP2 bool `yaml:"enable_http2"`

	// MaxIdleConnsPerHost limits idle connections per backend
	MaxIdleConnsPerHost int `yaml:"max_idle_conns_per_host"`

	// IdleConnTimeout is the idle connection timeout
	IdleConnTimeout time.Duration `yaml:"idle_conn_timeout"`
}

// Route represents an HTTP routing rule
type Route struct {
	// Name of the route
	Name string `yaml:"name"`

	// Host pattern for host-based routing (e.g., "api.example.com")
	Host string `yaml:"host,omitempty"`

	// PathPrefix for path-based routing (e.g., "/api/")
	PathPrefix string `yaml:"path_prefix,omitempty"`

	// Headers for header-based routing (e.g., {"X-API-Key": "secret"})
	Headers map[string]string `yaml:"headers,omitempty"`

	// Backends for this route (backend names)
	Backends []string `yaml:"backends"`

	// Priority for route matching (higher = higher priority)
	Priority int `yaml:"priority"`
}

// ConnectionPoolConfig represents connection pooling configuration (Phase 6)
type ConnectionPoolConfig struct {
	// Enabled enables connection pooling
	Enabled bool `yaml:"enabled"`

	// MaxSize maximum number of connections per backend
	MaxSize int `yaml:"max_size"`

	// MaxIdleTime maximum time a connection can be idle
	MaxIdleTime time.Duration `yaml:"max_idle_time"`
}

// TransformConfig represents request/response transformation configuration (Phase 6)
type TransformConfig struct {
	// RequestHeaders to add/set/remove
	RequestHeaders []HeaderTransform `yaml:"request_headers,omitempty"`

	// ResponseHeaders to add/set/remove
	ResponseHeaders []HeaderTransform `yaml:"response_headers,omitempty"`

	// StripPrefix removes prefix from request path
	StripPrefix string `yaml:"strip_prefix,omitempty"`

	// AddPrefix adds prefix to request path
	AddPrefix string `yaml:"add_prefix,omitempty"`
}

// HeaderTransform represents a header transformation
type HeaderTransform struct {
	// Action: "add", "set", or "remove"
	Action string `yaml:"action"`

	// Name of the header
	Name string `yaml:"name"`

	// Value of the header (not used for "remove")
	Value string `yaml:"value,omitempty"`
}

// TracingConfig represents distributed tracing configuration (Phase 6)
type TracingConfig struct {
	// Enabled enables distributed tracing
	Enabled bool `yaml:"enabled"`

	// ServiceName for tracing
	ServiceName string `yaml:"service_name"`

	// Endpoint for trace collector (e.g., Jaeger)
	Endpoint string `yaml:"endpoint"`

	// SampleRate (0.0-1.0) for sampling traces
	SampleRate float64 `yaml:"sample_rate"`
}

// LoggingConfig represents logging configuration (Phase 6)
type LoggingConfig struct {
	// Level: "debug", "info", "warn", "error", "fatal"
	Level string `yaml:"level"`

	// Format: "text" or "json"
	Format string `yaml:"format"`

	// AddCaller adds caller info to logs
	AddCaller bool `yaml:"add_caller"`

	// AccessLog enables HTTP access logging
	AccessLog bool `yaml:"access_log"`
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
		if c.HealthCheck.Type == "" {
			c.HealthCheck.Type = "tcp"
		}
		// Default passive health check settings
		if c.HealthCheck.PassiveChecks != nil && c.HealthCheck.PassiveChecks.Enabled {
			if c.HealthCheck.PassiveChecks.ErrorRateThreshold == 0 {
				c.HealthCheck.PassiveChecks.ErrorRateThreshold = 0.5
			}
			if c.HealthCheck.PassiveChecks.ConsecutiveFailures == 0 {
				c.HealthCheck.PassiveChecks.ConsecutiveFailures = 5
			}
			if c.HealthCheck.PassiveChecks.Window == 0 {
				c.HealthCheck.PassiveChecks.Window = 1 * time.Minute
			}
		}
	}

	// Default resilience settings
	if c.Resilience != nil {
		// Circuit breaker defaults
		if c.Resilience.CircuitBreaker != nil && c.Resilience.CircuitBreaker.Enabled {
			if c.Resilience.CircuitBreaker.MaxFailures == 0 {
				c.Resilience.CircuitBreaker.MaxFailures = 5
			}
			if c.Resilience.CircuitBreaker.Timeout == 0 {
				c.Resilience.CircuitBreaker.Timeout = 60 * time.Second
			}
			if c.Resilience.CircuitBreaker.MaxConcurrentRequests == 0 {
				c.Resilience.CircuitBreaker.MaxConcurrentRequests = 1
			}
		}

		// Retry defaults
		if c.Resilience.Retry != nil && c.Resilience.Retry.Enabled {
			if c.Resilience.Retry.MaxAttempts == 0 {
				c.Resilience.Retry.MaxAttempts = 3
			}
			if c.Resilience.Retry.InitialDelay == 0 {
				c.Resilience.Retry.InitialDelay = 100 * time.Millisecond
			}
			if c.Resilience.Retry.MaxDelay == 0 {
				c.Resilience.Retry.MaxDelay = 10 * time.Second
			}
			if c.Resilience.Retry.Multiplier == 0 {
				c.Resilience.Retry.Multiplier = 2.0
			}
			if c.Resilience.Retry.Jitter == 0 {
				c.Resilience.Retry.Jitter = 0.1
			}
		}
	}

	// Default metrics settings
	if c.Metrics.Enabled && c.Metrics.Path == "" {
		c.Metrics.Path = "/metrics"
	}

	// Default HTTP settings
	if c.Mode == "http" && c.HTTP == nil {
		c.HTTP = &HTTPConfig{
			EnableWebSocket:     true,
			EnableHTTP2:         true,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		}
	}
	if c.HTTP != nil {
		if c.HTTP.MaxIdleConnsPerHost == 0 {
			c.HTTP.MaxIdleConnsPerHost = 100
		}
		if c.HTTP.IdleConnTimeout == 0 {
			c.HTTP.IdleConnTimeout = 90 * time.Second
		}
	}

	// Phase 6: Connection pool defaults
	if c.ConnectionPool != nil && c.ConnectionPool.Enabled {
		if c.ConnectionPool.MaxSize == 0 {
			c.ConnectionPool.MaxSize = 10
		}
		if c.ConnectionPool.MaxIdleTime == 0 {
			c.ConnectionPool.MaxIdleTime = 5 * time.Minute
		}
	}

	// Phase 6: Tracing defaults
	if c.Tracing != nil && c.Tracing.Enabled {
		if c.Tracing.ServiceName == "" {
			c.Tracing.ServiceName = "balance-proxy"
		}
		if c.Tracing.SampleRate == 0 {
			c.Tracing.SampleRate = 1.0
		}
	}

	// Phase 6: Logging defaults
	if c.Logging != nil {
		if c.Logging.Level == "" {
			c.Logging.Level = "info"
		}
		if c.Logging.Format == "" {
			c.Logging.Format = "text"
		}
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
		"round-robin":                true,
		"least-connections":          true,
		"consistent-hash":            true,
		"bounded-consistent-hash":    true,
		"weighted-round-robin":       true,
		"weighted-least-connections": true,
	}
	if !validAlgorithms[c.LoadBalancer.Algorithm] {
		return fmt.Errorf("invalid load balancer algorithm: %s", c.LoadBalancer.Algorithm)
	}

	// Validate hash key for consistent hashing algorithms
	if (c.LoadBalancer.Algorithm == "consistent-hash" || c.LoadBalancer.Algorithm == "bounded-consistent-hash") &&
		c.LoadBalancer.HashKey == "" {
		// Set default hash key
		c.LoadBalancer.HashKey = "source-ip"
	}

	// Validate TLS configuration
	if c.TLS != nil && c.TLS.Enabled {
		// Check for either new-style certificates or old-style cert/key files
		if len(c.TLS.Certificates) == 0 && (c.TLS.CertFile == "" || c.TLS.KeyFile == "") {
			return fmt.Errorf("TLS certificates or cert_file/key_file is required when TLS is enabled")
		}

		// Validate certificate configurations
		for i, certCfg := range c.TLS.Certificates {
			if certCfg.CertFile == "" {
				return fmt.Errorf("TLS certificate %d: cert_file is required", i)
			}
			if certCfg.KeyFile == "" {
				return fmt.Errorf("TLS certificate %d: key_file is required", i)
			}
		}

		// Validate TLS versions
		if c.TLS.MinVersion != "" {
			validVersions := map[string]bool{"1.0": true, "1.1": true, "1.2": true, "1.3": true}
			if !validVersions[c.TLS.MinVersion] {
				return fmt.Errorf("invalid TLS min_version: %s (must be 1.0, 1.1, 1.2, or 1.3)", c.TLS.MinVersion)
			}
		}

		if c.TLS.MaxVersion != "" {
			validVersions := map[string]bool{"1.0": true, "1.1": true, "1.2": true, "1.3": true}
			if !validVersions[c.TLS.MaxVersion] {
				return fmt.Errorf("invalid TLS max_version: %s (must be 1.0, 1.1, 1.2, or 1.3)", c.TLS.MaxVersion)
			}
		}

		// Validate client auth
		if c.TLS.ClientAuth != "" {
			validClientAuth := map[string]bool{
				"none": true, "request": true, "require": true,
				"verify": true, "require-and-verify": true,
			}
			if !validClientAuth[c.TLS.ClientAuth] {
				return fmt.Errorf("invalid TLS client_auth: %s", c.TLS.ClientAuth)
			}
		}
	}

	// Validate security configuration
	if c.Security != nil {
		if c.Security.RateLimit != nil && c.Security.RateLimit.Enabled {
			if c.Security.RateLimit.Type != "token-bucket" && c.Security.RateLimit.Type != "sliding-window" {
				return fmt.Errorf("invalid rate limit type: %s (must be 'token-bucket' or 'sliding-window')", c.Security.RateLimit.Type)
			}
		}
	}

	return nil
}

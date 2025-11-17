package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server represents the admin HTTP server for health checks and metrics
type Server struct {
	addr       string
	server     *http.Server
	mu         sync.RWMutex
	startTime  time.Time
	healthFunc func() bool
}

// Config contains configuration for the admin server
type Config struct {
	Listen     string
	HealthFunc func() bool
}

// NewServer creates a new admin server
func NewServer(cfg Config) *Server {
	s := &Server{
		addr:       cfg.Listen,
		startTime:  time.Now(),
		healthFunc: cfg.HealthFunc,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/healthz", s.handleHealth) // Kubernetes-style health check
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/readyz", s.handleReady) // Kubernetes-style readiness check
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/version", s.handleVersion)
	mux.Handle("/metrics", promhttp.Handler())

	s.server = &http.Server{
		Addr:         cfg.Listen,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start starts the admin server
func (s *Server) Start() error {
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Admin server error: %v\n", err)
		}
	}()
	return nil
}

// Shutdown gracefully shuts down the admin server
func (s *Server) Shutdown() error {
	return s.server.Close()
}

// Health response structure
type HealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Status response structure
type StatusResponse struct {
	Status      string            `json:"status"`
	Uptime      string            `json:"uptime"`
	UptimeSeconds int64            `json:"uptime_seconds"`
	Version     string            `json:"version"`
	GoVersion   string            `json:"go_version"`
	NumGoroutine int              `json:"num_goroutine"`
	Memory      MemoryStats       `json:"memory"`
	Timestamp   time.Time         `json:"timestamp"`
}

// MemoryStats contains memory statistics
type MemoryStats struct {
	Alloc      uint64 `json:"alloc_bytes"`
	TotalAlloc uint64 `json:"total_alloc_bytes"`
	Sys        uint64 `json:"sys_bytes"`
	NumGC      uint32 `json:"num_gc"`
}

// Version response structure
type VersionResponse struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

var (
	// Version information (set during build)
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// handleHealth handles the /health endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	healthy := true
	if s.healthFunc != nil {
		healthy = s.healthFunc()
	}

	status := "healthy"
	statusCode := http.StatusOK
	if !healthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(HealthResponse{
		Status: status,
	})
}

// handleReady handles the /ready endpoint (for readiness probes)
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	// For now, readiness is the same as health
	// In the future, this could check if backends are available
	s.handleHealth(w, r)
}

// handleStatus handles the /status endpoint
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(s.startTime)
	uptimeSeconds := int64(uptime.Seconds())

	status := StatusResponse{
		Status:        "running",
		Uptime:        uptime.String(),
		UptimeSeconds: uptimeSeconds,
		Version:       Version,
		GoVersion:     runtime.Version(),
		NumGoroutine:  runtime.NumGoroutine(),
		Memory: MemoryStats{
			Alloc:      m.Alloc,
			TotalAlloc: m.TotalAlloc,
			Sys:        m.Sys,
			NumGC:      m.NumGC,
		},
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// handleVersion handles the /version endpoint
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	version := VersionResponse{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(version)
}

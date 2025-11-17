package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		healthFunc     func() bool
		expectedStatus int
		expectedHealth string
	}{
		{
			name:           "healthy",
			healthFunc:     func() bool { return true },
			expectedStatus: http.StatusOK,
			expectedHealth: "healthy",
		},
		{
			name:           "unhealthy",
			healthFunc:     func() bool { return false },
			expectedStatus: http.StatusServiceUnavailable,
			expectedHealth: "unhealthy",
		},
		{
			name:           "no health func",
			healthFunc:     nil,
			expectedStatus: http.StatusOK,
			expectedHealth: "healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(Config{
				Listen:     ":0",
				HealthFunc: tt.healthFunc,
			})

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			srv.handleHealth(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			var resp HealthResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Status != tt.expectedHealth {
				t.Errorf("expected status %s, got %s", tt.expectedHealth, resp.Status)
			}
		})
	}
}

func TestReadyEndpoint(t *testing.T) {
	srv := NewServer(Config{
		Listen:     ":0",
		HealthFunc: func() bool { return true },
	})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	srv.handleReady(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status healthy, got %s", resp.Status)
	}
}

func TestStatusEndpoint(t *testing.T) {
	srv := NewServer(Config{
		Listen: ":0",
	})

	// Wait a bit to ensure uptime is measurable
	time.Sleep(100 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()

	srv.handleStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp StatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "running" {
		t.Errorf("expected status running, got %s", resp.Status)
	}

	if resp.UptimeSeconds < 0 {
		t.Errorf("expected positive uptime, got %d", resp.UptimeSeconds)
	}

	if resp.NumGoroutine <= 0 {
		t.Errorf("expected positive goroutine count, got %d", resp.NumGoroutine)
	}

	if resp.Memory.Alloc == 0 {
		t.Error("expected non-zero memory allocation")
	}
}

func TestVersionEndpoint(t *testing.T) {
	// Set version info for testing
	Version = "1.0.0"
	GitCommit = "abc123"
	BuildTime = "2024-01-01"

	srv := NewServer(Config{
		Listen: ":0",
	})

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()

	srv.handleVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp VersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", resp.Version)
	}

	if resp.GitCommit != "abc123" {
		t.Errorf("expected git commit abc123, got %s", resp.GitCommit)
	}

	if resp.BuildTime != "2024-01-01" {
		t.Errorf("expected build time 2024-01-01, got %s", resp.BuildTime)
	}
}

func TestServerStartStop(t *testing.T) {
	srv := NewServer(Config{
		Listen: "127.0.0.1:0", // Use random port
	})

	// Start server
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown server
	if err := srv.Shutdown(); err != nil {
		t.Fatalf("failed to shutdown server: %v", err)
	}
}

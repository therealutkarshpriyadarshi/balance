package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/config"
)

// TestHTTPProxyBasic tests basic HTTP proxying
func TestHTTPProxyBasic(t *testing.T) {
	// Create test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from backend"))
	}))
	defer backend.Close()

	// Extract host and port from backend URL
	backendAddr := strings.TrimPrefix(backend.URL, "http://")

	// Create proxy server configuration
	cfg := &config.Config{
		Mode:   "http",
		Listen: "127.0.0.1:0", // Use random available port
		Backends: []config.Backend{
			{
				Name:    "backend1",
				Address: backendAddr,
				Weight:  1,
			},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm: "round-robin",
		},
		HTTP: &config.HTTPConfig{
			EnableWebSocket:     false,
			EnableHTTP2:         false,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
		Timeouts: config.TimeoutConfig{
			Connect: 5 * time.Second,
			Read:    30 * time.Second,
			Write:   30 * time.Second,
			Idle:    60 * time.Second,
		},
	}

	// Create and start proxy server
	server, err := NewHTTPServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer server.Shutdown()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Get the actual listen address
	proxyURL := fmt.Sprintf("http://%s", cfg.Listen)

	// Make request to proxy
	resp, err := http.Get(proxyURL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if string(body) != "Hello from backend" {
		t.Errorf("Expected 'Hello from backend', got '%s'", string(body))
	}
}

// TestHTTPProxyLoadBalancing tests load balancing across multiple backends
func TestHTTPProxyLoadBalancing(t *testing.T) {
	// Track which backend handled each request
	var mu sync.Mutex
	backendCounts := make(map[string]int)

	// Create multiple backend servers
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		backendCounts["backend1"]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend1"))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		backendCounts["backend2"]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend2"))
	}))
	defer backend2.Close()

	backend3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		backendCounts["backend3"]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend3"))
	}))
	defer backend3.Close()

	// Create proxy server configuration
	cfg := &config.Config{
		Mode:   "http",
		Listen: "127.0.0.1:18081",
		Backends: []config.Backend{
			{
				Name:    "backend1",
				Address: strings.TrimPrefix(backend1.URL, "http://"),
				Weight:  1,
			},
			{
				Name:    "backend2",
				Address: strings.TrimPrefix(backend2.URL, "http://"),
				Weight:  1,
			},
			{
				Name:    "backend3",
				Address: strings.TrimPrefix(backend3.URL, "http://"),
				Weight:  1,
			},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm: "round-robin",
		},
		HTTP: &config.HTTPConfig{
			EnableWebSocket:     false,
			EnableHTTP2:         false,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
		Timeouts: config.TimeoutConfig{
			Connect: 5 * time.Second,
			Read:    30 * time.Second,
			Write:   30 * time.Second,
			Idle:    60 * time.Second,
		},
	}

	// Create and start proxy server
	server, err := NewHTTPServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer server.Shutdown()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Make multiple requests
	proxyURL := "http://127.0.0.1:18081"
	numRequests := 30

	for i := 0; i < numRequests; i++ {
		resp, err := http.Get(proxyURL)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	// Check that requests were distributed across all backends
	mu.Lock()
	defer mu.Unlock()

	t.Logf("Backend counts: %v", backendCounts)

	// Each backend should have received approximately equal number of requests
	for _, backend := range []string{"backend1", "backend2", "backend3"} {
		count := backendCounts[backend]
		expected := numRequests / 3
		// Allow for some variance (+/- 2)
		if count < expected-2 || count > expected+2 {
			t.Errorf("Backend %s received %d requests, expected ~%d", backend, count, expected)
		}
	}
}

// TestHTTPProxyHeaders tests that proxy headers are set correctly
func TestHTTPProxyHeaders(t *testing.T) {
	// Create test backend server that checks headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for X-Forwarded headers
		if xff := r.Header.Get("X-Forwarded-For"); xff == "" {
			t.Error("X-Forwarded-For header not set")
		}
		if xfh := r.Header.Get("X-Forwarded-Host"); xfh == "" {
			t.Error("X-Forwarded-Host header not set")
		}
		if xfp := r.Header.Get("X-Forwarded-Proto"); xfp == "" {
			t.Error("X-Forwarded-Proto header not set")
		}
		if xri := r.Header.Get("X-Real-IP"); xri == "" {
			t.Error("X-Real-IP header not set")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backend.Close()

	// Create proxy server configuration
	cfg := &config.Config{
		Mode:   "http",
		Listen: "127.0.0.1:18082",
		Backends: []config.Backend{
			{
				Name:    "backend1",
				Address: strings.TrimPrefix(backend.URL, "http://"),
				Weight:  1,
			},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm: "round-robin",
		},
		HTTP: &config.HTTPConfig{
			EnableWebSocket:     false,
			EnableHTTP2:         false,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
		Timeouts: config.TimeoutConfig{
			Connect: 5 * time.Second,
			Read:    30 * time.Second,
			Write:   30 * time.Second,
			Idle:    60 * time.Second,
		},
	}

	// Create and start proxy server
	server, err := NewHTTPServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer server.Shutdown()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Make request to proxy
	resp, err := http.Get("http://127.0.0.1:18082")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestHTTPProxyShutdown tests graceful shutdown
func TestHTTPProxyShutdown(t *testing.T) {
	// Create test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backend.Close()

	// Create proxy server configuration
	cfg := &config.Config{
		Mode:   "http",
		Listen: "127.0.0.1:18083",
		Backends: []config.Backend{
			{
				Name:    "backend1",
				Address: strings.TrimPrefix(backend.URL, "http://"),
				Weight:  1,
			},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm: "round-robin",
		},
		HTTP: &config.HTTPConfig{
			EnableWebSocket:     false,
			EnableHTTP2:         false,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
		Timeouts: config.TimeoutConfig{
			Connect: 5 * time.Second,
			Read:    30 * time.Second,
			Write:   30 * time.Second,
			Idle:    60 * time.Second,
		},
	}

	// Create and start proxy server
	server, err := NewHTTPServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create HTTP server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Make a request in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := http.Get("http://127.0.0.1:18083")
		if err != nil {
			// This is acceptable during shutdown
			return
		}
		defer resp.Body.Close()
	}()

	// Give the request time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown the server (should wait for in-flight requests)
	shutdownDone := make(chan struct{})
	go func() {
		server.Shutdown()
		close(shutdownDone)
	}()

	// Wait for shutdown to complete
	select {
	case <-shutdownDone:
		// Shutdown completed successfully
	case <-time.After(35 * time.Second):
		t.Error("Shutdown took too long")
	}

	wg.Wait()
}

// TestIsWebSocketRequest tests WebSocket detection
func TestIsWebSocketRequest(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name: "Valid WebSocket request",
			headers: map[string]string{
				"Upgrade":    "websocket",
				"Connection": "Upgrade",
			},
			expected: true,
		},
		{
			name: "Case insensitive",
			headers: map[string]string{
				"Upgrade":    "WebSocket",
				"Connection": "upgrade",
			},
			expected: true,
		},
		{
			name: "Connection with multiple values",
			headers: map[string]string{
				"Upgrade":    "websocket",
				"Connection": "keep-alive, Upgrade",
			},
			expected: true,
		},
		{
			name: "Missing Upgrade header",
			headers: map[string]string{
				"Connection": "Upgrade",
			},
			expected: false,
		},
		{
			name: "Missing Connection header",
			headers: map[string]string{
				"Upgrade": "websocket",
			},
			expected: false,
		},
		{
			name: "Wrong Upgrade value",
			headers: map[string]string{
				"Upgrade":    "http2",
				"Connection": "Upgrade",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := isWebSocketRequest(req)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestGetClientIP tests client IP extraction
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "Direct connection",
			remoteAddr: "192.168.1.100:12345",
			headers:    map[string]string{},
			expected:   "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "192.168.1.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1",
			},
			expected: "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			remoteAddr: "192.168.1.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 198.51.100.1, 192.0.2.1",
			},
			expected: "203.0.113.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "192.168.1.1:12345",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.2",
			},
			expected: "203.0.113.2",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			remoteAddr: "192.168.1.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1",
				"X-Real-IP":       "203.0.113.2",
			},
			expected: "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := getClientIP(req)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestGetScheme tests scheme detection
func TestGetScheme(t *testing.T) {
	tests := []struct {
		name     string
		hasTLS   bool
		headers  map[string]string
		expected string
	}{
		{
			name:     "HTTP request",
			hasTLS:   false,
			headers:  map[string]string{},
			expected: "http",
		},
		{
			name:     "X-Forwarded-Proto header",
			hasTLS:   false,
			headers:  map[string]string{"X-Forwarded-Proto": "https"},
			expected: "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := getScheme(req)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// Benchmark tests

// BenchmarkHTTPProxy benchmarks basic HTTP proxying
func BenchmarkHTTPProxy(b *testing.B) {
	// Create test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backend.Close()

	// Create proxy server configuration
	cfg := &config.Config{
		Mode:   "http",
		Listen: "127.0.0.1:18084",
		Backends: []config.Backend{
			{
				Name:    "backend1",
				Address: strings.TrimPrefix(backend.URL, "http://"),
				Weight:  1,
			},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm: "round-robin",
		},
		HTTP: &config.HTTPConfig{
			EnableWebSocket:     false,
			EnableHTTP2:         false,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
		Timeouts: config.TimeoutConfig{
			Connect: 5 * time.Second,
			Read:    30 * time.Second,
			Write:   30 * time.Second,
			Idle:    60 * time.Second,
		},
	}

	// Create and start proxy server
	server, err := NewHTTPServer(cfg)
	if err != nil {
		b.Fatalf("Failed to create HTTP server: %v", err)
	}

	if err := server.Start(); err != nil {
		b.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer server.Shutdown()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Create HTTP client
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get("http://127.0.0.1:18084")
			if err != nil {
				b.Fatalf("Request failed: %v", err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}

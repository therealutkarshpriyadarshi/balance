package tests

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/config"
	"github.com/therealutkarshpriyadarshi/balance/pkg/proxy"
)

// TestTCPProxyBasic tests basic TCP proxying functionality
func TestTCPProxyBasic(t *testing.T) {
	// Start a test backend server
	backend := startTCPBackend(t, "backend-1")
	defer backend.Close()

	// Create proxy configuration
	cfg := &config.Config{
		Mode:   "tcp",
		Listen: "127.0.0.1:0", // Random port
		Backends: []config.Backend{
			{
				Name:    "backend-1",
				Address: backend.Addr().String(),
				Weight:  1,
			},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm: "round-robin",
		},
		Timeouts: &config.TimeoutConfig{
			Connect: 5 * time.Second,
			Read:    30 * time.Second,
			Write:   30 * time.Second,
			Idle:    60 * time.Second,
		},
	}

	// Create and start proxy server
	server, err := proxy.NewTCPServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start proxy server: %v", err)
	}
	defer server.Shutdown()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Connect to proxy
	conn, err := net.Dial("tcp", cfg.Listen)
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// Send data
	testData := "Hello, Backend!"
	if _, err := conn.Write([]byte(testData)); err != nil {
		t.Fatalf("Failed to write to proxy: %v", err)
	}

	// Read response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read from proxy: %v", err)
	}

	response := string(buf[:n])
	expected := "backend-1: " + testData
	if response != expected {
		t.Errorf("Expected response %q, got %q", expected, response)
	}
}

// TestHTTPProxyBasic tests basic HTTP proxying functionality
func TestHTTPProxyBasic(t *testing.T) {
	// Start a test backend server
	backend := startHTTPBackend(t, "backend-1")
	defer backend.Close()

	// Create proxy configuration
	cfg := &config.Config{
		Mode:   "http",
		Listen: "127.0.0.1:0", // Random port
		Backends: []config.Backend{
			{
				Name:    "backend-1",
				Address: backend.URL[7:], // Remove "http://"
				Weight:  1,
			},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm: "round-robin",
		},
		Timeouts: &config.TimeoutConfig{
			Connect: 5 * time.Second,
			Read:    30 * time.Second,
			Write:   30 * time.Second,
			Idle:    60 * time.Second,
		},
	}

	// Create and start proxy server
	server, err := proxy.NewHTTPServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start proxy server: %v", err)
	}
	defer server.Shutdown()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Make HTTP request to proxy
	resp, err := http.Get("http://" + cfg.Listen)
	if err != nil {
		t.Fatalf("Failed to make request to proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expected := "backend-1"
	if string(body) != expected {
		t.Errorf("Expected body %q, got %q", expected, string(body))
	}
}

// TestLoadBalancing tests load balancing across multiple backends
func TestLoadBalancing(t *testing.T) {
	// Start multiple test backend servers
	backend1 := startHTTPBackend(t, "backend-1")
	defer backend1.Close()
	backend2 := startHTTPBackend(t, "backend-2")
	defer backend2.Close()
	backend3 := startHTTPBackend(t, "backend-3")
	defer backend3.Close()

	// Create proxy configuration
	cfg := &config.Config{
		Mode:   "http",
		Listen: "127.0.0.1:0",
		Backends: []config.Backend{
			{Name: "backend-1", Address: backend1.URL[7:], Weight: 1},
			{Name: "backend-2", Address: backend2.URL[7:], Weight: 1},
			{Name: "backend-3", Address: backend3.URL[7:], Weight: 1},
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm: "round-robin",
		},
		Timeouts: &config.TimeoutConfig{
			Connect: 5 * time.Second,
			Read:    30 * time.Second,
			Write:   30 * time.Second,
			Idle:    60 * time.Second,
		},
	}

	// Create and start proxy server
	server, err := proxy.NewHTTPServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start proxy server: %v", err)
	}
	defer server.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Make multiple requests and track which backends are hit
	backendHits := make(map[string]int)
	for i := 0; i < 12; i++ {
		resp, err := http.Get("http://" + cfg.Listen)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		backendHits[string(body)]++
	}

	// Verify that all backends were hit
	if len(backendHits) != 3 {
		t.Errorf("Expected 3 backends to be hit, got %d", len(backendHits))
	}

	// With round-robin, each backend should be hit equally
	for backend, hits := range backendHits {
		if hits != 4 {
			t.Errorf("Backend %s hit %d times, expected 4", backend, hits)
		}
	}
}

// Helper: Start a simple TCP echo backend
func startTCPBackend(t *testing.T, name string) net.Listener {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start TCP backend: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				response := fmt.Sprintf("%s: %s", name, string(buf[:n]))
				c.Write([]byte(response))
			}(conn)
		}
	}()

	return listener
}

// Helper: Start a simple HTTP backend
func startHTTPBackend(t *testing.T, name string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(name))
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		server.Serve(listener)
	}()

	// Store the URL for easy access
	server.URL = "http://" + listener.Addr().String()

	return server
}

// Add URL field to http.Server for convenience
type testServer struct {
	*http.Server
	URL string
}

// Wrapper to properly type our test server
func (s *http.Server) Close() error {
	return s.Shutdown(context.Background())
}

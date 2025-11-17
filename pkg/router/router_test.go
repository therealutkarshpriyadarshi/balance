package router

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
	"github.com/therealutkarshpriyadarshi/balance/pkg/config"
)

func TestRouterHostMatching(t *testing.T) {
	// Create backend pool
	pool := backend.NewPool()
	api1 := backend.NewBackend("api1", "localhost:9001", 1)
	api2 := backend.NewBackend("api2", "localhost:9002", 1)
	web1 := backend.NewBackend("web1", "localhost:9003", 1)
	pool.Add(api1)
	pool.Add(api2)
	pool.Add(web1)

	// Create routes
	routes := []config.Route{
		{
			Name:     "api-route",
			Host:     "api.example.com",
			Backends: []string{"api1", "api2"},
			Priority: 10,
		},
		{
			Name:     "web-route",
			Host:     "www.example.com",
			Backends: []string{"web1"},
			Priority: 5,
		},
	}

	router := NewRouter(routes, pool)

	tests := []struct {
		name         string
		host         string
		expectedPool int // Expected number of backends in matched pool
	}{
		{
			name:         "Match api host",
			host:         "api.example.com",
			expectedPool: 2, // api1, api2
		},
		{
			name:         "Match web host",
			host:         "www.example.com",
			expectedPool: 1, // web1
		},
		{
			name:         "No match - use default",
			host:         "other.example.com",
			expectedPool: 3, // All backends
		},
		{
			name:         "Host with port",
			host:         "api.example.com:8080",
			expectedPool: 2, // api1, api2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tt.host

			matchedPool := router.Match(req)
			if matchedPool == nil {
				t.Fatal("Expected matched pool, got nil")
			}

			if matchedPool.Size() != tt.expectedPool {
				t.Errorf("Expected pool size %d, got %d", tt.expectedPool, matchedPool.Size())
			}
		})
	}
}

func TestRouterPathMatching(t *testing.T) {
	// Create backend pool
	pool := backend.NewPool()
	api := backend.NewBackend("api", "localhost:9001", 1)
	admin := backend.NewBackend("admin", "localhost:9002", 1)
	pool.Add(api)
	pool.Add(admin)

	// Create routes
	routes := []config.Route{
		{
			Name:       "api-route",
			PathPrefix: "/api/",
			Backends:   []string{"api"},
			Priority:   10,
		},
		{
			Name:       "admin-route",
			PathPrefix: "/admin/",
			Backends:   []string{"admin"},
			Priority:   10,
		},
	}

	router := NewRouter(routes, pool)

	tests := []struct {
		name         string
		path         string
		expectedPool int
	}{
		{
			name:         "Match /api/ prefix",
			path:         "/api/users",
			expectedPool: 1, // api
		},
		{
			name:         "Match /admin/ prefix",
			path:         "/admin/settings",
			expectedPool: 1, // admin
		},
		{
			name:         "No match - use default",
			path:         "/public/index.html",
			expectedPool: 2, // All backends
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)

			matchedPool := router.Match(req)
			if matchedPool == nil {
				t.Fatal("Expected matched pool, got nil")
			}

			if matchedPool.Size() != tt.expectedPool {
				t.Errorf("Expected pool size %d, got %d", tt.expectedPool, matchedPool.Size())
			}
		})
	}
}

func TestRouterHeaderMatching(t *testing.T) {
	// Create backend pool
	pool := backend.NewPool()
	premium := backend.NewBackend("premium", "localhost:9001", 1)
	standard := backend.NewBackend("standard", "localhost:9002", 1)
	pool.Add(premium)
	pool.Add(standard)

	// Create routes
	routes := []config.Route{
		{
			Name: "premium-route",
			Headers: map[string]string{
				"X-API-Key": "premium-key",
			},
			Backends: []string{"premium"},
			Priority: 10,
		},
	}

	router := NewRouter(routes, pool)

	tests := []struct {
		name         string
		headers      map[string]string
		expectedPool int
	}{
		{
			name: "Match premium header",
			headers: map[string]string{
				"X-API-Key": "premium-key",
			},
			expectedPool: 1, // premium
		},
		{
			name: "No header match - use default",
			headers: map[string]string{
				"X-API-Key": "other-key",
			},
			expectedPool: 2, // All backends
		},
		{
			name:         "No headers - use default",
			headers:      map[string]string{},
			expectedPool: 2, // All backends
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			matchedPool := router.Match(req)
			if matchedPool == nil {
				t.Fatal("Expected matched pool, got nil")
			}

			if matchedPool.Size() != tt.expectedPool {
				t.Errorf("Expected pool size %d, got %d", tt.expectedPool, matchedPool.Size())
			}
		})
	}
}

func TestRouterPriority(t *testing.T) {
	// Create backend pool
	pool := backend.NewPool()
	high := backend.NewBackend("high", "localhost:9001", 1)
	low := backend.NewBackend("low", "localhost:9002", 1)
	pool.Add(high)
	pool.Add(low)

	// Create routes with different priorities
	routes := []config.Route{
		{
			Name:       "low-priority",
			PathPrefix: "/api/",
			Backends:   []string{"low"},
			Priority:   5,
		},
		{
			Name:       "high-priority",
			PathPrefix: "/api/v1/",
			Backends:   []string{"high"},
			Priority:   10,
		},
	}

	router := NewRouter(routes, pool)

	// Request to /api/v1/users should match high-priority route
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	matchedPool := router.Match(req)

	if matchedPool == nil {
		t.Fatal("Expected matched pool, got nil")
	}

	// Should match high-priority route with 1 backend
	if matchedPool.Size() != 1 {
		t.Errorf("Expected pool size 1, got %d", matchedPool.Size())
	}

	// Verify it's the high-priority backend
	backends := matchedPool.All()
	if len(backends) > 0 && backends[0].Name() != "high" {
		t.Errorf("Expected 'high' backend, got '%s'", backends[0].Name())
	}
}

func TestRouterCombinedMatching(t *testing.T) {
	// Create backend pool
	pool := backend.NewPool()
	specific := backend.NewBackend("specific", "localhost:9001", 1)
	general := backend.NewBackend("general", "localhost:9002", 1)
	pool.Add(specific)
	pool.Add(general)

	// Create routes with combined matching
	routes := []config.Route{
		{
			Name:       "specific-route",
			Host:       "api.example.com",
			PathPrefix: "/v1/",
			Headers: map[string]string{
				"X-API-Version": "v1",
			},
			Backends: []string{"specific"},
			Priority: 10,
		},
		{
			Name:       "general-route",
			Host:       "api.example.com",
			PathPrefix: "/",
			Backends:   []string{"general"},
			Priority:   5,
		},
	}

	router := NewRouter(routes, pool)

	tests := []struct {
		name         string
		host         string
		path         string
		headers      map[string]string
		expectedPool int
		expectedName string
	}{
		{
			name: "Match specific route (all criteria)",
			host: "api.example.com",
			path: "/v1/users",
			headers: map[string]string{
				"X-API-Version": "v1",
			},
			expectedPool: 1,
			expectedName: "specific",
		},
		{
			name: "Match general route (host and path only)",
			host: "api.example.com",
			path: "/v1/users",
			headers: map[string]string{
				"X-API-Version": "v2",
			},
			expectedPool: 1,
			expectedName: "general",
		},
		{
			name:         "No match - use default",
			host:         "other.example.com",
			path:         "/users",
			headers:      map[string]string{},
			expectedPool: 2,
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.Host = tt.host
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			matchedPool := router.Match(req)
			if matchedPool == nil {
				t.Fatal("Expected matched pool, got nil")
			}

			if matchedPool.Size() != tt.expectedPool {
				t.Errorf("Expected pool size %d, got %d", tt.expectedPool, matchedPool.Size())
			}

			if tt.expectedName != "" {
				backends := matchedPool.All()
				if len(backends) > 0 && backends[0].Name() != tt.expectedName {
					t.Errorf("Expected '%s' backend, got '%s'", tt.expectedName, backends[0].Name())
				}
			}
		})
	}
}

func TestMatchHost(t *testing.T) {
	tests := []struct {
		name        string
		requestHost string
		routeHost   string
		expected    bool
	}{
		{
			name:        "Exact match",
			requestHost: "example.com",
			routeHost:   "example.com",
			expected:    true,
		},
		{
			name:        "Exact match with port",
			requestHost: "example.com:8080",
			routeHost:   "example.com",
			expected:    true,
		},
		{
			name:        "Wildcard match",
			requestHost: "api.example.com",
			routeHost:   "*.example.com",
			expected:    true,
		},
		{
			name:        "Wildcard match with subdomain",
			requestHost: "v1.api.example.com",
			routeHost:   "*.example.com",
			expected:    true,
		},
		{
			name:        "No match",
			requestHost: "example.com",
			routeHost:   "other.com",
			expected:    false,
		},
		{
			name:        "Wildcard no match",
			requestHost: "example.com",
			routeHost:   "*.other.com",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchHost(tt.requestHost, tt.routeHost)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// BenchmarkRouterMatch benchmarks route matching
func BenchmarkRouterMatch(b *testing.B) {
	// Create backend pool
	pool := backend.NewPool()
	for i := 0; i < 10; i++ {
		pool.Add(backend.NewBackend(
			fmt.Sprintf("backend-%d", i),
			fmt.Sprintf("localhost:900%d", i),
			1,
		))
	}

	// Create routes
	routes := []config.Route{
		{
			Name:       "api-route",
			Host:       "api.example.com",
			PathPrefix: "/api/",
			Priority:   10,
			Backends:   []string{"backend-0", "backend-1"},
		},
		{
			Name:       "web-route",
			Host:       "www.example.com",
			PathPrefix: "/",
			Priority:   5,
			Backends:   []string{"backend-2", "backend-3"},
		},
	}

	router := NewRouter(routes, pool)
	req := httptest.NewRequest("GET", "/api/users", nil)
	req.Host = "api.example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.Match(req)
	}
}

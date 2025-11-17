package health

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

func TestNewActiveChecker(t *testing.T) {
	config := ActiveCheckerConfig{
		CheckType: CheckTypeTCP,
		Timeout:   5 * time.Second,
	}

	checker := NewActiveChecker(config)

	if checker == nil {
		t.Fatal("Expected checker to be created")
	}

	if checker.checkType != CheckTypeTCP {
		t.Errorf("Expected check type TCP, got %s", checker.checkType)
	}

	if checker.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %s", checker.timeout)
	}
}

func TestNewActiveChecker_Defaults(t *testing.T) {
	config := ActiveCheckerConfig{}
	checker := NewActiveChecker(config)

	if checker.checkType != CheckTypeTCP {
		t.Errorf("Expected default check type TCP, got %s", checker.checkType)
	}

	if checker.timeout != 3*time.Second {
		t.Errorf("Expected default timeout 3s, got %s", checker.timeout)
	}

	if checker.httpPath != "/health" {
		t.Errorf("Expected default path /health, got %s", checker.httpPath)
	}
}

func TestActiveChecker_TCPCheck_Success(t *testing.T) {
	// Start a TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// Create checker
	checker := NewActiveChecker(ActiveCheckerConfig{
		CheckType: CheckTypeTCP,
		Timeout:   1 * time.Second,
	})

	// Create backend pointing to our listener
	b := backend.NewBackend("test", listener.Addr().String(), 1)

	// Perform check
	result := checker.Check(context.Background(), b)

	if !result.Success {
		t.Errorf("Expected check to succeed, got error: %v", result.Error)
	}

	if result.Backend != b {
		t.Error("Expected result to reference the backend")
	}

	if result.Duration == 0 {
		t.Error("Expected duration to be recorded")
	}
}

func TestActiveChecker_TCPCheck_Failure(t *testing.T) {
	checker := NewActiveChecker(ActiveCheckerConfig{
		CheckType: CheckTypeTCP,
		Timeout:   1 * time.Second,
	})

	// Create backend pointing to a non-existent server
	b := backend.NewBackend("test", "127.0.0.1:1", 1)

	// Perform check
	result := checker.Check(context.Background(), b)

	if result.Success {
		t.Error("Expected check to fail")
	}

	if result.Error == nil {
		t.Error("Expected error to be set")
	}
}

func TestActiveChecker_HTTPCheck_Success(t *testing.T) {
	// Start HTTP test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Extract host from server URL
	host := server.Listener.Addr().String()

	// Create checker
	checker := NewActiveChecker(ActiveCheckerConfig{
		CheckType: CheckTypeHTTP,
		Timeout:   1 * time.Second,
		HTTPPath:  "/health",
	})

	// Create backend
	b := backend.NewBackend("test", host, 1)

	// Perform check
	result := checker.Check(context.Background(), b)

	if !result.Success {
		t.Errorf("Expected check to succeed, got error: %v", result.Error)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", result.StatusCode)
	}
}

func TestActiveChecker_HTTPCheck_WrongStatusCode(t *testing.T) {
	// Start HTTP test server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	host := server.Listener.Addr().String()

	checker := NewActiveChecker(ActiveCheckerConfig{
		CheckType:           CheckTypeHTTP,
		Timeout:             1 * time.Second,
		ExpectedStatusCodes: []int{http.StatusOK},
	})

	b := backend.NewBackend("test", host, 1)
	result := checker.Check(context.Background(), b)

	if result.Success {
		t.Error("Expected check to fail due to wrong status code")
	}

	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code 500, got %d", result.StatusCode)
	}
}

func TestActiveChecker_CheckMultiple(t *testing.T) {
	// Start multiple test servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	checker := NewActiveChecker(ActiveCheckerConfig{
		CheckType: CheckTypeHTTP,
		Timeout:   1 * time.Second,
	})

	backends := []*backend.Backend{
		backend.NewBackend("test1", server1.Listener.Addr().String(), 1),
		backend.NewBackend("test2", server2.Listener.Addr().String(), 1),
	}

	results := checker.CheckMultiple(context.Background(), backends)

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.Success {
			t.Errorf("Check %d failed: %v", i, result.Error)
		}
	}
}

func TestActiveChecker_CheckWithTimeout(t *testing.T) {
	// Start a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := NewActiveChecker(ActiveCheckerConfig{
		CheckType: CheckTypeHTTP,
		Timeout:   100 * time.Millisecond, // Short timeout
	})

	b := backend.NewBackend("test", server.Listener.Addr().String(), 1)
	result := checker.CheckWithTimeout(b, 100*time.Millisecond)

	if result.Success {
		t.Error("Expected check to timeout")
	}
}

func TestActiveChecker_ContextCancellation(t *testing.T) {
	// Start a server that never responds
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	// Don't accept connections - this will cause the check to hang

	checker := NewActiveChecker(ActiveCheckerConfig{
		CheckType: CheckTypeTCP,
		Timeout:   10 * time.Second,
	})

	b := backend.NewBackend("test", listener.Addr().String(), 1)

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	result := checker.Check(ctx, b)

	if result.Success {
		t.Error("Expected check to fail due to cancelled context")
	}
}

func TestCheckType_String(t *testing.T) {
	tests := []struct {
		checkType CheckType
		expected  string
	}{
		{CheckTypeTCP, "tcp"},
		{CheckTypeHTTP, "http"},
		{CheckTypeHTTPS, "https"},
	}

	for _, tt := range tests {
		t.Run(string(tt.checkType), func(t *testing.T) {
			if string(tt.checkType) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.checkType))
			}
		})
	}
}

package lb

import (
	"testing"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

func TestSessionAffinity(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 1)
	b3 := backend.NewBackend("backend-3", "localhost:9003", 1)

	pool.Add(b1)
	pool.Add(b2)
	pool.Add(b3)

	baseBalancer := NewRoundRobin(pool)
	sa := NewSessionAffinity(baseBalancer, 5*time.Second)
	defer sa.Stop()

	// Test name
	expectedName := "round-robin-with-affinity"
	if sa.Name() != expectedName {
		t.Errorf("Expected name '%s', got '%s'", expectedName, sa.Name())
	}

	// Test that same client IP gets same backend
	clientIP := "192.168.1.100"
	firstSelection := sa.SelectWithClientIP(clientIP)
	if firstSelection == nil {
		t.Fatal("Expected backend, got nil")
	}

	// Subsequent selections should return same backend
	for i := 0; i < 10; i++ {
		selected := sa.SelectWithClientIP(clientIP)
		if selected == nil || selected.Name() != firstSelection.Name() {
			t.Errorf("Session affinity failed: expected %s, got %v", firstSelection.Name(), selected)
		}
	}

	// Different client IP should potentially get different backend
	clientIP2 := "192.168.1.101"
	secondSelection := sa.SelectWithClientIP(clientIP2)
	if secondSelection == nil {
		t.Fatal("Expected backend for second client, got nil")
	}

	// Second client should consistently get the same backend too
	for i := 0; i < 10; i++ {
		selected := sa.SelectWithClientIP(clientIP2)
		if selected == nil || selected.Name() != secondSelection.Name() {
			t.Errorf("Session affinity failed for second client: expected %s, got %v", secondSelection.Name(), selected)
		}
	}
}

func TestSessionAffinityTimeout(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	pool.Add(b1)

	baseBalancer := NewRoundRobin(pool)
	sa := NewSessionAffinity(baseBalancer, 100*time.Millisecond)
	defer sa.Stop()

	clientIP := "192.168.1.100"
	firstSelection := sa.SelectWithClientIP(clientIP)
	if firstSelection == nil {
		t.Fatal("Expected backend, got nil")
	}

	// Session should exist
	if sa.SessionCount() != 1 {
		t.Errorf("Expected 1 session, got %d", sa.SessionCount())
	}

	// Wait for session to expire
	time.Sleep(150 * time.Millisecond)

	// Trigger cleanup manually
	sa.cleanup()

	// Session should be removed
	if sa.SessionCount() != 0 {
		t.Errorf("Expected session to expire, but still have %d sessions", sa.SessionCount())
	}
}

func TestSessionAffinityUnhealthyBackend(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 1)

	pool.Add(b1)
	pool.Add(b2)

	baseBalancer := NewRoundRobin(pool)
	sa := NewSessionAffinity(baseBalancer, 5*time.Second)
	defer sa.Stop()

	clientIP := "192.168.1.100"
	firstSelection := sa.SelectWithClientIP(clientIP)
	if firstSelection == nil {
		t.Fatal("Expected backend, got nil")
	}

	// Mark the selected backend as unhealthy
	firstSelection.MarkUnhealthy()

	// Next selection should use a different backend
	secondSelection := sa.SelectWithClientIP(clientIP)
	if secondSelection == nil {
		t.Fatal("Expected backend, got nil")
	}

	if secondSelection.Name() == firstSelection.Name() {
		t.Error("Expected to select different backend when original becomes unhealthy")
	}
}

func TestSessionAffinityClearSession(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	pool.Add(b1)

	baseBalancer := NewRoundRobin(pool)
	sa := NewSessionAffinity(baseBalancer, 5*time.Second)
	defer sa.Stop()

	clientIP := "192.168.1.100"
	sa.SelectWithClientIP(clientIP)

	if sa.SessionCount() != 1 {
		t.Errorf("Expected 1 session, got %d", sa.SessionCount())
	}

	// Clear the session
	sa.ClearSession(clientIP)

	if sa.SessionCount() != 0 {
		t.Errorf("Expected 0 sessions after clear, got %d", sa.SessionCount())
	}
}

func TestSessionAffinityClearAllSessions(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	pool.Add(b1)

	baseBalancer := NewRoundRobin(pool)
	sa := NewSessionAffinity(baseBalancer, 5*time.Second)
	defer sa.Stop()

	// Create multiple sessions
	for i := 0; i < 10; i++ {
		clientIP := string(rune(i))
		sa.SelectWithClientIP(clientIP)
	}

	if sa.SessionCount() != 10 {
		t.Errorf("Expected 10 sessions, got %d", sa.SessionCount())
	}

	// Clear all sessions
	sa.ClearAllSessions()

	if sa.SessionCount() != 0 {
		t.Errorf("Expected 0 sessions after clear all, got %d", sa.SessionCount())
	}
}

func TestSessionAffinityNoBackends(t *testing.T) {
	pool := backend.NewPool()

	baseBalancer := NewRoundRobin(pool)
	sa := NewSessionAffinity(baseBalancer, 5*time.Second)
	defer sa.Stop()

	clientIP := "192.168.1.100"
	selected := sa.SelectWithClientIP(clientIP)

	if selected != nil {
		t.Errorf("Expected nil for empty pool, got %v", selected)
	}
}

func TestSessionAffinityDefaultTimeout(t *testing.T) {
	pool := backend.NewPool()
	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	pool.Add(b1)

	baseBalancer := NewRoundRobin(pool)

	// Pass 0 or negative timeout to test default
	sa := NewSessionAffinity(baseBalancer, 0)
	defer sa.Stop()

	if sa.timeout != 10*time.Minute {
		t.Errorf("Expected default timeout of 10 minutes, got %v", sa.timeout)
	}
}

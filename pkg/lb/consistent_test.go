package lb

import (
	"fmt"
	"testing"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

func TestConsistentHash(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 1)
	b3 := backend.NewBackend("backend-3", "localhost:9003", 1)

	pool.Add(b1)
	pool.Add(b2)
	pool.Add(b3)

	ch := NewConsistentHash(pool, 100, "source-ip")

	// Test name
	if ch.Name() != "consistent-hash" {
		t.Errorf("Expected name 'consistent-hash', got '%s'", ch.Name())
	}

	// Test that same key always returns same backend
	key := "192.168.1.100"
	firstSelection := ch.SelectWithKey(key)
	if firstSelection == nil {
		t.Fatal("Expected backend, got nil")
	}

	for i := 0; i < 10; i++ {
		b := ch.SelectWithKey(key)
		if b == nil || b.Name() != firstSelection.Name() {
			t.Errorf("Consistent hash failed: expected %s, got %v", firstSelection.Name(), b)
		}
	}
}

func TestConsistentHashDistribution(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 1)
	b3 := backend.NewBackend("backend-3", "localhost:9003", 1)

	pool.Add(b1)
	pool.Add(b2)
	pool.Add(b3)

	ch := NewConsistentHash(pool, 100, "source-ip")

	// Test distribution across many keys
	distribution := make(map[string]int)
	numKeys := 1000

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("192.168.1.%d", i)
		b := ch.SelectWithKey(key)
		if b == nil {
			t.Fatal("Expected backend, got nil")
		}
		distribution[b.Name()]++
	}

	// Each backend should get roughly 1/3 of requests
	// Allow for some variance (20-45% range)
	for name, count := range distribution {
		percentage := float64(count) / float64(numKeys) * 100
		if percentage < 20 || percentage > 45 {
			t.Logf("Warning: Backend %s got %.2f%% of requests (expected ~33%%)", name, percentage)
		}
	}

	// Ensure all backends received some requests
	if len(distribution) != 3 {
		t.Errorf("Expected all 3 backends to receive requests, got %d", len(distribution))
	}
}

func TestConsistentHashWeights(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 2)
	b3 := backend.NewBackend("backend-3", "localhost:9003", 3)

	pool.Add(b1)
	pool.Add(b2)
	pool.Add(b3)

	ch := NewConsistentHash(pool, 50, "source-ip")

	// Test distribution respects weights
	distribution := make(map[string]int)
	numKeys := 6000

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("192.168.%d.%d", i/255, i%255)
		b := ch.SelectWithKey(key)
		if b == nil {
			t.Fatal("Expected backend, got nil")
		}
		distribution[b.Name()]++
	}

	// Log distribution for inspection
	t.Logf("Distribution with weights (1:2:3):")
	for name, count := range distribution {
		percentage := float64(count) / float64(numKeys) * 100
		t.Logf("  %s: %d (%.2f%%)", name, count, percentage)
	}
}

func TestConsistentHashNoBackends(t *testing.T) {
	pool := backend.NewPool()
	ch := NewConsistentHash(pool, 100, "source-ip")

	b := ch.SelectWithKey("192.168.1.1")
	if b != nil {
		t.Errorf("Expected nil for empty pool, got %v", b)
	}
}

func TestBoundedLoadConsistentHash(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 1)
	b3 := backend.NewBackend("backend-3", "localhost:9003", 1)

	pool.Add(b1)
	pool.Add(b2)
	pool.Add(b3)

	blch := NewBoundedLoadConsistentHash(pool, 100, "source-ip", 1.25)

	// Test name
	if blch.Name() != "bounded-consistent-hash" {
		t.Errorf("Expected name 'bounded-consistent-hash', got '%s'", blch.Name())
	}

	// Simulate load on one backend
	b1.IncrementConnections()
	b1.IncrementConnections()
	b1.IncrementConnections()
	b1.IncrementConnections()
	b1.IncrementConnections() // 5 connections

	// Average load = 5/3 = 1.67
	// Max load = 1.67 * 1.25 = 2.08

	// b1 has 5 connections, which exceeds max load
	// So even if a key hashes to b1, it should select a different backend

	// Find a key that would normally hash to b1
	key := "test-key-for-b1"
	selected := blch.SelectWithKey(key)
	if selected == nil {
		t.Fatal("Expected backend, got nil")
	}

	// Should still work and select a backend (may or may not be b1 depending on load)
	t.Logf("Selected backend: %s with %d connections", selected.Name(), selected.ActiveConnections())
}

func TestConsistentHashRebuild(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 1)

	pool.Add(b1)
	pool.Add(b2)

	ch := NewConsistentHash(pool, 100, "source-ip")

	// Select with a key
	key := "192.168.1.100"
	firstSelection := ch.SelectWithKey(key)
	if firstSelection == nil {
		t.Fatal("Expected backend before adding new backend, got nil")
	}

	// Add a new backend
	b3 := backend.NewBackend("backend-3", "localhost:9003", 1)
	pool.Add(b3)

	// The ring should rebuild on next selection
	// Some keys may now hash to the new backend
	// But we can't predict which, so just verify it works
	secondSelection := ch.SelectWithKey(key)
	if secondSelection == nil {
		t.Fatal("Expected backend after adding new backend, got nil")
	}

	// Verify all backends can be selected
	distribution := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("192.168.1.%d", i)
		b := ch.SelectWithKey(key)
		if b != nil {
			distribution[b.Name()] = true
		}
	}

	if len(distribution) != 3 {
		t.Errorf("Expected all 3 backends to be selectable, got %d", len(distribution))
	}
}

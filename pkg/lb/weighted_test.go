package lb

import (
	"testing"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

func TestWeightedRoundRobin(t *testing.T) {
	pool := backend.NewPool()

	// Create backends with different weights
	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 2)
	b3 := backend.NewBackend("backend-3", "localhost:9003", 3)

	pool.Add(b1)
	pool.Add(b2)
	pool.Add(b3)

	wrr := NewWeightedRoundRobin(pool)

	// Test name
	if wrr.Name() != "weighted-round-robin" {
		t.Errorf("Expected name 'weighted-round-robin', got '%s'", wrr.Name())
	}

	// Select backends and count distribution
	distribution := make(map[string]int)
	numSelections := 600 // Should be divisible by sum of weights (1+2+3=6)

	for i := 0; i < numSelections; i++ {
		b := wrr.Select()
		if b == nil {
			t.Fatal("Expected backend, got nil")
		}
		distribution[b.Name()]++
	}

	// Check distribution matches weights
	// backend-1 (weight 1): should get 1/6 = 100 requests
	// backend-2 (weight 2): should get 2/6 = 200 requests
	// backend-3 (weight 3): should get 3/6 = 300 requests

	expectedRatio := numSelections / 6
	if distribution["backend-1"] != expectedRatio {
		t.Errorf("Expected backend-1 to receive %d requests, got %d", expectedRatio, distribution["backend-1"])
	}
	if distribution["backend-2"] != expectedRatio*2 {
		t.Errorf("Expected backend-2 to receive %d requests, got %d", expectedRatio*2, distribution["backend-2"])
	}
	if distribution["backend-3"] != expectedRatio*3 {
		t.Errorf("Expected backend-3 to receive %d requests, got %d", expectedRatio*3, distribution["backend-3"])
	}
}

func TestWeightedRoundRobinSingleBackend(t *testing.T) {
	pool := backend.NewPool()
	b1 := backend.NewBackend("backend-1", "localhost:9001", 5)
	pool.Add(b1)

	wrr := NewWeightedRoundRobin(pool)

	// Should always return the same backend
	for i := 0; i < 10; i++ {
		b := wrr.Select()
		if b == nil || b.Name() != "backend-1" {
			t.Errorf("Expected backend-1, got %v", b)
		}
	}
}

func TestWeightedRoundRobinNoBackends(t *testing.T) {
	pool := backend.NewPool()
	wrr := NewWeightedRoundRobin(pool)

	b := wrr.Select()
	if b != nil {
		t.Errorf("Expected nil for empty pool, got %v", b)
	}
}

func TestWeightedRoundRobinUnhealthyBackend(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 2)

	pool.Add(b1)
	pool.Add(b2)

	// Mark backend-2 as unhealthy
	b2.MarkUnhealthy()

	wrr := NewWeightedRoundRobin(pool)

	// Should only select backend-1
	for i := 0; i < 10; i++ {
		b := wrr.Select()
		if b == nil || b.Name() != "backend-1" {
			t.Errorf("Expected backend-1, got %v", b)
		}
	}
}

func TestWeightedLeastConnections(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 1)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 2)
	b3 := backend.NewBackend("backend-3", "localhost:9003", 3)

	pool.Add(b1)
	pool.Add(b2)
	pool.Add(b3)

	wlc := NewWeightedLeastConnections(pool)

	// Test name
	if wlc.Name() != "weighted-least-connections" {
		t.Errorf("Expected name 'weighted-least-connections', got '%s'", wlc.Name())
	}

	// Initially, all have 0 connections
	// Should distribute based on weights initially
	// But weighted least connections considers ratio: connections/weight

	// Set different connection counts
	b1.IncrementConnections() // 1 connection, weight 1, ratio = 1.0
	b2.IncrementConnections() // 1 connection, weight 2, ratio = 0.5
	b2.IncrementConnections() // 2 connections, weight 2, ratio = 1.0
	b3.IncrementConnections() // 1 connection, weight 3, ratio = 0.33

	// Should select b3 (lowest ratio)
	selected := wlc.Select()
	if selected == nil || selected.Name() != "backend-3" {
		t.Errorf("Expected backend-3 (lowest ratio), got %v", selected)
	}

	// Add more connections to b3
	b3.IncrementConnections() // 2 connections, weight 3, ratio = 0.67
	b3.IncrementConnections() // 3 connections, weight 3, ratio = 1.0

	// Now b2 has ratio 1.0 (2/2), should select it or others with lower ratio
	selected = wlc.Select()
	if selected == nil {
		t.Fatal("Expected a backend, got nil")
	}
	// All have ratio 1.0 now, any is acceptable
}

func TestWeightedLeastConnectionsSingleBackend(t *testing.T) {
	pool := backend.NewPool()
	b1 := backend.NewBackend("backend-1", "localhost:9001", 5)
	pool.Add(b1)

	wlc := NewWeightedLeastConnections(pool)

	// Should always return the same backend
	for i := 0; i < 10; i++ {
		b := wlc.Select()
		if b == nil || b.Name() != "backend-1" {
			t.Errorf("Expected backend-1, got %v", b)
		}
	}
}

func TestWeightedLeastConnectionsNoBackends(t *testing.T) {
	pool := backend.NewPool()
	wlc := NewWeightedLeastConnections(pool)

	b := wlc.Select()
	if b != nil {
		t.Errorf("Expected nil for empty pool, got %v", b)
	}
}

func TestWeightedLeastConnectionsZeroWeight(t *testing.T) {
	pool := backend.NewPool()

	b1 := backend.NewBackend("backend-1", "localhost:9001", 0)
	b2 := backend.NewBackend("backend-2", "localhost:9002", 2)

	pool.Add(b1)
	pool.Add(b2)

	wlc := NewWeightedLeastConnections(pool)

	// Zero weight should be treated as 1
	b := wlc.Select()
	if b == nil {
		t.Fatal("Expected a backend, got nil")
	}
}

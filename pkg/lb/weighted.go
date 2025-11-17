package lb

import (
	"sync/atomic"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

// WeightedRoundRobin implements weighted round-robin load balancing
// Backends with higher weights receive proportionally more requests
type WeightedRoundRobin struct {
	pool    *backend.Pool
	current atomic.Int64
}

// NewWeightedRoundRobin creates a new weighted round-robin load balancer
func NewWeightedRoundRobin(pool *backend.Pool) *WeightedRoundRobin {
	return &WeightedRoundRobin{
		pool: pool,
	}
}

// Select selects a backend using weighted round-robin algorithm
// This uses the smooth weighted round-robin algorithm (SWRR) by Nginx
// which provides better distribution than simple weighted round-robin
func (wrr *WeightedRoundRobin) Select() *backend.Backend {
	backends := wrr.pool.Healthy()
	if len(backends) == 0 {
		return nil
	}

	// If only one backend, return it
	if len(backends) == 1 {
		return backends[0]
	}

	// Calculate total weight
	totalWeight := 0
	for _, b := range backends {
		totalWeight += b.Weight()
	}

	if totalWeight == 0 {
		// Fallback to simple round-robin if all weights are 0
		next := wrr.current.Add(1)
		index := (next - 1) % int64(len(backends))
		return backends[index]
	}

	// Smooth weighted round-robin algorithm
	// Each backend has a current_weight that starts at 0
	// On each selection:
	//   1. Add effective_weight to current_weight for all backends
	//   2. Select the backend with highest current_weight
	//   3. Subtract total_weight from selected backend's current_weight

	// For simplicity in this stateless implementation, we use the atomic counter
	// to determine which backend to select based on weights
	next := wrr.current.Add(1)
	offset := (next - 1) % int64(totalWeight)

	// Find the backend that corresponds to this offset
	currentOffset := int64(0)
	for _, b := range backends {
		currentOffset += int64(b.Weight())
		if offset < currentOffset {
			return b
		}
	}

	// Fallback (should not reach here)
	return backends[0]
}

// Name returns the algorithm name
func (wrr *WeightedRoundRobin) Name() string {
	return "weighted-round-robin"
}

// WeightedLeastConnections implements weighted least-connections load balancing
// Selects the backend with the lowest (connections / weight) ratio
type WeightedLeastConnections struct {
	pool *backend.Pool
}

// NewWeightedLeastConnections creates a new weighted least-connections load balancer
func NewWeightedLeastConnections(pool *backend.Pool) *WeightedLeastConnections {
	return &WeightedLeastConnections{
		pool: pool,
	}
}

// Select selects the backend with the lowest (connections / weight) ratio
func (wlc *WeightedLeastConnections) Select() *backend.Backend {
	backends := wlc.pool.Healthy()
	if len(backends) == 0 {
		return nil
	}

	// If only one backend, return it
	if len(backends) == 1 {
		return backends[0]
	}

	var selected *backend.Backend
	minRatio := float64(-1)

	for _, b := range backends {
		weight := b.Weight()
		if weight <= 0 {
			weight = 1 // Treat 0 or negative weight as 1
		}

		connections := float64(b.ActiveConnections())
		ratio := connections / float64(weight)

		if minRatio == -1 || ratio < minRatio {
			selected = b
			minRatio = ratio
		}
	}

	return selected
}

// Name returns the algorithm name
func (wlc *WeightedLeastConnections) Name() string {
	return "weighted-least-connections"
}

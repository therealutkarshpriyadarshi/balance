package lb

import (
	"sync/atomic"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

// RoundRobin implements round-robin load balancing
type RoundRobin struct {
	pool    *backend.Pool
	current atomic.Uint64
}

// NewRoundRobin creates a new round-robin load balancer
func NewRoundRobin(pool *backend.Pool) *RoundRobin {
	return &RoundRobin{
		pool: pool,
	}
}

// Select selects the next backend using round-robin
func (rr *RoundRobin) Select() *backend.Backend {
	backends := rr.pool.Healthy()
	if len(backends) == 0 {
		return nil
	}

	// Atomically increment and get the next index
	next := rr.current.Add(1)
	index := (next - 1) % uint64(len(backends))

	return backends[index]
}

// Name returns the algorithm name
func (rr *RoundRobin) Name() string {
	return "round-robin"
}

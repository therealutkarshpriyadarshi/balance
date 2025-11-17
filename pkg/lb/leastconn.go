package lb

import (
	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

// LeastConnections implements least-connections load balancing
type LeastConnections struct {
	pool *backend.Pool
}

// NewLeastConnections creates a new least-connections load balancer
func NewLeastConnections(pool *backend.Pool) *LeastConnections {
	return &LeastConnections{
		pool: pool,
	}
}

// Select selects the backend with the least active connections
func (lc *LeastConnections) Select() *backend.Backend {
	backends := lc.pool.Healthy()
	if len(backends) == 0 {
		return nil
	}

	// Find backend with least connections
	var selected *backend.Backend
	minConnections := int64(-1)

	for _, b := range backends {
		connections := b.ActiveConnections()
		if minConnections == -1 || connections < minConnections {
			selected = b
			minConnections = connections
		}
	}

	return selected
}

// Name returns the algorithm name
func (lc *LeastConnections) Name() string {
	return "least-connections"
}

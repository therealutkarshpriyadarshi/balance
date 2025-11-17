package backend

import (
	"sync"
	"sync/atomic"
)

// Backend represents a backend server
type Backend struct {
	name    string
	address string
	weight  int

	// Connection tracking
	activeConnections atomic.Int64

	// Health status
	healthy atomic.Bool

	mu sync.RWMutex
}

// NewBackend creates a new backend
func NewBackend(name, address string, weight int) *Backend {
	b := &Backend{
		name:    name,
		address: address,
		weight:  weight,
	}
	b.healthy.Store(true) // Start as healthy
	return b
}

// Name returns the backend name
func (b *Backend) Name() string {
	return b.name
}

// Address returns the backend address
func (b *Backend) Address() string {
	return b.address
}

// Weight returns the backend weight
func (b *Backend) Weight() int {
	return b.weight
}

// IsHealthy returns true if the backend is healthy
func (b *Backend) IsHealthy() bool {
	return b.healthy.Load()
}

// MarkHealthy marks the backend as healthy
func (b *Backend) MarkHealthy() {
	b.healthy.Store(true)
}

// MarkUnhealthy marks the backend as unhealthy
func (b *Backend) MarkUnhealthy() {
	b.healthy.Store(false)
}

// ActiveConnections returns the number of active connections
func (b *Backend) ActiveConnections() int64 {
	return b.activeConnections.Load()
}

// IncrementConnections increments the active connection count
func (b *Backend) IncrementConnections() {
	b.activeConnections.Add(1)
}

// DecrementConnections decrements the active connection count
func (b *Backend) DecrementConnections() {
	b.activeConnections.Add(-1)
}

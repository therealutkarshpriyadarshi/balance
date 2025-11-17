package backend

import (
	"sync"
)

// Pool manages a collection of backends
type Pool struct {
	backends []*Backend
	mu       sync.RWMutex
}

// NewPool creates a new backend pool
func NewPool() *Pool {
	return &Pool{
		backends: make([]*Backend, 0),
	}
}

// Add adds a backend to the pool
func (p *Pool) Add(backend *Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backends = append(p.backends, backend)
}

// Remove removes a backend from the pool
func (p *Pool) Remove(name string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, b := range p.backends {
		if b.Name() == name {
			p.backends = append(p.backends[:i], p.backends[i+1:]...)
			return true
		}
	}
	return false
}

// Get returns a backend by name
func (p *Pool) Get(name string) *Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, b := range p.backends {
		if b.Name() == name {
			return b
		}
	}
	return nil
}

// GetByName is an alias for Get (for clarity)
func (p *Pool) GetByName(name string) *Backend {
	return p.Get(name)
}

// All returns all backends
func (p *Pool) All() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]*Backend, len(p.backends))
	copy(result, p.backends)
	return result
}

// Healthy returns all healthy backends
func (p *Pool) Healthy() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*Backend, 0, len(p.backends))
	for _, b := range p.backends {
		if b.IsHealthy() {
			result = append(result, b)
		}
	}
	return result
}

// Size returns the total number of backends
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.backends)
}

// HealthySize returns the number of healthy backends
func (p *Pool) HealthySize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := 0
	for _, b := range p.backends {
		if b.IsHealthy() {
			count++
		}
	}
	return count
}

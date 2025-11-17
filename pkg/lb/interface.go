package lb

import "github.com/therealutkarshpriyadarshi/balance/pkg/backend"

// LoadBalancer defines the interface for load balancing algorithms
type LoadBalancer interface {
	// Select selects a backend using the load balancing algorithm
	// Returns nil if no backend is available
	Select() *backend.Backend

	// Name returns the name of the load balancing algorithm
	Name() string
}

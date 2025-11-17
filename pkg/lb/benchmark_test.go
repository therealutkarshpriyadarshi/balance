package lb

import (
	"fmt"
	"sync"
	"testing"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

// createTestPool creates a pool with test backends
func createTestPool(numBackends int) *backend.Pool {
	pool := backend.NewPool()
	for i := 0; i < numBackends; i++ {
		b := backend.NewBackend(
			fmt.Sprintf("backend-%d", i),
			fmt.Sprintf("localhost:%d", 9000+i),
			i+1, // Progressive weights: 1, 2, 3, ...
		)
		pool.Add(b)
	}
	return pool
}

// BenchmarkRoundRobin benchmarks round-robin algorithm
func BenchmarkRoundRobin(b *testing.B) {
	pool := createTestPool(10)
	lb := NewRoundRobin(pool)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.Select()
	}
}

// BenchmarkRoundRobinParallel benchmarks round-robin under concurrent load
func BenchmarkRoundRobinParallel(b *testing.B) {
	pool := createTestPool(10)
	lb := NewRoundRobin(pool)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lb.Select()
		}
	})
}

// BenchmarkLeastConnections benchmarks least-connections algorithm
func BenchmarkLeastConnections(b *testing.B) {
	pool := createTestPool(10)
	lb := NewLeastConnections(pool)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.Select()
	}
}

// BenchmarkLeastConnectionsParallel benchmarks least-connections under concurrent load
func BenchmarkLeastConnectionsParallel(b *testing.B) {
	pool := createTestPool(10)
	lb := NewLeastConnections(pool)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lb.Select()
		}
	})
}

// BenchmarkWeightedRoundRobin benchmarks weighted round-robin algorithm
func BenchmarkWeightedRoundRobin(b *testing.B) {
	pool := createTestPool(10)
	lb := NewWeightedRoundRobin(pool)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.Select()
	}
}

// BenchmarkWeightedRoundRobinParallel benchmarks weighted round-robin under concurrent load
func BenchmarkWeightedRoundRobinParallel(b *testing.B) {
	pool := createTestPool(10)
	lb := NewWeightedRoundRobin(pool)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lb.Select()
		}
	})
}

// BenchmarkWeightedLeastConnections benchmarks weighted least-connections algorithm
func BenchmarkWeightedLeastConnections(b *testing.B) {
	pool := createTestPool(10)
	lb := NewWeightedLeastConnections(pool)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.Select()
	}
}

// BenchmarkWeightedLeastConnectionsParallel benchmarks weighted least-connections under concurrent load
func BenchmarkWeightedLeastConnectionsParallel(b *testing.B) {
	pool := createTestPool(10)
	lb := NewWeightedLeastConnections(pool)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lb.Select()
		}
	})
}

// BenchmarkConsistentHash benchmarks consistent hashing algorithm
func BenchmarkConsistentHash(b *testing.B) {
	pool := createTestPool(10)
	lb := NewConsistentHash(pool, DefaultVirtualNodes, "source-ip")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("192.168.1.%d", i%255)
		lb.SelectWithKey(key)
	}
}

// BenchmarkConsistentHashParallel benchmarks consistent hashing under concurrent load
func BenchmarkConsistentHashParallel(b *testing.B) {
	pool := createTestPool(10)
	lb := NewConsistentHash(pool, DefaultVirtualNodes, "source-ip")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("192.168.1.%d", i%255)
			lb.SelectWithKey(key)
			i++
		}
	})
}

// BenchmarkBoundedConsistentHash benchmarks bounded load consistent hashing
func BenchmarkBoundedConsistentHash(b *testing.B) {
	pool := createTestPool(10)
	lb := NewBoundedLoadConsistentHash(pool, DefaultVirtualNodes, "source-ip", 1.25)

	// Simulate some load
	backends := pool.Healthy()
	for _, backend := range backends {
		backend.IncrementConnections()
		backend.IncrementConnections()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("192.168.1.%d", i%255)
		lb.SelectWithKey(key)
	}
}

// BenchmarkBoundedConsistentHashParallel benchmarks bounded consistent hashing under concurrent load
func BenchmarkBoundedConsistentHashParallel(b *testing.B) {
	pool := createTestPool(10)
	lb := NewBoundedLoadConsistentHash(pool, DefaultVirtualNodes, "source-ip", 1.25)

	// Simulate some load
	backends := pool.Healthy()
	for _, backend := range backends {
		backend.IncrementConnections()
		backend.IncrementConnections()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("192.168.1.%d", i%255)
			lb.SelectWithKey(key)
			i++
		}
	})
}

// BenchmarkSessionAffinity benchmarks session affinity wrapper
func BenchmarkSessionAffinity(b *testing.B) {
	pool := createTestPool(10)
	baseBalancer := NewRoundRobin(pool)
	lb := NewSessionAffinity(baseBalancer, 0)
	defer lb.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientIP := fmt.Sprintf("192.168.1.%d", i%100)
		lb.SelectWithClientIP(clientIP)
	}
}

// BenchmarkSessionAffinityParallel benchmarks session affinity under concurrent load
func BenchmarkSessionAffinityParallel(b *testing.B) {
	pool := createTestPool(10)
	baseBalancer := NewRoundRobin(pool)
	lb := NewSessionAffinity(baseBalancer, 0)
	defer lb.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			clientIP := fmt.Sprintf("192.168.1.%d", i%100)
			lb.SelectWithClientIP(clientIP)
			i++
		}
	})
}

// BenchmarkAlgorithmComparison benchmarks all algorithms side-by-side
func BenchmarkAlgorithmComparison(b *testing.B) {
	algorithms := []struct {
		name string
		lb   LoadBalancer
	}{
		{"RoundRobin", NewRoundRobin(createTestPool(10))},
		{"LeastConnections", NewLeastConnections(createTestPool(10))},
		{"WeightedRoundRobin", NewWeightedRoundRobin(createTestPool(10))},
		{"WeightedLeastConnections", NewWeightedLeastConnections(createTestPool(10))},
		{"ConsistentHash", NewConsistentHash(createTestPool(10), DefaultVirtualNodes, "source-ip")},
		{"BoundedConsistentHash", NewBoundedLoadConsistentHash(createTestPool(10), DefaultVirtualNodes, "source-ip", 1.25)},
	}

	for _, alg := range algorithms {
		b.Run(alg.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				alg.lb.Select()
			}
		})
	}
}

// BenchmarkDistribution tests the distribution quality of each algorithm
func BenchmarkDistribution(b *testing.B) {
	numBackends := 5
	pool := createTestPool(numBackends)

	algorithms := []struct {
		name string
		lb   LoadBalancer
	}{
		{"RoundRobin", NewRoundRobin(pool)},
		{"LeastConnections", NewLeastConnections(pool)},
		{"WeightedRoundRobin", NewWeightedRoundRobin(pool)},
		{"WeightedLeastConnections", NewWeightedLeastConnections(pool)},
	}

	for _, alg := range algorithms {
		b.Run(alg.name, func(b *testing.B) {
			distribution := make(map[string]int)
			var mu sync.Mutex

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					backend := alg.lb.Select()
					if backend != nil {
						mu.Lock()
						distribution[backend.Name()]++
						mu.Unlock()
					}
				}
			})

			// Report distribution in benchmark results
			b.StopTimer()
			b.Logf("Distribution for %s:", alg.name)
			for name, count := range distribution {
				b.Logf("  %s: %d (%.2f%%)", name, count, float64(count)/float64(b.N)*100)
			}
		})
	}
}

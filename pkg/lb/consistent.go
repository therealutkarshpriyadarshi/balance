package lb

import (
	"fmt"
	"hash/fnv"
	"sort"
	"sync"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

const (
	// DefaultVirtualNodes is the default number of virtual nodes per backend
	DefaultVirtualNodes = 150
)

// ConsistentHash implements consistent hashing load balancing
// Uses a hash ring with virtual nodes for better distribution
type ConsistentHash struct {
	pool          *backend.Pool
	virtualNodes  int
	ring          []uint32
	ringMap       map[uint32]*backend.Backend
	mu            sync.RWMutex
	lastKnownSize int
	hashKey       string // "source-ip" or custom key extractor
}

// NewConsistentHash creates a new consistent hash load balancer
func NewConsistentHash(pool *backend.Pool, virtualNodes int, hashKey string) *ConsistentHash {
	if virtualNodes <= 0 {
		virtualNodes = DefaultVirtualNodes
	}
	if hashKey == "" {
		hashKey = "source-ip"
	}

	ch := &ConsistentHash{
		pool:         pool,
		virtualNodes: virtualNodes,
		ringMap:      make(map[uint32]*backend.Backend),
		hashKey:      hashKey,
	}

	// Initialize the ring
	ch.rebuildRing()

	return ch
}

// rebuildRing rebuilds the hash ring with current backends
func (ch *ConsistentHash) rebuildRing() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	backends := ch.pool.Healthy()
	currentSize := len(backends)

	// Check if we need to rebuild
	if currentSize == ch.lastKnownSize && len(ch.ring) > 0 {
		return
	}

	// Clear existing ring
	ch.ring = make([]uint32, 0, currentSize*ch.virtualNodes)
	ch.ringMap = make(map[uint32]*backend.Backend)

	// Add virtual nodes for each backend
	for _, b := range backends {
		weight := b.Weight()
		if weight <= 0 {
			weight = 1
		}

		// Number of virtual nodes proportional to weight
		numVirtualNodes := ch.virtualNodes * weight

		for i := 0; i < numVirtualNodes; i++ {
			// Create unique key for this virtual node
			key := fmt.Sprintf("%s-%d", b.Address(), i)
			hash := ch.hash(key)

			ch.ring = append(ch.ring, hash)
			ch.ringMap[hash] = b
		}
	}

	// Sort the ring
	sort.Slice(ch.ring, func(i, j int) bool {
		return ch.ring[i] < ch.ring[j]
	})

	ch.lastKnownSize = currentSize
}

// Select selects a backend using consistent hashing
// The key parameter should be extracted from the connection (e.g., source IP)
func (ch *ConsistentHash) Select() *backend.Backend {
	return ch.SelectWithKey("")
}

// SelectWithKey selects a backend using consistent hashing with a custom key
func (ch *ConsistentHash) SelectWithKey(key string) *backend.Backend {
	// Rebuild ring if backends changed
	ch.rebuildRing()

	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.ring) == 0 {
		return nil
	}

	// Hash the key
	hash := ch.hash(key)

	// Binary search to find the first node >= hash
	idx := sort.Search(len(ch.ring), func(i int) bool {
		return ch.ring[i] >= hash
	})

	// Wrap around if needed
	if idx >= len(ch.ring) {
		idx = 0
	}

	return ch.ringMap[ch.ring[idx]]
}

// hash returns the hash of a string using FNV-1a
func (ch *ConsistentHash) hash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// Name returns the algorithm name
func (ch *ConsistentHash) Name() string {
	return "consistent-hash"
}

// BoundedLoadConsistentHash implements consistent hashing with bounded load
// Prevents any single backend from being overloaded by limiting connections
// based on average load across all backends
type BoundedLoadConsistentHash struct {
	*ConsistentHash
	loadFactor float64 // Maximum load as multiple of average (e.g., 1.25 = 125%)
}

// NewBoundedLoadConsistentHash creates a new bounded load consistent hash load balancer
func NewBoundedLoadConsistentHash(pool *backend.Pool, virtualNodes int, hashKey string, loadFactor float64) *BoundedLoadConsistentHash {
	if loadFactor <= 0 {
		loadFactor = 1.25 // Default to 125% of average load
	}

	return &BoundedLoadConsistentHash{
		ConsistentHash: NewConsistentHash(pool, virtualNodes, hashKey),
		loadFactor:     loadFactor,
	}
}

// SelectWithKey selects a backend using bounded load consistent hashing
func (blch *BoundedLoadConsistentHash) SelectWithKey(key string) *backend.Backend {
	// Rebuild ring if backends changed
	blch.rebuildRing()

	blch.mu.RLock()
	defer blch.mu.RUnlock()

	if len(blch.ring) == 0 {
		return nil
	}

	backends := blch.pool.Healthy()
	if len(backends) == 0 {
		return nil
	}

	// Calculate average load and max allowed load
	totalConnections := int64(0)
	for _, b := range backends {
		totalConnections += b.ActiveConnections()
	}
	avgLoad := float64(totalConnections) / float64(len(backends))
	maxLoad := avgLoad * blch.loadFactor

	// Hash the key
	hash := blch.hash(key)

	// Find the first node >= hash
	idx := sort.Search(len(blch.ring), func(i int) bool {
		return blch.ring[i] >= hash
	})

	// Try to find a backend that's not overloaded
	// Walk the ring up to N times (where N is number of backends)
	for i := 0; i < len(backends); i++ {
		if idx >= len(blch.ring) {
			idx = 0
		}

		backend := blch.ringMap[blch.ring[idx]]
		if backend != nil {
			// Check if this backend is within load bounds
			if float64(backend.ActiveConnections()) <= maxLoad {
				return backend
			}
		}

		// Move to next position in ring
		idx++
	}

	// If all backends are overloaded, fall back to least loaded
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

// Select selects a backend using bounded load consistent hashing
func (blch *BoundedLoadConsistentHash) Select() *backend.Backend {
	return blch.SelectWithKey("")
}

// Name returns the algorithm name
func (blch *BoundedLoadConsistentHash) Name() string {
	return "bounded-consistent-hash"
}

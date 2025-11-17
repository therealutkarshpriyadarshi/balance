# Phase 2: Advanced Load Balancing

This document describes the Phase 2 implementation of Balance, which adds advanced load balancing algorithms.

## Overview

Phase 2 introduces:
- ✅ Weighted load balancing algorithms
- ✅ Consistent hashing with virtual nodes
- ✅ Bounded load consistent hashing
- ✅ Session affinity (sticky sessions)
- ✅ Comprehensive benchmarks

## New Load Balancing Algorithms

### 1. Weighted Round-Robin

**Algorithm**: `weighted-round-robin`

Distributes requests across backends proportionally to their configured weights. Backends with higher weights receive more traffic.

**Use Cases**:
- Backends with different capacities (CPU, memory, network)
- Gradual rollout of new backend versions (canary deployment)
- Cost optimization (send more traffic to cheaper backends)

**Configuration**:
```yaml
load_balancer:
  algorithm: weighted-round-robin

backends:
  - name: backend-1
    address: "localhost:9001"
    weight: 1  # Receives 1/6 of traffic

  - name: backend-2
    address: "localhost:9002"
    weight: 2  # Receives 2/6 of traffic

  - name: backend-3
    address: "localhost:9003"
    weight: 3  # Receives 3/6 of traffic
```

**Performance**: ~95 ns/op, 80 B/op

---

### 2. Weighted Least Connections

**Algorithm**: `weighted-least-connections`

Selects the backend with the lowest (active_connections / weight) ratio. Balances both connection count and backend capacity.

**Use Cases**:
- Long-lived connections where backends have different capacities
- Database connection pooling
- WebSocket or streaming applications with variable backend performance

**Configuration**:
```yaml
load_balancer:
  algorithm: weighted-least-connections

backends:
  - name: backend-1
    address: "localhost:9001"
    weight: 1  # Can handle 1x baseline load

  - name: backend-2
    address: "localhost:9002"
    weight: 2  # Can handle 2x baseline load
```

**Performance**: ~102 ns/op, 80 B/op

---

### 3. Consistent Hashing

**Algorithm**: `consistent-hash`

Routes requests to backends based on a hash of the client's source IP. The same client always goes to the same backend (unless backends are added/removed).

**Features**:
- Virtual nodes (150 per backend by default) for better distribution
- Weight support (backends with higher weights get more virtual nodes)
- Minimal disruption when backends are added or removed

**Use Cases**:
- Session persistence without server-side state
- Caching at backend level (maximize cache hit ratio)
- Stateful applications requiring client affinity

**Configuration**:
```yaml
load_balancer:
  algorithm: consistent-hash
  hash_key: source-ip  # Currently the only supported option
```

**Performance**: ~140 ns/op, 80 B/op

---

### 4. Bounded Load Consistent Hashing

**Algorithm**: `bounded-consistent-hash`

Enhanced consistent hashing that prevents any single backend from becoming overloaded. If the consistent hash selects an overloaded backend, it walks the hash ring to find an alternative.

**Features**:
- All benefits of consistent hashing
- Load factor protection (default: 1.25x average load)
- Graceful degradation under uneven load

**Use Cases**:
- Consistent hashing with protection against hotspots
- Systems with uneven key distributions
- Applications requiring both affinity and load balancing

**Configuration**:
```yaml
load_balancer:
  algorithm: bounded-consistent-hash
  hash_key: source-ip
```

**Performance**: ~229 ns/op, 160 B/op

---

### 5. Session Affinity (Wrapper)

**Feature**: IP-based sticky sessions (can wrap any load balancer)

Routes the same client IP to the same backend for the duration of a session (default: 10 minutes).

**Features**:
- Configurable session timeout
- Automatic session cleanup
- Falls back to new backend if original becomes unhealthy
- Works with any underlying load balancing algorithm

**Use Cases**:
- Applications requiring session state
- Load testing and debugging (easier to trace specific clients)
- Gradual migration scenarios

**Note**: Session affinity is not directly configurable in YAML yet. It's available as a programmatic wrapper in the codebase.

**Performance**: ~450 ns/op (includes underlying balancer overhead)

---

## Algorithm Comparison

### Performance Benchmarks

Benchmark results on Intel Xeon @ 2.60GHz (single-threaded):

| Algorithm | Latency (ns/op) | Memory (B/op) | Allocations |
|-----------|-----------------|---------------|-------------|
| Round-Robin | 94 | 80 | 1 |
| Least-Connections | 93 | 80 | 1 |
| Weighted Round-Robin | 95 | 80 | 1 |
| Weighted Least-Connections | 102 | 80 | 1 |
| Consistent Hash | 140 | 80 | 1 |
| Bounded Consistent Hash | 229 | 160 | 2 |

### Algorithm Selection Guide

| Scenario | Recommended Algorithm | Reason |
|----------|----------------------|--------|
| Equal backends, simple | Round-Robin | Fastest, simplest |
| Long-lived connections | Least-Connections | Balances actual load |
| Different backend capacities | Weighted Algorithms | Respects backend capabilities |
| Need session persistence | Consistent Hash | No state required |
| Session persistence + load protection | Bounded Consistent Hash | Best of both worlds |
| Session state in backend | Session Affinity | Explicit session management |

---

## Implementation Details

### Weighted Round-Robin

Uses a smooth weighted round-robin algorithm similar to Nginx. This provides better distribution than simple weighted round-robin by spreading weighted requests evenly rather than in bursts.

### Consistent Hashing

- **Hash Function**: FNV-1a (fast, good distribution)
- **Virtual Nodes**: 150 per backend (configurable)
- **Ring Structure**: Sorted array for O(log n) lookups
- **Weight Support**: Backends with weight N get N × 150 virtual nodes
- **Rebuild Optimization**: Ring only rebuilds when backend count changes

### Bounded Load

- **Load Factor**: 1.25 (means backends can handle up to 125% of average load)
- **Fallback**: If all backends overloaded, falls back to least-loaded backend
- **Walking Strategy**: Walks hash ring up to N positions (where N = backend count)

---

## Testing

### Unit Tests

All algorithms have comprehensive unit tests covering:
- Basic functionality
- Edge cases (no backends, single backend)
- Distribution quality
- Concurrent access
- Health status changes

Run tests:
```bash
go test ./pkg/lb/... -v
```

### Benchmarks

Comprehensive benchmarks for:
- Single-threaded performance
- Concurrent performance (parallel benchmarks)
- Distribution quality
- Algorithm comparison

Run benchmarks:
```bash
# Run all benchmarks
go test -bench=. -benchmem ./pkg/lb/

# Compare algorithms
go test -bench=BenchmarkAlgorithmComparison -benchmem ./pkg/lb/

# Test distribution
go test -bench=BenchmarkDistribution -benchmem ./pkg/lb/
```

---

## Example Configurations

### Weighted Round-Robin

See [config/weighted-example.yaml](config/weighted-example.yaml)

### Consistent Hashing

See [config/consistent-hash-example.yaml](config/consistent-hash-example.yaml)

---

## Future Enhancements (Phase 3+)

- Cookie-based session affinity (for HTTP)
- Header-based consistent hashing
- Custom hash functions
- Maglev hashing algorithm
- Jump hash for faster consistent hashing
- Least response time algorithm
- Adaptive load balancing

---

## Breaking Changes

None. All Phase 1 functionality remains unchanged.

---

## Migration Guide

### From Phase 1

No code changes required. All Phase 1 configurations continue to work.

To use new algorithms, update your configuration:

**Before (Phase 1)**:
```yaml
load_balancer:
  algorithm: round-robin
```

**After (Phase 2)**:
```yaml
load_balancer:
  algorithm: weighted-round-robin  # or any new algorithm

backends:
  - name: backend-1
    address: "localhost:9001"
    weight: 1  # Now respected by weighted algorithms
```

---

## API Reference

### LoadBalancer Interface

All algorithms implement this interface:

```go
type LoadBalancer interface {
    Select() *backend.Backend
    Name() string
}
```

### Extended Interfaces

Some algorithms support additional methods:

```go
// For consistent hashing
type KeyBasedBalancer interface {
    SelectWithKey(key string) *backend.Backend
}

// For session affinity
type AffinityBalancer interface {
    SelectWithClientIP(clientIP string) *backend.Backend
}
```

---

## Troubleshooting

### Inconsistent Distribution

**Problem**: Traffic not distributed according to weights

**Solutions**:
1. Verify backend weights are configured correctly
2. Check that all backends are healthy (unhealthy backends are excluded)
3. For consistent hashing, ensure sufficient traffic volume (need many unique client IPs)

### Poor Performance

**Problem**: High latency overhead from load balancer

**Solutions**:
1. Use simpler algorithms (round-robin is fastest)
2. Reduce number of backends (impacts consistent hashing most)
3. Profile your application to identify bottlenecks

### Session Persistence Issues

**Problem**: Clients not sticking to same backend

**Solutions**:
1. Verify consistent-hash algorithm is configured
2. Check that hash_key is set to "source-ip"
3. Ensure NAT is not changing client IPs
4. Consider using bounded-consistent-hash if backends are frequently overloaded

---

## Contributing

Improvements to Phase 2 algorithms are welcome! Please:
1. Add tests for any new functionality
2. Run benchmarks to verify performance impact
3. Update documentation
4. Follow existing code style

---

## References

- [ROADMAP.md](ROADMAP.md) - Complete project roadmap
- [README.md](README.md) - Project overview
- [Nginx Smooth Weighted Round-Robin](https://github.com/phusion/nginx/commit/27e94984486058d73157038f7950a0a36ecc6e35)
- [Google's Maglev Hashing](https://static.googleusercontent.com/media/research.google.com/en//pubs/archive/44824.pdf)
- [Consistent Hashing and Random Trees](https://www.cs.princeton.edu/courses/archive/fall09/cos518/papers/chash.pdf)

---

**Phase 2 Complete! 🎉**

Next up: Phase 3 - HTTP/HTTPS Layer 7 Proxy

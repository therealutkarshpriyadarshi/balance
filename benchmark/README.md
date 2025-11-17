# Balance Benchmark Suite

This directory contains comprehensive benchmarks and load testing utilities for the Balance proxy.

## Benchmark Types

### 1. Unit Benchmarks
Run unit benchmarks for individual packages:

```bash
# Run all benchmarks
go test -bench=. -benchmem ./...

# Run specific package benchmarks
go test -bench=. -benchmem ./pkg/pool/
go test -bench=. -benchmem ./pkg/lb/
go test -bench=. -benchmem ./pkg/proxy/

# Run with CPU profiling
go test -bench=. -benchmem -cpuprofile=cpu.prof ./pkg/pool/

# Run with memory profiling
go test -bench=. -benchmem -memprofile=mem.prof ./pkg/pool/
```

### 2. Load Testing

#### Using wrk (HTTP)
```bash
# Install wrk
# Ubuntu: apt-get install wrk
# Mac: brew install wrk

# Basic load test
wrk -t4 -c100 -d30s http://localhost:8080

# With custom script
wrk -t8 -c200 -d60s -s benchmark/wrk-script.lua http://localhost:8080
```

#### Using hey (HTTP)
```bash
# Install hey
go install github.com/rakyll/hey@latest

# Basic load test
hey -z 30s -c 100 http://localhost:8080

# With rate limiting
hey -z 30s -c 100 -q 1000 http://localhost:8080
```

#### Using k6 (HTTP)
```bash
# Install k6
# See: https://k6.io/docs/getting-started/installation/

# Run load test
k6 run benchmark/k6-script.js
```

### 3. TCP Benchmarks

```bash
# Run TCP benchmark
go run benchmark/tcp_benchmark.go
```

### 4. Latency Analysis

```bash
# Run latency benchmark
go run benchmark/latency_benchmark.go

# Output includes p50, p95, p99, p999 latencies
```

### 5. Connection Limit Testing

```bash
# Test concurrent connection limits
go run benchmark/connection_test.go -connections=10000
```

## Performance Targets

| Metric | Target | Command |
|--------|--------|---------|
| Requests/second | 100,000+ | `wrk -t8 -c100 -d30s` |
| p99 Latency | <10ms | `go run benchmark/latency_benchmark.go` |
| Concurrent Connections | 50,000+ | `go run benchmark/connection_test.go` |
| Memory Footprint | <100MB | Monitor during tests |

## Profiling

### CPU Profiling
```bash
# Start proxy with CPU profiling
go run cmd/balance/main.go -config=config/example.yaml -cpuprofile=cpu.prof

# Analyze profile
go tool pprof cpu.prof
```

### Memory Profiling
```bash
# Start proxy with memory profiling
go run cmd/balance/main.go -config=config/example.yaml -memprofile=mem.prof

# Analyze profile
go tool pprof mem.prof
```

### Live Profiling
```bash
# Enable pprof HTTP server in your proxy
# Then access profiles at:
# http://localhost:6060/debug/pprof/

# CPU profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof

# Memory profile
curl http://localhost:6060/debug/pprof/heap > heap.prof

# Goroutine profile
curl http://localhost:6060/debug/pprof/goroutine > goroutine.prof
```

## Optimization Checklist

- [ ] Buffer pooling reduces allocations
- [ ] Goroutine pooling limits goroutine count
- [ ] Zero-copy transfer for TCP (Linux)
- [ ] String interning for HTTP headers
- [ ] Connection pooling for backends
- [ ] Lock-free data structures where possible
- [ ] Minimal GC pressure
- [ ] Optimized hot paths

## Comparing Results

```bash
# Run benchmark and save results
go test -bench=. -benchmem ./pkg/pool/ > old.txt

# Make changes...

# Run again and compare
go test -bench=. -benchmem ./pkg/pool/ > new.txt
benchcmp old.txt new.txt
```

## Continuous Benchmarking

Set up CI to track performance over time:

```yaml
# .github/workflows/benchmark.yml
name: Benchmarks
on: [push, pull_request]
jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
      - run: go test -bench=. -benchmem ./... > benchmark.txt
      - uses: benchmark-action/github-action-benchmark@v1
```

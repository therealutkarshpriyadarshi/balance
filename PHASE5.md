# Phase 5: Health Checking & Resilience - Implementation Complete ✅

## Overview

Phase 5 implements comprehensive health checking and fault tolerance features for the Balance load balancer. This phase adds production-grade health monitoring, circuit breakers, retry logic, and timeout management to ensure high availability and resilience.

**Status**: ✅ **Completed**
**Timeline**: Weeks 9-10
**Complexity**: High

## Features Implemented

### Week 9: Health Checking System ✅

#### 1. Backend State Machine
- **File**: `pkg/backend/state.go`
- State-based backend health management
- Transition tracking between healthy, unhealthy, and draining states
- Comprehensive metrics collection
- State change notifications

**Key Features**:
- Three states: Healthy, Unhealthy, Draining
- Configurable thresholds for state transitions
- Passive and active health metrics
- State change listeners for observability
- Request tracking and error rate calculation

**Code Example**:
```go
sm := backend.NewStateMachine(backend, healthyThreshold, unhealthyThreshold)

// Record health check results
sm.RecordSuccess()
sm.RecordFailure()

// Track requests for passive health checks
sm.RecordRequest(success, responseTime)

// Listen for state changes
sm.AddListener(func(b *backend.Backend, oldState, newState backend.State) {
    log.Printf("Backend %s: %s -> %s", b.Name(), oldState, newState)
})
```

**States**:
- **StateHealthy**: Backend is healthy and accepting traffic
- **StateUnhealthy**: Backend has failed health checks
- **StateDraining**: Backend is being gracefully removed

#### 2. Active Health Checks
- **File**: `pkg/health/active.go`
- TCP connection checks
- HTTP/HTTPS endpoint checks
- Concurrent health checking
- Configurable intervals and timeouts

**Key Features**:
- Multiple check types: TCP, HTTP, HTTPS
- Parallel health checks for multiple backends
- Context-aware with timeout support
- Configurable expected HTTP status codes
- Detailed check results with duration tracking

**Code Example**:
```go
checker := health.NewActiveChecker(health.ActiveCheckerConfig{
    CheckType: health.CheckTypeHTTP,
    Timeout:   3 * time.Second,
    HTTPPath:  "/health",
    ExpectedStatusCodes: []int{http.StatusOK, http.StatusNoContent},
})

// Check single backend
result := checker.Check(ctx, backend)

// Check multiple backends concurrently
results := checker.CheckMultiple(ctx, backends)
```

**Check Types**:
- **TCP**: Establishes TCP connection to verify backend is reachable
- **HTTP**: Sends GET request to specified path
- **HTTPS**: Sends HTTPS GET request with TLS verification

#### 3. Passive Health Checks
- **File**: `pkg/health/passive.go`
- Request failure tracking
- Consecutive failure detection
- Time-windowed failure tracking
- Automatic backend marking

**Key Features**:
- Track failures without explicit health checks
- Configurable error rate thresholds
- Time window for failure analysis
- Consecutive failure detection
- Minimal overhead on request path

**Code Example**:
```go
checker := health.NewPassiveChecker(health.PassiveCheckerConfig{
    ErrorRateThreshold:  0.5,  // 50% error rate
    MinRequests:         10,   // Minimum requests before checking
    ConsecutiveFailures: 5,    // Failures to mark unhealthy
    Window:              1 * time.Minute,
})

// Record request outcomes
checker.RecordSuccess(backend, responseTime)
shouldMarkUnhealthy := checker.RecordFailure(backend)
```

#### 4. Health Check Orchestration
- **File**: `pkg/health/checker.go`
- Coordinates active and passive health checks
- Integrates with backend state machines
- Periodic health check scheduling
- State change notifications

**Key Features**:
- Combined active and passive health checking
- Automatic state machine integration
- Concurrent health checking
- Dynamic backend addition/removal
- Health check statistics

**Code Example**:
```go
checker := health.NewChecker(pool, health.CheckerConfig{
    Interval:             10 * time.Second,
    Timeout:              3 * time.Second,
    HealthyThreshold:     2,
    UnhealthyThreshold:   3,
    ActiveCheckType:      health.CheckTypeHTTP,
    HTTPPath:             "/health",
    EnablePassiveChecks:  true,
    ErrorRateThreshold:   0.5,
    ConsecutiveFailures:  5,
})

checker.Start()
defer checker.Stop()

// Record requests for passive health checking
checker.RecordRequest(backend, success, responseTime)
```

### Week 10: Circuit Breaking & Fault Tolerance ✅

#### 5. Circuit Breaker
- **File**: `pkg/resilience/circuit.go`
- Three-state circuit breaker (Closed, Open, Half-Open)
- Automatic failure detection
- Configurable recovery attempts
- Per-backend or per-service protection

**Key Features**:
- Prevents cascading failures
- Automatic recovery with half-open testing
- Configurable failure thresholds
- Request rejection in open state
- Detailed metrics and state tracking

**States**:
- **Closed**: Normal operation, all requests allowed
- **Open**: Too many failures, all requests rejected
- **Half-Open**: Testing recovery, limited requests allowed

**Code Example**:
```go
cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
    Name:                  "backend-1",
    MaxFailures:           5,
    Timeout:               60 * time.Second,
    MaxConcurrentRequests: 1,
})

// Execute with circuit breaker protection
err := cb.Execute(func() error {
    return callBackend()
})

if errors.Is(err, resilience.ErrCircuitOpen) {
    // Circuit is open, use fallback
}

// Listen for state changes
cb.AddListener(func(name string, from, to resilience.CircuitState) {
    log.Printf("Circuit %s: %s -> %s", name, from, to)
})
```

**Metrics**:
- Total requests, successes, failures
- Rejected requests (when open)
- Consecutive failures
- State change timestamps

#### 6. Retry Logic
- **File**: `pkg/resilience/retry.go`
- Exponential backoff with jitter
- Configurable retry policies
- Context-aware retry logic
- Retry budget to prevent retry storms

**Key Features**:
- Exponential backoff with configurable multiplier
- Jitter to prevent thundering herd
- Retryable error detection
- Maximum delay capping
- OnRetry callbacks for monitoring

**Code Example**:
```go
policy := resilience.DefaultRetryPolicy()
policy.MaxAttempts = 3
policy.InitialDelay = 100 * time.Millisecond
policy.Multiplier = 2.0
policy.Jitter = 0.1

err := resilience.Retry(func() error {
    return callBackend()
}, policy)

// With context
err = resilience.RetryWithContext(ctx, func(ctx context.Context) error {
    return callBackendWithContext(ctx)
}, policy)
```

**Retry Budget**:
Prevents retry storms by limiting the ratio of retries to requests:

```go
budget := resilience.NewRetryBudget(
    1*time.Second,  // TTL
    10,             // Min retries per second
    0.2,            // Max 20% retry ratio
)

budget.RecordRequest()
if budget.CanRetry() {
    // Retry allowed
}
```

#### 7. Timeout Management
- **File**: `pkg/resilience/timeout.go`
- Request-level timeouts
- Connection timeouts
- Read/Write timeouts
- Deadline propagation

**Key Features**:
- Multiple timeout types
- Context-based timeout management
- Timeout metrics tracking
- Deadline propagation across contexts

**Code Example**:
```go
tm := resilience.NewTimeoutManager(resilience.TimeoutConfig{
    RequestTimeout: 30 * time.Second,
    ConnectTimeout: 5 * time.Second,
    ReadTimeout:    30 * time.Second,
    WriteTimeout:   30 * time.Second,
    IdleTimeout:    60 * time.Second,
})

// Execute with request timeout
err := tm.WithRequestTimeout(func(ctx context.Context) error {
    return callBackend(ctx)
})

// Create context with timeout
ctx, cancel := tm.CreateRequestContext(parentCtx)
defer cancel()
```

## Configuration

### Health Check Configuration

```yaml
health_check:
  enabled: true
  interval: 10s
  timeout: 3s
  healthy_threshold: 2
  unhealthy_threshold: 3
  type: http  # tcp, http, or https
  path: /health  # for HTTP checks

  # Passive health checks
  passive_checks:
    enabled: true
    error_rate_threshold: 0.5
    consecutive_failures: 5
    window: 1m
```

### Resilience Configuration

```yaml
resilience:
  # Circuit breaker configuration
  circuit_breaker:
    enabled: true
    max_failures: 5
    timeout: 60s
    max_concurrent_requests: 1

  # Retry configuration
  retry:
    enabled: true
    max_attempts: 3
    initial_delay: 100ms
    max_delay: 10s
    multiplier: 2.0
    jitter: 0.1
```

## Architecture

### Health Check Flow

```
┌─────────────────────────────────────────────────────────┐
│                  Health Check Orchestrator              │
│  ┌──────────────────────────────────────────────────┐   │
│  │         Active Health Checker (Periodic)         │   │
│  │  - TCP Connection Checks                         │   │
│  │  - HTTP/HTTPS Endpoint Checks                    │   │
│  │  - Concurrent Checking                           │   │
│  └──────────────────┬───────────────────────────────┘   │
│                     │                                    │
│                     ▼                                    │
│  ┌──────────────────────────────────────────────────┐   │
│  │           Backend State Machine                  │   │
│  │  States: Healthy → Unhealthy → Draining          │   │
│  └──────────────────┬───────────────────────────────┘   │
│                     │                                    │
│                     ▼                                    │
│  ┌──────────────────────────────────────────────────┐   │
│  │        Passive Health Checker (On Request)       │   │
│  │  - Failure Tracking                              │   │
│  │  - Error Rate Monitoring                         │   │
│  │  - Consecutive Failure Detection                 │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### Circuit Breaker State Machine

```
                    Failures >= Threshold
     ┌──────────────────────────────────────────┐
     │                                          │
     │                                          ▼
┌────────┐                                 ┌────────┐
│        │  Success after timeout          │        │
│ CLOSED │◄────────────────────────────────┤  OPEN  │
│        │                                 │        │
└────┬───┘                                 └────────┘
     │                                          ▲
     │                                          │
     │    ┌──────────┐                          │
     └───►│          │  Any Failure             │
          │ HALF-OPEN├──────────────────────────┘
          │          │
          └────┬─────┘
               │
               │ Enough Successes
               ▼
          (Back to CLOSED)
```

## Testing

### Run All Tests

```bash
# Backend state machine tests
go test ./pkg/backend -v -run TestStateMachine

# Health check tests
go test ./pkg/health -v

# Circuit breaker tests
go test ./pkg/resilience -v -run TestCircuitBreaker

# Retry tests
go test ./pkg/resilience -v -run TestRetry

# All Phase 5 tests
go test ./pkg/backend ./pkg/health ./pkg/resilience -v
```

### Test Coverage

```bash
# Generate coverage report
go test ./pkg/backend ./pkg/health ./pkg/resilience -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Usage Examples

### Example 1: Basic Health Checking

```go
package main

import (
    "github.com/therealutkarshpriyadarshi/balance/pkg/backend"
    "github.com/therealutkarshpriyadarshi/balance/pkg/health"
)

func main() {
    // Create backend pool
    pool := backend.NewPool()
    pool.Add(backend.NewBackend("srv1", "localhost:8001", 1))
    pool.Add(backend.NewBackend("srv2", "localhost:8002", 1))

    // Create health checker
    checker := health.NewChecker(pool, health.CheckerConfig{
        Interval:            10 * time.Second,
        Timeout:             3 * time.Second,
        HealthyThreshold:    2,
        UnhealthyThreshold:  3,
        ActiveCheckType:     health.CheckTypeHTTP,
        HTTPPath:            "/health",
    })

    // Start health checking
    checker.Start()
    defer checker.Stop()
}
```

### Example 2: Circuit Breaker with Retry

```go
package main

import (
    "github.com/therealutkarshpriyadarshi/balance/pkg/resilience"
)

func main() {
    // Create circuit breaker
    cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
        Name:        "api-service",
        MaxFailures: 5,
        Timeout:     60 * time.Second,
    })

    // Create retry policy
    policy := resilience.RetryPolicy{
        MaxAttempts:  3,
        InitialDelay: 100 * time.Millisecond,
        Multiplier:   2.0,
    }

    // Execute with both protections
    err := cb.Execute(func() error {
        return resilience.Retry(func() error {
            return callAPI()
        }, policy)
    })

    if err != nil {
        log.Printf("Request failed: %v", err)
    }
}
```

### Example 3: Complete Integration

```go
package main

import (
    "context"
    "github.com/therealutkarshpriyadarshi/balance/pkg/backend"
    "github.com/therealutkarshpriyadarshi/balance/pkg/health"
    "github.com/therealutkarshpriyadarshi/balance/pkg/resilience"
)

func main() {
    // Setup backends
    pool := backend.NewPool()
    b := backend.NewBackend("api-1", "localhost:8080", 1)
    pool.Add(b)

    // Setup health checking
    healthChecker := health.NewChecker(pool, health.CheckerConfig{
        Interval:            10 * time.Second,
        HealthyThreshold:    2,
        UnhealthyThreshold:  3,
        EnablePassiveChecks: true,
    })
    healthChecker.Start()
    defer healthChecker.Stop()

    // Setup circuit breaker
    cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
        Name:        "api-1",
        MaxFailures: 5,
        Timeout:     60 * time.Second,
    })

    // Setup timeout manager
    tm := resilience.NewTimeoutManager(resilience.DefaultTimeoutConfig())

    // Make request with all protections
    err := cb.Execute(func() error {
        return tm.WithRequestTimeout(func(ctx context.Context) error {
            start := time.Now()
            err := makeRequest(ctx, b)
            duration := time.Since(start)

            // Record for passive health checking
            healthChecker.RecordRequest(b, err == nil, duration)

            return err
        })
    })
}
```

## Performance Considerations

### Health Checks

- **Active Checks**: Run concurrently for all backends to minimize check time
- **Passive Checks**: Minimal overhead, only tracks metrics
- **State Machines**: Lock-free atomic operations for high throughput

### Circuit Breaker

- **Lock-free**: Uses atomic operations for state management
- **Minimal Overhead**: Fast-path checking for closed state
- **Concurrent Safe**: Thread-safe for multi-goroutine usage

### Retry Logic

- **Exponential Backoff**: Reduces load on failing services
- **Jitter**: Prevents thundering herd problem
- **Retry Budget**: Prevents retry storms

## Monitoring and Observability

### Health Check Metrics

```go
// Get state machine metrics
sm, _ := checker.GetStateMachine("backend-1")
metrics := sm.GetMetrics()

fmt.Printf("Consecutive Successes: %d\n", sm.GetConsecutiveSuccesses())
fmt.Printf("Consecutive Failures: %d\n", sm.GetConsecutiveFailures())
fmt.Printf("Error Rate: %.2f%%\n", sm.GetErrorRate()*100)
fmt.Printf("Avg Response Time: %s\n", sm.GetAverageResponseTime())
```

### Circuit Breaker Metrics

```go
metrics := cb.GetMetrics()
fmt.Printf("State: %s\n", metrics.State)
fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
fmt.Printf("Successes: %d\n", metrics.TotalSuccesses)
fmt.Printf("Failures: %d\n", metrics.TotalFailures)
fmt.Printf("Rejected: %d\n", metrics.TotalRejected)
```

### Health Check Statistics

```go
total, success, failed := checker.GetStats()
fmt.Printf("Total Checks: %d\n", total)
fmt.Printf("Success Rate: %.2f%%\n", float64(success)/float64(total)*100)
```

## Benefits

### 1. High Availability
- Automatic failure detection and recovery
- Prevents routing to unhealthy backends
- Graceful degradation under failures

### 2. Fault Tolerance
- Circuit breakers prevent cascading failures
- Retry logic handles transient failures
- Timeout management prevents hung requests

### 3. Operational Excellence
- Detailed health metrics
- State change notifications
- Configurable thresholds

### 4. Performance
- Minimal overhead on request path
- Concurrent health checking
- Lock-free state management

## What's Next

Phase 6 (Weeks 11-12) will add:
- Connection pooling for backend connections
- Advanced rate limiting per route
- Request/response transformation
- Comprehensive observability (Prometheus metrics, tracing)

## Files Added

### Core Implementation
- `pkg/backend/state.go` - Backend state machine
- `pkg/health/active.go` - Active health checks
- `pkg/health/passive.go` - Passive health checks
- `pkg/health/checker.go` - Health check orchestration
- `pkg/resilience/circuit.go` - Circuit breaker
- `pkg/resilience/retry.go` - Retry logic
- `pkg/resilience/timeout.go` - Timeout management

### Tests
- `pkg/backend/state_test.go` - State machine tests
- `pkg/health/active_test.go` - Active health check tests
- `pkg/health/passive_test.go` - Passive health check tests
- `pkg/resilience/circuit_test.go` - Circuit breaker tests
- `pkg/resilience/retry_test.go` - Retry logic tests

### Configuration
- Updated `pkg/config/config.go` with health check and resilience configs

## Summary

Phase 5 adds production-grade health checking and fault tolerance to Balance:

✅ Active health checks (TCP, HTTP, HTTPS)
✅ Passive health checks (failure tracking)
✅ Backend state machine with transitions
✅ Circuit breaker pattern
✅ Retry logic with exponential backoff
✅ Timeout management
✅ Comprehensive test coverage
✅ Detailed metrics and monitoring

Balance now has the resilience features needed for production deployments!

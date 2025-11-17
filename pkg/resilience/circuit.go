package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrCircuitOpen is returned when the circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrTooManyRequests is returned when too many requests are in half-open state
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// StateClosed allows all requests through
	StateClosed CircuitState = iota

	// StateOpen rejects all requests
	StateOpen

	// StateHalfOpen allows limited requests to test recovery
	StateHalfOpen
)

// String returns the string representation of the circuit state
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name string

	// Configuration
	maxFailures     uint32
	timeout         time.Duration
	halfOpenMaxReqs uint32

	// State
	state            atomic.Value // CircuitState
	failures         atomic.Uint32
	successes        atomic.Uint32
	consecutiveFails atomic.Uint32
	halfOpenReqs     atomic.Uint32
	lastFailTime     atomic.Value // time.Time
	stateChangedTime atomic.Value // time.Time

	// Metrics
	totalRequests atomic.Uint64
	totalSuccesses atomic.Uint64
	totalFailures atomic.Uint64
	totalRejected atomic.Uint64

	// Listeners
	listeners []StateChangeListener
	mu        sync.RWMutex
}

// StateChangeListener is called when circuit breaker state changes
type StateChangeListener func(name string, from, to CircuitState)

// CircuitBreakerConfig configures a circuit breaker
type CircuitBreakerConfig struct {
	// Name of the circuit breaker
	Name string

	// MaxFailures before opening the circuit
	MaxFailures uint32

	// Timeout before attempting to recover (transition to half-open)
	Timeout time.Duration

	// MaxConcurrentRequests in half-open state
	MaxConcurrentRequests uint32
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	// Set defaults
	if config.MaxFailures == 0 {
		config.MaxFailures = 5
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.MaxConcurrentRequests == 0 {
		config.MaxConcurrentRequests = 1
	}

	cb := &CircuitBreaker{
		name:            config.Name,
		maxFailures:     config.MaxFailures,
		timeout:         config.Timeout,
		halfOpenMaxReqs: config.MaxConcurrentRequests,
	}

	cb.state.Store(StateClosed)
	cb.stateChangedTime.Store(time.Now())

	return cb
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	// Check if request is allowed
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.afterRequest(err)

	return err
}

// ExecuteWithContext executes a function with context and circuit breaker protection
func (cb *CircuitBreaker) ExecuteWithContext(ctx context.Context, fn func(context.Context) error) error {
	// Check if request is allowed
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the function
	err := fn(ctx)

	// Record the result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if the request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.totalRequests.Add(1)

	state := cb.GetState()

	switch state {
	case StateClosed:
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		lastFail := cb.getLastFailTime()
		if time.Since(lastFail) > cb.timeout {
			// Transition to half-open
			cb.setState(StateHalfOpen)
			return nil
		}
		cb.totalRejected.Add(1)
		return ErrCircuitOpen

	case StateHalfOpen:
		// Allow limited concurrent requests
		current := cb.halfOpenReqs.Add(1)
		if current > cb.halfOpenMaxReqs {
			cb.halfOpenReqs.Add(^uint32(0)) // Decrement
			cb.totalRejected.Add(1)
			return ErrTooManyRequests
		}
		return nil

	default:
		return nil
	}
}

// afterRequest records the result of a request
func (cb *CircuitBreaker) afterRequest(err error) {
	state := cb.GetState()

	if err != nil {
		cb.onFailure(state)
	} else {
		cb.onSuccess(state)
	}

	// Decrement half-open requests counter
	if state == StateHalfOpen {
		cb.halfOpenReqs.Add(^uint32(0)) // Decrement
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess(state CircuitState) {
	cb.totalSuccesses.Add(1)

	switch state {
	case StateClosed:
		// Reset failure count on success
		cb.failures.Store(0)
		cb.consecutiveFails.Store(0)

	case StateHalfOpen:
		cb.successes.Add(1)
		// After some successes in half-open, transition to closed
		// We'll use a simple heuristic: if we get half of max failures as successes, close the circuit
		if cb.successes.Load() >= cb.maxFailures/2 {
			cb.setState(StateClosed)
			cb.failures.Store(0)
			cb.consecutiveFails.Store(0)
			cb.successes.Store(0)
		}
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure(state CircuitState) {
	cb.totalFailures.Add(1)
	cb.failures.Add(1)
	cb.consecutiveFails.Add(1)
	cb.lastFailTime.Store(time.Now())

	switch state {
	case StateClosed:
		// Check if we should open the circuit
		if cb.failures.Load() >= cb.maxFailures {
			cb.setState(StateOpen)
		}

	case StateHalfOpen:
		// On any failure in half-open, go back to open
		cb.setState(StateOpen)
		cb.successes.Store(0)
	}
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() CircuitState {
	return cb.state.Load().(CircuitState)
}

// setState sets the state and notifies listeners
func (cb *CircuitBreaker) setState(newState CircuitState) {
	oldState := cb.GetState()
	if oldState == newState {
		return
	}

	cb.state.Store(newState)
	cb.stateChangedTime.Store(time.Now())

	// Notify listeners
	cb.mu.RLock()
	listeners := make([]StateChangeListener, len(cb.listeners))
	copy(listeners, cb.listeners)
	cb.mu.RUnlock()

	for _, listener := range listeners {
		listener(cb.name, oldState, newState)
	}
}

// AddListener adds a state change listener
func (cb *CircuitBreaker) AddListener(listener StateChangeListener) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.listeners = append(cb.listeners, listener)
}

// getLastFailTime returns the last failure time
func (cb *CircuitBreaker) getLastFailTime() time.Time {
	val := cb.lastFailTime.Load()
	if val == nil {
		return time.Time{}
	}
	return val.(time.Time)
}

// GetStateChangedTime returns when the state last changed
func (cb *CircuitBreaker) GetStateChangedTime() time.Time {
	val := cb.stateChangedTime.Load()
	if val == nil {
		return time.Time{}
	}
	return val.(time.Time)
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.setState(StateClosed)
	cb.failures.Store(0)
	cb.successes.Store(0)
	cb.consecutiveFails.Store(0)
	cb.halfOpenReqs.Store(0)
}

// GetMetrics returns circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() CircuitBreakerMetrics {
	return CircuitBreakerMetrics{
		State:              cb.GetState(),
		TotalRequests:      cb.totalRequests.Load(),
		TotalSuccesses:     cb.totalSuccesses.Load(),
		TotalFailures:      cb.totalFailures.Load(),
		TotalRejected:      cb.totalRejected.Load(),
		ConsecutiveFailures: cb.consecutiveFails.Load(),
		StateChangedAt:     cb.GetStateChangedTime(),
	}
}

// CircuitBreakerMetrics contains circuit breaker metrics
type CircuitBreakerMetrics struct {
	State               CircuitState
	TotalRequests       uint64
	TotalSuccesses      uint64
	TotalFailures       uint64
	TotalRejected       uint64
	ConsecutiveFailures uint32
	StateChangedAt      time.Time
}

// String returns a string representation of the metrics
func (m CircuitBreakerMetrics) String() string {
	return fmt.Sprintf("State: %s, Requests: %d, Successes: %d, Failures: %d, Rejected: %d, ConsecutiveFailures: %d",
		m.State, m.TotalRequests, m.TotalSuccesses, m.TotalFailures, m.TotalRejected, m.ConsecutiveFailures)
}

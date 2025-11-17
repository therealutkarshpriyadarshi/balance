package backend

import (
	"sync"
	"sync/atomic"
	"time"
)

// State represents the health state of a backend
type State int

const (
	// StateHealthy indicates the backend is healthy and accepting traffic
	StateHealthy State = iota

	// StateUnhealthy indicates the backend has failed health checks
	StateUnhealthy

	// StateDraining indicates the backend is being gracefully removed
	StateDraining
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateHealthy:
		return "healthy"
	case StateUnhealthy:
		return "unhealthy"
	case StateDraining:
		return "draining"
	default:
		return "unknown"
	}
}

// StateChangeListener is called when backend state changes
type StateChangeListener func(backend *Backend, oldState, newState State)

// HealthMetrics tracks health-related metrics for a backend
type HealthMetrics struct {
	// Consecutive successful health checks
	consecutiveSuccesses atomic.Int64

	// Consecutive failed health checks
	consecutiveFailures atomic.Int64

	// Total successful health checks
	totalSuccesses atomic.Int64

	// Total failed health checks
	totalFailures atomic.Int64

	// Last health check time
	lastCheckTime atomic.Value // time.Time

	// Last state change time
	lastStateChange atomic.Value // time.Time

	// Total requests to this backend
	totalRequests atomic.Int64

	// Failed requests (passive health check)
	failedRequests atomic.Int64

	// Response time tracking (for passive health checks)
	totalResponseTime atomic.Int64 // in nanoseconds
}

// StateMachine manages backend health state transitions
type StateMachine struct {
	backend *Backend
	state   atomic.Value // State

	metrics HealthMetrics

	// Thresholds for state transitions
	healthyThreshold   int
	unhealthyThreshold int

	// State change listeners
	listeners []StateChangeListener
	mu        sync.RWMutex
}

// NewStateMachine creates a new state machine for a backend
func NewStateMachine(backend *Backend, healthyThreshold, unhealthyThreshold int) *StateMachine {
	sm := &StateMachine{
		backend:            backend,
		healthyThreshold:   healthyThreshold,
		unhealthyThreshold: unhealthyThreshold,
	}
	sm.state.Store(StateHealthy)
	now := time.Now()
	sm.metrics.lastCheckTime.Store(now)
	sm.metrics.lastStateChange.Store(now)
	return sm
}

// GetState returns the current state
func (sm *StateMachine) GetState() State {
	return sm.state.Load().(State)
}

// IsHealthy returns true if the backend is in a healthy state
func (sm *StateMachine) IsHealthy() bool {
	return sm.GetState() == StateHealthy
}

// IsDraining returns true if the backend is draining
func (sm *StateMachine) IsDraining() bool {
	return sm.GetState() == StateDraining
}

// RecordSuccess records a successful health check
func (sm *StateMachine) RecordSuccess() {
	sm.metrics.consecutiveSuccesses.Add(1)
	sm.metrics.consecutiveFailures.Store(0)
	sm.metrics.totalSuccesses.Add(1)
	sm.metrics.lastCheckTime.Store(time.Now())

	// Check if we should transition to healthy
	if sm.metrics.consecutiveSuccesses.Load() >= int64(sm.healthyThreshold) {
		sm.transitionTo(StateHealthy)
	}
}

// RecordFailure records a failed health check
func (sm *StateMachine) RecordFailure() {
	sm.metrics.consecutiveFailures.Add(1)
	sm.metrics.consecutiveSuccesses.Store(0)
	sm.metrics.totalFailures.Add(1)
	sm.metrics.lastCheckTime.Store(time.Now())

	// Check if we should transition to unhealthy
	if sm.metrics.consecutiveFailures.Load() >= int64(sm.unhealthyThreshold) {
		sm.transitionTo(StateUnhealthy)
	}
}

// RecordRequest records a request to this backend (for passive health checks)
func (sm *StateMachine) RecordRequest(success bool, responseTime time.Duration) {
	sm.metrics.totalRequests.Add(1)

	if success {
		sm.metrics.totalResponseTime.Add(int64(responseTime))
	} else {
		sm.metrics.failedRequests.Add(1)
	}
}

// StartDraining transitions the backend to draining state
func (sm *StateMachine) StartDraining() {
	sm.transitionTo(StateDraining)
}

// ForceHealthy forces the backend to healthy state (for manual intervention)
func (sm *StateMachine) ForceHealthy() {
	sm.metrics.consecutiveSuccesses.Store(int64(sm.healthyThreshold))
	sm.metrics.consecutiveFailures.Store(0)
	sm.transitionTo(StateHealthy)
}

// ForceUnhealthy forces the backend to unhealthy state (for manual intervention)
func (sm *StateMachine) ForceUnhealthy() {
	sm.metrics.consecutiveFailures.Store(int64(sm.unhealthyThreshold))
	sm.metrics.consecutiveSuccesses.Store(0)
	sm.transitionTo(StateUnhealthy)
}

// transitionTo transitions to a new state and notifies listeners
func (sm *StateMachine) transitionTo(newState State) {
	oldState := sm.GetState()
	if oldState == newState {
		return
	}

	sm.state.Store(newState)
	sm.metrics.lastStateChange.Store(time.Now())

	// Update backend's health status for backward compatibility
	if newState == StateHealthy {
		sm.backend.MarkHealthy()
	} else {
		sm.backend.MarkUnhealthy()
	}

	// Notify listeners
	sm.mu.RLock()
	listeners := make([]StateChangeListener, len(sm.listeners))
	copy(listeners, sm.listeners)
	sm.mu.RUnlock()

	for _, listener := range listeners {
		listener(sm.backend, oldState, newState)
	}
}

// AddListener adds a state change listener
func (sm *StateMachine) AddListener(listener StateChangeListener) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.listeners = append(sm.listeners, listener)
}

// GetMetrics returns the current health metrics
func (sm *StateMachine) GetMetrics() HealthMetrics {
	return sm.metrics
}

// GetConsecutiveSuccesses returns consecutive successful health checks
func (sm *StateMachine) GetConsecutiveSuccesses() int64 {
	return sm.metrics.consecutiveSuccesses.Load()
}

// GetConsecutiveFailures returns consecutive failed health checks
func (sm *StateMachine) GetConsecutiveFailures() int64 {
	return sm.metrics.consecutiveFailures.Load()
}

// GetTotalRequests returns total requests to this backend
func (sm *StateMachine) GetTotalRequests() int64 {
	return sm.metrics.totalRequests.Load()
}

// GetFailedRequests returns failed requests to this backend
func (sm *StateMachine) GetFailedRequests() int64 {
	return sm.metrics.failedRequests.Load()
}

// GetErrorRate returns the error rate (0.0 to 1.0)
func (sm *StateMachine) GetErrorRate() float64 {
	total := sm.metrics.totalRequests.Load()
	if total == 0 {
		return 0.0
	}
	failed := sm.metrics.failedRequests.Load()
	return float64(failed) / float64(total)
}

// GetAverageResponseTime returns average response time
func (sm *StateMachine) GetAverageResponseTime() time.Duration {
	total := sm.metrics.totalRequests.Load() - sm.metrics.failedRequests.Load()
	if total == 0 {
		return 0
	}
	totalTime := sm.metrics.totalResponseTime.Load()
	return time.Duration(totalTime / total)
}

// GetLastCheckTime returns the last health check time
func (sm *StateMachine) GetLastCheckTime() time.Time {
	val := sm.metrics.lastCheckTime.Load()
	if val == nil {
		return time.Time{}
	}
	return val.(time.Time)
}

// GetLastStateChangeTime returns the last state change time
func (sm *StateMachine) GetLastStateChangeTime() time.Time {
	val := sm.metrics.lastStateChange.Load()
	if val == nil {
		return time.Time{}
	}
	return val.(time.Time)
}

// Reset resets the metrics
func (sm *StateMachine) Reset() {
	sm.metrics.consecutiveSuccesses.Store(0)
	sm.metrics.consecutiveFailures.Store(0)
	sm.metrics.totalSuccesses.Store(0)
	sm.metrics.totalFailures.Store(0)
	sm.metrics.totalRequests.Store(0)
	sm.metrics.failedRequests.Store(0)
	sm.metrics.totalResponseTime.Store(0)
	now := time.Now()
	sm.metrics.lastCheckTime.Store(now)
	sm.metrics.lastStateChange.Store(now)
}

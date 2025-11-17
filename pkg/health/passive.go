package health

import (
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

// PassiveCheckerConfig configures a passive health checker
type PassiveCheckerConfig struct {
	// ErrorRateThreshold is the error rate (0.0-1.0) that triggers unhealthy
	ErrorRateThreshold float64

	// MinRequests is the minimum number of requests before checking error rate
	MinRequests int64

	// ConsecutiveFailures is the number of consecutive failures to mark unhealthy
	ConsecutiveFailures int

	// Window is the time window for tracking failures
	Window time.Duration
}

// PassiveChecker monitors backend failures and marks them unhealthy
type PassiveChecker struct {
	config PassiveCheckerConfig

	// Track failures per backend
	failures map[*backend.Backend]*failureTracker
	mu       sync.RWMutex
}

// failureTracker tracks failures for a single backend
type failureTracker struct {
	consecutiveFailures int
	lastFailureTime     time.Time
	windowFailures      []time.Time
	mu                  sync.Mutex
}

// NewPassiveChecker creates a new passive health checker
func NewPassiveChecker(config PassiveCheckerConfig) *PassiveChecker {
	// Default values
	if config.ErrorRateThreshold == 0 {
		config.ErrorRateThreshold = 0.5 // 50% error rate
	}
	if config.MinRequests == 0 {
		config.MinRequests = 10
	}
	if config.ConsecutiveFailures == 0 {
		config.ConsecutiveFailures = 5
	}
	if config.Window == 0 {
		config.Window = 1 * time.Minute
	}

	return &PassiveChecker{
		config:   config,
		failures: make(map[*backend.Backend]*failureTracker),
	}
}

// RecordSuccess records a successful request to a backend
func (pc *PassiveChecker) RecordSuccess(b *backend.Backend, responseTime time.Duration) {
	pc.mu.Lock()
	tracker, exists := pc.failures[b]
	if !exists {
		tracker = &failureTracker{
			windowFailures: make([]time.Time, 0),
		}
		pc.failures[b] = tracker
	}
	pc.mu.Unlock()

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// Reset consecutive failures on success
	tracker.consecutiveFailures = 0
}

// RecordFailure records a failed request to a backend
func (pc *PassiveChecker) RecordFailure(b *backend.Backend) bool {
	pc.mu.Lock()
	tracker, exists := pc.failures[b]
	if !exists {
		tracker = &failureTracker{
			windowFailures: make([]time.Time, 0),
		}
		pc.failures[b] = tracker
	}
	pc.mu.Unlock()

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	now := time.Now()
	tracker.consecutiveFailures++
	tracker.lastFailureTime = now
	tracker.windowFailures = append(tracker.windowFailures, now)

	// Clean old failures outside the window
	cutoff := now.Add(-pc.config.Window)
	validFailures := make([]time.Time, 0)
	for _, t := range tracker.windowFailures {
		if t.After(cutoff) {
			validFailures = append(validFailures, t)
		}
	}
	tracker.windowFailures = validFailures

	// Check if backend should be marked unhealthy
	return pc.shouldMarkUnhealthy(tracker)
}

// shouldMarkUnhealthy determines if a backend should be marked unhealthy
func (pc *PassiveChecker) shouldMarkUnhealthy(tracker *failureTracker) bool {
	// Check consecutive failures
	if tracker.consecutiveFailures >= pc.config.ConsecutiveFailures {
		return true
	}

	// Check error rate in window
	if int64(len(tracker.windowFailures)) >= pc.config.MinRequests {
		// In passive mode, we don't have total requests, so we use a simpler heuristic
		// If we have enough failures in the window, mark as unhealthy
		return true
	}

	return false
}

// GetConsecutiveFailures returns the consecutive failure count for a backend
func (pc *PassiveChecker) GetConsecutiveFailures(b *backend.Backend) int {
	pc.mu.RLock()
	tracker, exists := pc.failures[b]
	pc.mu.RUnlock()

	if !exists {
		return 0
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	return tracker.consecutiveFailures
}

// GetWindowFailures returns the number of failures in the current window
func (pc *PassiveChecker) GetWindowFailures(b *backend.Backend) int {
	pc.mu.RLock()
	tracker, exists := pc.failures[b]
	pc.mu.RUnlock()

	if !exists {
		return 0
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// Clean old failures
	now := time.Now()
	cutoff := now.Add(-pc.config.Window)
	count := 0
	for _, t := range tracker.windowFailures {
		if t.After(cutoff) {
			count++
		}
	}

	return count
}

// Reset resets the failure tracking for a backend
func (pc *PassiveChecker) Reset(b *backend.Backend) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.failures, b)
}

// ResetAll resets all failure tracking
func (pc *PassiveChecker) ResetAll() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.failures = make(map[*backend.Backend]*failureTracker)
}

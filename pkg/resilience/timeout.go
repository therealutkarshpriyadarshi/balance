package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

var (
	// ErrTimeout is returned when an operation times out
	ErrTimeout = errors.New("operation timeout")
)

// TimeoutConfig configures timeout behavior
type TimeoutConfig struct {
	// RequestTimeout is the total timeout for a request
	RequestTimeout time.Duration

	// ConnectTimeout is the timeout for establishing a connection
	ConnectTimeout time.Duration

	// ReadTimeout is the timeout for reading from a connection
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for writing to a connection
	WriteTimeout time.Duration

	// IdleTimeout is the timeout for idle connections
	IdleTimeout time.Duration
}

// DefaultTimeoutConfig returns a default timeout configuration
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		RequestTimeout: 30 * time.Second,
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
	}
}

// TimeoutManager manages timeouts for operations
type TimeoutManager struct {
	config TimeoutConfig

	// Metrics
	totalTimeouts atomic.Uint64
	connectTimeouts atomic.Uint64
	readTimeouts atomic.Uint64
	writeTimeouts atomic.Uint64
}

// NewTimeoutManager creates a new timeout manager
func NewTimeoutManager(config TimeoutConfig) *TimeoutManager {
	return &TimeoutManager{
		config: config,
	}
}

// WithTimeout executes a function with a timeout
func (tm *TimeoutManager) WithTimeout(timeout time.Duration, fn func() error) error {
	return tm.WithTimeoutContext(context.Background(), timeout, func(ctx context.Context) error {
		return fn()
	})
}

// WithTimeoutContext executes a function with a timeout and context
func (tm *TimeoutManager) WithTimeoutContext(parentCtx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	errChan := make(chan error, 1)

	go func() {
		errChan <- fn(ctx)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		tm.totalTimeouts.Add(1)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ErrTimeout
		}
		return ctx.Err()
	}
}

// WithRequestTimeout executes a function with the configured request timeout
func (tm *TimeoutManager) WithRequestTimeout(fn func(context.Context) error) error {
	return tm.WithTimeoutContext(context.Background(), tm.config.RequestTimeout, fn)
}

// WithConnectTimeout executes a function with the configured connect timeout
func (tm *TimeoutManager) WithConnectTimeout(fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), tm.config.ConnectTimeout)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- fn(ctx)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		tm.connectTimeouts.Add(1)
		tm.totalTimeouts.Add(1)
		return ErrTimeout
	}
}

// CreateRequestContext creates a context with request timeout
func (tm *TimeoutManager) CreateRequestContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, tm.config.RequestTimeout)
}

// CreateConnectContext creates a context with connect timeout
func (tm *TimeoutManager) CreateConnectContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, tm.config.ConnectTimeout)
}

// RecordConnectTimeout records a connect timeout
func (tm *TimeoutManager) RecordConnectTimeout() {
	tm.connectTimeouts.Add(1)
	tm.totalTimeouts.Add(1)
}

// RecordReadTimeout records a read timeout
func (tm *TimeoutManager) RecordReadTimeout() {
	tm.readTimeouts.Add(1)
	tm.totalTimeouts.Add(1)
}

// RecordWriteTimeout records a write timeout
func (tm *TimeoutManager) RecordWriteTimeout() {
	tm.writeTimeouts.Add(1)
	tm.totalTimeouts.Add(1)
}

// GetMetrics returns timeout metrics
func (tm *TimeoutManager) GetMetrics() TimeoutMetrics {
	return TimeoutMetrics{
		TotalTimeouts:   tm.totalTimeouts.Load(),
		ConnectTimeouts: tm.connectTimeouts.Load(),
		ReadTimeouts:    tm.readTimeouts.Load(),
		WriteTimeouts:   tm.writeTimeouts.Load(),
	}
}

// TimeoutMetrics contains timeout metrics
type TimeoutMetrics struct {
	TotalTimeouts   uint64
	ConnectTimeouts uint64
	ReadTimeouts    uint64
	WriteTimeouts   uint64
}

// GetConfig returns the timeout configuration
func (tm *TimeoutManager) GetConfig() TimeoutConfig {
	return tm.config
}

// UpdateConfig updates the timeout configuration
func (tm *TimeoutManager) UpdateConfig(config TimeoutConfig) {
	tm.config = config
}

// DeadlineManager helps manage operation deadlines
type DeadlineManager struct {
	defaultTimeout time.Duration
}

// NewDeadlineManager creates a new deadline manager
func NewDeadlineManager(defaultTimeout time.Duration) *DeadlineManager {
	return &DeadlineManager{
		defaultTimeout: defaultTimeout,
	}
}

// CreateDeadline creates a deadline for an operation
func (dm *DeadlineManager) CreateDeadline() time.Time {
	return time.Now().Add(dm.defaultTimeout)
}

// CreateDeadlineWithTimeout creates a deadline with a specific timeout
func (dm *DeadlineManager) CreateDeadlineWithTimeout(timeout time.Duration) time.Time {
	return time.Now().Add(timeout)
}

// IsExpired checks if a deadline has expired
func (dm *DeadlineManager) IsExpired(deadline time.Time) bool {
	return time.Now().After(deadline)
}

// RemainingTime returns the time remaining until the deadline
func (dm *DeadlineManager) RemainingTime(deadline time.Time) time.Duration {
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// PropagateDeadline propagates a deadline from parent context to child
func PropagateDeadline(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	// Check if parent has a deadline
	if deadline, ok := parent.Deadline(); ok {
		remaining := time.Until(deadline)
		// Use the shorter of parent deadline or requested timeout
		if remaining < timeout {
			// Parent deadline is sooner, use it
			return context.WithDeadline(parent, deadline)
		}
	}

	// Use requested timeout
	return context.WithTimeout(parent, timeout)
}

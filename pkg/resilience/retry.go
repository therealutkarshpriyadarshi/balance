package resilience

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

var (
	// ErrMaxRetriesExceeded is returned when max retries are exceeded
	ErrMaxRetriesExceeded = errors.New("maximum retry attempts exceeded")
)

// RetryPolicy defines the retry behavior
type RetryPolicy struct {
	// MaxAttempts is the maximum number of retry attempts (0 = no retries, -1 = unlimited)
	MaxAttempts int

	// InitialDelay is the initial backoff delay
	InitialDelay time.Duration

	// MaxDelay is the maximum backoff delay
	MaxDelay time.Duration

	// Multiplier is the backoff multiplier (exponential backoff)
	Multiplier float64

	// Jitter adds randomness to backoff (0.0 = no jitter, 1.0 = full jitter)
	Jitter float64

	// RetryableErrors is a function that determines if an error is retryable
	RetryableErrors func(error) bool

	// OnRetry is called before each retry attempt
	OnRetry func(attempt int, err error, delay time.Duration)
}

// DefaultRetryPolicy returns a default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryableErrors: func(err error) bool {
			// By default, retry all errors except context cancellation
			return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
		},
	}
}

// Retry executes a function with retry logic
func Retry(fn func() error, policy RetryPolicy) error {
	return RetryWithContext(context.Background(), func(ctx context.Context) error {
		return fn()
	}, policy)
}

// RetryWithContext executes a function with retry logic and context
func RetryWithContext(ctx context.Context, fn func(context.Context) error, policy RetryPolicy) error {
	var lastErr error
	attempts := 0

	for {
		attempts++

		// Execute the function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if policy.RetryableErrors != nil && !policy.RetryableErrors(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Check if we've exceeded max attempts
		if policy.MaxAttempts > 0 && attempts >= policy.MaxAttempts {
			return fmt.Errorf("%w after %d attempts: %v", ErrMaxRetriesExceeded, attempts, lastErr)
		}

		// Check if context is done
		if ctx.Err() != nil {
			return fmt.Errorf("context cancelled after %d attempts: %w", attempts, ctx.Err())
		}

		// Calculate backoff delay
		delay := calculateBackoff(attempts, policy)

		// Call OnRetry callback if provided
		if policy.OnRetry != nil {
			policy.OnRetry(attempts, err, delay)
		}

		// Wait for backoff delay
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}
}

// calculateBackoff calculates the backoff delay for a given attempt
func calculateBackoff(attempt int, policy RetryPolicy) time.Duration {
	// Calculate exponential backoff
	delay := float64(policy.InitialDelay) * math.Pow(policy.Multiplier, float64(attempt-1))

	// Cap at max delay
	if delay > float64(policy.MaxDelay) {
		delay = float64(policy.MaxDelay)
	}

	// Add jitter if configured
	if policy.Jitter > 0 {
		jitter := delay * policy.Jitter * (rand.Float64()*2 - 1) // Random value between -jitter and +jitter
		delay += jitter
		if delay < 0 {
			delay = 0
		}
	}

	return time.Duration(delay)
}

// Retryer provides a reusable retry executor
type Retryer struct {
	policy RetryPolicy
}

// NewRetryer creates a new retryer with the given policy
func NewRetryer(policy RetryPolicy) *Retryer {
	return &Retryer{policy: policy}
}

// Execute executes a function with retry logic
func (r *Retryer) Execute(fn func() error) error {
	return Retry(fn, r.policy)
}

// ExecuteWithContext executes a function with retry logic and context
func (r *Retryer) ExecuteWithContext(ctx context.Context, fn func(context.Context) error) error {
	return RetryWithContext(ctx, fn, r.policy)
}

// RetryBudget prevents retry storms by limiting the ratio of retries to requests
type RetryBudget struct {
	// TTL is the time window for the budget
	TTL time.Duration

	// MinRetriesPerSecond is the minimum number of retries allowed per second
	MinRetriesPerSecond int

	// RetryRatio is the maximum ratio of retries to requests (0.0 to 1.0)
	RetryRatio float64

	// State
	requests        int64
	retries         int64
	lastReset       time.Time
	mu              sync.RWMutex
}

// NewRetryBudget creates a new retry budget
func NewRetryBudget(ttl time.Duration, minRetriesPerSec int, retryRatio float64) *RetryBudget {
	return &RetryBudget{
		TTL:                 ttl,
		MinRetriesPerSecond: minRetriesPerSec,
		RetryRatio:          retryRatio,
		lastReset:           time.Now(),
	}
}

// CanRetry checks if a retry is allowed within the budget
func (rb *RetryBudget) CanRetry() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.maybeReset()

	// Calculate minimum retries allowed
	elapsed := time.Since(rb.lastReset)
	minRetries := int64(float64(rb.MinRetriesPerSecond) * elapsed.Seconds())

	// Calculate ratio-based retries allowed
	ratioRetries := int64(float64(rb.requests) * rb.RetryRatio)

	// Allow retry if either minimum is not reached or ratio allows
	allowed := rb.retries < minRetries || rb.retries < ratioRetries

	if allowed {
		rb.retries++
	}

	return allowed
}

// RecordRequest records a request
func (rb *RetryBudget) RecordRequest() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.maybeReset()
	rb.requests++
}

// maybeReset resets the budget if TTL has elapsed
func (rb *RetryBudget) maybeReset() {
	if time.Since(rb.lastReset) > rb.TTL {
		rb.requests = 0
		rb.retries = 0
		rb.lastReset = time.Now()
	}
}

// GetStats returns current budget statistics
func (rb *RetryBudget) GetStats() (requests, retries int64, ratio float64) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	requests = rb.requests
	retries = rb.retries

	if requests > 0 {
		ratio = float64(retries) / float64(requests)
	}

	return
}

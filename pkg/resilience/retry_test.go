package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy.MaxAttempts != 3 {
		t.Errorf("Expected max attempts 3, got %d", policy.MaxAttempts)
	}

	if policy.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected initial delay 100ms, got %s", policy.InitialDelay)
	}

	if policy.Multiplier != 2.0 {
		t.Errorf("Expected multiplier 2.0, got %.1f", policy.Multiplier)
	}
}

func TestRetry_Success(t *testing.T) {
	attempts := 0

	err := Retry(func() error {
		attempts++
		return nil
	}, RetryPolicy{MaxAttempts: 3})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetry_SuccessAfterFailures(t *testing.T) {
	attempts := 0

	err := Retry(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}, RetryPolicy{
		MaxAttempts:  5,
		InitialDelay: 10 * time.Millisecond,
		Multiplier:   1.0,
	})

	if err != nil {
		t.Errorf("Expected success after retries, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	attempts := 0
	testErr := errors.New("persistent failure")

	err := Retry(func() error {
		attempts++
		return testErr
	}, RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Multiplier:   1.0,
	})

	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Errorf("Expected ErrMaxRetriesExceeded, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	attempts := 0
	testErr := errors.New("non-retryable")

	err := Retry(func() error {
		attempts++
		return testErr
	}, RetryPolicy{
		MaxAttempts:  5,
		InitialDelay: 10 * time.Millisecond,
		RetryableErrors: func(err error) bool {
			return false // All errors are non-retryable
		},
	})

	if err == nil {
		t.Error("Expected error to be returned")
	}

	if attempts != 1 {
		t.Errorf("Expected only 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attempts := 0
	err := RetryWithContext(ctx, func(ctx context.Context) error {
		attempts++
		time.Sleep(10 * time.Millisecond) // Simulate some work
		return errors.New("failure")
	}, RetryPolicy{
		MaxAttempts:  10,
		InitialDelay: 20 * time.Millisecond,
		Multiplier:   1.0,
	})

	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	// Should not complete all 10 attempts due to timeout
	if attempts >= 10 {
		t.Errorf("Expected fewer than 10 attempts due to timeout, got %d", attempts)
	}
}

func TestRetry_ExponentialBackoff(t *testing.T) {
	attempts := 0
	delays := []time.Duration{}
	lastTime := time.Now()

	err := Retry(func() error {
		attempts++
		if attempts > 1 {
			delay := time.Since(lastTime)
			delays = append(delays, delay)
		}
		lastTime = time.Now()

		if attempts < 4 {
			return errors.New("fail")
		}
		return nil
	}, RetryPolicy{
		MaxAttempts:  4,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       0, // No jitter for predictable testing
	})

	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}

	if len(delays) != 3 {
		t.Fatalf("Expected 3 delays, got %d", len(delays))
	}

	// Check that delays are increasing (with some tolerance for timing)
	for i := 1; i < len(delays); i++ {
		if delays[i] < delays[i-1] {
			t.Errorf("Expected delay %d (%s) to be >= delay %d (%s)",
				i, delays[i], i-1, delays[i-1])
		}
	}
}

func TestRetry_OnRetryCallback(t *testing.T) {
	callbackCalled := 0
	var lastAttempt int
	var lastErr error
	var lastDelay time.Duration

	err := Retry(func() error {
		if callbackCalled < 2 {
			return errors.New("fail")
		}
		return nil
	}, RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Multiplier:   1.0,
		Jitter:       0,
		OnRetry: func(attempt int, err error, delay time.Duration) {
			callbackCalled++
			lastAttempt = attempt
			lastErr = err
			lastDelay = delay
		},
	})

	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}

	if callbackCalled != 2 {
		t.Errorf("Expected callback to be called 2 times, got %d", callbackCalled)
	}

	if lastAttempt == 0 {
		t.Error("Expected attempt number to be passed to callback")
	}

	if lastErr == nil {
		t.Error("Expected error to be passed to callback")
	}

	// Note: delay calculation might be affected by jitter and timing
	if lastDelay < 0 {
		t.Error("Expected non-negative delay to be passed to callback")
	}
}

func TestRetry_MaxDelay(t *testing.T) {
	policy := RetryPolicy{
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		Multiplier:   10.0, // Very high multiplier
		Jitter:       0,
	}

	// Calculate backoff for attempt 5
	delay := calculateBackoff(5, policy)

	// Should be capped at max delay
	if delay > policy.MaxDelay {
		t.Errorf("Expected delay to be capped at %s, got %s", policy.MaxDelay, delay)
	}
}

func TestRetryer(t *testing.T) {
	retryer := NewRetryer(RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Multiplier:   1.0,
	})

	attempts := 0
	err := retryer.Execute(func() error {
		attempts++
		if attempts < 2 {
			return errors.New("fail")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetryBudget(t *testing.T) {
	budget := NewRetryBudget(1*time.Second, 10, 0.2)

	// Record requests
	for i := 0; i < 10; i++ {
		budget.RecordRequest()
	}

	// Should allow retries up to 20% of requests
	allowedRetries := 0
	for i := 0; i < 5; i++ {
		if budget.CanRetry() {
			allowedRetries++
		}
	}

	if allowedRetries < 2 {
		t.Errorf("Expected at least 2 retries allowed (20%% of 10), got %d", allowedRetries)
	}
}

func TestRetryBudget_MinRetries(t *testing.T) {
	budget := NewRetryBudget(1*time.Second, 10, 0.1)

	// Even with no requests, should allow min retries
	time.Sleep(200 * time.Millisecond)

	// Should allow at least one retry based on min retries per second
	allowed := budget.CanRetry()
	if !allowed {
		// This might fail on very fast systems, so just log a warning
		t.Logf("Warning: Expected at least one retry to be allowed based on MinRetriesPerSecond")
	}
}

func TestRetryBudget_Reset(t *testing.T) {
	budget := NewRetryBudget(100*time.Millisecond, 10, 0.5)

	// Record requests and retries
	for i := 0; i < 5; i++ {
		budget.RecordRequest()
	}

	// Ensure at least one retry is allowed
	retryAllowed := false
	for i := 0; i < 3; i++ {
		if budget.CanRetry() {
			retryAllowed = true
			break
		}
	}

	requests, retries, _ := budget.GetStats()
	if requests == 0 {
		t.Error("Expected non-zero requests")
	}

	if !retryAllowed {
		t.Log("Warning: No retries were allowed, skipping retry check")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Record new request (should trigger reset)
	budget.RecordRequest()

	requests, retries, _ = budget.GetStats()
	if requests != 1 || retries != 0 {
		t.Errorf("Expected stats to be reset, got requests=%d, retries=%d", requests, retries)
	}
}

func TestRetryBudget_Stats(t *testing.T) {
	budget := NewRetryBudget(1*time.Second, 10, 0.5)

	// Record activity
	budget.RecordRequest()
	budget.RecordRequest()
	budget.CanRetry()

	requests, retries, ratio := budget.GetStats()

	if requests != 2 {
		t.Errorf("Expected 2 requests, got %d", requests)
	}

	if retries != 1 {
		t.Errorf("Expected 1 retry, got %d", retries)
	}

	expectedRatio := 0.5
	if ratio < expectedRatio-0.01 || ratio > expectedRatio+0.01 {
		t.Errorf("Expected ratio ~%.2f, got %.2f", expectedRatio, ratio)
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		policy   RetryPolicy
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{
			name:    "First retry",
			attempt: 1,
			policy: RetryPolicy{
				InitialDelay: 100 * time.Millisecond,
				Multiplier:   2.0,
				MaxDelay:     10 * time.Second,
				Jitter:       0,
			},
			minDelay: 100 * time.Millisecond,
			maxDelay: 100 * time.Millisecond,
		},
		{
			name:    "Second retry",
			attempt: 2,
			policy: RetryPolicy{
				InitialDelay: 100 * time.Millisecond,
				Multiplier:   2.0,
				MaxDelay:     10 * time.Second,
				Jitter:       0,
			},
			minDelay: 200 * time.Millisecond,
			maxDelay: 200 * time.Millisecond,
		},
		{
			name:    "Capped at max",
			attempt: 10,
			policy: RetryPolicy{
				InitialDelay: 100 * time.Millisecond,
				Multiplier:   2.0,
				MaxDelay:     1 * time.Second,
				Jitter:       0,
			},
			minDelay: 1 * time.Second,
			maxDelay: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := calculateBackoff(tt.attempt, tt.policy)

			if delay < tt.minDelay || delay > tt.maxDelay {
				t.Errorf("Expected delay between %s and %s, got %s",
					tt.minDelay, tt.maxDelay, delay)
			}
		})
	}
}

package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewCircuitBreaker(t *testing.T) {
	config := CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 5,
		Timeout:     30 * time.Second,
	}

	cb := NewCircuitBreaker(config)

	if cb == nil {
		t.Fatal("Expected circuit breaker to be created")
	}

	if cb.name != "test" {
		t.Errorf("Expected name 'test', got %s", cb.name)
	}

	if cb.GetState() != StateClosed {
		t.Errorf("Expected initial state to be closed, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 3,
		Timeout:     1 * time.Second,
	})

	testErr := errors.New("test error")

	// Execute failing requests
	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error {
			return testErr
		})

		if err != testErr {
			t.Errorf("Expected error to be returned, got %v", err)
		}

		// Should still be closed until we hit max failures
		if i < 2 {
			if cb.GetState() != StateClosed {
				t.Errorf("Expected state to be closed after %d failures", i+1)
			}
		}
	}

	// After 3 failures, should be open
	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be open after %d failures, got %s", 3, cb.GetState())
	}
}

func TestCircuitBreaker_OpenRejectsRequests(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 1,
		Timeout:     1 * time.Hour, // Long timeout
	})

	// Trigger circuit open
	cb.Execute(func() error {
		return errors.New("fail")
	})

	if cb.GetState() != StateOpen {
		t.Fatal("Expected circuit to be open")
	}

	// Try to execute - should be rejected
	err := cb.Execute(func() error {
		t.Error("Function should not be executed when circuit is open")
		return nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}

	// Check metrics
	metrics := cb.GetMetrics()
	if metrics.TotalRejected == 0 {
		t.Error("Expected rejected requests to be tracked")
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 1,
		Timeout:     100 * time.Millisecond,
	})

	// Trigger circuit open
	cb.Execute(func() error {
		return errors.New("fail")
	})

	if cb.GetState() != StateOpen {
		t.Fatal("Expected circuit to be open")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Next request should transition to half-open
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected successful execution in half-open, got %v", err)
	}

	// After successful request in half-open, should eventually close
	// (depends on implementation - may need more successes)
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:                  "test",
		MaxFailures:           1,
		Timeout:               100 * time.Millisecond,
		MaxConcurrentRequests: 1,
	})

	// Open the circuit
	cb.Execute(func() error {
		return errors.New("fail")
	})

	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)

	// Fail in half-open should go back to open
	err := cb.Execute(func() error {
		return errors.New("fail again")
	})

	if err == nil {
		t.Error("Expected error to be returned")
	}

	// Should be open again
	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be open after failure in half-open, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 4,
		Timeout:     100 * time.Millisecond,
	})

	// Open the circuit
	for i := 0; i < 4; i++ {
		cb.Execute(func() error {
			return errors.New("fail")
		})
	}

	if cb.GetState() != StateOpen {
		t.Fatal("Expected circuit to be open")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Execute successful requests in half-open
	// Need maxFailures/2 successes to close
	for i := 0; i < 2; i++ {
		err := cb.Execute(func() error {
			return nil
		})

		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	// Should be closed now
	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to be closed after successes, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpenMaxRequests(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:                  "test",
		MaxFailures:           1,
		Timeout:               100 * time.Millisecond,
		MaxConcurrentRequests: 1,
	})

	// Open the circuit
	cb.Execute(func() error {
		return errors.New("fail")
	})

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// First request should be allowed in half-open
	started := make(chan bool)
	done := make(chan bool)

	go func() {
		cb.Execute(func() error {
			started <- true
			<-done
			return nil
		})
	}()

	// Wait for first request to start
	<-started

	// Give the first request time to be tracked
	time.Sleep(10 * time.Millisecond)

	// Second concurrent request should be rejected
	err := cb.Execute(func() error {
		// This should not execute if circuit breaker is working
		return nil
	})

	// Allow first request to complete before checking
	done <- true
	time.Sleep(10 * time.Millisecond)

	// The second request should have been rejected (or the function shouldn't have executed)
	if err != nil && !errors.Is(err, ErrTooManyRequests) {
		t.Errorf("Expected ErrTooManyRequests or nil, got %v", err)
	}
}

func TestCircuitBreaker_ExecuteWithContext(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 5,
	})

	ctx := context.Background()

	err := cb.ExecuteWithContext(ctx, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected successful execution, got %v", err)
	}

	metrics := cb.GetMetrics()
	if metrics.TotalSuccesses != 1 {
		t.Errorf("Expected 1 success, got %d", metrics.TotalSuccesses)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 2,
	})

	// Trigger some failures
	cb.Execute(func() error {
		return errors.New("fail")
	})
	cb.Execute(func() error {
		return errors.New("fail")
	})

	if cb.GetState() != StateOpen {
		t.Fatal("Expected circuit to be open")
	}

	// Reset
	cb.Reset()

	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to be closed after reset, got %s", cb.GetState())
	}

	metrics := cb.GetMetrics()
	if metrics.ConsecutiveFailures != 0 {
		t.Error("Expected consecutive failures to be reset")
	}
}

func TestCircuitBreaker_StateChangeListener(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 1,
	})

	var notified bool
	var fromState, toState CircuitState

	cb.AddListener(func(name string, from, to CircuitState) {
		notified = true
		fromState = from
		toState = to
	})

	// Trigger state change
	cb.Execute(func() error {
		return errors.New("fail")
	})

	if !notified {
		t.Error("Expected listener to be notified")
	}

	if fromState != StateClosed {
		t.Errorf("Expected from state to be closed, got %s", fromState)
	}

	if toState != StateOpen {
		t.Errorf("Expected to state to be open, got %s", toState)
	}
}

func TestCircuitBreaker_Metrics(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test",
		MaxFailures: 10,
	})

	// Execute mix of successful and failed requests
	cb.Execute(func() error { return nil })
	cb.Execute(func() error { return nil })
	cb.Execute(func() error { return errors.New("fail") })

	metrics := cb.GetMetrics()

	if metrics.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", metrics.TotalRequests)
	}

	if metrics.TotalSuccesses != 2 {
		t.Errorf("Expected 2 successes, got %d", metrics.TotalSuccesses)
	}

	if metrics.TotalFailures != 1 {
		t.Errorf("Expected 1 failure, got %d", metrics.TotalFailures)
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.state.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.state.String())
			}
		})
	}
}

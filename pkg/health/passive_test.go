package health

import (
	"testing"
	"time"

	"github.com/therealutkarshpriyadarshi/balance/pkg/backend"
)

func TestNewPassiveChecker(t *testing.T) {
	config := PassiveCheckerConfig{
		ErrorRateThreshold:  0.5,
		MinRequests:         10,
		ConsecutiveFailures: 5,
		Window:              1 * time.Minute,
	}

	checker := NewPassiveChecker(config)

	if checker == nil {
		t.Fatal("Expected checker to be created")
	}

	if checker.config.ErrorRateThreshold != 0.5 {
		t.Errorf("Expected error rate threshold 0.5, got %.2f", checker.config.ErrorRateThreshold)
	}
}

func TestNewPassiveChecker_Defaults(t *testing.T) {
	config := PassiveCheckerConfig{}
	checker := NewPassiveChecker(config)

	if checker.config.ErrorRateThreshold != 0.5 {
		t.Errorf("Expected default error rate threshold 0.5, got %.2f", checker.config.ErrorRateThreshold)
	}

	if checker.config.MinRequests != 10 {
		t.Errorf("Expected default min requests 10, got %d", checker.config.MinRequests)
	}

	if checker.config.ConsecutiveFailures != 5 {
		t.Errorf("Expected default consecutive failures 5, got %d", checker.config.ConsecutiveFailures)
	}
}

func TestPassiveChecker_RecordSuccess(t *testing.T) {
	checker := NewPassiveChecker(PassiveCheckerConfig{
		ConsecutiveFailures: 3,
	})

	b := backend.NewBackend("test", "localhost:8080", 1)

	// Record a failure first
	checker.RecordFailure(b)
	if checker.GetConsecutiveFailures(b) != 1 {
		t.Error("Expected 1 consecutive failure")
	}

	// Record a success
	checker.RecordSuccess(b, 100*time.Millisecond)

	// Consecutive failures should be reset
	if checker.GetConsecutiveFailures(b) != 0 {
		t.Error("Expected consecutive failures to be reset after success")
	}
}

func TestPassiveChecker_ConsecutiveFailures(t *testing.T) {
	checker := NewPassiveChecker(PassiveCheckerConfig{
		ConsecutiveFailures: 3,
	})

	b := backend.NewBackend("test", "localhost:8080", 1)

	// Record failures
	shouldMark := checker.RecordFailure(b)
	if shouldMark {
		t.Error("Should not mark unhealthy after 1 failure")
	}

	shouldMark = checker.RecordFailure(b)
	if shouldMark {
		t.Error("Should not mark unhealthy after 2 failures")
	}

	shouldMark = checker.RecordFailure(b)
	if !shouldMark {
		t.Error("Should mark unhealthy after 3 consecutive failures")
	}

	if checker.GetConsecutiveFailures(b) != 3 {
		t.Errorf("Expected 3 consecutive failures, got %d", checker.GetConsecutiveFailures(b))
	}
}

func TestPassiveChecker_WindowFailures(t *testing.T) {
	checker := NewPassiveChecker(PassiveCheckerConfig{
		ConsecutiveFailures: 10, // High threshold so we test window logic
		MinRequests:         5,
		Window:              100 * time.Millisecond,
	})

	b := backend.NewBackend("test", "localhost:8080", 1)

	// Record failures
	for i := 0; i < 5; i++ {
		checker.RecordFailure(b)
	}

	// Should have 5 failures in window
	if checker.GetWindowFailures(b) != 5 {
		t.Errorf("Expected 5 window failures, got %d", checker.GetWindowFailures(b))
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Window failures should be cleared
	if checker.GetWindowFailures(b) != 0 {
		t.Errorf("Expected 0 window failures after window expiry, got %d", checker.GetWindowFailures(b))
	}
}

func TestPassiveChecker_Reset(t *testing.T) {
	checker := NewPassiveChecker(PassiveCheckerConfig{
		ConsecutiveFailures: 3,
	})

	b := backend.NewBackend("test", "localhost:8080", 1)

	// Record some failures
	checker.RecordFailure(b)
	checker.RecordFailure(b)

	if checker.GetConsecutiveFailures(b) != 2 {
		t.Error("Expected 2 consecutive failures")
	}

	// Reset
	checker.Reset(b)

	// Should be cleared
	if checker.GetConsecutiveFailures(b) != 0 {
		t.Error("Expected consecutive failures to be reset")
	}

	if checker.GetWindowFailures(b) != 0 {
		t.Error("Expected window failures to be reset")
	}
}

func TestPassiveChecker_ResetAll(t *testing.T) {
	checker := NewPassiveChecker(PassiveCheckerConfig{
		ConsecutiveFailures: 3,
	})

	b1 := backend.NewBackend("test1", "localhost:8080", 1)
	b2 := backend.NewBackend("test2", "localhost:8081", 1)

	// Record failures for both backends
	checker.RecordFailure(b1)
	checker.RecordFailure(b2)

	// Reset all
	checker.ResetAll()

	// Both should be cleared
	if checker.GetConsecutiveFailures(b1) != 0 {
		t.Error("Expected b1 consecutive failures to be reset")
	}

	if checker.GetConsecutiveFailures(b2) != 0 {
		t.Error("Expected b2 consecutive failures to be reset")
	}
}

func TestPassiveChecker_MultipleBackends(t *testing.T) {
	checker := NewPassiveChecker(PassiveCheckerConfig{
		ConsecutiveFailures: 2,
	})

	b1 := backend.NewBackend("test1", "localhost:8080", 1)
	b2 := backend.NewBackend("test2", "localhost:8081", 1)

	// Record failures for b1
	checker.RecordFailure(b1)
	checker.RecordFailure(b1)

	// Record success for b2
	checker.RecordSuccess(b2, 100*time.Millisecond)

	// Check b1 has failures
	if checker.GetConsecutiveFailures(b1) != 2 {
		t.Errorf("Expected b1 to have 2 failures, got %d", checker.GetConsecutiveFailures(b1))
	}

	// Check b2 has no failures
	if checker.GetConsecutiveFailures(b2) != 0 {
		t.Errorf("Expected b2 to have 0 failures, got %d", checker.GetConsecutiveFailures(b2))
	}
}

func TestPassiveChecker_MinRequests(t *testing.T) {
	checker := NewPassiveChecker(PassiveCheckerConfig{
		ConsecutiveFailures: 100, // Very high to not trigger
		MinRequests:         5,
		Window:              1 * time.Minute,
	})

	b := backend.NewBackend("test", "localhost:8080", 1)

	// Record failures below min requests
	for i := 0; i < 4; i++ {
		shouldMark := checker.RecordFailure(b)
		if shouldMark {
			t.Errorf("Should not mark unhealthy with only %d failures (min: 5)", i+1)
		}
	}

	// 5th failure should trigger
	shouldMark := checker.RecordFailure(b)
	if !shouldMark {
		t.Error("Should mark unhealthy after reaching min requests threshold")
	}
}

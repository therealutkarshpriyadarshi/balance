package backend

import (
	"testing"
	"time"
)

func TestStateMachine(t *testing.T) {
	backend := NewBackend("test", "localhost:8080", 1)
	sm := NewStateMachine(backend, 2, 3)

	// Initial state should be healthy
	if !sm.IsHealthy() {
		t.Error("Expected initial state to be healthy")
	}

	if sm.GetState() != StateHealthy {
		t.Errorf("Expected state to be StateHealthy, got %s", sm.GetState())
	}
}

func TestStateMachine_HealthyToUnhealthy(t *testing.T) {
	backend := NewBackend("test", "localhost:8080", 1)
	sm := NewStateMachine(backend, 2, 3)

	// Record failures
	sm.RecordFailure()
	if !sm.IsHealthy() {
		t.Error("Should still be healthy after 1 failure (threshold is 3)")
	}

	sm.RecordFailure()
	if !sm.IsHealthy() {
		t.Error("Should still be healthy after 2 failures")
	}

	sm.RecordFailure()
	if sm.IsHealthy() {
		t.Error("Should be unhealthy after 3 failures")
	}

	if sm.GetConsecutiveFailures() != 3 {
		t.Errorf("Expected 3 consecutive failures, got %d", sm.GetConsecutiveFailures())
	}
}

func TestStateMachine_UnhealthyToHealthy(t *testing.T) {
	backend := NewBackend("test", "localhost:8080", 1)
	sm := NewStateMachine(backend, 2, 3)

	// Make it unhealthy
	sm.RecordFailure()
	sm.RecordFailure()
	sm.RecordFailure()

	if sm.IsHealthy() {
		t.Error("Should be unhealthy")
	}

	// Record successes
	sm.RecordSuccess()
	if sm.IsHealthy() {
		t.Error("Should still be unhealthy after 1 success (threshold is 2)")
	}

	sm.RecordSuccess()
	if !sm.IsHealthy() {
		t.Error("Should be healthy after 2 consecutive successes")
	}

	if sm.GetConsecutiveSuccesses() != 2 {
		t.Errorf("Expected 2 consecutive successes, got %d", sm.GetConsecutiveSuccesses())
	}
}

func TestStateMachine_ResetOnSuccess(t *testing.T) {
	backend := NewBackend("test", "localhost:8080", 1)
	sm := NewStateMachine(backend, 2, 3)

	// Record some failures
	sm.RecordFailure()
	sm.RecordFailure()

	if sm.GetConsecutiveFailures() != 2 {
		t.Errorf("Expected 2 consecutive failures, got %d", sm.GetConsecutiveFailures())
	}

	// A success should reset consecutive failures
	sm.RecordSuccess()

	if sm.GetConsecutiveFailures() != 0 {
		t.Errorf("Expected consecutive failures to be reset, got %d", sm.GetConsecutiveFailures())
	}
}

func TestStateMachine_StateChangeListener(t *testing.T) {
	backend := NewBackend("test", "localhost:8080", 1)
	sm := NewStateMachine(backend, 2, 3)

	var notified bool
	var oldStateReceived, newStateReceived State

	sm.AddListener(func(b *Backend, oldState, newState State) {
		notified = true
		oldStateReceived = oldState
		newStateReceived = newState
	})

	// Trigger state change to unhealthy
	sm.RecordFailure()
	sm.RecordFailure()
	sm.RecordFailure()

	if !notified {
		t.Error("Expected listener to be notified")
	}

	if oldStateReceived != StateHealthy {
		t.Errorf("Expected old state to be StateHealthy, got %s", oldStateReceived)
	}

	if newStateReceived != StateUnhealthy {
		t.Errorf("Expected new state to be StateUnhealthy, got %s", newStateReceived)
	}
}

func TestStateMachine_Draining(t *testing.T) {
	backend := NewBackend("test", "localhost:8080", 1)
	sm := NewStateMachine(backend, 2, 3)

	sm.StartDraining()

	if !sm.IsDraining() {
		t.Error("Expected backend to be draining")
	}

	if sm.GetState() != StateDraining {
		t.Errorf("Expected state to be StateDraining, got %s", sm.GetState())
	}
}

func TestStateMachine_ForceStates(t *testing.T) {
	backend := NewBackend("test", "localhost:8080", 1)
	sm := NewStateMachine(backend, 2, 3)

	// Force unhealthy
	sm.ForceUnhealthy()
	if sm.IsHealthy() {
		t.Error("Expected backend to be unhealthy after ForceUnhealthy")
	}

	// Force healthy
	sm.ForceHealthy()
	if !sm.IsHealthy() {
		t.Error("Expected backend to be healthy after ForceHealthy")
	}
}

func TestStateMachine_RequestTracking(t *testing.T) {
	backend := NewBackend("test", "localhost:8080", 1)
	sm := NewStateMachine(backend, 2, 3)

	// Record some requests
	sm.RecordRequest(true, 100*time.Millisecond)
	sm.RecordRequest(true, 200*time.Millisecond)
	sm.RecordRequest(false, 0)

	if sm.GetTotalRequests() != 3 {
		t.Errorf("Expected 3 total requests, got %d", sm.GetTotalRequests())
	}

	if sm.GetFailedRequests() != 1 {
		t.Errorf("Expected 1 failed request, got %d", sm.GetFailedRequests())
	}

	errorRate := sm.GetErrorRate()
	expectedRate := 1.0 / 3.0
	if errorRate < expectedRate-0.01 || errorRate > expectedRate+0.01 {
		t.Errorf("Expected error rate ~%.2f, got %.2f", expectedRate, errorRate)
	}
}

func TestStateMachine_Metrics(t *testing.T) {
	backend := NewBackend("test", "localhost:8080", 1)
	sm := NewStateMachine(backend, 2, 3)

	// Record some activity
	sm.RecordSuccess()
	sm.RecordFailure()
	sm.RecordSuccess()

	metrics := sm.GetMetrics()

	if metrics.totalSuccesses.Load() != 2 {
		t.Errorf("Expected 2 total successes, got %d", metrics.totalSuccesses.Load())
	}

	if metrics.totalFailures.Load() != 1 {
		t.Errorf("Expected 1 total failure, got %d", metrics.totalFailures.Load())
	}

	// Check last check time is recent
	lastCheck := sm.GetLastCheckTime()
	if time.Since(lastCheck) > time.Second {
		t.Error("Last check time should be recent")
	}
}

func TestStateMachine_Reset(t *testing.T) {
	backend := NewBackend("test", "localhost:8080", 1)
	sm := NewStateMachine(backend, 2, 3)

	// Record some activity
	sm.RecordSuccess()
	sm.RecordFailure()
	sm.RecordRequest(true, 100*time.Millisecond)

	// Reset
	sm.Reset()

	// Check all metrics are reset
	if sm.GetConsecutiveSuccesses() != 0 {
		t.Error("Expected consecutive successes to be reset")
	}

	if sm.GetConsecutiveFailures() != 0 {
		t.Error("Expected consecutive failures to be reset")
	}

	if sm.GetTotalRequests() != 0 {
		t.Error("Expected total requests to be reset")
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateHealthy, "healthy"},
		{StateUnhealthy, "unhealthy"},
		{StateDraining, "draining"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.state.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.state.String())
			}
		})
	}
}

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package agent

import (
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker()
	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed, got %v", cb.State())
	}
	if !cb.AllowRequest() {
		t.Error("expected closed circuit to allow requests")
	}
	if cb.FailureCount() != 0 {
		t.Errorf("expected 0 failures, got %d", cb.FailureCount())
	}
}

func TestCircuitBreaker_RecordSuccess(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.RecordSuccess()
	if cb.TotalSuccesses() != 1 {
		t.Errorf("expected 1 success, got %d", cb.TotalSuccesses())
	}
	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed after success, got %v", cb.State())
	}
}

func TestCircuitBreaker_TripsOnConsecutiveFailures(t *testing.T) {
	cb := NewCircuitBreaker()

	for i := 0; i < DefaultFailureThreshold; i++ {
		cb.RecordFailure()
	}

	if cb.State() != CircuitOpen {
		t.Errorf("expected CircuitOpen after %d failures, got %v", DefaultFailureThreshold, cb.State())
	}
	if cb.AllowRequest() {
		t.Error("expected open circuit to reject requests")
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker()

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()

	if cb.FailureCount() != 0 {
		t.Errorf("expected 0 failures after success, got %d", cb.FailureCount())
	}
	if cb.State() != CircuitClosed {
		t.Error("expected CircuitClosed after success reset")
	}
}

func TestCircuitBreaker_DoesNotTripOnIntermittentFailures(t *testing.T) {
	cb := NewCircuitBreaker()

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // resets count
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitClosed {
		t.Error("expected CircuitClosed with intermittent failures")
	}
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	cb := NewCircuitBreakerWithConfig(3, 50*time.Millisecond)

	// Trip the circuit.
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.State() != CircuitOpen {
		t.Fatal("expected CircuitOpen")
	}

	// Wait for recovery.
	time.Sleep(60 * time.Millisecond)

	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected CircuitHalfOpen after recovery timeout, got %v", cb.State())
	}
	if !cb.AllowRequest() {
		t.Error("expected half-open circuit to allow probe request")
	}
}

func TestCircuitBreaker_HalfOpenToClosedOnSuccess(t *testing.T) {
	cb := NewCircuitBreakerWithConfig(3, 50*time.Millisecond)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	time.Sleep(60 * time.Millisecond)

	// Should be half-open now.
	if cb.State() != CircuitHalfOpen {
		t.Fatal("expected CircuitHalfOpen")
	}

	// Success in half-open state closes the circuit.
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed after success in half-open, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToOpenOnFailure(t *testing.T) {
	cb := NewCircuitBreakerWithConfig(1, 50*time.Millisecond)

	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatal("expected CircuitOpen")
	}

	time.Sleep(60 * time.Millisecond)
	if cb.State() != CircuitHalfOpen {
		t.Fatal("expected CircuitHalfOpen")
	}

	// Failure in half-open re-opens the circuit.
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("expected CircuitOpen after failure in half-open, got %v", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker()

	for i := 0; i < DefaultFailureThreshold; i++ {
		cb.RecordFailure()
	}

	cb.Reset()
	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed after reset, got %v", cb.State())
	}
	if cb.FailureCount() != 0 {
		t.Errorf("expected 0 failures after reset, got %d", cb.FailureCount())
	}
}

func TestCircuitBreaker_CustomConfig(t *testing.T) {
	cb := NewCircuitBreakerWithConfig(5, 30*time.Second)

	// Should need 5 failures.
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	if cb.State() != CircuitClosed {
		t.Error("expected CircuitClosed with 4/5 failures")
	}

	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Error("expected CircuitOpen with 5/5 failures")
	}
}

func TestCircuitBreaker_InvalidConfig(t *testing.T) {
	cb := NewCircuitBreakerWithConfig(0, 0) // should use defaults
	if cb.State() != CircuitClosed {
		t.Error("expected CircuitClosed for invalid config")
	}

	for i := 0; i < DefaultFailureThreshold; i++ {
		cb.RecordFailure()
	}
	if cb.State() != CircuitOpen {
		t.Error("expected default threshold to apply")
	}
}

func TestCircuitBreaker_TotalCounters(t *testing.T) {
	cb := NewCircuitBreaker()

	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()

	if cb.TotalSuccesses() != 3 {
		t.Errorf("expected 3 total successes, got %d", cb.TotalSuccesses())
	}
	if cb.TotalFailures() != 2 {
		t.Errorf("expected 2 total failures, got %d", cb.TotalFailures())
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

// --- HealthMonitor Tests ---

func TestHealthMonitor_NewHealthMonitor(t *testing.T) {
	hm := NewHealthMonitor()
	if hm == nil {
		t.Fatal("NewHealthMonitor returned nil")
	}
}

func TestHealthMonitor_IsHealthy_NewAgent(t *testing.T) {
	hm := NewHealthMonitor()
	// New agents should be healthy (circuit closed).
	if !hm.IsHealthy("agent-1") {
		t.Error("new agent should be healthy")
	}
}

func TestHealthMonitor_RecordFailures_MakesUnhealthy(t *testing.T) {
	hm := NewHealthMonitor()

	for i := 0; i < DefaultFailureThreshold; i++ {
		hm.RecordFailure("agent-1")
	}

	if hm.IsHealthy("agent-1") {
		t.Error("agent should be unhealthy after failures")
	}
}

func TestHealthMonitor_RecordSuccess_MakesHealthy(t *testing.T) {
	hm := NewHealthMonitor()

	hm.RecordFailure("agent-1")
	hm.RecordFailure("agent-1")
	hm.RecordSuccess("agent-1")

	if !hm.IsHealthy("agent-1") {
		t.Error("agent should be healthy after success")
	}
}

func TestHealthMonitor_IndependentBreakers(t *testing.T) {
	hm := NewHealthMonitor()

	for i := 0; i < DefaultFailureThreshold; i++ {
		hm.RecordFailure("agent-1")
	}

	if hm.IsHealthy("agent-1") {
		t.Error("agent-1 should be unhealthy")
	}
	if !hm.IsHealthy("agent-2") {
		t.Error("agent-2 should still be healthy")
	}
}

func TestHealthMonitor_AllStatuses(t *testing.T) {
	hm := NewHealthMonitor()

	hm.RecordSuccess("agent-1")
	for i := 0; i < DefaultFailureThreshold; i++ {
		hm.RecordFailure("agent-2")
	}

	statuses := hm.AllStatuses()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if statuses["agent-1"] != CircuitClosed {
		t.Errorf("expected agent-1 closed, got %v", statuses["agent-1"])
	}
	if statuses["agent-2"] != CircuitOpen {
		t.Errorf("expected agent-2 open, got %v", statuses["agent-2"])
	}
}

func TestHealthMonitor_Reset(t *testing.T) {
	hm := NewHealthMonitor()

	for i := 0; i < DefaultFailureThreshold; i++ {
		hm.RecordFailure("agent-1")
	}

	hm.Reset("agent-1")
	if !hm.IsHealthy("agent-1") {
		t.Error("agent should be healthy after reset")
	}
}

func TestHealthMonitor_ResetAll(t *testing.T) {
	hm := NewHealthMonitor()

	for i := 0; i < DefaultFailureThreshold; i++ {
		hm.RecordFailure("agent-1")
		hm.RecordFailure("agent-2")
	}

	hm.ResetAll()

	if !hm.IsHealthy("agent-1") {
		t.Error("agent-1 should be healthy after reset all")
	}
	if !hm.IsHealthy("agent-2") {
		t.Error("agent-2 should be healthy after reset all")
	}
}

func TestHealthMonitor_GetBreaker_CreateOnDemand(t *testing.T) {
	hm := NewHealthMonitor()
	cb := hm.GetBreaker("new-agent")
	if cb == nil {
		t.Fatal("GetBreaker returned nil")
	}
	if cb.State() != CircuitClosed {
		t.Error("new breaker should be closed")
	}
}

func TestHealthMonitor_GetBreaker_ReturnsSame(t *testing.T) {
	hm := NewHealthMonitor()
	cb1 := hm.GetBreaker("agent-1")
	cb2 := hm.GetBreaker("agent-1")
	if cb1 != cb2 {
		t.Error("GetBreaker should return the same instance for same agent ID")
	}
}

// --- Stress Tests ---

func TestCircuitBreaker_Stress_ConcurrentRecording(t *testing.T) {
	cb := NewCircuitBreaker()
	var wg sync.WaitGroup
	const workers = 50
	const iterations = 100

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if i%3 == 0 {
					cb.RecordFailure()
				} else {
					cb.RecordSuccess()
				}
				_ = cb.State()
				_ = cb.AllowRequest()
			}
		}(w)
	}

	wg.Wait()
	// Should not panic or deadlock.
}

func TestHealthMonitor_Stress_ConcurrentOperations(t *testing.T) {
	hm := NewHealthMonitor()
	var wg sync.WaitGroup
	const workers = 30
	const iterations = 100

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			agentID := "agent-" + string(rune('A'+id%5))
			for i := 0; i < iterations; i++ {
				switch i % 4 {
				case 0:
					hm.RecordSuccess(agentID)
				case 1:
					hm.RecordFailure(agentID)
				case 2:
					_ = hm.IsHealthy(agentID)
				case 3:
					_ = hm.AllStatuses()
				}
			}
		}(w)
	}

	wg.Wait()
}

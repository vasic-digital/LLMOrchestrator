// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package agent

import (
	"sync"
	"time"
)

const (
	// DefaultFailureThreshold is the number of consecutive failures before
	// an agent is marked unhealthy by the circuit breaker.
	DefaultFailureThreshold = 3

	// DefaultRecoveryTimeout is how long the circuit stays open before
	// allowing a health probe.
	DefaultRecoveryTimeout = 60 * time.Second
)

// CircuitState represents the state of the circuit breaker.
type CircuitState int

const (
	// CircuitClosed means the agent is healthy and accepting requests.
	CircuitClosed CircuitState = iota
	// CircuitOpen means the agent has failed too many times and is temporarily blocked.
	CircuitOpen
	// CircuitHalfOpen means the agent is being probed for recovery.
	CircuitHalfOpen
)

// String returns the string representation of the circuit state.
func (cs CircuitState) String() string {
	switch cs {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern for agents.
// After N consecutive failures, the circuit opens and rejects requests
// for a recovery period before allowing a probe.
type CircuitBreaker struct {
	mu               sync.Mutex
	failureCount     int
	failureThreshold int
	state            CircuitState
	lastFailure      time.Time
	recoveryTimeout  time.Duration
	lastStateChange  time.Time
	totalFailures    int
	totalSuccesses   int
}

// NewCircuitBreaker creates a circuit breaker with default settings.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: DefaultFailureThreshold,
		state:            CircuitClosed,
		recoveryTimeout:  DefaultRecoveryTimeout,
	}
}

// NewCircuitBreakerWithConfig creates a circuit breaker with custom settings.
func NewCircuitBreakerWithConfig(failureThreshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = DefaultFailureThreshold
	}
	if recoveryTimeout <= 0 {
		recoveryTimeout = DefaultRecoveryTimeout
	}
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		state:            CircuitClosed,
		recoveryTimeout:  recoveryTimeout,
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.checkRecovery()
	return cb.state
}

// AllowRequest returns true if the circuit allows a request through.
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.checkRecovery()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitHalfOpen:
		return true // allow probe request
	case CircuitOpen:
		return false
	default:
		return false
	}
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.totalSuccesses++
	cb.failureCount = 0
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
		cb.lastStateChange = time.Now()
	}
}

// RecordFailure records a failed operation.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.totalFailures++
	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.failureCount >= cb.failureThreshold && cb.state != CircuitOpen {
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
	}
}

// Reset returns the circuit to its initial closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.state = CircuitClosed
	cb.lastStateChange = time.Now()
}

// FailureCount returns the current consecutive failure count.
func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failureCount
}

// TotalFailures returns the total number of recorded failures.
func (cb *CircuitBreaker) TotalFailures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.totalFailures
}

// TotalSuccesses returns the total number of recorded successes.
func (cb *CircuitBreaker) TotalSuccesses() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.totalSuccesses
}

// checkRecovery transitions from Open to HalfOpen if recovery timeout has elapsed.
// Must be called with cb.mu held.
func (cb *CircuitBreaker) checkRecovery() {
	if cb.state == CircuitOpen && time.Since(cb.lastFailure) >= cb.recoveryTimeout {
		cb.state = CircuitHalfOpen
		cb.lastStateChange = time.Now()
	}
}

// HealthMonitor tracks agent health across the pool using circuit breakers.
type HealthMonitor struct {
	mu       sync.Mutex
	breakers map[string]*CircuitBreaker // agentID -> breaker
}

// NewHealthMonitor creates a new HealthMonitor.
func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetBreaker returns the circuit breaker for an agent, creating one if needed.
func (hm *HealthMonitor) GetBreaker(agentID string) *CircuitBreaker {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	cb, exists := hm.breakers[agentID]
	if !exists {
		cb = NewCircuitBreaker()
		hm.breakers[agentID] = cb
	}
	return cb
}

// IsHealthy returns true if the agent's circuit is not open.
func (hm *HealthMonitor) IsHealthy(agentID string) bool {
	return hm.GetBreaker(agentID).AllowRequest()
}

// RecordSuccess records a successful operation for an agent.
func (hm *HealthMonitor) RecordSuccess(agentID string) {
	hm.GetBreaker(agentID).RecordSuccess()
}

// RecordFailure records a failed operation for an agent.
func (hm *HealthMonitor) RecordFailure(agentID string) {
	hm.GetBreaker(agentID).RecordFailure()
}

// AllStatuses returns the circuit state for all monitored agents.
func (hm *HealthMonitor) AllStatuses() map[string]CircuitState {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	statuses := make(map[string]CircuitState, len(hm.breakers))
	for id, cb := range hm.breakers {
		statuses[id] = cb.State()
	}
	return statuses
}

// Reset resets the circuit breaker for a specific agent.
func (hm *HealthMonitor) Reset(agentID string) {
	hm.GetBreaker(agentID).Reset()
}

// ResetAll resets all circuit breakers.
func (hm *HealthMonitor) ResetAll() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	for _, cb := range hm.breakers {
		cb.Reset()
	}
}

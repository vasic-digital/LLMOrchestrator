// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MultiProviderPool manages agents from multiple CLI providers (OpenCode, Claude, Gemini, Junie, Qwen)
type MultiProviderPool struct {
	pools    map[string]AgentPool
	selector AgentSelector
	mu       sync.RWMutex
}

// AgentSelector chooses the best provider for a given request
type AgentSelector interface {
	Select(pools map[string]AgentPool, req AgentRequirements) string
}

// NewMultiProviderPool creates a multi-provider pool.
//
// Every provider-specific factory (NewOpenCodePool / NewClaudeCodePool /
// NewGeminiPool / NewJuniePool / NewQwenCodePool) currently returns
// ErrProviderPoolNotImplemented per round-28 §11.4 audit (CONTRACT-bluff
// removal). Until each factory is replaced with a real provider-specific
// implementation, NewMultiProviderPool propagates that sentinel error
// rather than silently constructing a pool whose Acquire() would return
// nil agents that panic on first use.
//
// Callers MAY still construct a MultiProviderPool directly with externally
// provided AgentPool implementations (e.g., installed via dependency
// injection in tests) — see the test helpers in pkg/agent/*_test.go.
func NewMultiProviderPool(configs map[string]*PoolConfig) (*MultiProviderPool, error) {
	pools := make(map[string]AgentPool)

	for provider, cfg := range configs {
		var (
			p   AgentPool
			err error
		)
		switch provider {
		case "opencode":
			p, err = NewOpenCodePool(cfg)
		case "claude-code":
			p, err = NewClaudeCodePool(cfg)
		case "gemini":
			p, err = NewGeminiPool(cfg)
		case "junie":
			p, err = NewJuniePool(cfg)
		case "qwen-code":
			p, err = NewQwenCodePool(cfg)
		default:
			return nil, fmt.Errorf("unknown provider: %s", provider)
		}
		if err != nil {
			return nil, fmt.Errorf("provider %q: %w", provider, err)
		}
		pools[provider] = p
	}

	return &MultiProviderPool{
		pools:    pools,
		selector: NewRoundRobinSelector(),
	}, nil
}

// Acquire gets an agent from the best available pool
func (m *MultiProviderPool) Acquire(ctx context.Context, req AgentRequirements) (Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find best pool based on requirements
	selected := m.selector.Select(m.pools, req)
	if selected == "" {
		return nil, ErrNoSuitableAgent
	}

	return m.pools[selected].Acquire(ctx, req)
}

// Release returns an agent to its pool
func (m *MultiProviderPool) Release(agent Agent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find which pool this agent belongs to and release it
	for _, pool := range m.pools {
		pool.Release(agent)
	}
}

// Available returns all available agents across all pools
func (m *MultiProviderPool) Available() []Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	available := make([]Agent, 0)
	for _, pool := range m.pools {
		available = append(available, pool.Available()...)
	}
	return available
}

// HealthCheck checks health of all pools
func (m *MultiProviderPool) HealthCheck(ctx context.Context) []HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]HealthStatus, 0)
	for _, pool := range m.pools {
		statuses = append(statuses, pool.HealthCheck(ctx)...)
	}
	return statuses
}

// Shutdown gracefully shuts down all pools
func (m *MultiProviderPool) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, pool := range m.pools {
		if err := pool.Shutdown(ctx); err != nil {
			lastErr = fmt.Errorf("shutdown %s: %w", name, err)
		}
	}
	return lastErr
}

// RoundRobinSelector implements round-robin selection across providers
type RoundRobinSelector struct {
	mu        sync.Mutex
	counter   int
	providers []string
}

// NewRoundRobinSelector creates a round-robin selector
func NewRoundRobinSelector() *RoundRobinSelector {
	return &RoundRobinSelector{
		providers: make([]string, 0),
	}
}

// Select chooses the next provider in round-robin fashion
func (r *RoundRobinSelector) Select(pools map[string]AgentPool, req AgentRequirements) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update providers list
	r.providers = make([]string, 0, len(pools))
	for name := range pools {
		r.providers = append(r.providers, name)
	}

	if len(r.providers) == 0 {
		return ""
	}

	// Try to find a provider that can meet requirements
	for i := 0; i < len(r.providers); i++ {
		idx := (r.counter + i) % len(r.providers)
		provider := r.providers[idx]

		// Check if this provider has available agents that meet requirements
		if pool, ok := pools[provider]; ok {
			available := pool.Available()
			for _, agent := range available {
				if meetsRequirements(agent, req) {
					r.counter = (r.counter + 1) % len(r.providers)
					return provider
				}
			}
		}
	}

	// Fallback: just return the first provider with any available agent
	for _, provider := range r.providers {
		if pool, ok := pools[provider]; ok {
			if len(pool.Available()) > 0 {
				return provider
			}
		}
	}

	return ""
}

func meetsRequirements(agent Agent, req AgentRequirements) bool {
	caps := agent.Capabilities()

	if req.NeedsVision && !caps.Vision {
		return false
	}

	if req.NeedsStreaming && !caps.Streaming {
		return false
	}

	return true
}

// PreferenceSelector selects based on user preferences
type PreferenceSelector struct {
	preferredOrder []string
}

// NewPreferenceSelector creates a selector with provider preferences
func NewPreferenceSelector(preferences []string) *PreferenceSelector {
	return &PreferenceSelector{
		preferredOrder: preferences,
	}
}

// Select chooses the first available preferred provider
func (p *PreferenceSelector) Select(pools map[string]AgentPool, req AgentRequirements) string {
	for _, preferred := range p.preferredOrder {
		if pool, ok := pools[preferred]; ok {
			available := pool.Available()
			for _, agent := range available {
				if meetsRequirements(agent, req) {
					return preferred
				}
			}
		}
	}

	// Fallback to any available provider
	for name := range pools {
		return name
	}

	return ""
}

// PoolConfig holds configuration for creating a provider pool
type PoolConfig struct {
	Size       int
	Timeout    time.Duration
	MaxRetries int
	BinaryPath string
	Provider   string
	Model      string
	APIKey     string
}

// ErrProviderPoolNotImplemented is returned by every provider-specific
// pool factory (NewOpenCodePool / NewClaudeCodePool / NewGeminiPool /
// NewJuniePool / NewQwenCodePool) until each one is replaced with a real
// provider-specific implementation that spawns and manages live CLI
// agents.
//
// Round-28 §11.4 audit (HelixCode close-out, 2026-05-17): the previous
// factory bodies returned a MockPool seeded with `nil` agent slots
// (`agents[i] = nil // Placeholder`). MultiProviderPool.Acquire() would
// happily return one of those nil agents to a caller, which then panicked
// on the first method call. The test suite passed because no test
// exercised the round-trip — a CRITICAL CONTRACT-bluff at the
// multi-provider-pool layer.
//
// Fix: every factory now surfaces this sentinel error so callers (and
// MultiProviderPool itself) FAIL LOUDLY when a provider-specific
// implementation is missing, instead of silently returning a pool of
// nil-agent panic-traps. MockPool has been moved to *_test.go per
// CONST-050(A) (no fakes outside unit tests). Tests that require a
// deterministic stand-in install one via dependency injection in
// the test file.
//
// Constitutional anchors: CONST-035 (anti-bluff), CONST-050(A)
// (no-fakes-beyond-unit-tests), Article XI §11.9 (forensic anchor).
var ErrProviderPoolNotImplemented = fmt.Errorf("llmorchestrator: provider-specific pool factory has not been implemented — NewOpenCodePool/NewClaudeCodePool/NewGeminiPool/NewJuniePool/NewQwenCodePool currently return a sentinel error to avoid the previous MockPool-with-nil-agents CONTRACT-bluff (every Acquire would have returned a nil agent that panicked on use); §11.4 PASS-bluff removed")

// NewOpenCodePool returns ErrProviderPoolNotImplemented until a real
// OpenCode-specific pool (spawning + supervising live `opencode`
// CLI agents) is wired in. See ErrProviderPoolNotImplemented for the
// round-28 §11.4 forensic anchor.
func NewOpenCodePool(_ *PoolConfig) (AgentPool, error) {
	return nil, fmt.Errorf("opencode: %w", ErrProviderPoolNotImplemented)
}

// NewClaudeCodePool returns ErrProviderPoolNotImplemented until a real
// Claude-Code-specific pool is wired in.
func NewClaudeCodePool(_ *PoolConfig) (AgentPool, error) {
	return nil, fmt.Errorf("claude-code: %w", ErrProviderPoolNotImplemented)
}

// NewGeminiPool returns ErrProviderPoolNotImplemented until a real
// Gemini-specific pool is wired in.
func NewGeminiPool(_ *PoolConfig) (AgentPool, error) {
	return nil, fmt.Errorf("gemini: %w", ErrProviderPoolNotImplemented)
}

// NewJuniePool returns ErrProviderPoolNotImplemented until a real
// Junie-specific pool is wired in.
func NewJuniePool(_ *PoolConfig) (AgentPool, error) {
	return nil, fmt.Errorf("junie: %w", ErrProviderPoolNotImplemented)
}

// NewQwenCodePool returns ErrProviderPoolNotImplemented until a real
// Qwen-Code-specific pool is wired in.
func NewQwenCodePool(_ *PoolConfig) (AgentPool, error) {
	return nil, fmt.Errorf("qwen-code: %w", ErrProviderPoolNotImplemented)
}

var (
	ErrNoSuitableAgent   = fmt.Errorf("no suitable agent available")
	ErrNoAgentsAvailable = fmt.Errorf("no agents available in pool")
)

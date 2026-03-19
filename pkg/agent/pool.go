// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package agent

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrAgentAlreadyRegistered is returned when registering an agent with a duplicate ID.
	ErrAgentAlreadyRegistered = errors.New("agent already registered")
	// ErrNoAvailableAgent is returned when no agent matches requirements.
	ErrNoAvailableAgent = errors.New("no available agent matching requirements")
	// ErrPoolShutdown is returned when operations are attempted on a shutdown pool.
	ErrPoolShutdown = errors.New("agent pool is shut down")
	// ErrAgentNotFound is returned when releasing an agent not in the pool.
	ErrAgentNotFound = errors.New("agent not found in pool")
)

// AgentPool is a thread-safe agent pool with capability matching.
type AgentPool interface {
	// Register adds an agent to the pool.
	Register(agent Agent) error
	// Acquire obtains an agent matching requirements, blocking until one is available.
	Acquire(ctx context.Context, requirements AgentRequirements) (Agent, error)
	// Release returns an agent to the pool.
	Release(agent Agent)
	// Available returns all agents currently available (not acquired).
	Available() []Agent
	// HealthCheck runs health checks on all registered agents.
	HealthCheck(ctx context.Context) []HealthStatus
	// Shutdown gracefully stops all agents and shuts down the pool.
	Shutdown(ctx context.Context) error
}

// agentEntry tracks an agent's state within the pool.
type agentEntry struct {
	agent    Agent
	acquired bool
}

// pool is the default AgentPool implementation.
type pool struct {
	mu       sync.Mutex
	cond     *sync.Cond
	agents   map[string]*agentEntry
	shutdown bool
}

// NewPool creates a new thread-safe AgentPool.
func NewPool() AgentPool {
	p := &pool{
		agents: make(map[string]*agentEntry),
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// Register adds an agent to the pool.
func (p *pool) Register(agent Agent) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.shutdown {
		return ErrPoolShutdown
	}

	if _, exists := p.agents[agent.ID()]; exists {
		return ErrAgentAlreadyRegistered
	}

	p.agents[agent.ID()] = &agentEntry{
		agent:    agent,
		acquired: false,
	}

	// Signal any waiters that a new agent is available.
	p.cond.Broadcast()
	return nil
}

// Acquire obtains an agent matching requirements. Blocks until one is available
// or the context is cancelled.
func (p *pool) Acquire(ctx context.Context, requirements AgentRequirements) (Agent, error) {
	// Start a goroutine to broadcast on context cancellation so waiters wake up.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			p.cond.Broadcast()
		case <-done:
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		if p.shutdown {
			return nil, ErrPoolShutdown
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Try to find a matching available agent.
		agent := p.findAvailable(requirements)
		if agent != nil {
			p.agents[agent.ID()].acquired = true
			return agent, nil
		}

		// No matching agent available; wait for one.
		p.cond.Wait()
	}
}

// findAvailable returns a matching available agent, or nil if none found.
// Must be called with p.mu held.
func (p *pool) findAvailable(req AgentRequirements) Agent {
	// First pass: check preferred agent.
	if req.PreferredAgent != "" {
		for _, entry := range p.agents {
			if entry.acquired {
				continue
			}
			if entry.agent.Name() == req.PreferredAgent && p.meetsRequirements(entry.agent, req) {
				return entry.agent
			}
		}
	}

	// Second pass: any matching agent.
	for _, entry := range p.agents {
		if entry.acquired {
			continue
		}
		if p.meetsRequirements(entry.agent, req) {
			return entry.agent
		}
	}

	return nil
}

// meetsRequirements checks if an agent satisfies the given requirements.
func (p *pool) meetsRequirements(agent Agent, req AgentRequirements) bool {
	caps := agent.Capabilities()

	if req.NeedsVision && !caps.Vision {
		return false
	}
	if req.NeedsStreaming && !caps.Streaming {
		return false
	}
	if req.MinTokens > 0 && caps.MaxTokens < req.MinTokens {
		return false
	}

	return true
}

// Release returns an agent to the pool.
func (p *pool) Release(agent Agent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, exists := p.agents[agent.ID()]; exists {
		entry.acquired = false
		p.cond.Broadcast()
	}
}

// Available returns all agents currently not acquired.
func (p *pool) Available() []Agent {
	p.mu.Lock()
	defer p.mu.Unlock()

	var available []Agent
	for _, entry := range p.agents {
		if !entry.acquired {
			available = append(available, entry.agent)
		}
	}
	return available
}

// HealthCheck runs health checks on all registered agents.
func (p *pool) HealthCheck(ctx context.Context) []HealthStatus {
	p.mu.Lock()
	agents := make([]Agent, 0, len(p.agents))
	for _, entry := range p.agents {
		agents = append(agents, entry.agent)
	}
	p.mu.Unlock()

	statuses := make([]HealthStatus, len(agents))
	var wg sync.WaitGroup
	for i, a := range agents {
		wg.Add(1)
		go func(idx int, ag Agent) {
			defer wg.Done()
			statuses[idx] = ag.Health(ctx)
		}(i, a)
	}
	wg.Wait()
	return statuses
}

// Shutdown gracefully stops all agents and marks the pool as shut down.
func (p *pool) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	p.shutdown = true
	agents := make([]Agent, 0, len(p.agents))
	for _, entry := range p.agents {
		agents = append(agents, entry.agent)
	}
	p.cond.Broadcast()
	p.mu.Unlock()

	var errs []error
	for _, a := range agents {
		if a.IsRunning() {
			if err := a.Stop(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

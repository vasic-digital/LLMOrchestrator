// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPool_NewPool(t *testing.T) {
	p := NewPool()
	if p == nil {
		t.Fatal("NewPool returned nil")
	}
	if len(p.Available()) != 0 {
		t.Errorf("expected 0 available agents, got %d", len(p.Available()))
	}
}

func TestPool_Register(t *testing.T) {
	p := NewPool()
	agent := newMockAgent("agent-1", "test")

	err := p.Register(agent)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	available := p.Available()
	if len(available) != 1 {
		t.Fatalf("expected 1 available, got %d", len(available))
	}
	if available[0].ID() != "agent-1" {
		t.Errorf("unexpected agent ID: %s", available[0].ID())
	}
}

func TestPool_RegisterDuplicate(t *testing.T) {
	p := NewPool()
	agent := newMockAgent("agent-1", "test")

	_ = p.Register(agent)
	err := p.Register(agent)
	if !errors.Is(err, ErrAgentAlreadyRegistered) {
		t.Errorf("expected ErrAgentAlreadyRegistered, got: %v", err)
	}
}

func TestPool_RegisterMultiple(t *testing.T) {
	p := NewPool()
	for i := 0; i < 5; i++ {
		agent := newMockAgent("agent-"+string(rune('0'+i)), "test")
		if err := p.Register(agent); err != nil {
			t.Fatalf("Register %d failed: %v", i, err)
		}
	}
	if len(p.Available()) != 5 {
		t.Errorf("expected 5 available, got %d", len(p.Available()))
	}
}

func TestPool_Acquire_Simple(t *testing.T) {
	p := NewPool()
	agent := newMockAgent("agent-1", "test")
	_ = p.Register(agent)

	ctx := context.Background()
	acquired, err := p.Acquire(ctx, AgentRequirements{})
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if acquired.ID() != "agent-1" {
		t.Errorf("unexpected agent: %s", acquired.ID())
	}

	// After acquire, no agents should be available.
	if len(p.Available()) != 0 {
		t.Errorf("expected 0 available after acquire, got %d", len(p.Available()))
	}
}

func TestPool_Acquire_WithVisionRequirement(t *testing.T) {
	p := NewPool()

	noVision := newMockAgent("no-vision", "basic")
	noVision.caps.Vision = false
	_ = p.Register(noVision)

	withVision := newMockAgent("with-vision", "advanced")
	withVision.caps.Vision = true
	_ = p.Register(withVision)

	ctx := context.Background()
	acquired, err := p.Acquire(ctx, AgentRequirements{NeedsVision: true})
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if acquired.ID() != "with-vision" {
		t.Errorf("expected with-vision agent, got %s", acquired.ID())
	}
}

func TestPool_Acquire_WithStreamingRequirement(t *testing.T) {
	p := NewPool()

	noStreaming := newMockAgent("no-stream", "basic")
	noStreaming.caps.Streaming = false
	_ = p.Register(noStreaming)

	withStreaming := newMockAgent("with-stream", "advanced")
	withStreaming.caps.Streaming = true
	_ = p.Register(withStreaming)

	ctx := context.Background()
	acquired, err := p.Acquire(ctx, AgentRequirements{NeedsStreaming: true})
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if acquired.ID() != "with-stream" {
		t.Errorf("expected with-stream agent, got %s", acquired.ID())
	}
}

func TestPool_Acquire_WithMinTokens(t *testing.T) {
	p := NewPool()

	small := newMockAgent("small", "basic")
	small.caps.MaxTokens = 4000
	_ = p.Register(small)

	large := newMockAgent("large", "advanced")
	large.caps.MaxTokens = 200000
	_ = p.Register(large)

	ctx := context.Background()
	acquired, err := p.Acquire(ctx, AgentRequirements{MinTokens: 100000})
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if acquired.ID() != "large" {
		t.Errorf("expected large agent, got %s", acquired.ID())
	}
}

func TestPool_Acquire_PreferredAgent(t *testing.T) {
	p := NewPool()

	a1 := newMockAgent("agent-1", "opencode")
	_ = p.Register(a1)
	a2 := newMockAgent("agent-2", "claude-code")
	_ = p.Register(a2)

	ctx := context.Background()
	acquired, err := p.Acquire(ctx, AgentRequirements{PreferredAgent: "claude-code"})
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if acquired.Name() != "claude-code" {
		t.Errorf("expected claude-code, got %s", acquired.Name())
	}
}

func TestPool_Acquire_ContextCancellation(t *testing.T) {
	p := NewPool()
	// No agents registered, acquire should block until context cancelled.

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := p.Acquire(ctx, AgentRequirements{})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestPool_Release(t *testing.T) {
	p := NewPool()
	agent := newMockAgent("agent-1", "test")
	_ = p.Register(agent)

	ctx := context.Background()
	acquired, _ := p.Acquire(ctx, AgentRequirements{})

	if len(p.Available()) != 0 {
		t.Error("expected 0 available after acquire")
	}

	p.Release(acquired)

	if len(p.Available()) != 1 {
		t.Errorf("expected 1 available after release, got %d", len(p.Available()))
	}
}

func TestPool_Acquire_BlocksUntilRelease(t *testing.T) {
	p := NewPool()
	agent := newMockAgent("agent-1", "test")
	_ = p.Register(agent)

	ctx := context.Background()
	acquired, _ := p.Acquire(ctx, AgentRequirements{})

	// Start goroutine to release after a delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.Release(acquired)
	}()

	ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	reAcquired, err := p.Acquire(ctx2, AgentRequirements{})
	if err != nil {
		t.Fatalf("re-acquire failed: %v", err)
	}
	if reAcquired.ID() != "agent-1" {
		t.Errorf("unexpected agent: %s", reAcquired.ID())
	}
}

func TestPool_HealthCheck(t *testing.T) {
	p := NewPool()
	a1 := newMockAgent("agent-1", "test")
	a1.running = true
	a2 := newMockAgent("agent-2", "test")
	a2.running = false

	_ = p.Register(a1)
	_ = p.Register(a2)

	ctx := context.Background()
	statuses := p.HealthCheck(ctx)

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	healthyCount := 0
	for _, s := range statuses {
		if s.Healthy {
			healthyCount++
		}
	}
	if healthyCount != 1 {
		t.Errorf("expected 1 healthy agent, got %d", healthyCount)
	}
}

func TestPool_Shutdown(t *testing.T) {
	p := NewPool()
	a1 := newMockAgent("agent-1", "test")
	a1.running = true
	_ = p.Register(a1)

	ctx := context.Background()
	err := p.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Pool should reject new operations.
	err = p.Register(newMockAgent("new", "test"))
	if !errors.Is(err, ErrPoolShutdown) {
		t.Errorf("expected ErrPoolShutdown after shutdown, got: %v", err)
	}

	_, err = p.Acquire(ctx, AgentRequirements{})
	if !errors.Is(err, ErrPoolShutdown) {
		t.Errorf("expected ErrPoolShutdown on acquire after shutdown, got: %v", err)
	}
}

func TestPool_Shutdown_StopsRunningAgents(t *testing.T) {
	p := NewPool()
	a := newMockAgent("agent-1", "test")
	a.running = true
	_ = p.Register(a)

	ctx := context.Background()
	_ = p.Shutdown(ctx)

	if a.IsRunning() {
		t.Error("expected agent to be stopped after shutdown")
	}
}

func TestPool_Available_Empty(t *testing.T) {
	p := NewPool()
	available := p.Available()
	if available != nil && len(available) != 0 {
		t.Errorf("expected empty available list, got %d", len(available))
	}
}

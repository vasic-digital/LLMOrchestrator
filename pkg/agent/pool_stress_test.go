// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_Stress_ConcurrentAcquireRelease(t *testing.T) {
	p := NewPool()
	const numAgents = 5
	const numWorkers = 20
	const iterations = 50

	for i := 0; i < numAgents; i++ {
		a := newMockAgent("agent-"+string(rune('A'+i)), "test")
		if err := p.Register(a); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	var wg sync.WaitGroup
	var acquired int64

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				a, err := p.Acquire(ctx, AgentRequirements{})
				cancel()
				if err != nil {
					continue
				}
				atomic.AddInt64(&acquired, 1)
				// Simulate work.
				time.Sleep(time.Microsecond * 10)
				p.Release(a)
			}
		}()
	}

	wg.Wait()

	if acquired == 0 {
		t.Error("expected at least some successful acquisitions")
	}

	// All agents should be available after all releases.
	if len(p.Available()) != numAgents {
		t.Errorf("expected %d available after stress test, got %d", numAgents, len(p.Available()))
	}
}

func TestPool_Stress_ConcurrentRegister(t *testing.T) {
	p := NewPool()
	const numGoroutines = 50

	var wg sync.WaitGroup
	var registered int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			a := newMockAgent("concurrent-"+string(rune('A'+idx%26))+"-"+time.Now().String(), "test")
			if err := p.Register(a); err == nil {
				atomic.AddInt64(&registered, 1)
			}
		}(i)
	}

	wg.Wait()

	if registered == 0 {
		t.Error("expected at least some successful registrations")
	}
}

func TestPool_Stress_ConcurrentHealthCheck(t *testing.T) {
	p := NewPool()

	for i := 0; i < 10; i++ {
		a := newMockAgent("agent-"+string(rune('A'+i)), "test")
		a.running = true
		_ = p.Register(a)
	}

	var wg sync.WaitGroup
	const workers = 20

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < 10; i++ {
				statuses := p.HealthCheck(ctx)
				if len(statuses) != 10 {
					t.Errorf("expected 10 statuses, got %d", len(statuses))
				}
			}
		}()
	}

	wg.Wait()
}

func TestPool_Stress_AcquireReleaseDifferentRequirements(t *testing.T) {
	p := NewPool()

	// Register agents with different capabilities.
	visionAgent := newMockAgent("vision-agent", "vision")
	visionAgent.caps.Vision = true
	visionAgent.caps.Streaming = false
	_ = p.Register(visionAgent)

	streamAgent := newMockAgent("stream-agent", "stream")
	streamAgent.caps.Vision = false
	streamAgent.caps.Streaming = true
	_ = p.Register(streamAgent)

	fullAgent := newMockAgent("full-agent", "full")
	fullAgent.caps.Vision = true
	fullAgent.caps.Streaming = true
	fullAgent.caps.MaxTokens = 200000
	_ = p.Register(fullAgent)

	var wg sync.WaitGroup
	requirements := []AgentRequirements{
		{NeedsVision: true},
		{NeedsStreaming: true},
		{MinTokens: 100000},
		{}, // any agent
	}

	var acquired int64
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := requirements[idx%len(requirements)]
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			a, err := p.Acquire(ctx, req)
			if err != nil {
				return
			}
			atomic.AddInt64(&acquired, 1)
			time.Sleep(time.Microsecond * 100)
			p.Release(a)
		}(i)
	}

	wg.Wait()

	// Observable post-concurrency invariants (beyond -race):
	//  (1) the requirement-matched acquisitions actually happened; and
	//  (2) every acquire was balanced by a release, so all three agents
	//      are available again — a leaked acquisition would leave fewer.
	if acquired == 0 {
		t.Error("expected at least some requirement-matched acquisitions")
	}
	if got := len(p.Available()); got != 3 {
		t.Errorf("expected all 3 agents available after balanced acquire/release, got %d", got)
	}
}

func TestPool_Stress_ShutdownDuringAcquire(t *testing.T) {
	p := NewPool()
	a := newMockAgent("agent-1", "test")
	a.running = true
	_ = p.Register(a)

	ctx := context.Background()
	// Acquire the only agent.
	_, _ = p.Acquire(ctx, AgentRequirements{})

	var wg sync.WaitGroup

	// Start multiple goroutines trying to acquire.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_, _ = p.Acquire(ctx2, AgentRequirements{})
		}()
	}

	// Shutdown while goroutines are waiting.
	time.Sleep(50 * time.Millisecond)
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	wg.Wait()

	// Observable post-shutdown invariant (beyond -race): once the pool is shut
	// down, any further Acquire MUST be rejected with ErrPoolShutdown — the
	// waiting goroutines were unblocked and the pool will not hand out agents.
	ctx2, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := p.Acquire(ctx2, AgentRequirements{}); !errors.Is(err, ErrPoolShutdown) {
		t.Errorf("Acquire after Shutdown: expected ErrPoolShutdown, got %v", err)
	}
}

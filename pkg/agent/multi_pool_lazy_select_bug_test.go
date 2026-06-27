// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"testing"
)

// TestMultiProviderPool_Acquire_LazyPoolWithCapacityIsSelected is a
// reproduce-first RED test (§11.4.43 / §11.4.115) for the lazy-build
// selection defect in RoundRobinSelector.Select (multi_pool.go).
//
// Defect under test: RoundRobinSelector.Select only ever returns a
// provider whose pool.Available() is already non-empty. Both the
// primary requirement-matching loop AND the fallback loop iterate the
// already-materialised available set; neither considers a pool that has
// spare CAPACITY and a working lazy ClientBuilder but ZERO
// pre-registered/available agents. Such a lazy SimpleAgentPool can
// build an agent on demand (SimpleAgentPool.Acquire's build path), but
// Select never picks it, so MultiProviderPool.Acquire returns
// ErrNoSuitableAgent even though an agent could have been built. This
// defeats the entire lazy-build design for the multi-provider path —
// the canonical configuration produced by NewMultiProviderPool, where
// every SimpleAgentPool starts EMPTY and builds on first Acquire.
//
// End-user impact: a correctly-configured MultiProviderPool (one or
// more providers, each a real lazy SimpleAgentPool with capacity >= 1)
// reports "no suitable agent available" on the very first Acquire,
// before a single agent has ever been built. The pool is functionally
// dead.
//
// This test constructs that exact configuration directly (no
// pre-registered agents) and asserts Acquire succeeds and returns a
// built agent. It FAILs on the pre-fix Select.
func TestMultiProviderPool_Acquire_LazyPoolWithCapacityIsSelected(t *testing.T) {
	// One lazy SimpleAgentPool, capacity 1, working builder, ZERO
	// pre-registered agents — exactly what NewMultiProviderPool yields.
	builder, calls := makeMockBuilder("opencode")
	lazyPool := NewSimpleAgentPool("opencode", 1, builder)

	// Sanity: the lazy pool genuinely has no available agents yet but
	// CAN build one — proves the defect is in Select, not the pool.
	if got := len(lazyPool.Available()); got != 0 {
		t.Fatalf("precondition: lazyPool.Available() = %d, want 0 (no pre-registered agents)", got)
	}

	mp := &MultiProviderPool{
		pools:    map[string]AgentPool{"opencode": lazyPool},
		selector: NewRoundRobinSelector(),
	}

	agent, err := mp.Acquire(context.Background(), AgentRequirements{})
	if err != nil {
		t.Fatalf("MultiProviderPool.Acquire on a lazy pool with capacity>0 returned error %v; "+
			"want a built agent (RoundRobinSelector.Select ignored a selectable lazy pool)", err)
	}
	if agent == nil {
		t.Fatal("MultiProviderPool.Acquire returned nil agent without error")
	}
	if got := agent.Name(); got != "opencode" {
		t.Errorf("acquired agent.Name() = %q, want %q", got, "opencode")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("builder call count = %d, want 1 (the agent should have been lazily built)", got)
	}
}

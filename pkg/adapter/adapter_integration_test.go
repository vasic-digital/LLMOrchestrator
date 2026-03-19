// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package adapter

import (
	"context"
	"testing"
	"time"

	"digital.vasic.llmorchestrator/pkg/agent"
)

func TestIntegration_AllAdapters_InterfaceCompliance(t *testing.T) {
	adapters := []agent.Agent{
		NewOpenCodeAgent("oc", AdapterConfig{}),
		NewClaudeCodeAgent("cc", AdapterConfig{}),
		NewGeminiAgent("gem", AdapterConfig{}),
		NewJunieAgent("jun", AdapterConfig{}),
		NewQwenCodeAgent("qw", AdapterConfig{}),
	}

	for _, a := range adapters {
		t.Run(a.Name(), func(t *testing.T) {
			if a.ID() == "" {
				t.Error("ID should not be empty")
			}
			if a.Name() == "" {
				t.Error("Name should not be empty")
			}
			if a.IsRunning() {
				t.Error("should not be running initially")
			}

			caps := a.Capabilities()
			if caps.MaxTokens <= 0 {
				t.Error("MaxTokens should be positive")
			}

			info := a.ModelInfo()
			if info.ID == "" {
				t.Error("ModelInfo.ID should not be empty")
			}
			if info.Name == "" {
				t.Error("ModelInfo.Name should not be empty")
			}
		})
	}
}

func TestIntegration_AllAdapters_HealthWhenNotRunning(t *testing.T) {
	adapters := []agent.Agent{
		NewOpenCodeAgent("oc", AdapterConfig{}),
		NewClaudeCodeAgent("cc", AdapterConfig{}),
		NewGeminiAgent("gem", AdapterConfig{}),
		NewJunieAgent("jun", AdapterConfig{}),
		NewQwenCodeAgent("qw", AdapterConfig{}),
	}

	ctx := context.Background()
	for _, a := range adapters {
		t.Run(a.Name(), func(t *testing.T) {
			status := a.Health(ctx)
			if status.Healthy {
				t.Error("should not be healthy when not running")
			}
			if status.AgentID == "" {
				t.Error("status should have agent ID")
			}
		})
	}
}

func TestIntegration_AllAdapters_SendFailsWhenNotRunning(t *testing.T) {
	adapters := []agent.Agent{
		NewOpenCodeAgent("oc", AdapterConfig{}),
		NewClaudeCodeAgent("cc", AdapterConfig{}),
		NewGeminiAgent("gem", AdapterConfig{}),
		NewJunieAgent("jun", AdapterConfig{}),
		NewQwenCodeAgent("qw", AdapterConfig{}),
	}

	ctx := context.Background()
	for _, a := range adapters {
		t.Run(a.Name(), func(t *testing.T) {
			_, err := a.Send(ctx, "test")
			if err == nil {
				t.Error("Send should fail when not running")
			}
		})
	}
}

func TestIntegration_PoolWithAdapters(t *testing.T) {
	pool := agent.NewPool()

	adapters := []agent.Agent{
		NewOpenCodeAgent("oc-1", AdapterConfig{}),
		NewClaudeCodeAgent("cc-1", AdapterConfig{}),
		NewGeminiAgent("gem-1", AdapterConfig{}),
	}

	for _, a := range adapters {
		err := pool.Register(a)
		if err != nil {
			t.Fatalf("Register %s failed: %v", a.Name(), err)
		}
	}

	available := pool.Available()
	if len(available) != 3 {
		t.Errorf("expected 3 available, got %d", len(available))
	}

	// Acquire with vision requirement.
	ctx := context.Background()
	acquired, err := pool.Acquire(ctx, agent.AgentRequirements{NeedsVision: true})
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if !acquired.SupportsVision() {
		t.Error("acquired agent should support vision")
	}

	pool.Release(acquired)

	// Acquire preferred.
	acquired, err = pool.Acquire(ctx, agent.AgentRequirements{PreferredAgent: "gemini"})
	if err != nil {
		t.Fatalf("Acquire preferred failed: %v", err)
	}
	if acquired.Name() != "gemini" {
		t.Errorf("expected gemini, got %s", acquired.Name())
	}

	pool.Release(acquired)
}

func TestIntegration_PoolHealthCheck(t *testing.T) {
	pool := agent.NewPool()

	_ = pool.Register(NewOpenCodeAgent("oc-1", AdapterConfig{}))
	_ = pool.Register(NewClaudeCodeAgent("cc-1", AdapterConfig{}))

	ctx := context.Background()
	statuses := pool.HealthCheck(ctx)

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	// None should be healthy since none are started.
	for _, s := range statuses {
		if s.Healthy {
			t.Errorf("agent %s should not be healthy when not started", s.AgentName)
		}
	}
}

func TestIntegration_PoolShutdown(t *testing.T) {
	pool := agent.NewPool()

	_ = pool.Register(NewOpenCodeAgent("oc-1", AdapterConfig{BinaryPath: "/bin/cat", Timeout: 2 * time.Second}))

	ctx := context.Background()
	err := pool.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestIntegration_AdapterWithCircuitBreaker(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewOpenCodeAgent("oc-1", cfg)

	cb := a.CircuitBreaker()
	if cb.State() != agent.CircuitClosed {
		t.Error("initial circuit should be closed")
	}

	// Simulate failures.
	for i := 0; i < agent.DefaultFailureThreshold; i++ {
		cb.RecordFailure()
	}

	if cb.State() != agent.CircuitOpen {
		t.Error("circuit should be open after failures")
	}

	// Health should reflect circuit state.
	ctx := context.Background()
	status := a.Health(ctx)
	if status.Healthy {
		t.Error("should not be healthy with open circuit")
	}
}

func TestIntegration_AdapterOutputDirConfiguration(t *testing.T) {
	type testCase struct {
		agent       agent.Agent
		expectedDir string
	}
	cases := map[string]testCase{
		"opencode":    {NewOpenCodeAgent("oc-1", AdapterConfig{OutputDir: "/tmp/oc-out"}), "/tmp/oc-out"},
		"claude-code": {NewClaudeCodeAgent("cc-1", AdapterConfig{OutputDir: "/tmp/cc-out"}), "/tmp/cc-out"},
		"gemini":      {NewGeminiAgent("gem-1", AdapterConfig{OutputDir: "/tmp/gem-out"}), "/tmp/gem-out"},
		"junie":       {NewJunieAgent("jun-1", AdapterConfig{OutputDir: "/tmp/jun-out"}), "/tmp/jun-out"},
		"qwen-code":   {NewQwenCodeAgent("qw-1", AdapterConfig{OutputDir: "/tmp/qw-out"}), "/tmp/qw-out"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.agent.OutputDir() != tc.expectedDir {
				t.Errorf("expected output dir %q, got %q", tc.expectedDir, tc.agent.OutputDir())
			}
		})
	}
}

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package adapter

import (
	"context"
	"testing"
	"time"

	"digital.vasic.llmorchestrator/pkg/agent"
)

// --- OpenCode Adapter Tests ---

func TestOpenCodeAgent_New(t *testing.T) {
	cfg := AdapterConfig{BinaryPath: "/usr/local/bin/opencode"}
	a := NewOpenCodeAgent("oc-1", cfg)

	if a.ID() != "oc-1" {
		t.Errorf("expected ID 'oc-1', got %q", a.ID())
	}
	if a.Name() != "opencode" {
		t.Errorf("expected name 'opencode', got %q", a.Name())
	}
}

func TestOpenCodeAgent_DefaultBinary(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewOpenCodeAgent("oc-1", cfg)
	if a.config.BinaryPath != "opencode" {
		t.Errorf("expected default binary 'opencode', got %q", a.config.BinaryPath)
	}
}

func TestOpenCodeAgent_HeadlessFlags(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewOpenCodeAgent("oc-1", cfg)

	hasHeadless := false
	hasNonInteractive := false
	for _, arg := range a.config.Args {
		if arg == "--headless" {
			hasHeadless = true
		}
		if arg == "--non-interactive" {
			hasNonInteractive = true
		}
	}
	if !hasHeadless {
		t.Error("expected --headless flag")
	}
	if !hasNonInteractive {
		t.Error("expected --non-interactive flag")
	}
}

func TestOpenCodeAgent_Capabilities(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewOpenCodeAgent("oc-1", cfg)
	caps := a.Capabilities()

	if !caps.Vision {
		t.Error("OpenCode should have vision capability")
	}
	if !caps.Streaming {
		t.Error("OpenCode should have streaming capability")
	}
	if !caps.ToolUse {
		t.Error("OpenCode should have tool use capability")
	}
}

func TestOpenCodeAgent_InterfaceCompliance(t *testing.T) {
	cfg := AdapterConfig{}
	var _ agent.Agent = NewOpenCodeAgent("oc-1", cfg)
}

func TestOpenCodeAgent_ParseResponse_JSON(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewOpenCodeAgent("oc-1", cfg)

	resp, err := a.parseOpenCodeResponse(`{"content": "Hello from OpenCode", "tool_use": true, "tokens_in": 100, "tokens_out": 50}`)
	if err != nil {
		t.Fatalf("parseOpenCodeResponse failed: %v", err)
	}
	if resp.Content != "Hello from OpenCode" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if resp.TokensUsed != 150 {
		t.Errorf("expected 150 tokens, got %d", resp.TokensUsed)
	}
}

func TestOpenCodeAgent_ParseResponse_PlainText(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewOpenCodeAgent("oc-1", cfg)

	resp, err := a.parseOpenCodeResponse("Just plain text response")
	if err != nil {
		t.Fatalf("parseOpenCodeResponse failed: %v", err)
	}
	if resp.Content != "Just plain text response" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
}

// --- Claude Code Adapter Tests ---

func TestClaudeCodeAgent_New(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewClaudeCodeAgent("cc-1", cfg)

	if a.Name() != "claude-code" {
		t.Errorf("expected name 'claude-code', got %q", a.Name())
	}
}

func TestClaudeCodeAgent_DefaultBinary(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewClaudeCodeAgent("cc-1", cfg)
	if a.config.BinaryPath != "claude" {
		t.Errorf("expected default binary 'claude', got %q", a.config.BinaryPath)
	}
}

func TestClaudeCodeAgent_Flags(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewClaudeCodeAgent("cc-1", cfg)

	hasPrint := false
	hasOutputFormat := false
	for _, arg := range a.config.Args {
		if arg == "--print" {
			hasPrint = true
		}
		if arg == "json" {
			hasOutputFormat = true
		}
	}
	if !hasPrint {
		t.Error("expected --print flag")
	}
	if !hasOutputFormat {
		t.Error("expected json output format")
	}
}

func TestClaudeCodeAgent_Capabilities(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewClaudeCodeAgent("cc-1", cfg)
	caps := a.Capabilities()

	if !caps.Vision {
		t.Error("Claude should have vision capability")
	}
	if caps.MaxTokens != 200000 {
		t.Errorf("expected 200000 max tokens, got %d", caps.MaxTokens)
	}
}

func TestClaudeCodeAgent_InterfaceCompliance(t *testing.T) {
	cfg := AdapterConfig{}
	var _ agent.Agent = NewClaudeCodeAgent("cc-1", cfg)
}

func TestClaudeCodeAgent_ParseResponse(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewClaudeCodeAgent("cc-1", cfg)

	resp, err := a.parseClaudeResponse(`{"result": "Analysis complete", "usage": {"input_tokens": 500, "output_tokens": 200}, "model": "claude-3-opus"}`)
	if err != nil {
		t.Fatalf("parseClaudeResponse failed: %v", err)
	}
	if resp.Content != "Analysis complete" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if resp.TokensUsed != 700 {
		t.Errorf("expected 700 tokens, got %d", resp.TokensUsed)
	}
	if resp.Metadata["model"] != "claude-3-opus" {
		t.Errorf("unexpected model: %s", resp.Metadata["model"])
	}
}

// --- Gemini Adapter Tests ---

func TestGeminiAgent_New(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewGeminiAgent("gem-1", cfg)

	if a.Name() != "gemini" {
		t.Errorf("expected name 'gemini', got %q", a.Name())
	}
}

func TestGeminiAgent_DefaultBinary(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewGeminiAgent("gem-1", cfg)
	if a.config.BinaryPath != "gemini" {
		t.Errorf("expected default binary 'gemini', got %q", a.config.BinaryPath)
	}
}

func TestGeminiAgent_Flags(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewGeminiAgent("gem-1", cfg)

	hasNonInteractive := false
	for _, arg := range a.config.Args {
		if arg == "--non-interactive" {
			hasNonInteractive = true
		}
	}
	if !hasNonInteractive {
		t.Error("expected --non-interactive flag")
	}
}

func TestGeminiAgent_Capabilities(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewGeminiAgent("gem-1", cfg)
	caps := a.Capabilities()

	if !caps.Vision {
		t.Error("Gemini should have vision capability")
	}
	if caps.MaxTokens != 1000000 {
		t.Errorf("expected 1000000 max tokens, got %d", caps.MaxTokens)
	}
}

func TestGeminiAgent_InterfaceCompliance(t *testing.T) {
	cfg := AdapterConfig{}
	var _ agent.Agent = NewGeminiAgent("gem-1", cfg)
}

func TestGeminiAgent_ParseResponse(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewGeminiAgent("gem-1", cfg)

	resp, err := a.parseGeminiResponse(`{"text": "Gemini sees a form", "token_count": 300, "finish_reason": "stop"}`)
	if err != nil {
		t.Fatalf("parseGeminiResponse failed: %v", err)
	}
	if resp.Content != "Gemini sees a form" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if resp.TokensUsed != 300 {
		t.Errorf("expected 300 tokens, got %d", resp.TokensUsed)
	}
}

// --- Junie Adapter Tests ---

func TestJunieAgent_New(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewJunieAgent("jun-1", cfg)

	if a.Name() != "junie" {
		t.Errorf("expected name 'junie', got %q", a.Name())
	}
}

func TestJunieAgent_DefaultBinary(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewJunieAgent("jun-1", cfg)
	if a.config.BinaryPath != "junie" {
		t.Errorf("expected default binary 'junie', got %q", a.config.BinaryPath)
	}
}

func TestJunieAgent_HeadlessFlag(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewJunieAgent("jun-1", cfg)

	hasHeadless := false
	for _, arg := range a.config.Args {
		if arg == "--headless" {
			hasHeadless = true
		}
	}
	if !hasHeadless {
		t.Error("expected --headless flag")
	}
}

func TestJunieAgent_InterfaceCompliance(t *testing.T) {
	cfg := AdapterConfig{}
	var _ agent.Agent = NewJunieAgent("jun-1", cfg)
}

func TestJunieAgent_ParseResponse(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewJunieAgent("jun-1", cfg)

	resp, err := a.parseJunieResponse(`{"response": "Junie analysis", "status": "complete", "tokens": 250}`)
	if err != nil {
		t.Fatalf("parseJunieResponse failed: %v", err)
	}
	if resp.Content != "Junie analysis" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if resp.TokensUsed != 250 {
		t.Errorf("expected 250 tokens, got %d", resp.TokensUsed)
	}
}

// --- QwenCode Adapter Tests ---

func TestQwenCodeAgent_New(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewQwenCodeAgent("qw-1", cfg)

	if a.Name() != "qwen-code" {
		t.Errorf("expected name 'qwen-code', got %q", a.Name())
	}
}

func TestQwenCodeAgent_DefaultBinary(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewQwenCodeAgent("qw-1", cfg)
	if a.config.BinaryPath != "qwen-code" {
		t.Errorf("expected default binary 'qwen-code', got %q", a.config.BinaryPath)
	}
}

func TestQwenCodeAgent_Flags(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewQwenCodeAgent("qw-1", cfg)

	hasHeadless := false
	hasNonInteractive := false
	for _, arg := range a.config.Args {
		if arg == "--headless" {
			hasHeadless = true
		}
		if arg == "--non-interactive" {
			hasNonInteractive = true
		}
	}
	if !hasHeadless {
		t.Error("expected --headless flag")
	}
	if !hasNonInteractive {
		t.Error("expected --non-interactive flag")
	}
}

func TestQwenCodeAgent_InterfaceCompliance(t *testing.T) {
	cfg := AdapterConfig{}
	var _ agent.Agent = NewQwenCodeAgent("qw-1", cfg)
}

func TestQwenCodeAgent_ParseResponse(t *testing.T) {
	cfg := AdapterConfig{}
	a := NewQwenCodeAgent("qw-1", cfg)

	resp, err := a.parseQwenResponse(`{"output": "Qwen analysis complete", "token_usage": {"input": 100, "output": 200}, "model": "qwen-vl-plus"}`)
	if err != nil {
		t.Fatalf("parseQwenResponse failed: %v", err)
	}
	if resp.Content != "Qwen analysis complete" {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if resp.TokensUsed != 300 {
		t.Errorf("expected 300 tokens, got %d", resp.TokensUsed)
	}
	if resp.Metadata["model"] != "qwen-vl-plus" {
		t.Errorf("unexpected model: %s", resp.Metadata["model"])
	}
}

// --- BaseAdapter Tests ---

func TestBaseAdapter_New(t *testing.T) {
	cfg := AdapterConfig{BinaryPath: "/bin/echo"}
	ba := NewBaseAdapter("test-1", "test", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	if ba.ID() != "test-1" {
		t.Errorf("expected ID 'test-1', got %q", ba.ID())
	}
	if ba.Name() != "test" {
		t.Errorf("expected name 'test', got %q", ba.Name())
	}
}

func TestBaseAdapter_IsRunning_Initial(t *testing.T) {
	cfg := AdapterConfig{BinaryPath: "/bin/echo"}
	ba := NewBaseAdapter("test-1", "test", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	if ba.IsRunning() {
		t.Error("new adapter should not be running")
	}
}

func TestBaseAdapter_Health_NotRunning(t *testing.T) {
	cfg := AdapterConfig{BinaryPath: "/bin/echo"}
	ba := NewBaseAdapter("test-1", "test", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	ctx := context.Background()
	status := ba.Health(ctx)

	if status.Healthy {
		t.Error("expected unhealthy when not running")
	}
	if status.AgentID != "test-1" {
		t.Errorf("unexpected agent ID: %s", status.AgentID)
	}
}

func TestBaseAdapter_Send_NotRunning(t *testing.T) {
	cfg := AdapterConfig{BinaryPath: "/bin/echo"}
	ba := NewBaseAdapter("test-1", "test", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	ctx := context.Background()
	_, err := ba.Send(ctx, "hello")
	if err != ErrAgentNotRunning {
		t.Errorf("expected ErrAgentNotRunning, got: %v", err)
	}
}

func TestBaseAdapter_SendStream_NotRunning(t *testing.T) {
	cfg := AdapterConfig{BinaryPath: "/bin/echo"}
	ba := NewBaseAdapter("test-1", "test", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	ctx := context.Background()
	_, err := ba.SendStream(ctx, "hello")
	if err != ErrAgentNotRunning {
		t.Errorf("expected ErrAgentNotRunning, got: %v", err)
	}
}

func TestBaseAdapter_SendWithAttachments_NotRunning(t *testing.T) {
	cfg := AdapterConfig{BinaryPath: "/bin/echo"}
	ba := NewBaseAdapter("test-1", "test", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	ctx := context.Background()
	_, err := ba.SendWithAttachments(ctx, "hello", nil)
	if err != ErrAgentNotRunning {
		t.Errorf("expected ErrAgentNotRunning, got: %v", err)
	}
}

func TestBaseAdapter_OutputDir(t *testing.T) {
	cfg := AdapterConfig{OutputDir: "/tmp/test-output"}
	ba := NewBaseAdapter("test-1", "test", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	if ba.OutputDir() != "/tmp/test-output" {
		t.Errorf("unexpected output dir: %s", ba.OutputDir())
	}
}

func TestBaseAdapter_SupportsVision(t *testing.T) {
	tests := []struct {
		name   string
		vision bool
	}{
		{"with vision", true},
		{"without vision", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AdapterConfig{}
			caps := agent.AgentCapabilities{Vision: tt.vision}
			ba := NewBaseAdapter("test", "test", cfg, caps, agent.ModelInfo{})
			if ba.SupportsVision() != tt.vision {
				t.Errorf("expected SupportsVision() = %v", tt.vision)
			}
		})
	}
}

func TestBaseAdapter_ModelInfo(t *testing.T) {
	cfg := AdapterConfig{}
	info := agent.ModelInfo{
		ID:       "model-1",
		Provider: "test-provider",
		Name:     "Test Model",
		Score:    0.85,
	}
	ba := NewBaseAdapter("test", "test", cfg, agent.AgentCapabilities{}, info)

	got := ba.ModelInfo()
	if got.ID != "model-1" {
		t.Errorf("unexpected model ID: %s", got.ID)
	}
	if got.Provider != "test-provider" {
		t.Errorf("unexpected provider: %s", got.Provider)
	}
	if got.Score != 0.85 {
		t.Errorf("unexpected score: %f", got.Score)
	}
}

func TestBaseAdapter_CircuitBreaker(t *testing.T) {
	cfg := AdapterConfig{}
	ba := NewBaseAdapter("test", "test", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	cb := ba.CircuitBreaker()
	if cb == nil {
		t.Fatal("CircuitBreaker returned nil")
	}
	if cb.State() != agent.CircuitClosed {
		t.Error("initial circuit should be closed")
	}
}

func TestBaseAdapter_Stop_WhenNotRunning(t *testing.T) {
	cfg := AdapterConfig{BinaryPath: "/bin/echo"}
	ba := NewBaseAdapter("test", "test", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	ctx := context.Background()
	err := ba.Stop(ctx)
	if err != nil {
		t.Errorf("Stop on non-running adapter should not error, got: %v", err)
	}
}

// --- appendIfMissing Tests ---

func TestAppendIfMissing(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		flag     string
		expected int
	}{
		{"add new", []string{"--a"}, "--b", 2},
		{"already present", []string{"--a", "--b"}, "--b", 2},
		{"empty slice", nil, "--a", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendIfMissing(tt.input, tt.flag)
			if len(result) != tt.expected {
				t.Errorf("expected length %d, got %d", tt.expected, len(result))
			}
		})
	}
}

// --- isImageMIME Tests ---

func TestIsImageMIME(t *testing.T) {
	tests := []struct {
		mime     string
		expected bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"image/gif", true},
		{"image/webp", true},
		{"image/bmp", true},
		{"text/plain", false},
		{"application/json", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isImageMIME(tt.mime); got != tt.expected {
			t.Errorf("isImageMIME(%q) = %v, want %v", tt.mime, got, tt.expected)
		}
	}
}

// --- E2E Test with Mock Subprocess ---

func TestBaseAdapter_E2E_WithEchoProcess(t *testing.T) {
	// This test starts a real process (/bin/cat) to test the full pipe lifecycle.
	cfg := AdapterConfig{
		BinaryPath: "/bin/cat",
		Timeout:    5 * time.Second,
	}
	ba := NewBaseAdapter("e2e-test", "cat-agent", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	ctx := context.Background()
	err := ba.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer ba.Stop(ctx)

	if !ba.IsRunning() {
		t.Error("expected agent to be running after Start")
	}

	// Verify health status while running.
	status := ba.Health(ctx)
	if !status.Healthy {
		t.Error("expected healthy status while running")
	}

	// Stop the agent.
	err = ba.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if ba.IsRunning() {
		t.Error("expected agent to not be running after Stop")
	}
}

func TestBaseAdapter_Start_AlreadyRunning(t *testing.T) {
	cfg := AdapterConfig{
		BinaryPath: "/bin/cat",
		Timeout:    5 * time.Second,
	}
	ba := NewBaseAdapter("test", "test", cfg, agent.AgentCapabilities{}, agent.ModelInfo{})

	ctx := context.Background()
	_ = ba.Start(ctx)
	defer ba.Stop(ctx)

	err := ba.Start(ctx)
	if err != ErrAgentAlreadyRunning {
		t.Errorf("expected ErrAgentAlreadyRunning, got: %v", err)
	}
}

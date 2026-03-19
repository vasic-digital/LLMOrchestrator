// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if cfg.AgentTimeout != 60*time.Second {
		t.Errorf("expected 60s timeout, got %v", cfg.AgentTimeout)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected 3 max retries, got %d", cfg.MaxRetries)
	}
	if cfg.PoolSize != 3 {
		t.Errorf("expected pool size 3, got %d", cfg.PoolSize)
	}
	if len(cfg.EnabledAgents) != 3 {
		t.Errorf("expected 3 enabled agents, got %d", len(cfg.EnabledAgents))
	}
	if len(cfg.AgentPaths) != 5 {
		t.Errorf("expected 5 agent paths, got %d", len(cfg.AgentPaths))
	}
}

func TestDefaultConfig_AgentPaths(t *testing.T) {
	cfg := DefaultConfig()

	expectedPaths := map[string]string{
		"opencode":    "opencode",
		"claude-code": "claude",
		"gemini":      "gemini",
		"junie":       "junie",
		"qwen-code":   "qwen-code",
	}

	for name, expected := range expectedPaths {
		if got, ok := cfg.AgentPaths[name]; !ok || got != expected {
			t.Errorf("agent %s: expected path %q, got %q", name, expected, got)
		}
	}
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed on default config: %v", err)
	}
}

func TestConfig_Validate_InvalidTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AgentTimeout = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for zero timeout")
	}
}

func TestConfig_Validate_NegativeRetries(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRetries = -1
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative retries")
	}
}

func TestConfig_Validate_ZeroPoolSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PoolSize = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for zero pool size")
	}
}

func TestConfig_Validate_NoEnabledAgents(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnabledAgents = nil
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for no enabled agents")
	}
}

func TestConfig_IsAgentEnabled(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.IsAgentEnabled("opencode") {
		t.Error("opencode should be enabled by default")
	}
	if !cfg.IsAgentEnabled("claude-code") {
		t.Error("claude-code should be enabled by default")
	}
	if cfg.IsAgentEnabled("unknown-agent") {
		t.Error("unknown-agent should not be enabled")
	}
}

func TestConfig_SessionDir(t *testing.T) {
	cfg := DefaultConfig()
	dir := cfg.SessionDir("test-123")
	if dir != "/tmp/helix-session-test-123" {
		t.Errorf("unexpected session dir: %s", dir)
	}
}

func TestConfig_AgentBinaryPath_RelativePath(t *testing.T) {
	cfg := DefaultConfig()
	path, err := cfg.AgentBinaryPath("opencode")
	if err != nil {
		t.Fatalf("AgentBinaryPath failed: %v", err)
	}
	if path != "opencode" {
		t.Errorf("expected 'opencode', got %q", path)
	}
}

func TestConfig_AgentBinaryPath_Unknown(t *testing.T) {
	cfg := DefaultConfig()
	_, err := cfg.AgentBinaryPath("nonexistent")
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestConfig_AgentBinaryPath_AbsoluteNotFound(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AgentPaths["test"] = "/nonexistent/path/to/binary"
	_, err := cfg.AgentBinaryPath("test")
	if err == nil {
		t.Error("expected error for nonexistent absolute path")
	}
}

func TestLoadFromEnv(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")

	content := `# Test environment
HELIX_AGENTS_ENABLED=claude-code,gemini
HELIX_AGENT_CLAUDE_PATH=/usr/local/bin/claude
HELIX_AGENT_TIMEOUT=120s
HELIX_AGENT_MAX_RETRIES=5
HELIX_AGENT_POOL_SIZE=4
OPENAI_API_KEY=sk-test-key
`
	os.WriteFile(envFile, []byte(content), 0644)

	cfg, err := LoadFromEnv(envFile)
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if len(cfg.EnabledAgents) != 2 {
		t.Errorf("expected 2 enabled agents, got %d", len(cfg.EnabledAgents))
	}
	if cfg.EnabledAgents[0] != "claude-code" {
		t.Errorf("expected claude-code first, got %s", cfg.EnabledAgents[0])
	}
	if cfg.AgentPaths["claude-code"] != "/usr/local/bin/claude" {
		t.Errorf("unexpected claude path: %s", cfg.AgentPaths["claude-code"])
	}
	if cfg.AgentTimeout != 120*time.Second {
		t.Errorf("expected 120s timeout, got %v", cfg.AgentTimeout)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("expected 5 retries, got %d", cfg.MaxRetries)
	}
	if cfg.PoolSize != 4 {
		t.Errorf("expected pool size 4, got %d", cfg.PoolSize)
	}
	if cfg.APIKeys["OPENAI_API_KEY"] != "sk-test-key" {
		t.Error("expected OPENAI_API_KEY to be loaded")
	}
}

func TestLoadFromEnv_NotFound(t *testing.T) {
	_, err := LoadFromEnv("/nonexistent/.env")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadFromEnv_EmptyLines_Comments(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")

	content := `
# This is a comment

HELIX_AGENT_POOL_SIZE=2

# Another comment
HELIX_AGENT_TIMEOUT=30s
`
	os.WriteFile(envFile, []byte(content), 0644)

	cfg, err := LoadFromEnv(envFile)
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if cfg.PoolSize != 2 {
		t.Errorf("expected pool size 2, got %d", cfg.PoolSize)
	}
	if cfg.AgentTimeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", cfg.AgentTimeout)
	}
}

func TestLoadFromEnv_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")

	content := `OPENAI_API_KEY="sk-quoted-key"
ANTHROPIC_API_KEY='sk-ant-quoted'
`
	os.WriteFile(envFile, []byte(content), 0644)

	cfg, err := LoadFromEnv(envFile)
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if cfg.APIKeys["OPENAI_API_KEY"] != "sk-quoted-key" {
		t.Errorf("unexpected API key: %s", cfg.APIKeys["OPENAI_API_KEY"])
	}
	if cfg.APIKeys["ANTHROPIC_API_KEY"] != "sk-ant-quoted" {
		t.Errorf("unexpected API key: %s", cfg.APIKeys["ANTHROPIC_API_KEY"])
	}
}

func TestLoadFromEnv_AllAgentPaths(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")

	content := `HELIX_AGENT_OPENCODE_PATH=/path/to/opencode
HELIX_AGENT_CLAUDE_PATH=/path/to/claude
HELIX_AGENT_GEMINI_PATH=/path/to/gemini
HELIX_AGENT_JUNIE_PATH=/path/to/junie
HELIX_AGENT_QWEN_PATH=/path/to/qwen
`
	os.WriteFile(envFile, []byte(content), 0644)

	cfg, err := LoadFromEnv(envFile)
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	expected := map[string]string{
		"opencode":    "/path/to/opencode",
		"claude-code": "/path/to/claude",
		"gemini":      "/path/to/gemini",
		"junie":       "/path/to/junie",
		"qwen-code":   "/path/to/qwen",
	}

	for agent, path := range expected {
		if cfg.AgentPaths[agent] != path {
			t.Errorf("agent %s: expected %q, got %q", agent, path, cfg.AgentPaths[agent])
		}
	}
}

func TestLoadFromEnv_AllAPIKeys(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")

	keys := []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY",
		"GROQ_API_KEY", "MISTRAL_API_KEY", "DEEPSEEK_API_KEY",
		"XAI_API_KEY", "TOGETHER_API_KEY", "QWEN_API_KEY", "JUNIE_API_KEY",
	}

	content := ""
	for _, key := range keys {
		content += key + "=test-" + key + "\n"
	}
	os.WriteFile(envFile, []byte(content), 0644)

	cfg, err := LoadFromEnv(envFile)
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	for _, key := range keys {
		if cfg.APIKeys[key] != "test-"+key {
			t.Errorf("expected API key %s = 'test-%s', got %q", key, key, cfg.APIKeys[key])
		}
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sk-abcdefghijklmnop", "sk-a...mnop"},
		{"short", "****"},
		{"12345678", "****"},
		{"123456789", "1234...6789"},
		{"", "****"},
	}

	for _, tt := range tests {
		result := MaskAPIKey(tt.input)
		if result != tt.expected {
			t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMaskAPIKey_NeverExposesFullKey(t *testing.T) {
	key := "sk-ant-very-secret-key-12345"
	masked := MaskAPIKey(key)

	if masked == key {
		t.Error("masked key should not equal original key")
	}
	if len(masked) >= len(key) {
		t.Error("masked key should be shorter than original")
	}
}

// --- Security Tests ---

func TestConfig_Security_APIKeyNotInDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.APIKeys) != 0 {
		t.Error("default config should not contain any API keys")
	}
}

func TestConfig_Security_NoSensitiveDefaultPaths(t *testing.T) {
	cfg := DefaultConfig()
	for name, path := range cfg.AgentPaths {
		if filepath.IsAbs(path) {
			t.Errorf("default path for %s should be relative, got %s", name, path)
		}
	}
}

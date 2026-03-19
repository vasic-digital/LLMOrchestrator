// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrConfigNotFound is returned when the config file doesn't exist.
	ErrConfigNotFound = errors.New("config file not found")
	// ErrInvalidConfig is returned when a config value is malformed.
	ErrInvalidConfig = errors.New("invalid config value")
)

// Config holds all LLMOrchestrator configuration.
type Config struct {
	// Agent paths
	AgentPaths map[string]string // agent name -> binary path
	// Enabled agents
	EnabledAgents []string
	// Timeouts
	AgentTimeout time.Duration
	MaxRetries   int
	PoolSize     int
	// Session directory template
	SessionDirTemplate string
	// API keys
	APIKeys map[string]string
}

// DefaultConfig returns a Config with sane defaults.
func DefaultConfig() *Config {
	return &Config{
		AgentPaths: map[string]string{
			"opencode":    "opencode",
			"claude-code": "claude",
			"gemini":      "gemini",
			"junie":       "junie",
			"qwen-code":   "qwen-code",
		},
		EnabledAgents:      []string{"opencode", "claude-code", "gemini"},
		AgentTimeout:       60 * time.Second,
		MaxRetries:         3,
		PoolSize:           3,
		SessionDirTemplate: "/tmp/helix-session-{id}",
		APIKeys:            make(map[string]string),
	}
}

// LoadFromEnv loads configuration from a .env file.
func LoadFromEnv(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, path)
	}

	envMap, err := parseEnvFile(path)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	applyEnvMap(cfg, envMap)
	return cfg, nil
}

// LoadFromEnvironment loads configuration from OS environment variables.
func LoadFromEnvironment() *Config {
	cfg := DefaultConfig()

	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "HELIX_") {
			envMap[parts[0]] = parts[1]
		}
	}

	// Also check API keys.
	apiKeyEnvs := []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY",
		"GROQ_API_KEY", "MISTRAL_API_KEY", "DEEPSEEK_API_KEY",
		"XAI_API_KEY", "TOGETHER_API_KEY", "QWEN_API_KEY", "JUNIE_API_KEY",
	}
	for _, key := range apiKeyEnvs {
		if val := os.Getenv(key); val != "" {
			envMap[key] = val
		}
	}

	applyEnvMap(cfg, envMap)
	return cfg
}

// AgentBinaryPath returns the resolved binary path for an agent.
func (c *Config) AgentBinaryPath(name string) (string, error) {
	path, ok := c.AgentPaths[name]
	if !ok {
		return "", fmt.Errorf("no path configured for agent: %s", name)
	}

	// If it's an absolute path, check it exists.
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("agent binary not found: %s", path)
		}
		return path, nil
	}

	// Otherwise return as-is (will be found on PATH).
	return path, nil
}

// IsAgentEnabled returns true if the given agent is in the enabled list.
func (c *Config) IsAgentEnabled(name string) bool {
	for _, a := range c.EnabledAgents {
		if a == name {
			return true
		}
	}
	return false
}

// SessionDir returns a session directory path for the given session ID.
func (c *Config) SessionDir(sessionID string) string {
	return strings.ReplaceAll(c.SessionDirTemplate, "{id}", sessionID)
}

// Validate checks the config for obvious errors.
func (c *Config) Validate() error {
	if c.AgentTimeout <= 0 {
		return fmt.Errorf("%w: agent timeout must be positive", ErrInvalidConfig)
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("%w: max retries must be non-negative", ErrInvalidConfig)
	}
	if c.PoolSize <= 0 {
		return fmt.Errorf("%w: pool size must be positive", ErrInvalidConfig)
	}
	if len(c.EnabledAgents) == 0 {
		return fmt.Errorf("%w: at least one agent must be enabled", ErrInvalidConfig)
	}
	return nil
}

// parseEnvFile reads a .env file and returns a map of key-value pairs.
func parseEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes.
		value = strings.Trim(value, "\"'")

		result[key] = value
	}

	return result, scanner.Err()
}

// applyEnvMap applies environment variable values to a Config.
func applyEnvMap(cfg *Config, envMap map[string]string) {
	// Enabled agents.
	if v, ok := envMap["HELIX_AGENTS_ENABLED"]; ok {
		agents := strings.Split(v, ",")
		var trimmed []string
		for _, a := range agents {
			a = strings.TrimSpace(a)
			if a != "" {
				trimmed = append(trimmed, a)
			}
		}
		if len(trimmed) > 0 {
			cfg.EnabledAgents = trimmed
		}
	}

	// Agent paths.
	pathKeys := map[string]string{
		"HELIX_AGENT_OPENCODE_PATH": "opencode",
		"HELIX_AGENT_CLAUDE_PATH":   "claude-code",
		"HELIX_AGENT_GEMINI_PATH":   "gemini",
		"HELIX_AGENT_JUNIE_PATH":    "junie",
		"HELIX_AGENT_QWEN_PATH":     "qwen-code",
	}
	for envKey, agentName := range pathKeys {
		if v, ok := envMap[envKey]; ok && v != "" {
			cfg.AgentPaths[agentName] = v
		}
	}

	// Timeout.
	if v, ok := envMap["HELIX_AGENT_TIMEOUT"]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.AgentTimeout = d
		}
	}

	// Max retries.
	if v, ok := envMap["HELIX_AGENT_MAX_RETRIES"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxRetries = n
		}
	}

	// Pool size.
	if v, ok := envMap["HELIX_AGENT_POOL_SIZE"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.PoolSize = n
		}
	}

	// API keys.
	apiKeyEnvs := []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY",
		"GROQ_API_KEY", "MISTRAL_API_KEY", "DEEPSEEK_API_KEY",
		"XAI_API_KEY", "TOGETHER_API_KEY", "QWEN_API_KEY", "JUNIE_API_KEY",
	}
	for _, key := range apiKeyEnvs {
		if v, ok := envMap[key]; ok && v != "" {
			cfg.APIKeys[key] = v
		}
	}
}

// MaskAPIKey masks an API key for safe logging (shows first 4 and last 4 chars).
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

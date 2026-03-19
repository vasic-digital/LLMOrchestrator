// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package adapter

import (
	"encoding/json"

	"digital.vasic.llmorchestrator/pkg/agent"
)

// ClaudeCodeAgent is the adapter for the Claude Code CLI agent.
// Claude Code uses `--print --output-format json` mode.
type ClaudeCodeAgent struct {
	*BaseAdapter
}

// NewClaudeCodeAgent creates a new Claude Code adapter.
func NewClaudeCodeAgent(id string, config AdapterConfig) *ClaudeCodeAgent {
	if config.BinaryPath == "" {
		config.BinaryPath = "claude"
	}

	config.Args = appendIfMissing(config.Args, "--print")
	config.Args = appendIfMissing(config.Args, "--output-format")
	config.Args = appendIfMissing(config.Args, "json")

	caps := agent.AgentCapabilities{
		Vision:    true, // native Claude vision
		Streaming: true,
		ToolUse:   true,
		MaxTokens: 200000,
		MaxImages: 20,
		Providers: []string{"anthropic"},
	}

	modelInfo := agent.ModelInfo{
		ID:           "claude-code",
		Provider:     "anthropic",
		Name:         "Claude Code",
		Capabilities: caps,
	}

	adapter := &ClaudeCodeAgent{
		BaseAdapter: NewBaseAdapter(id, "claude-code", config, caps, modelInfo),
	}
	adapter.BaseAdapter.parseResponse = adapter.parseClaudeResponse
	return adapter
}

// parseClaudeResponse parses Claude Code's JSON output format.
func (c *ClaudeCodeAgent) parseClaudeResponse(raw string) (agent.Response, error) {
	resp := agent.Response{
		Content: raw,
		Metadata: map[string]string{
			"agent": "claude-code",
		},
	}

	// Claude Code returns structured JSON with result and usage.
	var jsonResp struct {
		Result string `json:"result"`
		Usage  struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}

	if err := json.Unmarshal([]byte(raw), &jsonResp); err == nil {
		if jsonResp.Result != "" {
			resp.Content = jsonResp.Result
		}
		resp.TokensUsed = jsonResp.Usage.InputTokens + jsonResp.Usage.OutputTokens
		if jsonResp.Model != "" {
			resp.Metadata["model"] = jsonResp.Model
		}
	}

	parsed, err := c.parser.Parse(resp.Content)
	if err == nil {
		resp.Actions = parsed.Actions
	}

	return resp, nil
}

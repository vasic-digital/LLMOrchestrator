// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package adapter

import (
	"encoding/json"

	"digital.vasic.llmorchestrator/pkg/agent"
)

// QwenCodeAgent is the adapter for the Qwen Code CLI agent.
// Qwen Code uses `--headless --non-interactive` mode.
type QwenCodeAgent struct {
	*BaseAdapter
}

// NewQwenCodeAgent creates a new Qwen Code adapter.
func NewQwenCodeAgent(id string, config AdapterConfig) *QwenCodeAgent {
	if config.BinaryPath == "" {
		config.BinaryPath = "qwen-code"
	}

	config.Args = appendIfMissing(config.Args, "--headless")
	config.Args = appendIfMissing(config.Args, "--non-interactive")

	caps := agent.AgentCapabilities{
		Vision:    true, // Qwen-VL native
		Streaming: true,
		ToolUse:   true,
		MaxTokens: 128000,
		MaxImages: 10,
		Providers: []string{"alibaba"},
	}

	modelInfo := agent.ModelInfo{
		ID:           "qwen-code",
		Provider:     "alibaba",
		Name:         "Qwen Code",
		Capabilities: caps,
	}

	adapter := &QwenCodeAgent{
		BaseAdapter: NewBaseAdapter(id, "qwen-code", config, caps, modelInfo),
	}
	adapter.BaseAdapter.parseResponse = adapter.parseQwenResponse
	return adapter
}

// parseQwenResponse parses Qwen Code's specific output format.
func (q *QwenCodeAgent) parseQwenResponse(raw string) (agent.Response, error) {
	resp := agent.Response{
		Content: raw,
		Metadata: map[string]string{
			"agent": "qwen-code",
		},
	}

	var jsonResp struct {
		Output     string `json:"output"`
		TokenUsage struct {
			Input  int `json:"input"`
			Output int `json:"output"`
		} `json:"token_usage"`
		Model string `json:"model"`
	}

	if err := json.Unmarshal([]byte(raw), &jsonResp); err == nil {
		if jsonResp.Output != "" {
			resp.Content = jsonResp.Output
		}
		resp.TokensUsed = jsonResp.TokenUsage.Input + jsonResp.TokenUsage.Output
		if jsonResp.Model != "" {
			resp.Metadata["model"] = jsonResp.Model
		}
	}

	parsed, err := q.parser.Parse(resp.Content)
	if err == nil {
		resp.Actions = parsed.Actions
	}

	return resp, nil
}

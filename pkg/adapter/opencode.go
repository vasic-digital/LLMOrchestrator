// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package adapter

import (
	"encoding/json"
	"fmt"

	"digital.vasic.llmorchestrator/pkg/agent"
)

// OpenCodeAgent is the adapter for the OpenCode CLI agent.
// OpenCode uses `--headless --non-interactive` mode.
type OpenCodeAgent struct {
	*BaseAdapter
}

// NewOpenCodeAgent creates a new OpenCode adapter.
func NewOpenCodeAgent(id string, config AdapterConfig) *OpenCodeAgent {
	if config.BinaryPath == "" {
		config.BinaryPath = "opencode"
	}

	// Ensure headless flags are set.
	config.Args = appendIfMissing(config.Args, "--headless")
	config.Args = appendIfMissing(config.Args, "--non-interactive")

	caps := agent.AgentCapabilities{
		Vision:    true, // via configured LLM
		Streaming: true,
		ToolUse:   true,
		MaxTokens: 128000,
		MaxImages: 10,
		Providers: []string{"openai", "anthropic", "google"},
	}

	modelInfo := agent.ModelInfo{
		ID:           "opencode",
		Provider:     "multi",
		Name:         "OpenCode",
		Capabilities: caps,
	}

	adapter := &OpenCodeAgent{
		BaseAdapter: NewBaseAdapter(id, "opencode", config, caps, modelInfo),
	}
	adapter.BaseAdapter.parseResponse = adapter.parseOpenCodeResponse
	return adapter
}

// parseOpenCodeResponse parses OpenCode's specific output format.
func (o *OpenCodeAgent) parseOpenCodeResponse(raw string) (agent.Response, error) {
	resp := agent.Response{
		Content: raw,
		Metadata: map[string]string{
			"agent": "opencode",
		},
	}

	// OpenCode may return JSON with content and tool_use fields.
	var jsonResp struct {
		Content  string `json:"content"`
		ToolUse  bool   `json:"tool_use"`
		TokensIn int    `json:"tokens_in"`
		TokenOut int    `json:"tokens_out"`
	}

	if err := json.Unmarshal([]byte(raw), &jsonResp); err == nil {
		resp.Content = jsonResp.Content
		resp.TokensUsed = jsonResp.TokensIn + jsonResp.TokenOut
		resp.Metadata["tool_use"] = fmt.Sprintf("%v", jsonResp.ToolUse)
	}

	// Parse actions from content.
	parsed, err := o.parser.Parse(resp.Content)
	if err == nil {
		resp.Actions = parsed.Actions
	}

	return resp, nil
}

// appendIfMissing appends a flag if it's not already in the slice.
func appendIfMissing(slice []string, flag string) []string {
	for _, s := range slice {
		if s == flag {
			return slice
		}
	}
	return append(slice, flag)
}

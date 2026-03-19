// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package adapter

import (
	"encoding/json"

	"digital.vasic.llmorchestrator/pkg/agent"
)

// GeminiAgent is the adapter for the Gemini CLI agent.
// Gemini uses `--non-interactive` mode.
type GeminiAgent struct {
	*BaseAdapter
}

// NewGeminiAgent creates a new Gemini adapter.
func NewGeminiAgent(id string, config AdapterConfig) *GeminiAgent {
	if config.BinaryPath == "" {
		config.BinaryPath = "gemini"
	}

	config.Args = appendIfMissing(config.Args, "--non-interactive")

	caps := agent.AgentCapabilities{
		Vision:    true, // native Gemini vision
		Streaming: true,
		ToolUse:   true,
		MaxTokens: 1000000,
		MaxImages: 16,
		Providers: []string{"google"},
	}

	modelInfo := agent.ModelInfo{
		ID:           "gemini",
		Provider:     "google",
		Name:         "Gemini",
		Capabilities: caps,
	}

	adapter := &GeminiAgent{
		BaseAdapter: NewBaseAdapter(id, "gemini", config, caps, modelInfo),
	}
	adapter.BaseAdapter.parseResponse = adapter.parseGeminiResponse
	return adapter
}

// parseGeminiResponse parses Gemini's specific output format.
func (g *GeminiAgent) parseGeminiResponse(raw string) (agent.Response, error) {
	resp := agent.Response{
		Content: raw,
		Metadata: map[string]string{
			"agent": "gemini",
		},
	}

	var jsonResp struct {
		Text         string `json:"text"`
		TokenCount   int    `json:"token_count"`
		FinishReason string `json:"finish_reason"`
	}

	if err := json.Unmarshal([]byte(raw), &jsonResp); err == nil {
		if jsonResp.Text != "" {
			resp.Content = jsonResp.Text
		}
		resp.TokensUsed = jsonResp.TokenCount
		if jsonResp.FinishReason != "" {
			resp.Metadata["finish_reason"] = jsonResp.FinishReason
		}
	}

	parsed, err := g.parser.Parse(resp.Content)
	if err == nil {
		resp.Actions = parsed.Actions
	}

	return resp, nil
}

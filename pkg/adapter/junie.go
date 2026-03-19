// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package adapter

import (
	"encoding/json"

	"digital.vasic.llmorchestrator/pkg/agent"
)

// JunieAgent is the adapter for the Junie CLI agent.
// Junie uses `--headless` mode.
type JunieAgent struct {
	*BaseAdapter
}

// NewJunieAgent creates a new Junie adapter.
func NewJunieAgent(id string, config AdapterConfig) *JunieAgent {
	if config.BinaryPath == "" {
		config.BinaryPath = "junie"
	}

	config.Args = appendIfMissing(config.Args, "--headless")

	caps := agent.AgentCapabilities{
		Vision:    true, // via configured LLM
		Streaming: true,
		ToolUse:   true,
		MaxTokens: 128000,
		MaxImages: 8,
		Providers: []string{"jetbrains"},
	}

	modelInfo := agent.ModelInfo{
		ID:           "junie",
		Provider:     "jetbrains",
		Name:         "Junie",
		Capabilities: caps,
	}

	adapter := &JunieAgent{
		BaseAdapter: NewBaseAdapter(id, "junie", config, caps, modelInfo),
	}
	adapter.BaseAdapter.parseResponse = adapter.parseJunieResponse
	return adapter
}

// parseJunieResponse parses Junie's specific output format.
func (j *JunieAgent) parseJunieResponse(raw string) (agent.Response, error) {
	resp := agent.Response{
		Content: raw,
		Metadata: map[string]string{
			"agent": "junie",
		},
	}

	var jsonResp struct {
		Response string `json:"response"`
		Status   string `json:"status"`
		Tokens   int    `json:"tokens"`
	}

	if err := json.Unmarshal([]byte(raw), &jsonResp); err == nil {
		if jsonResp.Response != "" {
			resp.Content = jsonResp.Response
		}
		resp.TokensUsed = jsonResp.Tokens
		if jsonResp.Status != "" {
			resp.Metadata["status"] = jsonResp.Status
		}
	}

	parsed, err := j.parser.Parse(resp.Content)
	if err == nil {
		resp.Actions = parsed.Actions
	}

	return resp, nil
}

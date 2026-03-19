// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package agent

import (
	"context"
	"time"
)

// Agent represents a headless CLI agent abstraction.
type Agent interface {
	// ID returns the unique identifier for this agent instance.
	ID() string
	// Name returns the agent type name (e.g., "opencode", "claude-code", "gemini", "junie", "qwen-code").
	Name() string
	// Start launches the agent process.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the agent process.
	Stop(ctx context.Context) error
	// IsRunning returns true if the agent process is currently active.
	IsRunning() bool
	// Health returns the current health status of the agent.
	Health(ctx context.Context) HealthStatus
	// Send sends a prompt and waits for a complete response (stdin/stdout pipe).
	Send(ctx context.Context, prompt string) (Response, error)
	// SendStream sends a prompt and returns a channel of streaming chunks.
	SendStream(ctx context.Context, prompt string) (<-chan StreamChunk, error)
	// SendWithAttachments sends a prompt with file attachments (file-based exchange).
	SendWithAttachments(ctx context.Context, prompt string, attachments []Attachment) (Response, error)
	// OutputDir returns the directory where this agent writes artifacts.
	OutputDir() string
	// Capabilities returns what this agent can do.
	Capabilities() AgentCapabilities
	// SupportsVision returns true if the agent supports vision/image analysis.
	SupportsVision() bool
	// ModelInfo returns LLM model information for this agent.
	ModelInfo() ModelInfo
}

// Response is the parsed result from an Agent.Send() call.
type Response struct {
	Content    string            // raw text response
	Actions    []Action          // extracted structured actions
	Metadata   map[string]string // provider-specific metadata
	TokensUsed int               // total tokens consumed
	Latency    time.Duration     // round-trip time
	Error      error             // nil if successful
}

// StreamChunk is an individual chunk from Agent.SendStream().
type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

// Attachment is a file sent with SendWithAttachments.
type Attachment struct {
	Path     string // file path
	MimeType string // e.g., "image/png"
	Name     string // display name
}

// AgentCapabilities describes what an agent can do.
type AgentCapabilities struct {
	Vision    bool
	Streaming bool
	ToolUse   bool
	MaxTokens int
	MaxImages int
	Providers []string // which LLM providers this agent supports
}

// AgentRequirements describes what a caller needs from an agent.
type AgentRequirements struct {
	NeedsVision    bool
	NeedsStreaming bool
	MinTokens      int
	PreferredAgent string // optional: prefer specific agent name
}

// HealthStatus is the result of an agent health check.
type HealthStatus struct {
	AgentID   string
	AgentName string
	Healthy   bool
	Latency   time.Duration
	Error     error
	CheckedAt time.Time
}

// Action is a structured action extracted from an LLM response.
type Action struct {
	Type       string  // "click", "type", "scroll", "navigate", "back", "home"
	Target     string  // element label or coordinates
	Value      string  // text to type, scroll amount, etc.
	Confidence float64 // 0.0 - 1.0
}

// ParsedResponse is a fully parsed agent response.
type ParsedResponse struct {
	Raw     string
	Content string
	Actions []Action
	Issues  []Issue
	JSON    map[string]any // if response contained JSON
}

// Issue represents a problem detected by an agent.
type Issue struct {
	Type        string   // "visual", "ux", "accessibility", "functional", "performance", "crash"
	Severity    string   // "critical", "high", "medium", "low"
	Title       string
	Description string
	ScreenID    string
	Evidence    []string // screenshot paths
}

// ModelInfo contains LLM model information (passed from LLMsVerifier via HelixQA).
type ModelInfo struct {
	ID           string
	Provider     string
	Name         string
	Capabilities AgentCapabilities
	Score        float64 // from LLMsVerifier
}

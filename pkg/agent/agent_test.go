// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package agent

import (
	"context"
	"testing"
	"time"
)

// mockAgent is a test double implementing the Agent interface.
type mockAgent struct {
	id        string
	name      string
	running   bool
	caps      AgentCapabilities
	info      ModelInfo
	outputDir string
	sendFn    func(ctx context.Context, prompt string) (Response, error)
	healthFn  func(ctx context.Context) HealthStatus
}

func newMockAgent(id, name string) *mockAgent {
	return &mockAgent{
		id:   id,
		name: name,
		caps: AgentCapabilities{
			Vision:    true,
			Streaming: true,
			ToolUse:   true,
			MaxTokens: 100000,
			MaxImages: 10,
		},
		info: ModelInfo{
			ID:       id,
			Provider: "mock",
			Name:     name,
		},
		outputDir: "/tmp/mock-" + id,
	}
}

func (m *mockAgent) ID() string   { return m.id }
func (m *mockAgent) Name() string { return m.name }

func (m *mockAgent) Start(ctx context.Context) error {
	m.running = true
	return nil
}

func (m *mockAgent) Stop(ctx context.Context) error {
	m.running = false
	return nil
}

func (m *mockAgent) IsRunning() bool { return m.running }

func (m *mockAgent) Health(ctx context.Context) HealthStatus {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return HealthStatus{
		AgentID:   m.id,
		AgentName: m.name,
		Healthy:   m.running,
		CheckedAt: time.Now(),
	}
}

func (m *mockAgent) Send(ctx context.Context, prompt string) (Response, error) {
	if m.sendFn != nil {
		return m.sendFn(ctx, prompt)
	}
	return Response{Content: "mock response to: " + prompt}, nil
}

func (m *mockAgent) SendStream(ctx context.Context, prompt string) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 2)
	go func() {
		defer close(ch)
		ch <- StreamChunk{Content: "chunk1"}
		ch <- StreamChunk{Content: "chunk2", Done: true}
	}()
	return ch, nil
}

func (m *mockAgent) SendWithAttachments(ctx context.Context, prompt string, attachments []Attachment) (Response, error) {
	return Response{Content: "mock response with attachments"}, nil
}

func (m *mockAgent) OutputDir() string              { return m.outputDir }
func (m *mockAgent) Capabilities() AgentCapabilities { return m.caps }
func (m *mockAgent) SupportsVision() bool            { return m.caps.Vision }
func (m *mockAgent) ModelInfo() ModelInfo             { return m.info }

// --- Unit Tests ---

func TestAgent_InterfaceCompliance(t *testing.T) {
	var _ Agent = newMockAgent("test", "test-agent")
}

func TestResponse_Fields(t *testing.T) {
	resp := Response{
		Content:    "hello",
		Actions:    []Action{{Type: "click", Target: "button"}},
		Metadata:   map[string]string{"key": "value"},
		TokensUsed: 100,
		Latency:    time.Millisecond * 50,
	}

	if resp.Content != "hello" {
		t.Errorf("expected content 'hello', got %q", resp.Content)
	}
	if len(resp.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(resp.Actions))
	}
	if resp.Actions[0].Type != "click" {
		t.Errorf("expected action type 'click', got %q", resp.Actions[0].Type)
	}
	if resp.TokensUsed != 100 {
		t.Errorf("expected 100 tokens, got %d", resp.TokensUsed)
	}
}

func TestStreamChunk_Done(t *testing.T) {
	chunk := StreamChunk{Content: "data", Done: true}
	if !chunk.Done {
		t.Error("expected Done to be true")
	}
	if chunk.Content != "data" {
		t.Errorf("expected content 'data', got %q", chunk.Content)
	}
}

func TestAttachment_Fields(t *testing.T) {
	att := Attachment{
		Path:     "/tmp/screenshot.png",
		MimeType: "image/png",
		Name:     "screenshot",
	}
	if att.Path != "/tmp/screenshot.png" {
		t.Errorf("unexpected path: %s", att.Path)
	}
	if att.MimeType != "image/png" {
		t.Errorf("unexpected mime: %s", att.MimeType)
	}
}

func TestAgentCapabilities_Vision(t *testing.T) {
	caps := AgentCapabilities{Vision: true, MaxTokens: 200000}
	if !caps.Vision {
		t.Error("expected vision to be true")
	}
	if caps.MaxTokens != 200000 {
		t.Errorf("expected 200000 tokens, got %d", caps.MaxTokens)
	}
}

func TestAgentRequirements_Matching(t *testing.T) {
	req := AgentRequirements{
		NeedsVision:    true,
		NeedsStreaming: false,
		MinTokens:      50000,
		PreferredAgent: "claude-code",
	}
	if !req.NeedsVision {
		t.Error("expected NeedsVision true")
	}
	if req.PreferredAgent != "claude-code" {
		t.Errorf("expected preferred agent claude-code, got %s", req.PreferredAgent)
	}
}

func TestHealthStatus_Healthy(t *testing.T) {
	status := HealthStatus{
		AgentID:   "agent-1",
		AgentName: "test",
		Healthy:   true,
		CheckedAt: time.Now(),
	}
	if !status.Healthy {
		t.Error("expected healthy status")
	}
	if status.Error != nil {
		t.Error("expected no error for healthy status")
	}
}

func TestHealthStatus_Unhealthy(t *testing.T) {
	status := HealthStatus{
		AgentID:   "agent-1",
		Healthy:   false,
		Error:     ErrNoAvailableAgent,
		CheckedAt: time.Now(),
	}
	if status.Healthy {
		t.Error("expected unhealthy status")
	}
	if status.Error == nil {
		t.Error("expected error for unhealthy status")
	}
}

func TestAction_Fields(t *testing.T) {
	tests := []struct {
		name   string
		action Action
	}{
		{"click", Action{Type: "click", Target: "button", Confidence: 0.95}},
		{"type", Action{Type: "type", Value: "hello world", Confidence: 0.8}},
		{"scroll", Action{Type: "scroll", Value: "down", Confidence: 0.7}},
		{"navigate", Action{Type: "navigate", Target: "settings", Confidence: 0.9}},
		{"back", Action{Type: "back", Confidence: 1.0}},
		{"home", Action{Type: "home", Confidence: 1.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.action.Type != tt.name {
				t.Errorf("expected type %q, got %q", tt.name, tt.action.Type)
			}
			if tt.action.Confidence <= 0 || tt.action.Confidence > 1.0 {
				t.Errorf("confidence %f out of range (0, 1]", tt.action.Confidence)
			}
		})
	}
}

func TestParsedResponse_Fields(t *testing.T) {
	parsed := ParsedResponse{
		Raw:     "raw output",
		Content: "cleaned content",
		Actions: []Action{{Type: "click", Target: "button"}},
		Issues:  []Issue{{Type: "visual", Severity: "medium", Title: "Bug"}},
		JSON:    map[string]any{"key": "value"},
	}

	if parsed.Raw != "raw output" {
		t.Errorf("unexpected raw: %q", parsed.Raw)
	}
	if len(parsed.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(parsed.Actions))
	}
	if len(parsed.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(parsed.Issues))
	}
}

func TestIssue_Fields(t *testing.T) {
	issue := Issue{
		Type:        "visual",
		Severity:    "high",
		Title:       "Button truncated",
		Description: "The save button text is cut off on small screens",
		ScreenID:    "settings-screen",
		Evidence:    []string{"/screenshots/001.png", "/screenshots/002.png"},
	}

	if issue.Type != "visual" {
		t.Errorf("unexpected type: %s", issue.Type)
	}
	if len(issue.Evidence) != 2 {
		t.Errorf("expected 2 evidence items, got %d", len(issue.Evidence))
	}
}

func TestModelInfo_Fields(t *testing.T) {
	info := ModelInfo{
		ID:       "claude-3-opus",
		Provider: "anthropic",
		Name:     "Claude 3 Opus",
		Score:    0.95,
	}

	if info.Provider != "anthropic" {
		t.Errorf("unexpected provider: %s", info.Provider)
	}
	if info.Score != 0.95 {
		t.Errorf("unexpected score: %f", info.Score)
	}
}

func TestMockAgent_Start_Stop(t *testing.T) {
	agent := newMockAgent("test-1", "test")

	if agent.IsRunning() {
		t.Error("agent should not be running initially")
	}

	ctx := context.Background()
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	if !agent.IsRunning() {
		t.Error("agent should be running after start")
	}

	if err := agent.Stop(ctx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	if agent.IsRunning() {
		t.Error("agent should not be running after stop")
	}
}

func TestMockAgent_Send(t *testing.T) {
	agent := newMockAgent("test-1", "test")
	ctx := context.Background()

	resp, err := agent.Send(ctx, "hello")
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if resp.Content != "mock response to: hello" {
		t.Errorf("unexpected response: %q", resp.Content)
	}
}

func TestMockAgent_SendStream(t *testing.T) {
	agent := newMockAgent("test-1", "test")
	ctx := context.Background()

	ch, err := agent.SendStream(ctx, "hello")
	if err != nil {
		t.Fatalf("send stream failed: %v", err)
	}

	var chunks []StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Content != "chunk1" {
		t.Errorf("unexpected first chunk: %q", chunks[0].Content)
	}
	if !chunks[1].Done {
		t.Error("last chunk should be done")
	}
}

func TestMockAgent_OutputDir(t *testing.T) {
	agent := newMockAgent("test-1", "test")
	if agent.OutputDir() != "/tmp/mock-test-1" {
		t.Errorf("unexpected output dir: %s", agent.OutputDir())
	}
}

func TestMockAgent_Capabilities(t *testing.T) {
	agent := newMockAgent("test-1", "test")
	caps := agent.Capabilities()
	if !caps.Vision {
		t.Error("expected vision capability")
	}
	if !caps.Streaming {
		t.Error("expected streaming capability")
	}
}

func TestMockAgent_SupportsVision(t *testing.T) {
	agent := newMockAgent("test-1", "test")
	if !agent.SupportsVision() {
		t.Error("expected vision support")
	}
}

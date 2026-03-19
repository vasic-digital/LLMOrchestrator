// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package adapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.llmorchestrator/pkg/parser"
	"digital.vasic.llmorchestrator/pkg/protocol"
)

var (
	// ErrAgentNotRunning is returned when an operation requires a running agent.
	ErrAgentNotRunning = errors.New("agent is not running")
	// ErrAgentAlreadyRunning is returned when starting an already-running agent.
	ErrAgentAlreadyRunning = errors.New("agent is already running")
	// ErrStartFailed is returned when the agent process fails to start.
	ErrStartFailed = errors.New("agent start failed")
	// ErrSendFailed is returned when sending a prompt fails.
	ErrSendFailed = errors.New("send failed")
	// ErrTimeout is returned when an operation exceeds its timeout.
	ErrTimeout = errors.New("operation timed out")
)

// AdapterConfig holds configuration for a BaseAdapter.
type AdapterConfig struct {
	// BinaryPath is the path to the CLI binary.
	BinaryPath string
	// Args are additional command-line arguments.
	Args []string
	// Env are additional environment variables.
	Env []string
	// WorkDir is the working directory for the process.
	WorkDir string
	// OutputDir is where the agent writes artifacts.
	OutputDir string
	// Timeout is the per-operation timeout.
	Timeout time.Duration
	// MaxRetries is the number of retries on failure.
	MaxRetries int
}

// BaseAdapter provides shared process management for all CLI adapters.
// Each specific adapter (OpenCode, Claude, etc.) embeds BaseAdapter and
// only implements protocol-specific parsing.
type BaseAdapter struct {
	mu        sync.Mutex
	id        string
	name      string
	config    AdapterConfig
	caps      agent.AgentCapabilities
	modelInfo agent.ModelInfo
	parser    parser.ResponseParser
	running   bool
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    bytes.Buffer
	transport *protocol.PipeTransport
	breaker   *agent.CircuitBreaker

	// parseResponse is a function provided by the specific adapter to parse
	// the raw output from the CLI process into a Response.
	parseResponse func(raw string) (agent.Response, error)
}

// NewBaseAdapter creates a new BaseAdapter.
func NewBaseAdapter(id, name string, config AdapterConfig, caps agent.AgentCapabilities, modelInfo agent.ModelInfo) *BaseAdapter {
	return &BaseAdapter{
		id:        id,
		name:      name,
		config:    config,
		caps:      caps,
		modelInfo: modelInfo,
		parser:    parser.NewParser(),
		breaker:   agent.NewCircuitBreaker(),
	}
}

// ID returns the unique agent identifier.
func (ba *BaseAdapter) ID() string { return ba.id }

// Name returns the agent type name.
func (ba *BaseAdapter) Name() string { return ba.name }

// OutputDir returns the artifact output directory.
func (ba *BaseAdapter) OutputDir() string { return ba.config.OutputDir }

// Capabilities returns agent capabilities.
func (ba *BaseAdapter) Capabilities() agent.AgentCapabilities { return ba.caps }

// SupportsVision returns true if the agent supports vision.
func (ba *BaseAdapter) SupportsVision() bool { return ba.caps.Vision }

// ModelInfo returns the model information.
func (ba *BaseAdapter) ModelInfo() agent.ModelInfo { return ba.modelInfo }

// IsRunning returns true if the agent process is active.
func (ba *BaseAdapter) IsRunning() bool {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	return ba.running
}

// Start launches the agent process.
func (ba *BaseAdapter) Start(ctx context.Context) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if ba.running {
		return ErrAgentAlreadyRunning
	}

	args := make([]string, len(ba.config.Args))
	copy(args, ba.config.Args)

	ba.cmd = exec.CommandContext(ctx, ba.config.BinaryPath, args...)
	if ba.config.WorkDir != "" {
		ba.cmd.Dir = ba.config.WorkDir
	}
	if len(ba.config.Env) > 0 {
		ba.cmd.Env = append(os.Environ(), ba.config.Env...)
	}
	ba.cmd.Stderr = &ba.stderr

	var err error
	ba.stdin, err = ba.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("%w: stdin pipe: %v", ErrStartFailed, err)
	}

	ba.stdout, err = ba.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("%w: stdout pipe: %v", ErrStartFailed, err)
	}

	if err := ba.cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", ErrStartFailed, err)
	}

	ba.transport = protocol.NewPipeTransport(ba.stdout, ba.stdin)
	ba.running = true
	ba.breaker.Reset()

	return nil
}

// Stop gracefully shuts down the agent process.
func (ba *BaseAdapter) Stop(ctx context.Context) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if !ba.running {
		return nil
	}

	// Try graceful shutdown via transport.
	if ba.transport != nil {
		_ = ba.transport.SendShutdown(ctx)
		_ = ba.transport.Close()
	}

	// Close stdin to signal EOF.
	if ba.stdin != nil {
		_ = ba.stdin.Close()
	}

	// Wait for process with timeout.
	done := make(chan error, 1)
	go func() {
		done <- ba.cmd.Wait()
	}()

	timeout := ba.config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	select {
	case <-time.After(timeout):
		// Force kill on timeout.
		if ba.cmd.Process != nil {
			_ = ba.cmd.Process.Kill()
		}
	case <-done:
	case <-ctx.Done():
		if ba.cmd.Process != nil {
			_ = ba.cmd.Process.Kill()
		}
	}

	ba.running = false
	return nil
}

// Health returns the health status of the agent.
func (ba *BaseAdapter) Health(ctx context.Context) agent.HealthStatus {
	ba.mu.Lock()
	running := ba.running
	ba.mu.Unlock()

	status := agent.HealthStatus{
		AgentID:   ba.id,
		AgentName: ba.name,
		Healthy:   running && ba.breaker.AllowRequest(),
		CheckedAt: time.Now(),
	}

	if !running {
		status.Error = ErrAgentNotRunning
	}

	return status
}

// Send sends a prompt and waits for a response.
func (ba *BaseAdapter) Send(ctx context.Context, prompt string) (agent.Response, error) {
	ba.mu.Lock()
	if !ba.running {
		ba.mu.Unlock()
		return agent.Response{}, ErrAgentNotRunning
	}
	transport := ba.transport
	ba.mu.Unlock()

	if !ba.breaker.AllowRequest() {
		return agent.Response{}, fmt.Errorf("circuit breaker open for agent %s", ba.id)
	}

	start := time.Now()

	timeout := ba.config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Send prompt.
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	if err := transport.SendPrompt(ctx, requestID, prompt, ""); err != nil {
		ba.breaker.RecordFailure()
		return agent.Response{}, fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	// Read response.
	msg, err := transport.Receive(ctx)
	if err != nil {
		ba.breaker.RecordFailure()
		return agent.Response{}, fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	latency := time.Since(start)
	ba.breaker.RecordSuccess()

	// Use custom parseResponse if set, otherwise use default.
	if ba.parseResponse != nil {
		resp, err := ba.parseResponse(msg.Content)
		if err == nil {
			resp.Latency = latency
		}
		return resp, err
	}

	// Default response.
	resp := agent.Response{
		Content:  msg.Content,
		Latency:  latency,
		Metadata: msg.Metadata,
	}

	// Parse actions from the response.
	parsed, err := ba.parser.Parse(msg.Content)
	if err == nil {
		resp.Actions = parsed.Actions
	}

	return resp, nil
}

// SendStream sends a prompt and returns a channel of streaming chunks.
func (ba *BaseAdapter) SendStream(ctx context.Context, prompt string) (<-chan agent.StreamChunk, error) {
	ba.mu.Lock()
	if !ba.running {
		ba.mu.Unlock()
		return nil, ErrAgentNotRunning
	}
	transport := ba.transport
	ba.mu.Unlock()

	if !ba.breaker.AllowRequest() {
		return nil, fmt.Errorf("circuit breaker open for agent %s", ba.id)
	}

	// Send prompt.
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	if err := transport.SendPrompt(ctx, requestID, prompt, ""); err != nil {
		ba.breaker.RecordFailure()
		return nil, fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	ch := make(chan agent.StreamChunk, 16)
	go func() {
		defer close(ch)
		for {
			msg, err := transport.Receive(ctx)
			if err != nil {
				ba.breaker.RecordFailure()
				ch <- agent.StreamChunk{Error: err, Done: true}
				return
			}

			chunk := agent.StreamChunk{
				Content: msg.Content,
			}

			if msg.Type == protocol.MessageTypeError {
				chunk.Error = errors.New(msg.Error)
				chunk.Done = true
				ba.breaker.RecordFailure()
				ch <- chunk
				return
			}

			// Check if this is the final chunk.
			if done, ok := msg.Metadata["done"]; ok && done == "true" {
				chunk.Done = true
				ba.breaker.RecordSuccess()
				ch <- chunk
				return
			}

			ch <- chunk
		}
	}()

	return ch, nil
}

// SendWithAttachments sends a prompt with file attachments.
func (ba *BaseAdapter) SendWithAttachments(ctx context.Context, prompt string, attachments []agent.Attachment) (agent.Response, error) {
	ba.mu.Lock()
	if !ba.running {
		ba.mu.Unlock()
		return agent.Response{}, ErrAgentNotRunning
	}
	transport := ba.transport
	ba.mu.Unlock()

	if !ba.breaker.AllowRequest() {
		return agent.Response{}, fmt.Errorf("circuit breaker open for agent %s", ba.id)
	}

	start := time.Now()

	timeout := ba.config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the prompt with attachment references.
	fullPrompt := prompt
	for _, att := range attachments {
		fullPrompt += fmt.Sprintf("\n[Attachment: %s (%s) at %s]", att.Name, att.MimeType, att.Path)
	}

	// If there's an image attachment, use image_path.
	imagePath := ""
	for _, att := range attachments {
		if isImageMIME(att.MimeType) {
			imagePath = att.Path
			break
		}
	}

	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	if err := transport.SendPrompt(ctx, requestID, fullPrompt, imagePath); err != nil {
		ba.breaker.RecordFailure()
		return agent.Response{}, fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	msg, err := transport.Receive(ctx)
	if err != nil {
		ba.breaker.RecordFailure()
		return agent.Response{}, fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	latency := time.Since(start)
	ba.breaker.RecordSuccess()

	resp := agent.Response{
		Content:  msg.Content,
		Latency:  latency,
		Metadata: msg.Metadata,
	}

	parsed, err := ba.parser.Parse(msg.Content)
	if err == nil {
		resp.Actions = parsed.Actions
	}

	return resp, nil
}

// CircuitBreaker returns the agent's circuit breaker.
func (ba *BaseAdapter) CircuitBreaker() *agent.CircuitBreaker {
	return ba.breaker
}

// isImageMIME returns true if the MIME type is an image type.
func isImageMIME(mime string) bool {
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp":
		return true
	default:
		return false
	}
}

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPipeTransport_NewPipeTransport(t *testing.T) {
	r := strings.NewReader("")
	w := &bytes.Buffer{}
	pt := NewPipeTransport(r, w)
	if pt == nil {
		t.Fatal("NewPipeTransport returned nil")
	}
	if pt.IsClosed() {
		t.Error("new transport should not be closed")
	}
}

func TestPipeTransport_Send(t *testing.T) {
	var buf bytes.Buffer
	pt := NewPipeTransport(strings.NewReader(""), &buf)

	ctx := context.Background()
	msg := PipeMessage{
		Type:    MessageTypePrompt,
		Content: "Hello, agent!",
	}

	err := pt.Send(ctx, msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify JSON was written.
	var received PipeMessage
	if err := json.Unmarshal(buf.Bytes(), &received); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if received.Type != MessageTypePrompt {
		t.Errorf("expected prompt type, got %v", received.Type)
	}
	if received.Content != "Hello, agent!" {
		t.Errorf("unexpected content: %q", received.Content)
	}
}

func TestPipeTransport_Receive(t *testing.T) {
	msg := PipeMessage{
		Type:      MessageTypeResponse,
		Content:   "I see a login form",
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')

	pt := NewPipeTransport(bytes.NewReader(data), &bytes.Buffer{})

	ctx := context.Background()
	received, err := pt.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}
	if received.Type != MessageTypeResponse {
		t.Errorf("expected response type, got %v", received.Type)
	}
	if received.Content != "I see a login form" {
		t.Errorf("unexpected content: %q", received.Content)
	}
}

func TestPipeTransport_SendAndReceive_RoundTrip(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	ptSend := NewPipeTransport(strings.NewReader(""), w)
	ptRecv := NewPipeTransport(r, &bytes.Buffer{})

	ctx := context.Background()

	go func() {
		msg := PipeMessage{
			Type:    MessageTypePrompt,
			Content: "test prompt",
		}
		_ = ptSend.Send(ctx, msg)
	}()

	received, err := ptRecv.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}
	if received.Content != "test prompt" {
		t.Errorf("unexpected content: %q", received.Content)
	}
}

func TestPipeTransport_SendPrompt(t *testing.T) {
	var buf bytes.Buffer
	pt := NewPipeTransport(strings.NewReader(""), &buf)

	ctx := context.Background()
	err := pt.SendPrompt(ctx, "req-1", "What do you see?", "/tmp/screenshot.png")
	if err != nil {
		t.Fatalf("SendPrompt failed: %v", err)
	}

	var received PipeMessage
	json.Unmarshal(buf.Bytes(), &received)
	if received.Type != MessageTypePrompt {
		t.Errorf("expected prompt type, got %v", received.Type)
	}
	if received.RequestID != "req-1" {
		t.Errorf("unexpected request ID: %s", received.RequestID)
	}
	if received.ImagePath != "/tmp/screenshot.png" {
		t.Errorf("unexpected image path: %s", received.ImagePath)
	}
}

func TestPipeTransport_SendShutdown(t *testing.T) {
	var buf bytes.Buffer
	pt := NewPipeTransport(strings.NewReader(""), &buf)

	ctx := context.Background()
	err := pt.SendShutdown(ctx)
	if err != nil {
		t.Fatalf("SendShutdown failed: %v", err)
	}

	var received PipeMessage
	json.Unmarshal(buf.Bytes(), &received)
	if received.Type != MessageTypeShutdown {
		t.Errorf("expected shutdown type, got %v", received.Type)
	}
}

func TestPipeTransport_Close(t *testing.T) {
	pt := NewPipeTransport(strings.NewReader(""), &bytes.Buffer{})

	err := pt.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !pt.IsClosed() {
		t.Error("expected transport to be closed")
	}

	// Operations on closed transport should fail.
	ctx := context.Background()
	err = pt.Send(ctx, PipeMessage{Type: MessageTypePrompt})
	if err != ErrTransportClosed {
		t.Errorf("expected ErrTransportClosed, got: %v", err)
	}
}

func TestPipeTransport_Receive_ContextCancellation(t *testing.T) {
	// Use a reader that never returns data.
	r, _ := io.Pipe()
	defer r.Close()
	pt := NewPipeTransport(r, &bytes.Buffer{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := pt.Receive(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestPipeTransport_Receive_InvalidJSON(t *testing.T) {
	data := []byte("not valid json\n")
	pt := NewPipeTransport(bytes.NewReader(data), &bytes.Buffer{})

	ctx := context.Background()
	_, err := pt.Receive(ctx)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid message") {
		t.Errorf("expected invalid message error, got: %v", err)
	}
}

func TestPipeTransport_Receive_EOF(t *testing.T) {
	pt := NewPipeTransport(strings.NewReader(""), &bytes.Buffer{})

	ctx := context.Background()
	_, err := pt.Receive(ctx)
	if err != io.EOF {
		t.Errorf("expected EOF, got: %v", err)
	}
}

func TestPipeTransport_Send_ContextCancelled(t *testing.T) {
	pt := NewPipeTransport(strings.NewReader(""), &bytes.Buffer{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pt.Send(ctx, PipeMessage{Type: MessageTypePrompt})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestPipeTransport_Send_SetsTimestamp(t *testing.T) {
	var buf bytes.Buffer
	pt := NewPipeTransport(strings.NewReader(""), &buf)

	before := time.Now()
	ctx := context.Background()
	_ = pt.Send(ctx, PipeMessage{Type: MessageTypeHeartbeat})
	after := time.Now()

	var received PipeMessage
	json.Unmarshal(buf.Bytes(), &received)
	if received.Timestamp.Before(before) || received.Timestamp.After(after) {
		t.Error("timestamp should be set to current time")
	}
}

func TestPipeTransport_MultipleMessages(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()

	pt := NewPipeTransport(r, &bytes.Buffer{})
	ctx := context.Background()

	// Write multiple messages.
	go func() {
		defer w.Close()
		for i := 0; i < 3; i++ {
			msg := PipeMessage{
				Type:    MessageTypeResponse,
				Content: "msg-" + string(rune('0'+i)),
			}
			data, _ := json.Marshal(msg)
			data = append(data, '\n')
			w.Write(data)
		}
	}()

	for i := 0; i < 3; i++ {
		msg, err := pt.Receive(ctx)
		if err != nil {
			t.Fatalf("Receive %d failed: %v", i, err)
		}
		expected := "msg-" + string(rune('0'+i))
		if msg.Content != expected {
			t.Errorf("expected %q, got %q", expected, msg.Content)
		}
	}
}

func TestPipeMessage_WithActions(t *testing.T) {
	msg := PipeMessage{
		Type:    MessageTypeResponse,
		Content: "I see a form",
		Actions: []ActionPayload{
			{Type: "click", Target: "submit_button", Confidence: 0.95},
			{Type: "type", Target: "username_field", Value: "test@example.com"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded PipeMessage
	json.Unmarshal(data, &decoded)

	if len(decoded.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(decoded.Actions))
	}
	if decoded.Actions[0].Target != "submit_button" {
		t.Errorf("unexpected target: %s", decoded.Actions[0].Target)
	}
}

func TestPipeMessage_WithMetadata(t *testing.T) {
	msg := PipeMessage{
		Type: MessageTypeResponse,
		Metadata: map[string]string{
			"model":  "claude-3-opus",
			"tokens": "1500",
		},
	}

	data, _ := json.Marshal(msg)
	var decoded PipeMessage
	json.Unmarshal(data, &decoded)

	if decoded.Metadata["model"] != "claude-3-opus" {
		t.Errorf("unexpected model: %s", decoded.Metadata["model"])
	}
}

func TestMessageType_Constants(t *testing.T) {
	tests := []struct {
		typ      MessageType
		expected string
	}{
		{MessageTypePrompt, "prompt"},
		{MessageTypeResponse, "response"},
		{MessageTypeError, "error"},
		{MessageTypeHeartbeat, "heartbeat"},
		{MessageTypeShutdown, "shutdown"},
	}

	for _, tt := range tests {
		if string(tt.typ) != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, string(tt.typ))
		}
	}
}

// --- Stress Tests ---

func TestPipeTransport_Stress_ConcurrentSend(t *testing.T) {
	var buf bytes.Buffer
	pt := NewPipeTransport(strings.NewReader(""), &buf)

	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := PipeMessage{
				Type:    MessageTypePrompt,
				Content: "concurrent message",
			}
			_ = pt.Send(ctx, msg)
		}(i)
	}

	wg.Wait()
	// Should not panic or produce corrupt output.
}

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package protocol

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegration_PipeAndFileTransport(t *testing.T) {
	// Simulate a session using both pipe and file transports.
	dir := t.TempDir()
	ft, err := NewFileTransport(dir)
	if err != nil {
		t.Fatalf("NewFileTransport failed: %v", err)
	}

	// Write instructions to inbox (file-based).
	instruction := FileMessage{
		ID:      "inst-001",
		Type:    "instruction",
		Content: "Navigate to settings screen",
		Metadata: map[string]string{
			"platform": "android",
			"priority": "high",
		},
	}
	if err := ft.WriteToInbox(instruction); err != nil {
		t.Fatalf("WriteToInbox failed: %v", err)
	}

	// Write shared artifacts.
	featureMap := []byte(`{"features": ["settings", "editor", "preview"]}`)
	if err := ft.WriteSharedFile("feature-map.json", featureMap); err != nil {
		t.Fatalf("WriteSharedFile failed: %v", err)
	}

	// Simulate agent reading instructions.
	messages, _ := ft.ReadFromInbox()
	if len(messages) != 1 || messages[0].Content != "Navigate to settings screen" {
		t.Fatal("instruction not properly read")
	}

	// Simulate agent writing result to outbox.
	result := FileMessage{
		ID:      "result-001",
		Type:    "result",
		Content: "Settings screen verified",
		Attachments: []FileAttachment{
			{Path: "screenshots/settings.png", MimeType: "image/png", Name: "settings", Size: 50000},
		},
	}
	if err := ft.WriteToOutbox(result); err != nil {
		t.Fatalf("WriteToOutbox failed: %v", err)
	}

	// Verify result.
	results, _ := ft.ReadFromOutbox()
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	if results[0].Content != "Settings screen verified" {
		t.Errorf("unexpected result: %s", results[0].Content)
	}
}

func TestIntegration_PipeBidirectional(t *testing.T) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()

	// Simulate controller and agent communicating bidirectionally.
	controller := NewPipeTransport(r1, w2)
	agentPipe := NewPipeTransport(r2, w1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 2)

	// Agent goroutine: receives prompt, sends response.
	go func() {
		msg, err := agentPipe.Receive(ctx)
		if err != nil {
			errCh <- err
			return
		}
		if msg.Content != "What do you see?" {
			errCh <- fmt.Errorf("unexpected prompt: %q", msg.Content)
			return
		}
		resp := PipeMessage{
			Type:    MessageTypeResponse,
			Content: "I see a login form",
			Actions: []ActionPayload{
				{Type: "click", Target: "username_field"},
			},
		}
		errCh <- agentPipe.Send(ctx, resp)
	}()

	// Controller sends prompt (blocks until agent reads).
	if err := controller.SendPrompt(ctx, "req-1", "What do you see?", ""); err != nil {
		t.Fatalf("controller send failed: %v", err)
	}

	// Controller reads response (blocks until agent writes).
	resp, err := controller.Receive(ctx)
	if err != nil {
		t.Fatalf("controller receive failed: %v", err)
	}
	if resp.Content != "I see a login form" {
		t.Errorf("unexpected response: %q", resp.Content)
	}
	if len(resp.Actions) != 1 || resp.Actions[0].Type != "click" {
		t.Error("expected click action in response")
	}

	// Wait for agent goroutine.
	if agentErr := <-errCh; agentErr != nil {
		t.Errorf("agent error: %v", agentErr)
	}

	r1.Close()
	w1.Close()
	r2.Close()
	w2.Close()
}

func TestIntegration_FileTransport_MultipleRounds(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	// Round 1.
	_ = ft.WriteToInbox(FileMessage{ID: "round1-inst", Type: "instruction", Content: "First task"})
	_ = ft.WriteToOutbox(FileMessage{ID: "round1-result", Type: "result", Content: "First done"})

	// Round 2.
	_ = ft.WriteToInbox(FileMessage{ID: "round2-inst", Type: "instruction", Content: "Second task"})
	_ = ft.WriteToOutbox(FileMessage{ID: "round2-result", Type: "result", Content: "Second done"})

	inbox, _ := ft.ReadFromInbox()
	outbox, _ := ft.ReadFromOutbox()

	if len(inbox) != 2 {
		t.Errorf("expected 2 inbox messages, got %d", len(inbox))
	}
	if len(outbox) != 2 {
		t.Errorf("expected 2 outbox messages, got %d", len(outbox))
	}
}

func TestIntegration_FileTransport_SharedArtifacts(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	// Write multiple shared artifacts.
	artifacts := map[string]string{
		"feature-map.json": `{"features": []}`,
		"config.yaml":      "platforms:\n  - android\n  - desktop",
		"docs-summary.txt": "Application documentation summary",
	}

	for name, content := range artifacts {
		if err := ft.WriteSharedFile(name, []byte(content)); err != nil {
			t.Fatalf("WriteSharedFile(%s) failed: %v", name, err)
		}
	}

	files, _ := ft.ListSharedFiles()
	if len(files) != 3 {
		t.Errorf("expected 3 shared files, got %d", len(files))
	}

	// Read back.
	for name, expected := range artifacts {
		data, err := ft.ReadSharedFile(name)
		if err != nil {
			t.Errorf("ReadSharedFile(%s) failed: %v", name, err)
			continue
		}
		if string(data) != expected {
			t.Errorf("unexpected content for %s: %q", name, string(data))
		}
	}
}

func TestIntegration_FileTransport_SessionLifecycle(t *testing.T) {
	baseDir := t.TempDir()
	sessionDir := filepath.Join(baseDir, "session-test-001")

	// Create.
	ft, err := NewFileTransport(sessionDir)
	if err != nil {
		t.Fatalf("NewFileTransport failed: %v", err)
	}

	// Use.
	_ = ft.WriteToInbox(FileMessage{ID: "msg-1", Type: "instruction"})
	_ = ft.WriteSharedFile("test.txt", []byte("data"))

	// Cleanup.
	err = ft.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}

func TestIntegration_PipeTransport_Heartbeat(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	sender := NewPipeTransport(nil, w)
	receiver := NewPipeTransport(r, nil)

	ctx := context.Background()

	go func() {
		msg := PipeMessage{
			Type:      MessageTypeHeartbeat,
			Timestamp: time.Now(),
		}
		_ = sender.Send(ctx, msg)
	}()

	msg, err := receiver.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive heartbeat failed: %v", err)
	}
	if msg.Type != MessageTypeHeartbeat {
		t.Errorf("expected heartbeat, got %v", msg.Type)
	}
}

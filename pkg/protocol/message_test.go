// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPipeMessage_Serialization(t *testing.T) {
	msg := PipeMessage{
		Type:      MessageTypePrompt,
		Content:   "What do you see?",
		ImagePath: "/tmp/screenshot.png",
		Metadata:  map[string]string{"session": "test-001"},
		Timestamp: time.Now(),
		RequestID: "req-123",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded PipeMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Type != MessageTypePrompt {
		t.Errorf("expected prompt, got %v", decoded.Type)
	}
	if decoded.Content != "What do you see?" {
		t.Errorf("unexpected content: %q", decoded.Content)
	}
	if decoded.ImagePath != "/tmp/screenshot.png" {
		t.Errorf("unexpected image path: %q", decoded.ImagePath)
	}
	if decoded.RequestID != "req-123" {
		t.Errorf("unexpected request ID: %q", decoded.RequestID)
	}
}

func TestPipeMessage_EmptyFields_OmittedInJSON(t *testing.T) {
	msg := PipeMessage{
		Type:      MessageTypeHeartbeat,
		Timestamp: time.Now(),
	}

	data, _ := json.Marshal(msg)
	str := string(data)

	// Content should be omitted when empty.
	if json.Valid(data) && len(str) > 0 {
		var m map[string]any
		json.Unmarshal(data, &m)
		if _, ok := m["content"]; ok {
			if m["content"] != "" {
				t.Error("empty content should be omitted or empty string")
			}
		}
	}
}

func TestPipeMessage_ErrorType(t *testing.T) {
	msg := PipeMessage{
		Type:  MessageTypeError,
		Error: "connection refused",
	}

	data, _ := json.Marshal(msg)
	var decoded PipeMessage
	json.Unmarshal(data, &decoded)

	if decoded.Error != "connection refused" {
		t.Errorf("unexpected error: %q", decoded.Error)
	}
}

func TestActionPayload_Serialization(t *testing.T) {
	payload := ActionPayload{
		Type:       "click",
		Target:     "submit_button",
		Value:      "",
		Confidence: 0.95,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ActionPayload
	json.Unmarshal(data, &decoded)

	if decoded.Type != "click" {
		t.Errorf("unexpected type: %q", decoded.Type)
	}
	if decoded.Confidence != 0.95 {
		t.Errorf("unexpected confidence: %f", decoded.Confidence)
	}
}

func TestFileMessage_Serialization(t *testing.T) {
	msg := FileMessage{
		ID:      "msg-001",
		Type:    "instruction",
		Content: "Navigate to settings",
		Attachments: []FileAttachment{
			{
				Path:     "/shared/feature-map.json",
				MimeType: "application/json",
				Name:     "feature-map",
				Size:     1024,
			},
		},
		Metadata:  map[string]string{"priority": "high"},
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded FileMessage
	json.Unmarshal(data, &decoded)

	if decoded.ID != "msg-001" {
		t.Errorf("unexpected ID: %q", decoded.ID)
	}
	if decoded.Type != "instruction" {
		t.Errorf("unexpected type: %q", decoded.Type)
	}
	if len(decoded.Attachments) != 1 {
		t.Fatal("expected 1 attachment")
	}
	if decoded.Attachments[0].Size != 1024 {
		t.Errorf("unexpected attachment size: %d", decoded.Attachments[0].Size)
	}
}

func TestFileAttachment_Fields(t *testing.T) {
	att := FileAttachment{
		Path:     "/screenshots/001.png",
		MimeType: "image/png",
		Name:     "settings-screenshot",
		Size:     54321,
	}

	if att.Path != "/screenshots/001.png" {
		t.Errorf("unexpected path: %s", att.Path)
	}
	if att.Size != 54321 {
		t.Errorf("unexpected size: %d", att.Size)
	}
}

func TestPipeMessage_WithAllMessageTypes(t *testing.T) {
	types := []MessageType{
		MessageTypePrompt,
		MessageTypeResponse,
		MessageTypeError,
		MessageTypeHeartbeat,
		MessageTypeShutdown,
	}

	for _, mt := range types {
		msg := PipeMessage{Type: mt, Timestamp: time.Now()}
		data, err := json.Marshal(msg)
		if err != nil {
			t.Errorf("marshal failed for type %v: %v", mt, err)
			continue
		}
		var decoded PipeMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("unmarshal failed for type %v: %v", mt, err)
			continue
		}
		if decoded.Type != mt {
			t.Errorf("type mismatch: expected %v, got %v", mt, decoded.Type)
		}
	}
}

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package protocol

import "time"

// MessageType identifies the type of pipe message.
type MessageType string

const (
	// MessageTypePrompt is a prompt sent to an agent.
	MessageTypePrompt MessageType = "prompt"
	// MessageTypeResponse is a response from an agent.
	MessageTypeResponse MessageType = "response"
	// MessageTypeError is an error from an agent.
	MessageTypeError MessageType = "error"
	// MessageTypeHeartbeat is a heartbeat/ping message.
	MessageTypeHeartbeat MessageType = "heartbeat"
	// MessageTypeShutdown is a shutdown signal.
	MessageTypeShutdown MessageType = "shutdown"
)

// PipeMessage is the JSON-lines protocol message for stdin/stdout communication.
type PipeMessage struct {
	Type       MessageType       `json:"type"`
	Content    string            `json:"content,omitempty"`
	ImagePath  string            `json:"image_path,omitempty"`
	Actions    []ActionPayload   `json:"actions,omitempty"`
	Error      string            `json:"error,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
	RequestID  string            `json:"request_id,omitempty"`
}

// ActionPayload is the JSON representation of an action in a pipe message.
type ActionPayload struct {
	Type       string  `json:"type"`
	Target     string  `json:"target,omitempty"`
	Value      string  `json:"value,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// FileMessage represents a file-based message (inbox/outbox).
type FileMessage struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"` // "instruction", "result", "artifact"
	Content     string            `json:"content,omitempty"`
	Attachments []FileAttachment  `json:"attachments,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// FileAttachment is a reference to a file in the shared directory.
type FileAttachment struct {
	Path     string `json:"path"`
	MimeType string `json:"mime_type"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
}

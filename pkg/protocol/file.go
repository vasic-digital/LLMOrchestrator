// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	// ErrSessionDirNotExist is returned when the session directory does not exist.
	ErrSessionDirNotExist = errors.New("session directory does not exist")
	// ErrPathTraversal is returned when a path contains traversal sequences.
	ErrPathTraversal = errors.New("path traversal detected")
)

// FileTransport manages file-based communication using inbox/outbox/shared directories.
type FileTransport struct {
	sessionDir string
	inboxDir   string
	outboxDir  string
	sharedDir  string
}

// NewFileTransport creates a new FileTransport for the given session directory.
// It creates inbox/, outbox/, and shared/ subdirectories if they don't exist.
func NewFileTransport(sessionDir string) (*FileTransport, error) {
	if sessionDir == "" {
		return nil, errors.New("session directory cannot be empty")
	}

	ft := &FileTransport{
		sessionDir: sessionDir,
		inboxDir:   filepath.Join(sessionDir, "inbox"),
		outboxDir:  filepath.Join(sessionDir, "outbox"),
		sharedDir:  filepath.Join(sessionDir, "shared"),
	}

	for _, dir := range []string{ft.inboxDir, ft.outboxDir, ft.sharedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return ft, nil
}

// WriteToInbox writes a message to the inbox directory.
func (ft *FileTransport) WriteToInbox(msg FileMessage) error {
	return ft.writeMessage(ft.inboxDir, msg)
}

// WriteToOutbox writes a message to the outbox directory.
func (ft *FileTransport) WriteToOutbox(msg FileMessage) error {
	return ft.writeMessage(ft.outboxDir, msg)
}

// ReadFromInbox reads all messages from the inbox directory.
func (ft *FileTransport) ReadFromInbox() ([]FileMessage, error) {
	return ft.readMessages(ft.inboxDir)
}

// ReadFromOutbox reads all messages from the outbox directory.
func (ft *FileTransport) ReadFromOutbox() ([]FileMessage, error) {
	return ft.readMessages(ft.outboxDir)
}

// WriteSharedFile writes arbitrary data to the shared directory.
func (ft *FileTransport) WriteSharedFile(name string, data []byte) error {
	if err := validatePath(name); err != nil {
		return err
	}
	path := filepath.Join(ft.sharedDir, name)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ReadSharedFile reads a file from the shared directory.
func (ft *FileTransport) ReadSharedFile(name string) ([]byte, error) {
	if err := validatePath(name); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(ft.sharedDir, name))
}

// ListSharedFiles returns all files in the shared directory.
func (ft *FileTransport) ListSharedFiles() ([]string, error) {
	entries, err := os.ReadDir(ft.sharedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

// SessionDir returns the session directory path.
func (ft *FileTransport) SessionDir() string {
	return ft.sessionDir
}

// InboxDir returns the inbox directory path.
func (ft *FileTransport) InboxDir() string {
	return ft.inboxDir
}

// OutboxDir returns the outbox directory path.
func (ft *FileTransport) OutboxDir() string {
	return ft.outboxDir
}

// SharedDir returns the shared directory path.
func (ft *FileTransport) SharedDir() string {
	return ft.sharedDir
}

// Cleanup removes the entire session directory.
func (ft *FileTransport) Cleanup() error {
	return os.RemoveAll(ft.sessionDir)
}

// writeMessage writes a FileMessage to a directory as a JSON file.
func (ft *FileTransport) writeMessage(dir string, msg FileMessage) error {
	if msg.ID == "" {
		return errors.New("message ID cannot be empty")
	}
	if err := validatePath(msg.ID); err != nil {
		return err
	}

	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	data, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	filename := filepath.Join(dir, msg.ID+".json")
	return os.WriteFile(filename, data, 0644)
}

// readMessages reads all FileMessage JSON files from a directory.
func (ft *FileTransport) readMessages(dir string) ([]FileMessage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var messages []FileMessage
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var msg FileMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	// Sort by creation time.
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})

	return messages, nil
}

// validatePath checks for path traversal attempts.
func validatePath(name string) error {
	if strings.Contains(name, "..") {
		return ErrPathTraversal
	}
	if filepath.IsAbs(name) {
		return ErrPathTraversal
	}
	cleaned := filepath.Clean(name)
	if strings.HasPrefix(cleaned, "..") {
		return ErrPathTraversal
	}
	return nil
}

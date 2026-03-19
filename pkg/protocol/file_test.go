// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileTransport_NewFileTransport(t *testing.T) {
	dir := t.TempDir()
	ft, err := NewFileTransport(dir)
	if err != nil {
		t.Fatalf("NewFileTransport failed: %v", err)
	}

	// Verify directories were created.
	for _, subdir := range []string{"inbox", "outbox", "shared"} {
		path := filepath.Join(dir, subdir)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("directory %s not created: %v", subdir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", subdir)
		}
	}

	if ft.SessionDir() != dir {
		t.Errorf("unexpected session dir: %s", ft.SessionDir())
	}
}

func TestFileTransport_NewFileTransport_EmptyDir(t *testing.T) {
	_, err := NewFileTransport("")
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
}

func TestFileTransport_WriteReadInbox(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	msg := FileMessage{
		ID:      "msg-001",
		Type:    "instruction",
		Content: "Navigate to settings",
	}

	err := ft.WriteToInbox(msg)
	if err != nil {
		t.Fatalf("WriteToInbox failed: %v", err)
	}

	messages, err := ft.ReadFromInbox()
	if err != nil {
		t.Fatalf("ReadFromInbox failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].ID != "msg-001" {
		t.Errorf("unexpected ID: %s", messages[0].ID)
	}
	if messages[0].Content != "Navigate to settings" {
		t.Errorf("unexpected content: %s", messages[0].Content)
	}
}

func TestFileTransport_WriteReadOutbox(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	msg := FileMessage{
		ID:      "result-001",
		Type:    "result",
		Content: "Settings screen verified",
	}

	err := ft.WriteToOutbox(msg)
	if err != nil {
		t.Fatalf("WriteToOutbox failed: %v", err)
	}

	messages, err := ft.ReadFromOutbox()
	if err != nil {
		t.Fatalf("ReadFromOutbox failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Type != "result" {
		t.Errorf("unexpected type: %s", messages[0].Type)
	}
}

func TestFileTransport_MultipleMessages_SortedByTime(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	now := time.Now()
	msgs := []FileMessage{
		{ID: "msg-3", Type: "instruction", Content: "third", CreatedAt: now.Add(2 * time.Second)},
		{ID: "msg-1", Type: "instruction", Content: "first", CreatedAt: now},
		{ID: "msg-2", Type: "instruction", Content: "second", CreatedAt: now.Add(1 * time.Second)},
	}

	for _, m := range msgs {
		if err := ft.WriteToInbox(m); err != nil {
			t.Fatalf("WriteToInbox failed: %v", err)
		}
	}

	messages, _ := ft.ReadFromInbox()
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	if messages[0].Content != "first" {
		t.Errorf("expected first message to be 'first', got %q", messages[0].Content)
	}
	if messages[2].Content != "third" {
		t.Errorf("expected last message to be 'third', got %q", messages[2].Content)
	}
}

func TestFileTransport_WriteSharedFile(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	data := []byte("feature map contents")
	err := ft.WriteSharedFile("feature-map.json", data)
	if err != nil {
		t.Fatalf("WriteSharedFile failed: %v", err)
	}

	read, err := ft.ReadSharedFile("feature-map.json")
	if err != nil {
		t.Fatalf("ReadSharedFile failed: %v", err)
	}
	if string(read) != "feature map contents" {
		t.Errorf("unexpected content: %q", string(read))
	}
}

func TestFileTransport_ListSharedFiles(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	_ = ft.WriteSharedFile("file1.json", []byte("data1"))
	_ = ft.WriteSharedFile("file2.txt", []byte("data2"))

	files, err := ft.ListSharedFiles()
	if err != nil {
		t.Fatalf("ListSharedFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestFileTransport_Cleanup(t *testing.T) {
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, "session-123")
	ft, _ := NewFileTransport(sessionDir)

	_ = ft.WriteSharedFile("test.txt", []byte("data"))

	err := ft.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("session directory should be removed after cleanup")
	}
}

func TestFileTransport_EmptyMessageID(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	msg := FileMessage{Type: "instruction", Content: "test"}
	err := ft.WriteToInbox(msg)
	if err == nil {
		t.Fatal("expected error for empty message ID")
	}
}

func TestFileTransport_WithAttachments(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	msg := FileMessage{
		ID:   "msg-with-att",
		Type: "result",
		Attachments: []FileAttachment{
			{
				Path:     "/screenshots/001.png",
				MimeType: "image/png",
				Name:     "screenshot",
				Size:     12345,
			},
		},
	}

	err := ft.WriteToOutbox(msg)
	if err != nil {
		t.Fatalf("WriteToOutbox failed: %v", err)
	}

	messages, _ := ft.ReadFromOutbox()
	if len(messages) != 1 {
		t.Fatal("expected 1 message")
	}
	if len(messages[0].Attachments) != 1 {
		t.Fatal("expected 1 attachment")
	}
	if messages[0].Attachments[0].Name != "screenshot" {
		t.Errorf("unexpected attachment name: %s", messages[0].Attachments[0].Name)
	}
}

func TestFileTransport_ReadFromEmptyDir(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	messages, err := ft.ReadFromInbox()
	if err != nil {
		t.Fatalf("ReadFromInbox failed: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestFileTransport_DirPaths(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	if ft.InboxDir() != filepath.Join(dir, "inbox") {
		t.Errorf("unexpected inbox dir: %s", ft.InboxDir())
	}
	if ft.OutboxDir() != filepath.Join(dir, "outbox") {
		t.Errorf("unexpected outbox dir: %s", ft.OutboxDir())
	}
	if ft.SharedDir() != filepath.Join(dir, "shared") {
		t.Errorf("unexpected shared dir: %s", ft.SharedDir())
	}
}

func TestFileTransport_WithMetadata(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	msg := FileMessage{
		ID:   "msg-meta",
		Type: "instruction",
		Metadata: map[string]string{
			"priority": "high",
			"platform": "android",
		},
	}

	_ = ft.WriteToInbox(msg)
	messages, _ := ft.ReadFromInbox()

	if messages[0].Metadata["priority"] != "high" {
		t.Errorf("unexpected priority: %s", messages[0].Metadata["priority"])
	}
}

// --- Security Tests ---

func TestFileTransport_Security_PathTraversal_SharedFile(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	err := ft.WriteSharedFile("../../../etc/passwd", []byte("malicious"))
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if err != ErrPathTraversal {
		t.Errorf("expected ErrPathTraversal, got: %v", err)
	}
}

func TestFileTransport_Security_PathTraversal_Read(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	_, err := ft.ReadSharedFile("../../etc/shadow")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestFileTransport_Security_PathTraversal_MessageID(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	msg := FileMessage{
		ID:      "../../../tmp/evil",
		Type:    "instruction",
		Content: "malicious",
	}

	err := ft.WriteToInbox(msg)
	if err == nil {
		t.Fatal("expected error for path traversal in message ID")
	}
}

func TestFileTransport_Security_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	err := ft.WriteSharedFile("/etc/passwd", []byte("malicious"))
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestValidatePath_Safe(t *testing.T) {
	tests := []string{
		"file.json",
		"subdir/file.txt",
		"a/b/c.json",
	}
	for _, path := range tests {
		if err := validatePath(path); err != nil {
			t.Errorf("validatePath(%q) = %v, want nil", path, err)
		}
	}
}

func TestValidatePath_Traversal(t *testing.T) {
	tests := []string{
		"../etc/passwd",
		"../../tmp/file",
		"foo/../../../bar",
	}
	for _, path := range tests {
		if err := validatePath(path); err == nil {
			t.Errorf("validatePath(%q) = nil, want error", path)
		}
	}
}

func TestValidatePath_Absolute(t *testing.T) {
	if err := validatePath("/etc/passwd"); err == nil {
		t.Error("expected error for absolute path")
	}
}

func TestFileTransport_NonJSONFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	// Write a non-JSON file directly to inbox.
	inboxDir := filepath.Join(dir, "inbox")
	os.WriteFile(filepath.Join(inboxDir, "notes.txt"), []byte("not json"), 0644)

	messages, err := ft.ReadFromInbox()
	if err != nil {
		t.Fatalf("ReadFromInbox failed: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages (non-json ignored), got %d", len(messages))
	}
}

func TestFileTransport_MalformedJSONIgnored(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	// Write malformed JSON.
	inboxDir := filepath.Join(dir, "inbox")
	os.WriteFile(filepath.Join(inboxDir, "bad.json"), []byte("{invalid json"), 0644)

	messages, err := ft.ReadFromInbox()
	if err != nil {
		t.Fatalf("ReadFromInbox failed: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages (malformed ignored), got %d", len(messages))
	}
}

func TestFileTransport_Security_DotDotInName(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"normal file", "report.json", false},
		{"subdirectory", "sub/report.json", false},
		{"dotdot", "../escape.json", true},
		{"middle dotdot", "foo/../bar.json", true},
		{"absolute", "/etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for path %q", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for path %q: %v", tt.path, err)
			}
		})
	}
}

func TestFileMessage_TimestampAutoSet(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	msg := FileMessage{
		ID:   "auto-ts",
		Type: "instruction",
	}

	before := time.Now()
	_ = ft.WriteToInbox(msg)
	after := time.Now()

	messages, _ := ft.ReadFromInbox()
	if len(messages) != 1 {
		t.Fatal("expected 1 message")
	}

	ts := messages[0].CreatedAt
	if ts.Before(before) || ts.After(after) {
		t.Error("auto-set timestamp should be close to current time")
	}
}

func TestFileMessage_PreserveTimestamp(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	fixedTime := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	msg := FileMessage{
		ID:        "fixed-ts",
		Type:      "instruction",
		CreatedAt: fixedTime,
	}

	_ = ft.WriteToInbox(msg)
	messages, _ := ft.ReadFromInbox()

	if !messages[0].CreatedAt.Equal(fixedTime) {
		t.Errorf("expected timestamp %v, got %v", fixedTime, messages[0].CreatedAt)
	}
}

func TestFileTransport_LargeContent(t *testing.T) {
	dir := t.TempDir()
	ft, _ := NewFileTransport(dir)

	// Create a large content string.
	content := strings.Repeat("x", 100000)
	msg := FileMessage{
		ID:      "large",
		Type:    "result",
		Content: content,
	}

	err := ft.WriteToOutbox(msg)
	if err != nil {
		t.Fatalf("WriteToOutbox failed for large content: %v", err)
	}

	messages, _ := ft.ReadFromOutbox()
	if len(messages) != 1 {
		t.Fatal("expected 1 message")
	}
	if len(messages[0].Content) != 100000 {
		t.Errorf("expected 100000 chars, got %d", len(messages[0].Content))
	}
}

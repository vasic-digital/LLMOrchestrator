// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	// ErrTransportClosed is returned when operations are attempted on a closed transport.
	ErrTransportClosed = errors.New("transport is closed")
	// ErrWriteFailed is returned when writing to the pipe fails.
	ErrWriteFailed = errors.New("write to pipe failed")
	// ErrReadFailed is returned when reading from the pipe fails.
	ErrReadFailed = errors.New("read from pipe failed")
	// ErrInvalidMessage is returned when a message fails to parse.
	ErrInvalidMessage = errors.New("invalid message format")
)

// PipeTransport manages JSON-lines communication over stdin/stdout pipes.
type PipeTransport struct {
	mu      sync.Mutex
	reader  *bufio.Reader
	writer  io.Writer
	closed  bool
	encoder *json.Encoder
}

// NewPipeTransport creates a new PipeTransport from reader and writer.
func NewPipeTransport(reader io.Reader, writer io.Writer) *PipeTransport {
	return &PipeTransport{
		reader:  bufio.NewReader(reader),
		writer:  writer,
		encoder: json.NewEncoder(writer),
	}
}

// Send writes a PipeMessage as a JSON line.
func (pt *PipeTransport) Send(ctx context.Context, msg PipeMessage) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.closed {
		return ErrTransportClosed
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	msg.Timestamp = time.Now()

	if err := pt.encoder.Encode(msg); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteFailed, err)
	}

	return nil
}

// Receive reads and parses a single JSON-line message.
func (pt *PipeTransport) Receive(ctx context.Context) (PipeMessage, error) {
	if pt.closed {
		return PipeMessage{}, ErrTransportClosed
	}

	if err := ctx.Err(); err != nil {
		return PipeMessage{}, err
	}

	// Use a channel to make the read cancellable via context.
	type result struct {
		msg PipeMessage
		err error
	}
	ch := make(chan result, 1)

	go func() {
		line, err := pt.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				ch <- result{err: io.EOF}
			} else {
				ch <- result{err: fmt.Errorf("%w: %v", ErrReadFailed, err)}
			}
			return
		}

		var msg PipeMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			ch <- result{err: fmt.Errorf("%w: %v", ErrInvalidMessage, err)}
			return
		}

		ch <- result{msg: msg}
	}()

	select {
	case <-ctx.Done():
		return PipeMessage{}, ctx.Err()
	case r := <-ch:
		return r.msg, r.err
	}
}

// SendPrompt is a convenience method for sending a prompt message.
func (pt *PipeTransport) SendPrompt(ctx context.Context, requestID, content string, imagePath string) error {
	msg := PipeMessage{
		Type:      MessageTypePrompt,
		Content:   content,
		ImagePath: imagePath,
		RequestID: requestID,
	}
	return pt.Send(ctx, msg)
}

// SendShutdown sends a shutdown message.
func (pt *PipeTransport) SendShutdown(ctx context.Context) error {
	msg := PipeMessage{
		Type: MessageTypeShutdown,
	}
	return pt.Send(ctx, msg)
}

// Close marks the transport as closed.
func (pt *PipeTransport) Close() error {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.closed = true
	return nil
}

// IsClosed returns true if the transport has been closed.
func (pt *PipeTransport) IsClosed() bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.closed
}

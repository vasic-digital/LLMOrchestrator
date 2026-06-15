// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

// TestPipeTransport_Receive_CancellationDoesNotLoseMessage is a §11.4.115
// RED-on-broken-artifact regression guard for a genuine orchestration
// message-loss defect in PipeTransport.Receive.
//
// THE DEFECT (pre-fix): Receive spawns a goroutine that blocks on
// reader.ReadBytes('\n'). On ctx cancellation Receive returns via the
// ctx.Done() select arm, but the goroutine keeps running. When a complete
// message subsequently arrives on the pipe, the goroutine reads it and
// pushes it into the buffered result channel — where it is DROPPED, because
// the caller already returned. The next Receive call reads the FOLLOWING
// line from the stream, so the first message is silently lost / the stream
// is mis-ordered.
//
// This is the exact "orchestration losing/mis-ordering results" class:
// a JSON-lines response from an agent gets consumed-and-discarded, the
// caller never sees it, and a later Receive returns a DIFFERENT message
// than the one that was actually next on the wire.
//
// RED_MODE=1 asserts the defect is PRESENT (message #1 lost, Receive returns
// message #2). Default (RED_MODE unset/0) is the standing GREEN guard
// asserting message #1 is correctly delivered.
func TestPipeTransport_Receive_CancellationDoesNotLoseMessage(t *testing.T) {
	pr, pw := io.Pipe()
	pt := NewPipeTransport(pr, &bytes.Buffer{})

	// Encoder used to write framed JSON-lines into the pipe, exactly as a
	// real peer agent would.
	enc := json.NewEncoder(pw)

	// First Receive: cancel its context BEFORE any data is on the wire, so
	// the spawned reader goroutine is parked in ReadBytes when we cancel.
	ctx1, cancel1 := context.WithCancel(context.Background())

	firstDone := make(chan error, 1)
	go func() {
		_, err := pt.Receive(ctx1)
		firstDone <- err
	}()

	// Give the Receive goroutine time to park in ReadBytes.
	time.Sleep(30 * time.Millisecond)
	// Cancel the first Receive — caller walks away.
	cancel1()

	// The first Receive must have returned a cancellation error.
	select {
	case err := <-firstDone:
		if err == nil {
			t.Fatal("first Receive: expected cancellation error, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first Receive did not return after cancellation")
	}

	// NOW the peer sends two distinct messages, message #1 FIRST.
	// Writes go in a goroutine because io.Pipe writes block until a reader
	// consumes them — the orphaned goroutine from the first Receive is the
	// reader that (buggily) consumes message #1 into its dead channel.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = enc.Encode(PipeMessage{Type: MessageTypeResponse, Content: "MSG-1", RequestID: "req-1"})
		_ = enc.Encode(PipeMessage{Type: MessageTypeResponse, Content: "MSG-2", RequestID: "req-2"})
	}()

	// Settle: let the orphan goroutine (if any) consume whatever it grabs
	// from the wire before the fresh Receive runs. A correct transport has
	// NO orphan consuming the wire, so MSG-1 stays next-on-wire.
	time.Sleep(200 * time.Millisecond)

	// A fresh, healthy Receive. The NEXT message on the wire is MSG-1, so a
	// correct transport MUST return MSG-1 here.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	got, err := pt.Receive(ctx2)
	if err != nil {
		t.Fatalf("second Receive returned error: %v", err)
	}
	wg.Wait()

	if os.Getenv("RED_MODE") == "1" {
		// Pre-fix: assert the DEFECT is real — MSG-1 was eaten by the orphan
		// goroutine, so the caller incorrectly receives MSG-2.
		if got.Content != "MSG-2" {
			t.Fatalf("RED_MODE: expected to reproduce message-loss (Content=MSG-2, MSG-1 lost), but got Content=%q — defect not reproduced", got.Content)
		}
		t.Logf("RED reproduced: cancelled Receive's orphan goroutine consumed MSG-1; next Receive wrongly returned %q", got.Content)
	} else {
		// Post-fix: MSG-1 must be delivered in order, never lost.
		if got.Content != "MSG-1" {
			t.Fatalf("message lost/mis-ordered: next Receive must return MSG-1, got %q", got.Content)
		}
	}
}

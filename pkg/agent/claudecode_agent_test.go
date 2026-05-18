// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------
// Round-66 §11.4 — ClaudeCodeAgent real-wiring unit tests.
//
// Each test that depends on a binary stages a mock `claude` shell
// script in t.TempDir() and prepends that directory to PATH for the
// scope of the subtest. This is the same Go test pattern round-64
// used for OpenCodeAgent (see opencode_agent_test.go): real os/exec,
// real subprocess, real stdout/stderr, real exit codes — nothing
// about the bridge is mocked, only the CLI binary is replaced with
// a script that mimics Claude Code's surface (`claude --print
// <prompt>` echoes the prompt; `claude --version` returns a version
// string).
//
// This pattern keeps the production code under test (NewClaudeCodeAgent
// + Send + Health) honest per CONST-050(A) — production code never
// imports test mocks; the test substitutes the BINARY, not the agent.
// ---------------------------------------------------------------------

// stageMockClaude writes a shell script at <dir>/claude that reacts
// to `--version` / `--print` arguments. body lets the test inject
// custom behaviour (sleep, exit non-zero, etc.).
//
// Returns the full path to the staged script. The script is chmod 0755.
//
// Linux/macOS only — Windows is intentionally skipped (mirrors the
// round-64 opencode test SKIP-OK marker pattern).
func stageMockClaude(t *testing.T, dir string, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #LLMORCHESTRATOR-CLAUDECODE-ROUND66-WIN — mock claude shell script is POSIX-only")
	}
	path := filepath.Join(dir, "claude")
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("stage mock claude: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------
// NewClaudeCodeAgent — constructor tests
// ---------------------------------------------------------------------

func TestNewClaudeCodeAgent_NoBinaryOnPath_ReturnsBinaryNotFound(t *testing.T) {
	emptyDir := t.TempDir()
	// Reuse round-64's isolatePath helper from opencode_agent_test.go.
	isolatePath(t, emptyDir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err == nil {
		t.Fatal("NewClaudeCodeAgent with empty PATH returned nil error — ErrClaudeCodeBinaryNotFound regression")
	}
	if a != nil {
		t.Errorf("NewClaudeCodeAgent returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrClaudeCodeBinaryNotFound) {
		t.Errorf("errors.Is(err, ErrClaudeCodeBinaryNotFound) = false; err = %v", err)
	}
}

func TestNewClaudeCodeAgent_ExplicitBinaryMissing_ReturnsBinaryNotFound(t *testing.T) {
	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{Binary: "/nonexistent/path/claude"})
	if err == nil {
		t.Fatal("NewClaudeCodeAgent with bogus Binary returned nil error")
	}
	if a != nil {
		t.Errorf("NewClaudeCodeAgent returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrClaudeCodeBinaryNotFound) {
		t.Errorf("errors.Is(err, ErrClaudeCodeBinaryNotFound) = false; err = %v", err)
	}
}

func TestNewClaudeCodeAgent_DefaultBinary_FoundOnPath_OK(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockClaude(t, dir, `exit 0`)
	prependPath(t, dir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent (default Binary, staged PATH): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("NewClaudeCodeAgent returned nil agent + nil error — CONTRACT-bluff")
	}
	if a.Name() != "claudecode" {
		t.Errorf("Name() = %q, want %q", a.Name(), "claudecode")
	}
	if a.ID() == "" {
		t.Error("ID() returned empty string")
	}
	// Interface compliance — the *ClaudeCodeAgent MUST satisfy Agent so
	// SimpleAgentPool can hand it back from Acquire.
	var _ Agent = a
}

// ---------------------------------------------------------------------
// Send — real subprocess invocation
// ---------------------------------------------------------------------

func TestClaudeCodeAgent_Send_SuccessfulInvocation(t *testing.T) {
	dir := t.TempDir()
	// Mock script: `claude --print <message>` echoes a canned response
	// that references the message so the test can verify wire-through.
	_ = stageMockClaude(t, dir, `
if [ "$1" = "--print" ]; then
  shift
  echo "CLAUDECODE-CANNED-RESPONSE: $*"
  exit 0
fi
echo "unknown args: $*" >&2
exit 2
`)
	prependPath(t, dir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent: %v", err)
	}

	resp, err := a.Send(context.Background(), "what is 2+2?")
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "CLAUDECODE-CANNED-RESPONSE") {
		t.Errorf("Send response missing canned marker; got: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "what is 2+2?") {
		t.Errorf("Send response missing wired-through prompt; got: %q", resp.Content)
	}
	if resp.Latency <= 0 {
		t.Errorf("Send response Latency = %v, want > 0", resp.Latency)
	}
}

func TestClaudeCodeAgent_Send_ShortPromptFlagOverride_OK(t *testing.T) {
	dir := t.TempDir()
	// Mock script that ONLY recognises `-p` — proves the PromptFlag
	// override actually wires through to argv (default is --print).
	_ = stageMockClaude(t, dir, `
if [ "$1" = "-p" ]; then
  shift
  echo "CLAUDECODE-SHORT-FLAG: $*"
  exit 0
fi
echo "expected -p, got: $*" >&2
exit 3
`)
	prependPath(t, dir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{PromptFlag: "-p"})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent: %v", err)
	}
	resp, err := a.Send(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "CLAUDECODE-SHORT-FLAG") {
		t.Errorf("Send response missing short-flag marker; got: %q", resp.Content)
	}
}

func TestClaudeCodeAgent_Send_NonZeroExit_ReturnsInvocationFailed(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockClaude(t, dir, `
echo "synthetic claude stderr" >&2
exit 1
`)
	prependPath(t, dir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent: %v", err)
	}

	resp, err := a.Send(context.Background(), "anything")
	if err == nil {
		t.Fatal("Send returned nil error despite mock exit 1 — CONTRACT-bluff regression")
	}
	if !errors.Is(err, ErrClaudeCodeInvocationFailed) {
		t.Errorf("errors.Is(err, ErrClaudeCodeInvocationFailed) = false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "synthetic claude stderr") {
		t.Errorf("Send error should surface captured stderr; got: %v", err)
	}
	// The exec.ExitError MUST still be reachable via errors.As so
	// callers can extract the exit code.
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("errors.As(err, *exec.ExitError) = false; err = %v", err)
	} else if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}
	if resp.Error == nil {
		t.Error("Response.Error not populated on failure")
	}
}

func TestClaudeCodeAgent_Send_ContextCancel_ReturnsCtxErr(t *testing.T) {
	dir := t.TempDir()
	// Mock script: sleeps long enough that the test's ctx will cancel
	// before it exits. This proves the production code path honors
	// ctx via exec.CommandContext + process-group SIGKILL (setProcessGroup
	// / killProcessGroup, shared with OpenCodeAgent).
	_ = stageMockClaude(t, dir, `sleep 30`)
	prependPath(t, dir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = a.Send(ctx, "ignored")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Send returned nil error despite ctx cancellation — CONTRACT-bluff regression")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("err did not match context.DeadlineExceeded or context.Canceled; got: %v", err)
	}
	// SIGKILL via exec.CommandContext should reap within a few hundred ms,
	// not the full 30s the script would sleep otherwise.
	if elapsed > 5*time.Second {
		t.Errorf("Send took %v after ctx deadline 100ms — exec.CommandContext did not SIGKILL the child", elapsed)
	}
}

// ---------------------------------------------------------------------
// SendWithAttachments — file-flag handling
// ---------------------------------------------------------------------

func TestClaudeCodeAgent_SendWithAttachments_WiresFileFlags(t *testing.T) {
	dir := t.TempDir()
	// Mock script: dumps full argv to stdout so the test can verify
	// every --file flag landed in the correct slot.
	_ = stageMockClaude(t, dir, `
printf 'ARGV:'
for arg in "$@"; do
  printf ' %s' "$arg"
done
printf '\n'
exit 0
`)
	prependPath(t, dir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent: %v", err)
	}
	attachments := []Attachment{
		{Path: "/tmp/a.txt"},
		{Path: "/tmp/b.txt", Name: "file_abc"},
		{Path: "" /* skipped */},
	}
	resp, err := a.SendWithAttachments(context.Background(), "summarise", attachments)
	if err != nil {
		t.Fatalf("SendWithAttachments: %v", err)
	}
	if !strings.Contains(resp.Content, "--file /tmp/a.txt") {
		t.Errorf("argv missing bare --file <path>; got: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "--file file_abc:/tmp/b.txt") {
		t.Errorf("argv missing --file <name>:<path>; got: %q", resp.Content)
	}
	if strings.Contains(resp.Content, "--file ''") || strings.Contains(resp.Content, "--file \n") {
		t.Errorf("argv contains empty --file from skipped attachment; got: %q", resp.Content)
	}
	if !strings.HasSuffix(strings.TrimRight(resp.Content, "\n"), "summarise") {
		t.Errorf("argv should end with prompt; got: %q", resp.Content)
	}
}

// ---------------------------------------------------------------------
// SendStream — typed not-yet-wired error
// ---------------------------------------------------------------------

func TestClaudeCodeAgent_SendStream_ReturnsTypedNotWired(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockClaude(t, dir, `exit 0`)
	prependPath(t, dir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent: %v", err)
	}
	ch, err := a.SendStream(context.Background(), "ignored")
	if err == nil {
		t.Fatal("SendStream returned nil error — anti-bluff regression (must return typed not-wired error per CONST-050(A))")
	}
	if ch != nil {
		t.Errorf("SendStream returned non-nil channel alongside not-wired error: %v", ch)
	}
	if !strings.Contains(err.Error(), "SendStream not yet wired") {
		t.Errorf("SendStream error message missing not-wired narrative; got: %v", err)
	}
}

// ---------------------------------------------------------------------
// Health — `claude --version` probe
// ---------------------------------------------------------------------

func TestClaudeCodeAgent_Health_VersionPasses(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockClaude(t, dir, `
if [ "$1" = "--version" ]; then
  echo "claude v0.0.0-mock"
  exit 0
fi
exit 2
`)
	prependPath(t, dir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent: %v", err)
	}

	status := a.Health(context.Background())
	if !status.Healthy {
		t.Errorf("Health = unhealthy; err = %v", status.Error)
	}
	if status.AgentName != "claudecode" {
		t.Errorf("Health.AgentName = %q, want %q", status.AgentName, "claudecode")
	}
	if status.Error != nil {
		t.Errorf("Health.Error = %v, want nil", status.Error)
	}
}

func TestClaudeCodeAgent_Health_VersionFails_Unhealthy(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockClaude(t, dir, `exit 7`)
	prependPath(t, dir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent: %v", err)
	}

	status := a.Health(context.Background())
	if status.Healthy {
		t.Error("Health.Healthy = true despite mock exit 7 — CONTRACT-bluff regression")
	}
	if status.Error == nil {
		t.Error("Health.Error nil despite mock failure")
	}
}

// ---------------------------------------------------------------------
// Start / Stop / IsRunning
// ---------------------------------------------------------------------

func TestClaudeCodeAgent_StartStop_IsRunning(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockClaude(t, dir, `exit 0`)
	prependPath(t, dir)

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent: %v", err)
	}
	if a.IsRunning() {
		t.Error("IsRunning() = true on fresh agent, want false")
	}
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !a.IsRunning() {
		t.Error("IsRunning() = false after Start, want true")
	}
	if err := a.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if a.IsRunning() {
		t.Error("IsRunning() = true after Stop, want false")
	}
}

// ---------------------------------------------------------------------
// Builder integration tests — round-66 narrowed sentinels
// ---------------------------------------------------------------------

func TestClaudeCodeClientBuilderFromConfig_ZeroConfig_ReturnsNotConfigured(t *testing.T) {
	b := ClaudeCodeClientBuilderFromConfig(ClaudeCodeBuilderConfig{})
	a, err := b(context.Background())
	if err == nil {
		t.Fatal("ClaudeCodeClientBuilderFromConfig(zero) returned nil error — ErrClaudeCodeClientNotConfigured regression")
	}
	if a != nil {
		t.Errorf("ClaudeCodeClientBuilderFromConfig returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrClaudeCodeClientNotConfigured) {
		t.Errorf("errors.Is(err, ErrClaudeCodeClientNotConfigured) = false; err = %v", err)
	}
}

func TestClaudeCodeClientBuilderFromConfig_ValidConfig_BuildsAgent(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockClaude(t, dir, `exit 0`)
	prependPath(t, dir)

	b := ClaudeCodeClientBuilderFromConfig(ClaudeCodeBuilderConfig{Binary: "claude"})
	a, err := b(context.Background())
	if err != nil {
		t.Fatalf("ClaudeCodeClientBuilderFromConfig(valid): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("ClaudeCodeClientBuilderFromConfig returned nil agent + nil error")
	}
	if a.Name() != "claudecode" {
		t.Errorf("agent.Name() = %q, want %q", a.Name(), "claudecode")
	}
}

func TestClaudeCodeClientBuilder_BinaryOnPath_BuildsRealAgent(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockClaude(t, dir, `exit 0`)
	prependPath(t, dir)

	b := ClaudeCodeClientBuilder(&PoolConfig{Size: 1})
	a, err := b(context.Background())
	if err != nil {
		t.Fatalf("ClaudeCodeClientBuilder({Size:1}, claude on PATH): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("ClaudeCodeClientBuilder returned nil agent + nil error — round-66 wiring regression")
	}
	if _, ok := a.(*ClaudeCodeAgent); !ok {
		t.Errorf("ClaudeCodeClientBuilder returned %T, want *ClaudeCodeAgent", a)
	}
}

// ---------------------------------------------------------------------
// SimpleAgentPool end-to-end — round-66 real Agent through real pool
// ---------------------------------------------------------------------

func TestNewClaudeCodePool_WithRealBuilder_AcquireReturnsRealAgent(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockClaude(t, dir, `exit 0`)
	prependPath(t, dir)

	pool, err := NewClaudeCodePool(&PoolConfig{Size: 2})
	if err != nil {
		t.Fatalf("NewClaudeCodePool: %v", err)
	}

	a, err := pool.Acquire(context.Background(), AgentRequirements{})
	if err != nil {
		t.Fatalf("Acquire: unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("Acquire returned nil agent + nil error — CONTRACT-bluff")
	}
	if _, ok := a.(*ClaudeCodeAgent); !ok {
		t.Errorf("pool.Acquire returned %T, want *ClaudeCodeAgent", a)
	}
	if a.Name() != "claudecode" {
		t.Errorf("agent.Name() = %q, want %q", a.Name(), "claudecode")
	}
}

func TestNewClaudeCodePool_NoBinaryOnPath_AcquireSurfacesBinaryNotFound(t *testing.T) {
	emptyDir := t.TempDir()
	isolatePath(t, emptyDir)

	pool, err := NewClaudeCodePool(&PoolConfig{Size: 1})
	if err != nil {
		t.Fatalf("NewClaudeCodePool: %v", err)
	}

	a, err := pool.Acquire(context.Background(), AgentRequirements{})
	if err == nil {
		t.Fatal("Acquire returned nil error despite missing binary — round-66 wiring regression")
	}
	if a != nil {
		t.Errorf("Acquire returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrClaudeCodeBinaryNotFound) {
		t.Errorf("errors.Is(err, ErrClaudeCodeBinaryNotFound) = false; err = %v", err)
	}
}

// ---------------------------------------------------------------------
// Round-66 sentinel distinguishability — paired with round-60 + round-64
// ---------------------------------------------------------------------

// TestRound66Sentinels_AreDistinct — disambiguation invariant.
// Round-66 introduces 3 new sentinels (ErrClaudeCodeBinaryNotFound,
// ErrClaudeCodeClientNotConfigured, ErrClaudeCodeInvocationFailed) on
// top of round-60's 5 builder-wired sentinels and round-64's 3
// opencode-specific sentinels. All 11 MUST be distinct under
// errors.Is so callers can disambiguate which provider's wiring is in
// which state (binary missing vs config missing vs invocation failed
// vs the three non-wired providers still pending).
func TestRound66Sentinels_AreDistinct(t *testing.T) {
	sentinels := []error{
		// Round-66 (this round)
		ErrClaudeCodeBinaryNotFound,
		ErrClaudeCodeClientNotConfigured,
		ErrClaudeCodeInvocationFailed,
		// Round-64 (opencode wiring)
		ErrOpenCodeBinaryNotFound,
		ErrOpenCodeClientNotConfigured,
		ErrOpenCodeInvocationFailed,
		// Round-60 (narrowed builder sentinels)
		ErrOpenCodeClientNotWired,
		ErrClaudeCodeClientNotWired,
		ErrGeminiClientNotWired,
		ErrJunieClientNotWired,
		ErrQwenCodeClientNotWired,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("sentinel %d (%v) incorrectly matches sentinel %d (%v)", i, a, j, b)
			}
		}
	}
}

// ---------------------------------------------------------------------
// Real-binary integration test — runs against the actual installed
// `claude` CLI if present, otherwise SKIP per §11.4.1 SKIP-OK rules.
// ---------------------------------------------------------------------

func TestClaudeCodeAgent_RealBinary_VersionRoundtripsOK(t *testing.T) {
	if _, err := exec.LookPath(DefaultClaudeCodeBinary); err != nil {
		t.Skip("SKIP-OK: #LLMORCHESTRATOR-CLAUDECODE-REAL-ROUND66 — `claude` not installed on PATH; install Claude Code from https://docs.anthropic.com/claude-code to exercise the real-binary path")
	}

	a, err := NewClaudeCodeAgent(ClaudeCodeAgentConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCodeAgent (real binary): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status := a.Health(ctx)
	if !status.Healthy {
		t.Errorf("Health (real binary): unhealthy; err = %v", status.Error)
	}
	if status.Latency <= 0 {
		t.Errorf("Health.Latency = %v, want > 0", status.Latency)
	}
}

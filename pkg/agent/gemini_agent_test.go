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
// Round-69 §11.4 — GeminiAgent real-wiring unit tests.
//
// Each test that depends on a binary stages a mock `gemini` shell
// script in t.TempDir() and prepends that directory to PATH for the
// scope of the subtest. This is the same Go test pattern round-64 +
// round-66 used for OpenCodeAgent / ClaudeCodeAgent: real os/exec,
// real subprocess, real stdout/stderr, real exit codes — nothing
// about the bridge is mocked, only the CLI binary is replaced with a
// script that mimics Gemini CLI's surface (`gemini -p <prompt>` echoes
// the prompt; `gemini --version` returns a version string).
//
// This pattern keeps the production code under test (NewGeminiAgent +
// Send + Health) honest per CONST-050(A) — production code never
// imports test mocks; the test substitutes the BINARY, not the agent.
// ---------------------------------------------------------------------

// stageMockGemini writes a shell script at <dir>/gemini that reacts
// to `--version` / `-p` arguments. body lets the test inject custom
// behaviour (sleep, exit non-zero, etc.).
//
// Returns the full path to the staged script. The script is chmod 0755.
//
// Linux/macOS only — Windows is intentionally skipped (mirrors the
// round-64 / round-66 SKIP-OK marker pattern).
func stageMockGemini(t *testing.T, dir string, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #LLMORCHESTRATOR-GEMINI-ROUND69-WIN — mock gemini shell script is POSIX-only")
	}
	path := filepath.Join(dir, "gemini")
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("stage mock gemini: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------
// NewGeminiAgent — constructor tests
// ---------------------------------------------------------------------

func TestNewGeminiAgent_NoBinaryOnPath_ReturnsBinaryNotFound(t *testing.T) {
	emptyDir := t.TempDir()
	// Reuse round-64's isolatePath helper from opencode_agent_test.go.
	isolatePath(t, emptyDir)

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err == nil {
		t.Fatal("NewGeminiAgent with empty PATH returned nil error — ErrGeminiBinaryNotFound regression")
	}
	if a != nil {
		t.Errorf("NewGeminiAgent returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrGeminiBinaryNotFound) {
		t.Errorf("errors.Is(err, ErrGeminiBinaryNotFound) = false; err = %v", err)
	}
}

func TestNewGeminiAgent_ExplicitBinaryMissing_ReturnsBinaryNotFound(t *testing.T) {
	a, err := NewGeminiAgent(GeminiAgentConfig{Binary: "/nonexistent/path/gemini"})
	if err == nil {
		t.Fatal("NewGeminiAgent with bogus Binary returned nil error")
	}
	if a != nil {
		t.Errorf("NewGeminiAgent returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrGeminiBinaryNotFound) {
		t.Errorf("errors.Is(err, ErrGeminiBinaryNotFound) = false; err = %v", err)
	}
}

func TestNewGeminiAgent_DefaultBinary_FoundOnPath_OK(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockGemini(t, dir, `exit 0`)
	prependPath(t, dir)

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAgent (default Binary, staged PATH): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("NewGeminiAgent returned nil agent + nil error — CONTRACT-bluff")
	}
	if a.Name() != "gemini" {
		t.Errorf("Name() = %q, want %q", a.Name(), "gemini")
	}
	if a.ID() == "" {
		t.Error("ID() returned empty string")
	}
	// Interface compliance — the *GeminiAgent MUST satisfy Agent so
	// SimpleAgentPool can hand it back from Acquire.
	var _ Agent = a
}

// ---------------------------------------------------------------------
// Send — real subprocess invocation
// ---------------------------------------------------------------------

func TestGeminiAgent_Send_SuccessfulInvocation(t *testing.T) {
	dir := t.TempDir()
	// Mock script: `gemini -p <message>` echoes a canned response that
	// references the message so the test can verify wire-through.
	_ = stageMockGemini(t, dir, `
if [ "$1" = "-p" ]; then
  shift
  echo "GEMINI-CANNED-RESPONSE: $*"
  exit 0
fi
echo "unknown args: $*" >&2
exit 2
`)
	prependPath(t, dir)

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAgent: %v", err)
	}

	resp, err := a.Send(context.Background(), "what is 2+2?")
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "GEMINI-CANNED-RESPONSE") {
		t.Errorf("Send response missing canned marker; got: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "what is 2+2?") {
		t.Errorf("Send response missing wired-through prompt; got: %q", resp.Content)
	}
	if resp.Latency <= 0 {
		t.Errorf("Send response Latency = %v, want > 0", resp.Latency)
	}
}

func TestGeminiAgent_Send_LongPromptFlagOverride_OK(t *testing.T) {
	dir := t.TempDir()
	// Mock script that ONLY recognises `--prompt` — proves the
	// PromptFlag override actually wires through to argv (default is -p).
	_ = stageMockGemini(t, dir, `
if [ "$1" = "--prompt" ]; then
  shift
  echo "GEMINI-LONG-FLAG: $*"
  exit 0
fi
echo "expected --prompt, got: $*" >&2
exit 3
`)
	prependPath(t, dir)

	a, err := NewGeminiAgent(GeminiAgentConfig{PromptFlag: "--prompt"})
	if err != nil {
		t.Fatalf("NewGeminiAgent: %v", err)
	}
	resp, err := a.Send(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "GEMINI-LONG-FLAG") {
		t.Errorf("Send response missing long-flag marker; got: %q", resp.Content)
	}
}

func TestGeminiAgent_Send_NonZeroExit_ReturnsInvocationFailed(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockGemini(t, dir, `
echo "synthetic gemini stderr" >&2
exit 1
`)
	prependPath(t, dir)

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAgent: %v", err)
	}

	resp, err := a.Send(context.Background(), "anything")
	if err == nil {
		t.Fatal("Send returned nil error despite mock exit 1 — CONTRACT-bluff regression")
	}
	if !errors.Is(err, ErrGeminiInvocationFailed) {
		t.Errorf("errors.Is(err, ErrGeminiInvocationFailed) = false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "synthetic gemini stderr") {
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

func TestGeminiAgent_Send_ContextCancel_ReturnsCtxErr(t *testing.T) {
	dir := t.TempDir()
	// Mock script: sleeps long enough that the test's ctx will cancel
	// before it exits. This proves the production code path honors ctx
	// via exec.CommandContext + process-group SIGKILL (setProcessGroup
	// / killProcessGroup, shared with OpenCodeAgent + ClaudeCodeAgent).
	_ = stageMockGemini(t, dir, `sleep 30`)
	prependPath(t, dir)

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAgent: %v", err)
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

func TestGeminiAgent_SendWithAttachments_WiresIncludeDirsFlags(t *testing.T) {
	dir := t.TempDir()
	// Mock script: dumps full argv to stdout so the test can verify
	// every --include-directories flag landed in the correct slot.
	_ = stageMockGemini(t, dir, `
printf 'ARGV:'
for arg in "$@"; do
  printf ' %s' "$arg"
done
printf '\n'
exit 0
`)
	prependPath(t, dir)

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAgent: %v", err)
	}
	attachments := []Attachment{
		{Path: "/tmp/a"},
		{Path: "/tmp/b", Name: "dir_b"},
		{Path: "" /* skipped */},
	}
	resp, err := a.SendWithAttachments(context.Background(), "summarise", attachments)
	if err != nil {
		t.Fatalf("SendWithAttachments: %v", err)
	}
	if !strings.Contains(resp.Content, "--include-directories /tmp/a") {
		t.Errorf("argv missing --include-directories /tmp/a; got: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "--include-directories /tmp/b") {
		t.Errorf("argv missing --include-directories /tmp/b; got: %q", resp.Content)
	}
	if strings.Contains(resp.Content, "--include-directories \n") || strings.Contains(resp.Content, "--include-directories ''") {
		t.Errorf("argv contains empty --include-directories from skipped attachment; got: %q", resp.Content)
	}
	if !strings.HasSuffix(strings.TrimRight(resp.Content, "\n"), "summarise") {
		t.Errorf("argv should end with prompt; got: %q", resp.Content)
	}
}

// ---------------------------------------------------------------------
// SendStream — typed not-yet-wired error
// ---------------------------------------------------------------------

func TestGeminiAgent_SendStream_ReturnsTypedNotWired(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockGemini(t, dir, `exit 0`)
	prependPath(t, dir)

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAgent: %v", err)
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
// Health — `gemini --version` probe
// ---------------------------------------------------------------------

func TestGeminiAgent_Health_VersionPasses(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockGemini(t, dir, `
if [ "$1" = "--version" ]; then
  echo "0.0.0-mock"
  exit 0
fi
exit 2
`)
	prependPath(t, dir)

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAgent: %v", err)
	}

	status := a.Health(context.Background())
	if !status.Healthy {
		t.Errorf("Health = unhealthy; err = %v", status.Error)
	}
	if status.AgentName != "gemini" {
		t.Errorf("Health.AgentName = %q, want %q", status.AgentName, "gemini")
	}
	if status.Error != nil {
		t.Errorf("Health.Error = %v, want nil", status.Error)
	}
}

func TestGeminiAgent_Health_VersionFails_Unhealthy(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockGemini(t, dir, `exit 7`)
	prependPath(t, dir)

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAgent: %v", err)
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

func TestGeminiAgent_StartStop_IsRunning(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockGemini(t, dir, `exit 0`)
	prependPath(t, dir)

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAgent: %v", err)
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
// Builder integration tests — round-69 narrowed sentinels
// ---------------------------------------------------------------------

func TestGeminiClientBuilderFromConfig_ZeroConfig_ReturnsNotConfigured(t *testing.T) {
	b := GeminiClientBuilderFromConfig(GeminiBuilderConfig{})
	a, err := b(context.Background())
	if err == nil {
		t.Fatal("GeminiClientBuilderFromConfig(zero) returned nil error — ErrGeminiClientNotConfigured regression")
	}
	if a != nil {
		t.Errorf("GeminiClientBuilderFromConfig returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrGeminiClientNotConfigured) {
		t.Errorf("errors.Is(err, ErrGeminiClientNotConfigured) = false; err = %v", err)
	}
}

func TestGeminiClientBuilderFromConfig_ValidConfig_BuildsAgent(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockGemini(t, dir, `exit 0`)
	prependPath(t, dir)

	b := GeminiClientBuilderFromConfig(GeminiBuilderConfig{Binary: "gemini"})
	a, err := b(context.Background())
	if err != nil {
		t.Fatalf("GeminiClientBuilderFromConfig(valid): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("GeminiClientBuilderFromConfig returned nil agent + nil error")
	}
	if a.Name() != "gemini" {
		t.Errorf("agent.Name() = %q, want %q", a.Name(), "gemini")
	}
}

func TestGeminiClientBuilder_BinaryOnPath_BuildsRealAgent(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockGemini(t, dir, `exit 0`)
	prependPath(t, dir)

	b := GeminiClientBuilder(&PoolConfig{Size: 1})
	a, err := b(context.Background())
	if err != nil {
		t.Fatalf("GeminiClientBuilder({Size:1}, gemini on PATH): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("GeminiClientBuilder returned nil agent + nil error — round-69 wiring regression")
	}
	if _, ok := a.(*GeminiAgent); !ok {
		t.Errorf("GeminiClientBuilder returned %T, want *GeminiAgent", a)
	}
}

func TestGeminiClientBuilder_NilConfig_ReturnsNotWired(t *testing.T) {
	b := GeminiClientBuilder(nil)
	a, err := b(context.Background())
	if err == nil {
		t.Fatal("GeminiClientBuilder(nil) returned nil error — round-60 narrowed sentinel regression")
	}
	if a != nil {
		t.Errorf("GeminiClientBuilder(nil) returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrGeminiClientNotWired) {
		t.Errorf("errors.Is(err, ErrGeminiClientNotWired) = false; err = %v", err)
	}
}

// ---------------------------------------------------------------------
// SimpleAgentPool end-to-end — round-69 real Agent through real pool
// ---------------------------------------------------------------------

func TestNewGeminiPool_WithRealBuilder_AcquireReturnsRealAgent(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockGemini(t, dir, `exit 0`)
	prependPath(t, dir)

	pool, err := NewGeminiPool(&PoolConfig{Size: 2})
	if err != nil {
		t.Fatalf("NewGeminiPool: %v", err)
	}

	a, err := pool.Acquire(context.Background(), AgentRequirements{})
	if err != nil {
		t.Fatalf("Acquire: unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("Acquire returned nil agent + nil error — CONTRACT-bluff")
	}
	if _, ok := a.(*GeminiAgent); !ok {
		t.Errorf("pool.Acquire returned %T, want *GeminiAgent", a)
	}
	if a.Name() != "gemini" {
		t.Errorf("agent.Name() = %q, want %q", a.Name(), "gemini")
	}
}

func TestNewGeminiPool_NoBinaryOnPath_AcquireSurfacesBinaryNotFound(t *testing.T) {
	emptyDir := t.TempDir()
	isolatePath(t, emptyDir)

	pool, err := NewGeminiPool(&PoolConfig{Size: 1})
	if err != nil {
		t.Fatalf("NewGeminiPool: %v", err)
	}

	a, err := pool.Acquire(context.Background(), AgentRequirements{})
	if err == nil {
		t.Fatal("Acquire returned nil error despite missing binary — round-69 wiring regression")
	}
	if a != nil {
		t.Errorf("Acquire returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrGeminiBinaryNotFound) {
		t.Errorf("errors.Is(err, ErrGeminiBinaryNotFound) = false; err = %v", err)
	}
}

// ---------------------------------------------------------------------
// Round-69 sentinel distinguishability — paired with round-60 + 64 + 66
// ---------------------------------------------------------------------

// TestRound69Sentinels_AreDistinct — disambiguation invariant.
// Round-69 introduces 3 new sentinels (ErrGeminiBinaryNotFound,
// ErrGeminiClientNotConfigured, ErrGeminiInvocationFailed) on top of
// round-60's 5 builder-wired sentinels, round-64's 3 opencode-specific
// sentinels, and round-66's 3 claudecode-specific sentinels. All 14
// MUST be distinct under errors.Is so callers can disambiguate which
// provider's wiring is in which state (binary missing vs config
// missing vs invocation failed vs the two non-wired providers still
// pending).
func TestRound69Sentinels_AreDistinct(t *testing.T) {
	sentinels := []error{
		// Round-69 (this round)
		ErrGeminiBinaryNotFound,
		ErrGeminiClientNotConfigured,
		ErrGeminiInvocationFailed,
		// Round-66 (claudecode wiring)
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
// `gemini` CLI if present, otherwise SKIP per §11.4.1 SKIP-OK rules.
// ---------------------------------------------------------------------

func TestGeminiAgent_RealBinary_VersionRoundtripsOK(t *testing.T) {
	if _, err := exec.LookPath(DefaultGeminiBinary); err != nil {
		t.Skip("SKIP-OK: #LLMORCHESTRATOR-GEMINI-REAL-ROUND69 — `gemini` not installed on PATH; install Gemini CLI from https://github.com/google-gemini/gemini-cli to exercise the real-binary path")
	}

	a, err := NewGeminiAgent(GeminiAgentConfig{})
	if err != nil {
		t.Fatalf("NewGeminiAgent (real binary): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	status := a.Health(ctx)
	if !status.Healthy {
		t.Errorf("Health (real binary): unhealthy; err = %v", status.Error)
	}
	if status.Latency <= 0 {
		t.Errorf("Health.Latency = %v, want > 0", status.Latency)
	}
}

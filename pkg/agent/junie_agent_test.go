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
// Round-71 §11.4 — JunieAgent real-wiring unit tests.
//
// Each test that depends on a binary stages a mock `junie` shell
// script in t.TempDir() and prepends that directory to PATH for the
// scope of the subtest. This is the same Go test pattern round-64 /
// round-66 / round-69 used for OpenCodeAgent / ClaudeCodeAgent /
// GeminiAgent: real os/exec, real subprocess, real stdout/stderr,
// real exit codes — nothing about the bridge is mocked, only the CLI
// binary is replaced with a script that mimics Junie CLI's surface
// (`junie <prompt>` echoes the prompt; `junie --version` returns a
// version string).
//
// This pattern keeps the production code under test (NewJunieAgent +
// Send + Health) honest per CONST-050(A) — production code never
// imports test mocks; the test substitutes the BINARY, not the agent.
// ---------------------------------------------------------------------

// stageMockJunie writes a shell script at <dir>/junie that reacts
// to `--version` / positional / --task arguments. body lets the test
// inject custom behaviour (sleep, exit non-zero, etc.).
//
// Returns the full path to the staged script. The script is chmod 0755.
//
// Linux/macOS only — Windows is intentionally skipped (mirrors the
// round-64 / round-66 / round-69 SKIP-OK marker pattern).
func stageMockJunie(t *testing.T, dir string, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: #LLMORCHESTRATOR-JUNIE-ROUND71-WIN — mock junie shell script is POSIX-only")
	}
	path := filepath.Join(dir, "junie")
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("stage mock junie: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------
// NewJunieAgent — constructor tests
// ---------------------------------------------------------------------

func TestNewJunieAgent_NoBinaryOnPath_ReturnsBinaryNotFound(t *testing.T) {
	emptyDir := t.TempDir()
	// Reuse round-64's isolatePath helper from opencode_agent_test.go.
	isolatePath(t, emptyDir)

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err == nil {
		t.Fatal("NewJunieAgent with empty PATH returned nil error — ErrJunieBinaryNotFound regression")
	}
	if a != nil {
		t.Errorf("NewJunieAgent returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrJunieBinaryNotFound) {
		t.Errorf("errors.Is(err, ErrJunieBinaryNotFound) = false; err = %v", err)
	}
}

func TestNewJunieAgent_ExplicitBinaryMissing_ReturnsBinaryNotFound(t *testing.T) {
	a, err := NewJunieAgent(JunieAgentConfig{Binary: "/nonexistent/path/junie"})
	if err == nil {
		t.Fatal("NewJunieAgent with bogus Binary returned nil error")
	}
	if a != nil {
		t.Errorf("NewJunieAgent returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrJunieBinaryNotFound) {
		t.Errorf("errors.Is(err, ErrJunieBinaryNotFound) = false; err = %v", err)
	}
}

func TestNewJunieAgent_DefaultBinary_FoundOnPath_OK(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockJunie(t, dir, `exit 0`)
	prependPath(t, dir)

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err != nil {
		t.Fatalf("NewJunieAgent (default Binary, staged PATH): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("NewJunieAgent returned nil agent + nil error — CONTRACT-bluff")
	}
	if a.Name() != "junie" {
		t.Errorf("Name() = %q, want %q", a.Name(), "junie")
	}
	if a.ID() == "" {
		t.Error("ID() returned empty string")
	}
	// Interface compliance — the *JunieAgent MUST satisfy Agent so
	// SimpleAgentPool can hand it back from Acquire.
	var _ Agent = a
}

// ---------------------------------------------------------------------
// Send — real subprocess invocation
// ---------------------------------------------------------------------

func TestJunieAgent_Send_SuccessfulInvocation(t *testing.T) {
	dir := t.TempDir()
	// Mock script: `junie <message>` (positional default) echoes a
	// canned response that references the message so the test can
	// verify wire-through.
	_ = stageMockJunie(t, dir, `
echo "JUNIE-CANNED-RESPONSE: $*"
exit 0
`)
	prependPath(t, dir)

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err != nil {
		t.Fatalf("NewJunieAgent: %v", err)
	}

	resp, err := a.Send(context.Background(), "what is 2+2?")
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "JUNIE-CANNED-RESPONSE") {
		t.Errorf("Send response missing canned marker; got: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "what is 2+2?") {
		t.Errorf("Send response missing wired-through prompt; got: %q", resp.Content)
	}
	if resp.Latency <= 0 {
		t.Errorf("Send response Latency = %v, want > 0", resp.Latency)
	}
}

func TestJunieAgent_Send_TaskFlagOverride_OK(t *testing.T) {
	dir := t.TempDir()
	// Mock script that ONLY recognises a single argv slot starting
	// with `--task=` — proves the PromptFlag override actually wires
	// through to argv (default is positional).
	_ = stageMockJunie(t, dir, `
case "$1" in
  --task=*)
    echo "JUNIE-TASK-FLAG: ${1#--task=}"
    exit 0
    ;;
esac
echo "expected --task=…, got: $*" >&2
exit 3
`)
	prependPath(t, dir)

	a, err := NewJunieAgent(JunieAgentConfig{PromptFlag: "--task"})
	if err != nil {
		t.Fatalf("NewJunieAgent: %v", err)
	}
	resp, err := a.Send(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "JUNIE-TASK-FLAG") {
		t.Errorf("Send response missing task-flag marker; got: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "hello world") {
		t.Errorf("Send response missing wired-through prompt; got: %q", resp.Content)
	}
}

func TestJunieAgent_Send_NonZeroExit_ReturnsInvocationFailed(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockJunie(t, dir, `
echo "synthetic junie stderr" >&2
exit 1
`)
	prependPath(t, dir)

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err != nil {
		t.Fatalf("NewJunieAgent: %v", err)
	}

	resp, err := a.Send(context.Background(), "anything")
	if err == nil {
		t.Fatal("Send returned nil error despite mock exit 1 — CONTRACT-bluff regression")
	}
	if !errors.Is(err, ErrJunieInvocationFailed) {
		t.Errorf("errors.Is(err, ErrJunieInvocationFailed) = false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "synthetic junie stderr") {
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

func TestJunieAgent_Send_ContextCancel_ReturnsCtxErr(t *testing.T) {
	dir := t.TempDir()
	// Mock script: sleeps long enough that the test's ctx will cancel
	// before it exits. This proves the production code path honors ctx
	// via exec.CommandContext + process-group SIGKILL (setProcessGroup
	// / killProcessGroup, shared with OpenCodeAgent + ClaudeCodeAgent
	// + GeminiAgent).
	_ = stageMockJunie(t, dir, `exec tail -f /dev/null`)
	prependPath(t, dir)

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err != nil {
		t.Fatalf("NewJunieAgent: %v", err)
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
	// not block indefinitely on tail -f /dev/null.
	if elapsed > 5*time.Second {
		t.Errorf("Send took %v after ctx deadline 100ms — exec.CommandContext did not SIGKILL the child", elapsed)
	}
}

// ---------------------------------------------------------------------
// SendWithAttachments — file-flag handling
// ---------------------------------------------------------------------

func TestJunieAgent_SendWithAttachments_WiresProjectFlags(t *testing.T) {
	dir := t.TempDir()
	// Mock script: dumps full argv to stdout so the test can verify
	// every --project= flag landed in the correct slot.
	_ = stageMockJunie(t, dir, `
printf 'ARGV:'
for arg in "$@"; do
  printf ' %s' "$arg"
done
printf '\n'
exit 0
`)
	prependPath(t, dir)

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err != nil {
		t.Fatalf("NewJunieAgent: %v", err)
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
	if !strings.Contains(resp.Content, "--project=/tmp/a") {
		t.Errorf("argv missing --project=/tmp/a; got: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "--project=/tmp/b") {
		t.Errorf("argv missing --project=/tmp/b; got: %q", resp.Content)
	}
	if strings.Contains(resp.Content, "--project= ") || strings.Contains(resp.Content, "--project=\n") {
		t.Errorf("argv contains empty --project= from skipped attachment; got: %q", resp.Content)
	}
	if !strings.HasSuffix(strings.TrimRight(resp.Content, "\n"), "summarise") {
		t.Errorf("argv should end with prompt; got: %q", resp.Content)
	}
}

// ---------------------------------------------------------------------
// SendStream — typed not-yet-wired error
// ---------------------------------------------------------------------

func TestJunieAgent_SendStream_ReturnsTypedNotWired(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockJunie(t, dir, `exit 0`)
	prependPath(t, dir)

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err != nil {
		t.Fatalf("NewJunieAgent: %v", err)
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
// Health — `junie --version` probe
// ---------------------------------------------------------------------

func TestJunieAgent_Health_VersionPasses(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockJunie(t, dir, `
if [ "$1" = "--version" ]; then
  echo "Junie version: 0.0.0-mock"
  exit 0
fi
exit 2
`)
	prependPath(t, dir)

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err != nil {
		t.Fatalf("NewJunieAgent: %v", err)
	}

	status := a.Health(context.Background())
	if !status.Healthy {
		t.Errorf("Health = unhealthy; err = %v", status.Error)
	}
	if status.AgentName != "junie" {
		t.Errorf("Health.AgentName = %q, want %q", status.AgentName, "junie")
	}
	if status.Error != nil {
		t.Errorf("Health.Error = %v, want nil", status.Error)
	}
}

func TestJunieAgent_Health_VersionFails_Unhealthy(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockJunie(t, dir, `exit 7`)
	prependPath(t, dir)

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err != nil {
		t.Fatalf("NewJunieAgent: %v", err)
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

func TestJunieAgent_StartStop_IsRunning(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockJunie(t, dir, `exit 0`)
	prependPath(t, dir)

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err != nil {
		t.Fatalf("NewJunieAgent: %v", err)
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
// Builder integration tests — round-71 narrowed sentinels
// ---------------------------------------------------------------------

func TestJunieClientBuilderFromConfig_ZeroConfig_ReturnsNotConfigured(t *testing.T) {
	b := JunieClientBuilderFromConfig(JunieBuilderConfig{})
	a, err := b(context.Background())
	if err == nil {
		t.Fatal("JunieClientBuilderFromConfig(zero) returned nil error — ErrJunieClientNotConfigured regression")
	}
	if a != nil {
		t.Errorf("JunieClientBuilderFromConfig returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrJunieClientNotConfigured) {
		t.Errorf("errors.Is(err, ErrJunieClientNotConfigured) = false; err = %v", err)
	}
}

func TestJunieClientBuilderFromConfig_ValidConfig_BuildsAgent(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockJunie(t, dir, `exit 0`)
	prependPath(t, dir)

	b := JunieClientBuilderFromConfig(JunieBuilderConfig{Binary: "junie"})
	a, err := b(context.Background())
	if err != nil {
		t.Fatalf("JunieClientBuilderFromConfig(valid): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("JunieClientBuilderFromConfig returned nil agent + nil error")
	}
	if a.Name() != "junie" {
		t.Errorf("agent.Name() = %q, want %q", a.Name(), "junie")
	}
}

func TestJunieClientBuilder_BinaryOnPath_BuildsRealAgent(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockJunie(t, dir, `exit 0`)
	prependPath(t, dir)

	b := JunieClientBuilder(&PoolConfig{Size: 1})
	a, err := b(context.Background())
	if err != nil {
		t.Fatalf("JunieClientBuilder({Size:1}, junie on PATH): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("JunieClientBuilder returned nil agent + nil error — round-71 wiring regression")
	}
	if _, ok := a.(*JunieAgent); !ok {
		t.Errorf("JunieClientBuilder returned %T, want *JunieAgent", a)
	}
}

func TestJunieClientBuilder_NilConfig_ReturnsNotWired(t *testing.T) {
	b := JunieClientBuilder(nil)
	a, err := b(context.Background())
	if err == nil {
		t.Fatal("JunieClientBuilder(nil) returned nil error — round-60 narrowed sentinel regression")
	}
	if a != nil {
		t.Errorf("JunieClientBuilder(nil) returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrJunieClientNotWired) {
		t.Errorf("errors.Is(err, ErrJunieClientNotWired) = false; err = %v", err)
	}
}

// ---------------------------------------------------------------------
// SimpleAgentPool end-to-end — round-71 real Agent through real pool
// ---------------------------------------------------------------------

func TestNewJuniePool_WithRealBuilder_AcquireReturnsRealAgent(t *testing.T) {
	dir := t.TempDir()
	_ = stageMockJunie(t, dir, `exit 0`)
	prependPath(t, dir)

	pool, err := NewJuniePool(&PoolConfig{Size: 2})
	if err != nil {
		t.Fatalf("NewJuniePool: %v", err)
	}

	a, err := pool.Acquire(context.Background(), AgentRequirements{})
	if err != nil {
		t.Fatalf("Acquire: unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("Acquire returned nil agent + nil error — CONTRACT-bluff")
	}
	if _, ok := a.(*JunieAgent); !ok {
		t.Errorf("pool.Acquire returned %T, want *JunieAgent", a)
	}
	if a.Name() != "junie" {
		t.Errorf("agent.Name() = %q, want %q", a.Name(), "junie")
	}
}

func TestNewJuniePool_NoBinaryOnPath_AcquireSurfacesBinaryNotFound(t *testing.T) {
	emptyDir := t.TempDir()
	isolatePath(t, emptyDir)

	pool, err := NewJuniePool(&PoolConfig{Size: 1})
	if err != nil {
		t.Fatalf("NewJuniePool: %v", err)
	}

	a, err := pool.Acquire(context.Background(), AgentRequirements{})
	if err == nil {
		t.Fatal("Acquire returned nil error despite missing binary — round-71 wiring regression")
	}
	if a != nil {
		t.Errorf("Acquire returned non-nil agent alongside error: %v", a)
	}
	if !errors.Is(err, ErrJunieBinaryNotFound) {
		t.Errorf("errors.Is(err, ErrJunieBinaryNotFound) = false; err = %v", err)
	}
}

// ---------------------------------------------------------------------
// Round-71 sentinel distinguishability — paired with round-60 + 64 +
// 66 + 69
// ---------------------------------------------------------------------

// TestRound71Sentinels_AreDistinct — disambiguation invariant.
// Round-71 introduces 3 new sentinels (ErrJunieBinaryNotFound,
// ErrJunieClientNotConfigured, ErrJunieInvocationFailed) on top of
// round-60's 5 builder-wired sentinels, round-64's 3 opencode-specific
// sentinels, round-66's 3 claudecode-specific sentinels, and round-69's
// 3 gemini-specific sentinels. All 17 MUST be distinct under errors.Is
// so callers can disambiguate which provider's wiring is in which
// state (binary missing vs config missing vs invocation failed vs the
// one non-wired provider still pending — QwenCode → round 72).
func TestRound71Sentinels_AreDistinct(t *testing.T) {
	sentinels := []error{
		// Round-71 (this round)
		ErrJunieBinaryNotFound,
		ErrJunieClientNotConfigured,
		ErrJunieInvocationFailed,
		// Round-69 (gemini wiring)
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
	if got, want := len(sentinels), 17; got != want {
		t.Fatalf("sentinel grid size = %d, want %d (round-71 17-sentinel matrix)", got, want)
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
// `junie` CLI if present, otherwise SKIP per §11.4.1 SKIP-OK rules.
// ---------------------------------------------------------------------

func TestJunieAgent_RealBinary_VersionRoundtripsOK(t *testing.T) {
	if _, err := exec.LookPath(DefaultJunieBinary); err != nil {
		t.Skip("SKIP-OK: #LLMORCHESTRATOR-JUNIE-REAL-ROUND71 — `junie` not installed on PATH; install JetBrains Junie CLI from https://junie.jetbrains.com/cli to exercise the real-binary path")
	}

	a, err := NewJunieAgent(JunieAgentConfig{})
	if err != nil {
		t.Fatalf("NewJunieAgent (real binary): %v", err)
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

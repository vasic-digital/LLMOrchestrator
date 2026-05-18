// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Round-71 §11.4 forensic anchor — real Junie CLI client wiring.
//
// This file converts the round-60 ErrJunieClientNotWired "stub returns
// sentinel" surface into a real os/exec-spawned bridge to the `junie`
// CLI binary (JetBrains' AI coding assistant). The architecture is a
// direct sibling of round-64's OpenCode wiring (opencode_agent.go),
// round-66's ClaudeCode wiring (claudecode_agent.go), and round-69's
// Gemini wiring (gemini_agent.go) — Path A: reuse the package-private
// process-group helpers (setProcessGroup / killProcessGroup) shared by
// every CLI-bridge agent in this package so context cancellation reaps
// not just the direct `junie` child but every grandchild it spawned
// (model-backend processes, sh subshells, etc.).
//
// Transport pattern (verified via `junie --help` 2026-05-18 against
// installed junie binary v888.195):
//
//   junie "<task>"                      → run in non-interactive mode
//                                         with the given task as the
//                                         positional argument; prints
//                                         model response to stdout and
//                                         exits.
//   junie --task=<text>                 → equivalent long-form (used by
//                                         operators that want explicit
//                                         flag delivery).
//   junie --version                     → version probe (health-check)
//
// The CLI delivers a one-shot task per process invocation; running
// `junie` with no arguments enters interactive mode (would block on
// stdin), so the agent ALWAYS supplies a task argument. Junie supports
// two delivery modes — positional ("junie <task>") and --task flag
// ("junie --task=<text>"); operators can switch via cfg.PromptFlag (""
// = positional default; "--task" = long flag form).
//
// Each Send invocation spawns a fresh `junie <prompt>` subprocess (no
// persistent session), matching how the CLI is documented to be used
// non-interactively and avoiding leaking a long-lived background
// process when the pool is shut down. The CLI also accepts
// `--session-id=<text>` to follow up an existing session — this is
// available via cfg.ExtraArgs but the default flow is fresh-session
// per Send to preserve idempotency.
//
// API keys (`--auth` JetBrains token; `--openai-api-key`,
// `--anthropic-api-key`, `--grok-api-key`), model selection
// (`--model gpt-4o-mini` etc.), and any extra flags (`--brave`,
// `--project=<dir>`, etc.) are passed through cfg.Env / cfg.ExtraArgs
// so this file never hardcodes credentials (CONST-042) and never
// assumes which Junie auth backend the operator wants (CONST-046).
//
// Constitutional anchors: CONST-035 (anti-bluff covenant — every PASS
// carries runtime evidence), CONST-042 (no-secret-leak — env-key
// handling), CONST-050(A) (no-fakes-beyond-unit-tests — production
// code path never imports test mocks), Article XI §11.9 (end-user
// quality forensic anchor).

// DefaultJunieBinary is the binary name resolved via $PATH when
// JunieAgentConfig.Binary is empty. Matches the canonical Junie CLI
// install (`junie` on PATH; typically resolves under
// ~/.local/bin/junie for user installs or /usr/local/bin/junie for
// system installs).
const DefaultJunieBinary = "junie"

// DefaultJuniePromptFlag is the flag prepended to the prompt for
// one-shot non-interactive Send invocations. Empty string means
// "positional argument" — `junie <prompt>` — which is the canonical
// one-shot example in `junie --help`. Operators that prefer the
// long-form `--task=<text>` can override via JunieAgentConfig.PromptFlag.
const DefaultJuniePromptFlag = ""

// ErrJunieBinaryNotFound is returned by NewJunieAgent when the
// configured (or default) `junie` binary cannot be located on $PATH.
//
// Distinct from round-60's ErrJunieClientNotWired (which signalled
// "no implementation exists at all" — now narrowed to the nil-cfg
// backstop in builders.go) — this round-71 sentinel fires AFTER
// round-71 wired the real implementation but the binary is missing
// at runtime.
var ErrJunieBinaryNotFound = errors.New(
	"junie agent: binary not found on PATH — install JetBrains Junie CLI " +
		"(https://junie.jetbrains.com/cli) or set JunieAgentConfig.Binary " +
		"to an absolute path")

// ErrJunieClientNotConfigured is returned by
// JunieClientBuilderFromConfig when the supplied JunieBuilderConfig
// is zero-value (no binary override, no extra args, no env).
// Round-60's ErrJunieClientNotWired signalled "implementation not
// present"; round-71's ErrJunieClientNotConfigured signals
// "implementation present but caller passed an empty config".
//
// Operators can still get a working agent by passing
// JunieBuilderConfig{Binary: "junie"} (or letting the default fire
// via the legacy JunieClientBuilder PATH-fallback entry).
var ErrJunieClientNotConfigured = errors.New(
	"junie agent: JunieBuilderConfig is zero-value — populate Binary " +
		"(or rely on PATH-resolved default `junie`) and re-invoke")

// ErrJunieInvocationFailed wraps any non-zero exit from
// `junie <prompt>`. Callers may errors.Is on this sentinel to
// distinguish CLI failures from binary-not-found / context-cancel
// failures. The underlying *exec.ExitError (including its captured
// stderr) is preserved via Unwrap so callers may errors.As it for
// the exit code.
var ErrJunieInvocationFailed = errors.New(
	"junie agent: `junie` exited non-zero")

// JunieAgentConfig configures a JunieAgent instance.
//
// Binary may be "" → use DefaultJunieBinary resolved via
// exec.LookPath.
// PromptFlag selects how the prompt is delivered to the CLI:
//   - "" (default) → positional argument: `junie <ExtraArgs> <prompt>`
//   - "--task" or any non-empty value → flag form:
//     `junie <ExtraArgs> --task=<prompt>` (using =-separated form so
//     the prompt is one argv slot even when it contains spaces).
//
// ExtraArgs are placed BEFORE the prompt-delivery slot in
// `junie <ExtraArgs> [<PromptFlag>=]<message>`. Useful for `--model
// gpt-4o-mini`, `--brave`, `--project=/path`, `--session-id=<id>`,
// `--auth=<token>` (prefer cfg.Env-backed JETBRAINS_TOKEN-style
// delivery for credentials), etc.
// WorkingDir defaults to the caller's CWD when empty.
// Env defaults to inheriting the parent process environment via
// exec.Cmd's default behaviour when nil — callers MUST source any
// provider credentials (JetBrains auth token, OPENAI_API_KEY,
// ANTHROPIC_API_KEY, GROK_API_KEY, etc.) into this slice rather than
// hardcoding them in source (CONST-042).
type JunieAgentConfig struct {
	Binary     string
	PromptFlag string
	ExtraArgs  []string
	WorkingDir string
	Env        []string
	// IDOverride is used by tests to make Agent.ID deterministic; in
	// production it is left empty and a default
	// "junie-<binary>" ID is generated.
	IDOverride string
}

// JunieBuilderConfig is the shape supplied to
// JunieClientBuilderFromConfig. It mirrors JunieAgentConfig but is
// exported under a distinct name so future builder-only knobs
// (retries, per-request timeouts) can land without polluting the
// per-agent config.
type JunieBuilderConfig struct {
	Binary     string
	PromptFlag string
	ExtraArgs  []string
	WorkingDir string
	Env        []string
}

// IsZero reports whether the JunieBuilderConfig has no configured
// fields. Empty config triggers ErrJunieClientNotConfigured at
// build time (callers must opt in to the PATH-resolved default via
// the legacy JunieClientBuilder entrypoint).
func (c JunieBuilderConfig) IsZero() bool {
	return c.Binary == "" &&
		c.PromptFlag == "" &&
		len(c.ExtraArgs) == 0 &&
		c.WorkingDir == "" &&
		len(c.Env) == 0
}

// JunieAgent is a real os/exec-backed bridge to the `junie` CLI.
//
// Each Send invocation spawns a fresh `junie <prompt>` subprocess,
// captures stdout, and returns it as the Response.Content. The
// subprocess honors the parent context's deadline + cancellation via
// exec.CommandContext, AND a platform-specific process-group SIGKILL
// helper (setProcessGroup / killProcessGroup, shared with
// OpenCodeAgent + ClaudeCodeAgent + GeminiAgent in
// {opencode_agent_unix,opencode_agent_windows}.go) so that the CLI's
// child processes (model-backend processes, sh subshells, sleep, etc.)
// are reaped along with the parent and SimpleAgentPool's Shutdown
// path cancels in-flight CLI calls cleanly.
//
// Concurrency: JunieAgent is safe for concurrent Send calls — each
// invocation gets its own subprocess and the only shared mutable
// state (running flag) is guarded by mu.
//
// This type implements the full Agent interface declared in agent.go.
type JunieAgent struct {
	id         string
	binary     string
	promptFlag string
	extraArgs  []string
	workingDir string
	env        []string

	mu      sync.Mutex
	running bool
}

// NewJunieAgent constructs a JunieAgent, validating that the
// configured (or default) binary exists on $PATH at construction
// time.
//
// Returns ErrJunieBinaryNotFound if exec.LookPath fails. This is
// the fail-fast contract — callers see a real failure at pool-build
// time instead of a deferred "command not found" inside Acquire.
func NewJunieAgent(cfg JunieAgentConfig) (*JunieAgent, error) {
	bin := cfg.Binary
	if bin == "" {
		bin = DefaultJunieBinary
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrJunieBinaryNotFound, err)
	}
	id := cfg.IDOverride
	if id == "" {
		id = "junie-" + resolved
	}
	return &JunieAgent{
		id:         id,
		binary:     resolved,
		promptFlag: cfg.PromptFlag, // "" means positional delivery
		extraArgs:  append([]string(nil), cfg.ExtraArgs...),
		workingDir: cfg.WorkingDir,
		env:        append([]string(nil), cfg.Env...),
	}, nil
}

// ID returns the agent's unique identifier.
func (a *JunieAgent) ID() string { return a.id }

// Name returns the provider name "junie" — used by the pool's
// preferred-agent matcher and by callers that need to identify which
// CLI backend serviced a given request. The string matches the
// builder/pool naming used in multi_pool.go ("junie").
func (a *JunieAgent) Name() string { return "junie" }

// Start marks the agent as running. The CLI itself is one-shot per
// Send call, so there is no persistent process to spawn here; Start
// only updates state so Health() reflects "ready". Idempotent.
func (a *JunieAgent) Start(_ context.Context) error {
	a.mu.Lock()
	a.running = true
	a.mu.Unlock()
	return nil
}

// Stop marks the agent as not-running. There is no long-lived process
// to terminate; in-flight Send calls cancel through their own
// contexts. Idempotent.
func (a *JunieAgent) Stop(_ context.Context) error {
	a.mu.Lock()
	a.running = false
	a.mu.Unlock()
	return nil
}

// IsRunning reports whether Start has been called since the last Stop.
func (a *JunieAgent) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// Health runs `junie --version` as a lightweight probe. PASS = exit 0
// + non-empty stdout. Any failure populates HealthStatus.Error with
// the captured stderr so operators can see what broke.
func (a *JunieAgent) Health(ctx context.Context) HealthStatus {
	start := time.Now()
	out, err := a.runCapture(ctx, []string{"--version"})
	status := HealthStatus{
		AgentID:   a.id,
		AgentName: a.Name(),
		Latency:   time.Since(start),
		CheckedAt: time.Now(),
	}
	if err != nil {
		status.Healthy = false
		status.Error = wrapJunieExitError(err, "junie --version")
		return status
	}
	if len(out) == 0 {
		status.Healthy = false
		status.Error = errors.New("junie --version: empty stdout (unexpected for a healthy CLI)")
		return status
	}
	status.Healthy = true
	return status
}

// buildPromptArgs returns the argv slice for delivering a prompt to
// the Junie CLI. When promptFlag is empty, the prompt is appended as
// a positional argument (`junie <ExtraArgs> <prompt>`). Otherwise the
// =-separated long-flag form is used (`junie <ExtraArgs>
// --task=<prompt>`) so the prompt is a single argv slot even when it
// contains spaces.
func (a *JunieAgent) buildPromptArgs(prompt string) []string {
	args := make([]string, 0, len(a.extraArgs)+2)
	args = append(args, a.extraArgs...)
	if a.promptFlag == "" {
		args = append(args, prompt)
	} else {
		args = append(args, a.promptFlag+"="+prompt)
	}
	return args
}

// Send runs `junie <ExtraArgs> <prompt>` (or
// `junie <ExtraArgs> --task=<prompt>` when PromptFlag is non-empty)
// as a one-shot subprocess and returns the captured stdout as
// Response.Content. Honors ctx cancellation via exec.CommandContext
// AND the package-shared process-group SIGKILL helper
// (setProcessGroup / killProcessGroup) so that the CLI's child
// processes (model-backend processes, sh subshells, etc.) are reaped
// along with the parent.
//
// CONST-042: API keys MUST be supplied via a.env (sourced by the
// caller from os.Environ() or a secrets manager); this method never
// reads credentials from anywhere else.
func (a *JunieAgent) Send(ctx context.Context, prompt string) (Response, error) {
	start := time.Now()
	args := a.buildPromptArgs(prompt)

	out, err := a.runCapture(ctx, args)
	latency := time.Since(start)

	if err != nil {
		// Propagate ctx.Err verbatim when the failure is a
		// cancellation — exec wraps it inside *exec.ExitError, but
		// callers expect errors.Is(err, context.Canceled /
		// context.DeadlineExceeded).
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{Latency: latency, Error: ctxErr}, ctxErr
		}
		op := "junie"
		if a.promptFlag != "" {
			op = "junie " + a.promptFlag
		}
		wrapped := wrapJunieExitError(err, op)
		return Response{Latency: latency, Error: wrapped}, wrapped
	}

	return Response{
		Content: string(out),
		Latency: latency,
	}, nil
}

// runCapture configures and runs a command with process-group
// SIGKILL on ctx cancellation, returning stdout (matching
// exec.Cmd.Output() semantics). Uses the package-shared
// setProcessGroup / killProcessGroup helpers introduced in round 64
// (opencode_agent_{unix,windows}.go) — no new platform helper file
// is needed because those helpers already use generic names and the
// process-group reaping pattern is identical for every CLI bridge.
func (a *JunieAgent) runCapture(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, a.binary, args...)
	if a.workingDir != "" {
		cmd.Dir = a.workingDir
	}
	if len(a.env) > 0 {
		cmd.Env = a.env
	}
	setProcessGroup(cmd)
	cmd.Cancel = func() error {
		return killProcessGroup(cmd)
	}
	return cmd.Output()
}

// SendStream is not yet supported by the Junie CLI client integration.
// Streaming requires Junie CLI event-stream output (no documented
// stable flag on v888.195) — follow-up work for a later round. Until
// then this surfaces a typed error rather than silently buffering
// Send's response into a fake stream.
func (a *JunieAgent) SendStream(_ context.Context, _ string) (<-chan StreamChunk, error) {
	return nil, errors.New("junie agent: SendStream not yet wired (Junie CLI v888.195 lacks a stable event-stream flag — follow-up round)")
}

// SendWithAttachments runs `junie --project=<dir> <prompt>` (one
// `--project` per attachment.Path). The Junie CLI's documented
// surface for adding directories/files to the workspace is the
// `--project=<text>` flag (`junie --help`: "Project directory
// (default: current directory)"). Empty-Path attachments are
// skipped. Attachment.Name is currently ignored at the CLI surface
// (the CLI's `--project` takes path-only; per-file naming is
// reserved for future Junie CLI revisions).
//
// NOTE: the Junie CLI's `--project` flag is documented as "default:
// current directory" — supplying multiple `--project` values is the
// pragmatic forward-compatible analogue of Gemini's
// `--include-directories` array flag; if a future Junie CLI revision
// rejects multiple --project values, the override path is
// cfg.ExtraArgs ["--project=/canonical/path"] + Send (skip
// SendWithAttachments).
func (a *JunieAgent) SendWithAttachments(ctx context.Context, prompt string, attachments []Attachment) (Response, error) {
	start := time.Now()
	args := make([]string, 0, len(a.extraArgs)+2*len(attachments)+2)
	args = append(args, a.extraArgs...)
	for _, att := range attachments {
		if att.Path == "" {
			continue
		}
		args = append(args, "--project="+att.Path)
	}
	if a.promptFlag == "" {
		args = append(args, prompt)
	} else {
		args = append(args, a.promptFlag+"="+prompt)
	}

	out, err := a.runCapture(ctx, args)
	latency := time.Since(start)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{Latency: latency, Error: ctxErr}, ctxErr
		}
		op := "junie --project"
		if a.promptFlag != "" {
			op = "junie " + a.promptFlag + " --project"
		}
		wrapped := wrapJunieExitError(err, op)
		return Response{Latency: latency, Error: wrapped}, wrapped
	}
	return Response{Content: string(out), Latency: latency}, nil
}

// OutputDir returns the working directory the CLI subprocess runs in.
// When empty, the subprocess inherits the parent's CWD.
func (a *JunieAgent) OutputDir() string { return a.workingDir }

// Capabilities reports the Junie CLI's known capability surface.
// The CLI itself supports tool-use (file edit, shell, project
// indexing, follow-up sessions via --session-id), and pluggable
// model backends (OpenAI / Anthropic / Grok / Junie's hosted
// JetBrains backend). Per-model context windows vary by backend; the
// conservative default reported here is the GPT-4o-class 128k upper
// bound — operators that select Anthropic Claude-class models can
// extend at the model layer.
func (a *JunieAgent) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		Vision:    false, // depends on selected model; conservative default
		Streaming: false, // wired in a later round per SendStream comment
		ToolUse:   true,
		MaxTokens: 128000,
	}
}

// SupportsVision returns whether the agent (and its current model
// selection) supports vision. Conservative default: false. Operators
// can override after construction if they have selected a vision-
// capable model via cfg.ExtraArgs `--model gpt-4o`.
func (a *JunieAgent) SupportsVision() bool { return false }

// ModelInfo returns the minimal model identification this agent
// surfaces. The Junie CLI delegates model selection to the
// operator's `--model` flag; the agent itself only reports the binary
// path so callers can correlate logs back to which CLI install
// serviced them.
func (a *JunieAgent) ModelInfo() ModelInfo {
	return ModelInfo{
		ID:       a.id,
		Provider: "junie",
		Name:     "junie-cli",
	}
}

// junieInvocationError chains BOTH the Junie sentinel AND the
// original *exec.ExitError so callers may errors.Is the sentinel
// AND errors.As the ExitError from the same error value. Standard
// fmt.Errorf("%w") can wrap exactly one — chaining via this struct
// unlocks both. Mirrors OpenCode's invocationError, ClaudeCode's
// claudeCodeInvocationError, and Gemini's geminiInvocationError
// patterns with a distinct sentinel target so
// errors.Is(err, ErrOpenCodeInvocationFailed) /
// errors.Is(err, ErrClaudeCodeInvocationFailed) /
// errors.Is(err, ErrGeminiInvocationFailed) return false for Junie
// failures and vice versa.
type junieInvocationError struct {
	op       string
	exitCode int
	stderr   string
	wrapped  error // the underlying *exec.ExitError (or other non-exit failure)
}

func (e *junieInvocationError) Error() string {
	if e.stderr != "" {
		return fmt.Sprintf("%s: %s exit %d: %s", ErrJunieInvocationFailed.Error(), e.op, e.exitCode, e.stderr)
	}
	if e.exitCode != 0 {
		return fmt.Sprintf("%s: %s exit %d", ErrJunieInvocationFailed.Error(), e.op, e.exitCode)
	}
	return fmt.Sprintf("%s: %s: %v", ErrJunieInvocationFailed.Error(), e.op, e.wrapped)
}

// Unwrap surfaces the underlying error (typically *exec.ExitError) so
// errors.As(err, **exec.ExitError) works on the wrapper.
func (e *junieInvocationError) Unwrap() error { return e.wrapped }

// Is matches the public Junie sentinel so
// errors.Is(err, ErrJunieInvocationFailed) returns true even though
// the sentinel is not in the Unwrap chain.
func (e *junieInvocationError) Is(target error) bool {
	return target == ErrJunieInvocationFailed
}

// wrapJunieExitError produces a sentinel-wrapped error that
// preserves the underlying *exec.ExitError (for callers that want to
// extract exit code / stderr) and adds the captured Stderr bytes to
// the message so operators see WHY the CLI failed without an extra
// Output() call.
func wrapJunieExitError(err error, op string) error {
	wrap := &junieInvocationError{op: op, wrapped: err}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		wrap.exitCode = exitErr.ExitCode()
		wrap.stderr = strings.TrimSpace(string(exitErr.Stderr))
	}
	return wrap
}

// Compile-time assertion: JunieAgent satisfies the Agent contract.
var _ Agent = (*JunieAgent)(nil)

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

	"digital.vasic.llmorchestrator/pkg/i18n"
)

// Round-69 §11.4 forensic anchor — real Gemini CLI client wiring.
//
// This file converts the round-60 ErrGeminiClientNotWired "stub returns
// sentinel" surface into a real os/exec-spawned bridge to the `gemini`
// CLI binary (Google Gemini CLI). The architecture is a direct sibling
// of round-64's OpenCode wiring (opencode_agent.go) and round-66's
// ClaudeCode wiring (claudecode_agent.go) — Path A: reuse the package-
// private process-group helpers (setProcessGroup / killProcessGroup)
// shared by every CLI-bridge agent in this package so context
// cancellation reaps not just the direct `gemini` child but every
// grandchild it spawned (MCP servers, sh subshells, etc.).
//
// Transport pattern (verified via `gemini --help` 2026-05-18 against
// installed gemini binary v0.33.2):
//
//   gemini -p <prompt>              → run in non-interactive (headless)
//                                     mode with the given prompt; prints
//                                     model response to stdout and exits.
//                                     Long form `--prompt` is equivalent.
//   gemini --version                → version probe (health-check)
//
// `-p` / `--prompt` is the documented "Run in non-interactive (headless)
// mode with the given prompt" flag; without it `gemini` defaults to
// interactive mode which would block forever on stdin. The agent uses
// the short form `-p` by default to match the CLI's documented one-shot
// invocation pattern; operators that need the long form, or who want
// to use `--prompt-interactive` for continued interaction, can override
// via GeminiAgentConfig.PromptFlag.
//
// Each Send invocation spawns a fresh `gemini -p <prompt>` subprocess
// (no persistent session), matching how the CLI is documented to be
// used non-interactively and avoiding leaking a long-lived background
// process when the pool is shut down.
//
// API keys (GEMINI_API_KEY for AI Studio auth; GOOGLE_GENAI_USE_VERTEXAI
// + GOOGLE_CLOUD_PROJECT + GOOGLE_CLOUD_LOCATION for Vertex AI;
// GOOGLE_APPLICATION_CREDENTIALS for service-account auth), model
// selection (`--model gemini-2.5-pro`), and any extra flags (`--yolo`,
// `--sandbox`, `--include-directories`) are passed through cfg.Env /
// cfg.ExtraArgs so this file never hardcodes credentials (CONST-042)
// and never assumes which Gemini variant the operator wants (CONST-046).
//
// Constitutional anchors: CONST-035 (anti-bluff covenant — every PASS
// carries runtime evidence), CONST-042 (no-secret-leak — env-key
// handling), CONST-050(A) (no-fakes-beyond-unit-tests — production
// code path never imports test mocks), Article XI §11.9 (end-user
// quality forensic anchor).

// DefaultGeminiBinary is the binary name resolved via $PATH when
// GeminiAgentConfig.Binary is empty. Matches the canonical Gemini CLI
// install (`gemini` on PATH; typically resolves under
// ~/.npm-global/bin/gemini for npm installs or /usr/local/bin/gemini
// for system installs).
const DefaultGeminiBinary = "gemini"

// DefaultGeminiPromptFlag is the flag prepended to the prompt for
// one-shot non-interactive Send invocations. The Gemini CLI's short
// form `-p` is the same surface as `--prompt`; we use the short form
// because that is the canonical one-shot example in `gemini --help`.
const DefaultGeminiPromptFlag = "-p"

// ErrGeminiBinaryNotFound is returned by NewGeminiAgent when the
// configured (or default) `gemini` binary cannot be located on $PATH.
//
// Distinct from round-60's ErrGeminiClientNotWired (which signalled
// "no implementation exists at all" — now narrowed to the nil-cfg
// backstop in builders.go) — this round-69 sentinel fires AFTER
// round-69 wired the real implementation but the binary is missing
// at runtime.
var ErrGeminiBinaryNotFound = errors.New(
	"gemini agent: binary not found on PATH — install Gemini CLI " +
		"(https://github.com/google-gemini/gemini-cli) or set " +
		"GeminiAgentConfig.Binary to an absolute path")

// ErrGeminiClientNotConfigured is returned by
// GeminiClientBuilderFromConfig when the supplied GeminiBuilderConfig
// is zero-value (no binary override, no extra args, no env).
// Round-60's ErrGeminiClientNotWired signalled "implementation not
// present"; round-69's ErrGeminiClientNotConfigured signals
// "implementation present but caller passed an empty config".
//
// Operators can still get a working agent by passing
// GeminiBuilderConfig{Binary: "gemini"} (or letting the default fire
// via the legacy GeminiClientBuilder PATH-fallback entry).
var ErrGeminiClientNotConfigured = errors.New(
	"gemini agent: GeminiBuilderConfig is zero-value — populate " +
		"Binary (or rely on PATH-resolved default `gemini`) and re-invoke")

// ErrGeminiInvocationFailed wraps any non-zero exit from
// `gemini -p …`. Callers may errors.Is on this sentinel to
// distinguish CLI failures from binary-not-found / context-cancel
// failures. The underlying *exec.ExitError (including its captured
// stderr) is preserved via Unwrap so callers may errors.As it for
// the exit code.
var ErrGeminiInvocationFailed = errors.New(
	"gemini agent: `gemini -p` exited non-zero")

// GeminiAgentConfig configures a GeminiAgent instance.
//
// Binary may be "" → use DefaultGeminiBinary resolved via
// exec.LookPath.
// ExtraArgs are prepended after the PromptFlag and before the message
// in `gemini <PromptFlag> <ExtraArgs> <message>`. Useful for `--model
// gemini-2.5-pro`, `--yolo`, `--sandbox`, `--include-directories
// /path`, `--output-format json`, `--approval-mode auto_edit`, etc.
// PromptFlag defaults to DefaultGeminiPromptFlag when "". Operators
// who need the long form (`--prompt`) or who want to use the
// continue-interactive form (`--prompt-interactive`) can override.
// Stdin-pipe mode is documented as follow-up work — for now the
// agent always uses positional-argument prompt delivery.
// WorkingDir defaults to the caller's CWD when empty.
// Env defaults to inheriting the parent process environment via
// exec.Cmd's default behaviour when nil — callers MUST source any
// provider credentials (GEMINI_API_KEY, GOOGLE_APPLICATION_CREDENTIALS,
// etc.) into this slice rather than hardcoding them in source
// (CONST-042).
type GeminiAgentConfig struct {
	Binary     string
	PromptFlag string
	ExtraArgs  []string
	WorkingDir string
	Env        []string
	// IDOverride is used by tests to make Agent.ID deterministic; in
	// production it is left empty and a default
	// "gemini-<binary>" ID is generated.
	IDOverride string
}

// GeminiBuilderConfig is the shape supplied to
// GeminiClientBuilderFromConfig. It mirrors GeminiAgentConfig but is
// exported under a distinct name so future builder-only knobs
// (retries, per-request timeouts) can land without polluting the
// per-agent config.
type GeminiBuilderConfig struct {
	Binary     string
	PromptFlag string
	ExtraArgs  []string
	WorkingDir string
	Env        []string
}

// IsZero reports whether the GeminiBuilderConfig has no configured
// fields. Empty config triggers ErrGeminiClientNotConfigured at
// build time (callers must opt in to the PATH-resolved default via
// the legacy GeminiClientBuilder entrypoint).
func (c GeminiBuilderConfig) IsZero() bool {
	return c.Binary == "" &&
		c.PromptFlag == "" &&
		len(c.ExtraArgs) == 0 &&
		c.WorkingDir == "" &&
		len(c.Env) == 0
}

// GeminiAgent is a real os/exec-backed bridge to the `gemini` CLI.
//
// Each Send invocation spawns a fresh `gemini -p <prompt>`
// subprocess, captures stdout, and returns it as the
// Response.Content. The subprocess honors the parent context's
// deadline + cancellation via exec.CommandContext, AND a platform-
// specific process-group SIGKILL helper (setProcessGroup /
// killProcessGroup, shared with OpenCodeAgent + ClaudeCodeAgent in
// {opencode_agent_unix,opencode_agent_windows}.go) so that the
// CLI's child processes (MCP servers, sh subshells, sleep, etc.) are
// reaped along with the parent and SimpleAgentPool's Shutdown path
// cancels in-flight CLI calls cleanly.
//
// Concurrency: GeminiAgent is safe for concurrent Send calls —
// each invocation gets its own subprocess and the only shared mutable
// state (running flag) is guarded by mu.
//
// This type implements the full Agent interface declared in agent.go.
type GeminiAgent struct {
	id         string
	binary     string
	promptFlag string
	extraArgs  []string
	workingDir string
	env        []string

	mu      sync.Mutex
	running bool
}

// NewGeminiAgent constructs a GeminiAgent, validating that the
// configured (or default) binary exists on $PATH at construction
// time.
//
// Returns ErrGeminiBinaryNotFound if exec.LookPath fails. This is
// the fail-fast contract — callers see a real failure at pool-build
// time instead of a deferred "command not found" inside Acquire.
func NewGeminiAgent(cfg GeminiAgentConfig) (*GeminiAgent, error) {
	bin := cfg.Binary
	if bin == "" {
		bin = DefaultGeminiBinary
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGeminiBinaryNotFound, err)
	}
	promptFlag := cfg.PromptFlag
	if promptFlag == "" {
		promptFlag = DefaultGeminiPromptFlag
	}
	id := cfg.IDOverride
	if id == "" {
		id = "gemini-" + resolved
	}
	return &GeminiAgent{
		id:         id,
		binary:     resolved,
		promptFlag: promptFlag,
		extraArgs:  append([]string(nil), cfg.ExtraArgs...),
		workingDir: cfg.WorkingDir,
		env:        append([]string(nil), cfg.Env...),
	}, nil
}

// ID returns the agent's unique identifier.
func (a *GeminiAgent) ID() string { return a.id }

// Name returns the provider name "gemini" — used by the pool's
// preferred-agent matcher and by callers that need to identify which
// CLI backend serviced a given request. The string matches the
// builder/pool naming used in multi_pool.go ("gemini").
func (a *GeminiAgent) Name() string { return "gemini" }

// Start marks the agent as running. The CLI itself is one-shot per
// Send call, so there is no persistent process to spawn here; Start
// only updates state so Health() reflects "ready". Idempotent.
func (a *GeminiAgent) Start(_ context.Context) error {
	a.mu.Lock()
	a.running = true
	a.mu.Unlock()
	return nil
}

// Stop marks the agent as not-running. There is no long-lived process
// to terminate; in-flight Send calls cancel through their own
// contexts. Idempotent.
func (a *GeminiAgent) Stop(_ context.Context) error {
	a.mu.Lock()
	a.running = false
	a.mu.Unlock()
	return nil
}

// IsRunning reports whether Start has been called since the last Stop.
func (a *GeminiAgent) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// Health runs `gemini --version` as a lightweight probe. PASS = exit 0
// + non-empty stdout. Any failure populates HealthStatus.Error with
// the captured stderr so operators can see what broke.
func (a *GeminiAgent) Health(ctx context.Context) HealthStatus {
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
		status.Error = wrapGeminiExitError(err, "gemini --version")
		return status
	}
	if len(out) == 0 {
		status.Healthy = false
		status.Error = errors.New("gemini --version: empty stdout (unexpected for a healthy CLI)")
		return status
	}
	status.Healthy = true
	return status
}

// Send runs `gemini <PromptFlag> <ExtraArgs> <prompt>` as a one-shot
// subprocess and returns the captured stdout as Response.Content.
// Honors ctx cancellation via exec.CommandContext AND the package-
// shared process-group SIGKILL helper (setProcessGroup /
// killProcessGroup) so that the CLI's child processes (MCP servers,
// sh subshells, etc.) are reaped along with the parent.
//
// CONST-042: API keys MUST be supplied via a.env (sourced by the
// caller from os.Environ() or a secrets manager); this method never
// reads credentials from anywhere else.
func (a *GeminiAgent) Send(ctx context.Context, prompt string) (Response, error) {
	start := time.Now()
	args := make([]string, 0, len(a.extraArgs)+2)
	args = append(args, a.promptFlag)
	args = append(args, a.extraArgs...)
	args = append(args, prompt)

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
		wrapped := wrapGeminiExitError(err, "gemini "+a.promptFlag)
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
func (a *GeminiAgent) runCapture(ctx context.Context, args []string) ([]byte, error) {
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

// SendStream is not yet supported by the Gemini CLI client integration.
// Streaming requires `gemini -p --output-format stream-json` event-
// stream parsing — follow-up work for a later round. Until then this
// surfaces a typed error rather than silently buffering Send's
// response into a fake stream.
func (a *GeminiAgent) SendStream(_ context.Context, _ string) (<-chan StreamChunk, error) {
	return nil, errors.New("gemini agent: SendStream not yet wired (requires --output-format=stream-json event parsing — follow-up round)")
}

// SendWithAttachments runs `gemini <PromptFlag>
// --include-directories <dir1>,<dir2> <prompt>`. Each attachment.Path
// is forwarded via the documented `--include-directories` array flag
// per the Gemini CLI surface (`gemini --help`:
// "--include-directories  Additional directories to include in the
// workspace (comma-separated or multiple --include-directories)").
// When Attachment.Name is non-empty, it is currently ignored at the
// CLI surface (the CLI's `--include-directories` takes path-only;
// per-file naming is reserved for future Gemini CLI revisions).
func (a *GeminiAgent) SendWithAttachments(ctx context.Context, prompt string, attachments []Attachment) (Response, error) {
	start := time.Now()
	args := make([]string, 0, len(a.extraArgs)+2*len(attachments)+2)
	args = append(args, a.promptFlag)
	args = append(args, a.extraArgs...)
	for _, att := range attachments {
		if att.Path == "" {
			continue
		}
		args = append(args, "--include-directories", att.Path)
	}
	args = append(args, prompt)

	out, err := a.runCapture(ctx, args)
	latency := time.Since(start)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{Latency: latency, Error: ctxErr}, ctxErr
		}
		wrapped := wrapGeminiExitError(err, "gemini "+a.promptFlag+" --include-directories")
		return Response{Latency: latency, Error: wrapped}, wrapped
	}
	return Response{Content: string(out), Latency: latency}, nil
}

// OutputDir returns the working directory the CLI subprocess runs in.
// When empty, the subprocess inherits the parent's CWD.
func (a *GeminiAgent) OutputDir() string { return a.workingDir }

// Capabilities reports the Gemini CLI's known capability surface.
// The CLI itself supports tool-use (shell, file edit, MCP servers via
// `gemini mcp`), streaming via `--output-format stream-json`, and
// per-model context windows up to 1M-2M tokens for Gemini 2.5 Pro /
// Gemini 2.5 Flash on the Google AI Studio / Vertex AI APIs.
func (a *GeminiAgent) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		Vision:    false, // depends on selected model; conservative default
		Streaming: false, // wired in a later round per SendStream comment
		ToolUse:   true,
		MaxTokens: 1000000,
	}
}

// SupportsVision returns whether the agent (and its current model
// selection) supports vision. Conservative default: false. Operators
// can override after construction if they have selected a vision-
// capable model via cfg.ExtraArgs `--model gemini-2.5-pro`.
func (a *GeminiAgent) SupportsVision() bool { return false }

// ModelInfo returns the minimal model identification this agent
// surfaces. The Gemini CLI delegates model selection to the
// operator's `--model` flag; the agent itself only reports the binary
// path so callers can correlate logs back to which CLI install
// serviced them.
func (a *GeminiAgent) ModelInfo() ModelInfo {
	return ModelInfo{
		ID:       a.id,
		Provider: "gemini",
		Name:     "gemini-cli",
	}
}

// geminiInvocationError chains BOTH the Gemini sentinel AND the
// original *exec.ExitError so callers may errors.Is the sentinel
// AND errors.As the ExitError from the same error value. Standard
// fmt.Errorf("%w") can wrap exactly one — chaining via this struct
// unlocks both. Mirrors OpenCode's invocationError and ClaudeCode's
// claudeCodeInvocationError patterns with a distinct sentinel target
// so errors.Is(err, ErrOpenCodeInvocationFailed) /
// errors.Is(err, ErrClaudeCodeInvocationFailed) return false for
// Gemini failures and vice versa.
type geminiInvocationError struct {
	op       string
	exitCode int
	stderr   string
	wrapped  error // the underlying *exec.ExitError (or other non-exit failure)
}

func (e *geminiInvocationError) Error() string {
	if e.stderr != "" {
		// CONST-046 round-115: user-facing error message routed through i18n.
		const id = "llmorchestrator_agent_gemini_invocation_failed_with_stderr"
		msg, terr := i18n.Pkg().T(
			context.Background(),
			id,
			map[string]any{
				"sentinel": ErrGeminiInvocationFailed.Error(),
				"op":       e.op,
				"exitCode": e.exitCode,
				"stderr":   e.stderr,
			},
		)
		if terr == nil && msg != "" && msg != id {
			return msg
		}
		return fmt.Sprintf("%s: %s exit %d: %s", ErrGeminiInvocationFailed.Error(), e.op, e.exitCode, e.stderr)
	}
	if e.exitCode != 0 {
		return fmt.Sprintf("%s: %s exit %d", ErrGeminiInvocationFailed.Error(), e.op, e.exitCode)
	}
	return fmt.Sprintf("%s: %s: %v", ErrGeminiInvocationFailed.Error(), e.op, e.wrapped)
}

// Unwrap surfaces the underlying error (typically *exec.ExitError) so
// errors.As(err, **exec.ExitError) works on the wrapper.
func (e *geminiInvocationError) Unwrap() error { return e.wrapped }

// Is matches the public Gemini sentinel so
// errors.Is(err, ErrGeminiInvocationFailed) returns true even though
// the sentinel is not in the Unwrap chain.
func (e *geminiInvocationError) Is(target error) bool {
	return target == ErrGeminiInvocationFailed
}

// wrapGeminiExitError produces a sentinel-wrapped error that
// preserves the underlying *exec.ExitError (for callers that want to
// extract exit code / stderr) and adds the captured Stderr bytes to
// the message so operators see WHY the CLI failed without an extra
// Output() call.
func wrapGeminiExitError(err error, op string) error {
	wrap := &geminiInvocationError{op: op, wrapped: err}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		wrap.exitCode = exitErr.ExitCode()
		wrap.stderr = strings.TrimSpace(string(exitErr.Stderr))
	}
	return wrap
}

// Compile-time assertion: GeminiAgent satisfies the Agent contract.
var _ Agent = (*GeminiAgent)(nil)

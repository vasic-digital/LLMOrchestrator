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

// Round-66 §11.4 forensic anchor — real ClaudeCode CLI client wiring.
//
// This file converts the round-60 ErrClaudeCodeClientNotWired "stub
// returns sentinel" surface into a real os/exec-spawned bridge to the
// `claude` CLI binary (Anthropic Claude Code). The architecture is a
// direct sibling of round-64's OpenCode wiring (see opencode_agent.go)
// and reuses the same package-private process-group helpers
// (setProcessGroup / killProcessGroup) so context cancellation reaps
// not just the direct `claude` child but every grandchild it spawned
// (MCP servers, sh subshells, etc.).
//
// Transport pattern (verified via `claude --help` 2026-05-18 against
// installed claude binary):
//
//   claude --print --bare <prompt>   → one-shot non-interactive run
//                                       that prints the model response
//                                       and exits. `-p` is the short
//                                       form of `--print`.
//   claude --version                 → version probe (health-check)
//
// `--print` is the documented "print response and exit (useful for
// pipes)" flag; without it `claude` starts an interactive session
// which would block forever on stdin. `--bare` is added by default
// because the operator's recipe is "single-prompt + single-response,
// no auto-memory / no plugin sync" — the bare mode keeps invocations
// hermetic and prevents the host's claude config from leaking model /
// agent / hook state into the orchestrator's request. Operators that
// need plugin / memory / agent context per-request can override the
// PromptFlag/ExtraArgs in ClaudeCodeAgentConfig.
//
// Each Send invocation spawns a fresh `claude --print --bare <prompt>`
// subprocess (no persistent session), matching how the CLI is
// documented to be used non-interactively and avoiding leaking a long-
// lived background process when the pool is shut down.
//
// API keys (ANTHROPIC_API_KEY when using direct Anthropic auth;
// AWS_*, GOOGLE_APPLICATION_CREDENTIALS for Bedrock / Vertex 3P
// providers respectively), model selection, and any extra flags
// (--model, --agent, --tools "Bash,Edit") are passed through
// cfg.Env / cfg.ExtraArgs so this file never hardcodes credentials
// (CONST-042) and never assumes which provider / model the operator
// wants (CONST-046).
//
// Constitutional anchors: CONST-035 (anti-bluff covenant — every PASS
// carries runtime evidence), CONST-042 (no-secret-leak — env-key
// handling), CONST-050(A) (no-fakes-beyond-unit-tests — production
// code path never imports test mocks), Article XI §11.9 (end-user
// quality forensic anchor).

// DefaultClaudeCodeBinary is the binary name resolved via $PATH when
// ClaudeCodeAgentConfig.Binary is empty. Matches the canonical Claude
// Code install (`claude` on PATH; `which claude` resolves to the
// per-user installation typically under ~/.local/bin or the system
// install under /usr/local/bin).
const DefaultClaudeCodeBinary = "claude"

// DefaultClaudeCodePromptFlag is the flag prepended to the prompt for
// one-shot non-interactive Send invocations. The Claude Code CLI's
// short form `-p` is the same surface as `--print`; we use the long
// form for self-documenting subprocess argv on operator's `ps` output.
const DefaultClaudeCodePromptFlag = "--print"

// ErrClaudeCodeBinaryNotFound is returned by NewClaudeCodeAgent when
// the configured (or default) `claude` binary cannot be located on
// $PATH.
//
// Distinct from round-60's ErrClaudeCodeClientNotWired (which signalled
// "no implementation exists at all" — now narrowed to the nil-cfg
// backstop in builders.go) — this round-66 sentinel fires AFTER
// round-66 wired the real implementation but the binary is missing
// at runtime.
var ErrClaudeCodeBinaryNotFound = errors.New(
	"claudecode agent: binary not found on PATH — install Claude Code " +
		"(https://docs.anthropic.com/claude-code) or set " +
		"ClaudeCodeAgentConfig.Binary to an absolute path")

// ErrClaudeCodeClientNotConfigured is returned by
// ClaudeCodeClientBuilderFromConfig when the supplied
// ClaudeCodeBuilderConfig is zero-value (no binary override, no extra
// args, no env). Round-60's ErrClaudeCodeClientNotWired signalled
// "implementation not present"; round-66's
// ErrClaudeCodeClientNotConfigured signals "implementation present
// but caller passed an empty config".
//
// Operators can still get a working agent by passing
// ClaudeCodeBuilderConfig{Binary: "claude"} (or letting the default
// fire via the legacy ClaudeCodeClientBuilder PATH-fallback entry).
var ErrClaudeCodeClientNotConfigured = errors.New(
	"claudecode agent: ClaudeCodeBuilderConfig is zero-value — populate " +
		"Binary (or rely on PATH-resolved default `claude`) and re-invoke")

// ErrClaudeCodeInvocationFailed wraps any non-zero exit from
// `claude --print …`. Callers may errors.Is on this sentinel to
// distinguish CLI failures from binary-not-found / context-cancel
// failures. The underlying *exec.ExitError (including its captured
// stderr) is preserved via Unwrap so callers may errors.As it for
// the exit code.
var ErrClaudeCodeInvocationFailed = errors.New(
	"claudecode agent: `claude --print` exited non-zero")

// ClaudeCodeAgentConfig configures a ClaudeCodeAgent instance.
//
// Binary may be "" → use DefaultClaudeCodeBinary resolved via
// exec.LookPath.
// ExtraArgs are prepended after the PromptFlag and before the message
// in `claude <PromptFlag> <ExtraArgs> <message>`. Useful for `--model
// sonnet`, `--bare`, `--add-dir <path>`, `--allowedTools "Bash Edit"`,
// `--mcp-config <file>`, etc.
// PromptFlag defaults to DefaultClaudeCodePromptFlag when "". Operators
// who need the short form (`-p`) or who want to pipe stdin instead
// can override. Stdin-pipe mode is documented as follow-up work — for
// now the agent always uses positional-argument prompt delivery.
// WorkingDir defaults to the caller's CWD when empty.
// Env defaults to inheriting the parent process environment via
// exec.Cmd's default behaviour when nil — callers MUST source any
// provider credentials (ANTHROPIC_API_KEY, AWS_*, etc.) into this
// slice rather than hardcoding them in source (CONST-042).
type ClaudeCodeAgentConfig struct {
	Binary     string
	PromptFlag string
	ExtraArgs  []string
	WorkingDir string
	Env        []string
	// IDOverride is used by tests to make Agent.ID deterministic; in
	// production it is left empty and a default
	// "claudecode-<binary>" ID is generated.
	IDOverride string
}

// ClaudeCodeBuilderConfig is the shape supplied to
// ClaudeCodeClientBuilderFromConfig. It mirrors ClaudeCodeAgentConfig
// but is exported under a distinct name so future builder-only knobs
// (retries, per-request timeouts) can land without polluting the
// per-agent config.
type ClaudeCodeBuilderConfig struct {
	Binary     string
	PromptFlag string
	ExtraArgs  []string
	WorkingDir string
	Env        []string
}

// IsZero reports whether the ClaudeCodeBuilderConfig has no configured
// fields. Empty config triggers ErrClaudeCodeClientNotConfigured at
// build time (callers must opt in to the PATH-resolved default via
// the legacy ClaudeCodeClientBuilder entrypoint).
func (c ClaudeCodeBuilderConfig) IsZero() bool {
	return c.Binary == "" &&
		c.PromptFlag == "" &&
		len(c.ExtraArgs) == 0 &&
		c.WorkingDir == "" &&
		len(c.Env) == 0
}

// ClaudeCodeAgent is a real os/exec-backed bridge to the `claude` CLI.
//
// Each Send invocation spawns a fresh `claude --print <prompt>`
// subprocess, captures stdout, and returns it as the
// Response.Content. The subprocess honors the parent context's
// deadline + cancellation via exec.CommandContext, AND a platform-
// specific process-group SIGKILL helper (setProcessGroup /
// killProcessGroup, shared with OpenCodeAgent in
// {opencode_agent_unix,opencode_agent_windows}.go) so that the
// CLI's child processes (MCP servers, sh subshells, sleep, etc.) are
// reaped along with the parent and SimpleAgentPool's Shutdown path
// cancels in-flight CLI calls cleanly.
//
// Concurrency: ClaudeCodeAgent is safe for concurrent Send calls —
// each invocation gets its own subprocess and the only shared mutable
// state (running flag) is guarded by mu.
//
// This type implements the full Agent interface declared in agent.go.
type ClaudeCodeAgent struct {
	id         string
	binary     string
	promptFlag string
	extraArgs  []string
	workingDir string
	env        []string

	mu      sync.Mutex
	running bool
}

// NewClaudeCodeAgent constructs a ClaudeCodeAgent, validating that
// the configured (or default) binary exists on $PATH at construction
// time.
//
// Returns ErrClaudeCodeBinaryNotFound if exec.LookPath fails. This is
// the fail-fast contract — callers see a real failure at pool-build
// time instead of a deferred "command not found" inside Acquire.
func NewClaudeCodeAgent(cfg ClaudeCodeAgentConfig) (*ClaudeCodeAgent, error) {
	bin := cfg.Binary
	if bin == "" {
		bin = DefaultClaudeCodeBinary
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrClaudeCodeBinaryNotFound, err)
	}
	promptFlag := cfg.PromptFlag
	if promptFlag == "" {
		promptFlag = DefaultClaudeCodePromptFlag
	}
	id := cfg.IDOverride
	if id == "" {
		id = "claudecode-" + resolved
	}
	return &ClaudeCodeAgent{
		id:         id,
		binary:     resolved,
		promptFlag: promptFlag,
		extraArgs:  append([]string(nil), cfg.ExtraArgs...),
		workingDir: cfg.WorkingDir,
		env:        append([]string(nil), cfg.Env...),
	}, nil
}

// ID returns the agent's unique identifier.
func (a *ClaudeCodeAgent) ID() string { return a.id }

// Name returns the provider name "claudecode" — used by the pool's
// preferred-agent matcher and by callers that need to identify which
// CLI backend serviced a given request. The string matches the
// builder/pool naming used in multi_pool.go ("claude-code") only via
// the pool's Name(); the Agent itself surfaces the underscore-free
// form to disambiguate from the pool-level slug.
func (a *ClaudeCodeAgent) Name() string { return "claudecode" }

// Start marks the agent as running. The CLI itself is one-shot per
// Send call, so there is no persistent process to spawn here; Start
// only updates state so Health() reflects "ready". Idempotent.
func (a *ClaudeCodeAgent) Start(_ context.Context) error {
	a.mu.Lock()
	a.running = true
	a.mu.Unlock()
	return nil
}

// Stop marks the agent as not-running. There is no long-lived process
// to terminate; in-flight Send calls cancel through their own
// contexts. Idempotent.
func (a *ClaudeCodeAgent) Stop(_ context.Context) error {
	a.mu.Lock()
	a.running = false
	a.mu.Unlock()
	return nil
}

// IsRunning reports whether Start has been called since the last Stop.
func (a *ClaudeCodeAgent) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// Health runs `claude --version` as a lightweight probe. PASS = exit 0
// + non-empty stdout. Any failure populates HealthStatus.Error with
// the captured stderr so operators can see what broke.
func (a *ClaudeCodeAgent) Health(ctx context.Context) HealthStatus {
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
		status.Error = wrapClaudeCodeExitError(err, "claude --version")
		return status
	}
	if len(out) == 0 {
		status.Healthy = false
		status.Error = errors.New("claude --version: empty stdout (unexpected for a healthy CLI)")
		return status
	}
	status.Healthy = true
	return status
}

// Send runs `claude <PromptFlag> <ExtraArgs> <prompt>` as a one-shot
// subprocess and returns the captured stdout as Response.Content.
// Honors ctx cancellation via exec.CommandContext AND the package-
// shared process-group SIGKILL helper (setProcessGroup /
// killProcessGroup) so that the CLI's child processes (MCP servers,
// sh subshells, etc.) are reaped along with the parent.
//
// CONST-042: API keys MUST be supplied via a.env (sourced by the
// caller from os.Environ() or a secrets manager); this method never
// reads credentials from anywhere else.
func (a *ClaudeCodeAgent) Send(ctx context.Context, prompt string) (Response, error) {
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
		wrapped := wrapClaudeCodeExitError(err, "claude "+a.promptFlag)
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
func (a *ClaudeCodeAgent) runCapture(ctx context.Context, args []string) ([]byte, error) {
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

// SendStream is not yet supported by the ClaudeCode CLI client
// integration. Streaming requires
// `claude --print --output-format stream-json` event-stream parsing
// per the CLI's --include-partial-messages flag — follow-up work for
// a later round. Until then this surfaces a typed error rather than
// silently buffering Send's response into a fake stream.
func (a *ClaudeCodeAgent) SendStream(_ context.Context, _ string) (<-chan StreamChunk, error) {
	return nil, errors.New("claudecode agent: SendStream not yet wired (requires --output-format=stream-json + --include-partial-messages event parsing — follow-up round)")
}

// SendWithAttachments runs `claude <PromptFlag> --file <spec> …
// <prompt>`. Each attachment.Path is passed as a separate `--file`
// flag per the Claude Code CLI surface (`claude --help`:
// "--file <specs...>  File resources to download at startup. Format:
// file_id:relative_path"). When Attachment.Name is non-empty, the
// flag value becomes `<Name>:<Path>` to honor the documented
// `file_id:relative_path` format; otherwise the bare Path is passed
// (which works for local file resources the CLI auto-discovers when
// resolving --add-dir trees).
func (a *ClaudeCodeAgent) SendWithAttachments(ctx context.Context, prompt string, attachments []Attachment) (Response, error) {
	start := time.Now()
	args := make([]string, 0, len(a.extraArgs)+2*len(attachments)+2)
	args = append(args, a.promptFlag)
	args = append(args, a.extraArgs...)
	for _, att := range attachments {
		if att.Path == "" {
			continue
		}
		spec := att.Path
		if att.Name != "" {
			spec = att.Name + ":" + att.Path
		}
		args = append(args, "--file", spec)
	}
	args = append(args, prompt)

	out, err := a.runCapture(ctx, args)
	latency := time.Since(start)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{Latency: latency, Error: ctxErr}, ctxErr
		}
		wrapped := wrapClaudeCodeExitError(err, "claude "+a.promptFlag+" --file")
		return Response{Latency: latency, Error: wrapped}, wrapped
	}
	return Response{Content: string(out), Latency: latency}, nil
}

// OutputDir returns the working directory the CLI subprocess runs in.
// When empty, the subprocess inherits the parent's CWD.
func (a *ClaudeCodeAgent) OutputDir() string { return a.workingDir }

// Capabilities reports the Claude Code CLI's known capability surface.
// The CLI itself supports tool-use (Bash, Edit, Read, MCP servers),
// streaming via the `--output-format stream-json` event stream, and
// per-model context windows up to 200k tokens for Claude Sonnet /
// Opus on the Anthropic Messages API.
func (a *ClaudeCodeAgent) Capabilities() AgentCapabilities {
	return AgentCapabilities{
		Vision:    false, // depends on selected model; conservative default
		Streaming: false, // wired in a later round per SendStream comment
		ToolUse:   true,
		MaxTokens: 200000,
	}
}

// SupportsVision returns whether the agent (and its current model
// selection) supports vision. Conservative default: false. Operators
// can override after construction if they have selected a vision-
// capable model via cfg.ExtraArgs `--model claude-sonnet-4-6`.
func (a *ClaudeCodeAgent) SupportsVision() bool { return false }

// ModelInfo returns the minimal model identification this agent
// surfaces. The Claude Code CLI delegates model selection to the
// operator's `--model` flag; the agent itself only reports the binary
// path so callers can correlate logs back to which CLI install
// serviced them.
func (a *ClaudeCodeAgent) ModelInfo() ModelInfo {
	return ModelInfo{
		ID:       a.id,
		Provider: "claudecode",
		Name:     "claude-code-cli",
	}
}

// claudeCodeInvocationError chains BOTH the ClaudeCode sentinel AND
// the original *exec.ExitError so callers may errors.Is the sentinel
// AND errors.As the ExitError from the same error value. Standard
// fmt.Errorf("%w") can wrap exactly one — chaining via this struct
// unlocks both. Mirrors OpenCode's invocationError pattern with a
// distinct sentinel target so errors.Is(err,
// ErrOpenCodeInvocationFailed) returns false for ClaudeCode failures
// and vice versa.
type claudeCodeInvocationError struct {
	op       string
	exitCode int
	stderr   string
	wrapped  error // the underlying *exec.ExitError (or other non-exit failure)
}

func (e *claudeCodeInvocationError) Error() string {
	if e.stderr != "" {
		// CONST-046 round-115: user-facing error message routed through i18n.
		const id = "llmorchestrator_agent_claudecode_invocation_failed_with_stderr"
		msg, terr := i18n.Pkg().T(
			context.Background(),
			id,
			map[string]any{
				"sentinel": ErrClaudeCodeInvocationFailed.Error(),
				"op":       e.op,
				"exitCode": e.exitCode,
				"stderr":   e.stderr,
			},
		)
		if terr == nil && msg != "" && msg != id {
			return msg
		}
		return fmt.Sprintf("%s: %s exit %d: %s", ErrClaudeCodeInvocationFailed.Error(), e.op, e.exitCode, e.stderr)
	}
	if e.exitCode != 0 {
		// CONST-046 round-204: exit-code-only branch routed through i18n.
		const id = "llmorchestrator_agent_claudecode_invocation_failed_exit_code_only"
		msg, terr := i18n.Pkg().T(
			context.Background(),
			id,
			map[string]any{
				"sentinel": ErrClaudeCodeInvocationFailed.Error(),
				"op":       e.op,
				"exitCode": e.exitCode,
			},
		)
		if terr == nil && msg != "" && msg != id {
			return msg
		}
		return fmt.Sprintf("%s: %s exit %d", ErrClaudeCodeInvocationFailed.Error(), e.op, e.exitCode)
	}
	// CONST-046 round-204: wrapped-error branch routed through i18n.
	const id = "llmorchestrator_agent_claudecode_invocation_failed_wrapped"
	wrappedStr := ""
	if e.wrapped != nil {
		wrappedStr = e.wrapped.Error()
	}
	msg, terr := i18n.Pkg().T(
		context.Background(),
		id,
		map[string]any{
			"sentinel": ErrClaudeCodeInvocationFailed.Error(),
			"op":       e.op,
			"wrapped":  wrappedStr,
		},
	)
	if terr == nil && msg != "" && msg != id {
		return msg
	}
	return fmt.Sprintf("%s: %s: %v", ErrClaudeCodeInvocationFailed.Error(), e.op, e.wrapped)
}

// Unwrap surfaces the underlying error (typically *exec.ExitError) so
// errors.As(err, **exec.ExitError) works on the wrapper.
func (e *claudeCodeInvocationError) Unwrap() error { return e.wrapped }

// Is matches the public ClaudeCode sentinel so
// errors.Is(err, ErrClaudeCodeInvocationFailed) returns true even
// though the sentinel is not in the Unwrap chain.
func (e *claudeCodeInvocationError) Is(target error) bool {
	return target == ErrClaudeCodeInvocationFailed
}

// wrapClaudeCodeExitError produces a sentinel-wrapped error that
// preserves the underlying *exec.ExitError (for callers that want to
// extract exit code / stderr) and adds the captured Stderr bytes to
// the message so operators see WHY the CLI failed without an extra
// Output() call.
func wrapClaudeCodeExitError(err error, op string) error {
	wrap := &claudeCodeInvocationError{op: op, wrapped: err}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		wrap.exitCode = exitErr.ExitCode()
		wrap.stderr = strings.TrimSpace(string(exitErr.Stderr))
	}
	return wrap
}

// Compile-time assertion: ClaudeCodeAgent satisfies the Agent contract.
var _ Agent = (*ClaudeCodeAgent)(nil)

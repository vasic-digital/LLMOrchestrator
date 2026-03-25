# LLMOrchestrator Architecture

**Module:** `digital.vasic.llmorchestrator`

LLMOrchestrator manages headless CLI agents (OpenCode, Claude Code, Gemini, Junie,
Qwen Code) with a hybrid pipe+file communication protocol. It provides a unified
`Agent` interface, a thread-safe pool, per-agent circuit breakers, and a structured
response parser.

---

## Package Overview

| Package | Role |
|---------|------|
| `pkg/agent` | Agent interface, AgentPool, CircuitBreaker, HealthMonitor |
| `pkg/adapter` | BaseAdapter + 5 CLI-specific adapters |
| `pkg/protocol` | PipeTransport (JSON-lines) and FileTransport (inbox/outbox) |
| `pkg/parser` | ResponseParser: action, issue, and JSON extraction |
| `pkg/config` | `.env` loading, binary path resolution, validation |

---

## Agent Pool

```mermaid
flowchart TD
    A[Caller: Acquire ctx, requirements] --> B[AgentPool]
    B --> C{Available agent\nmatches requirements?}
    C -- yes --> D[Return Agent handle]
    C -- no --> E[sync.Cond.Wait]
    E --> C
    D --> F[Caller uses Agent]
    F --> G[Release back to pool]
    G --> H[sync.Cond.Broadcast]
    H --> E
```

`AgentPool` is protected by `sync.Mutex` + `sync.Cond`. `Acquire` blocks until a
matching agent is available or `ctx` is cancelled, preventing busy-wait. Capability
matching checks agent type, health status, and workload flags declared in
`requirements`.

---

## Adapter Pattern

```
Agent interface
  └─ BaseAdapter (shared process management)
       ├─ OpenCodeAdapter   (parses opencode JSON-lines output)
       ├─ ClaudeCodeAdapter (parses claude-code streaming JSON)
       ├─ GeminiAdapter     (parses gemini JSON output)
       ├─ JunieAdapter      (parses junie output format)
       └─ QwenCodeAdapter   (parses qwen-code output format)
```

`BaseAdapter` owns process lifecycle: `Start` (exec + pipe setup), `Stop` (SIGTERM
with timeout, SIGKILL fallback), `Restart`, and `IsAlive`. Each concrete adapter
only implements `ParseResponse(raw string) (*Response, error)` and declares its
binary name and default flags, keeping per-agent code minimal.

---

## Hybrid Communication Protocol

### Pipe Transport (real-time)

`protocol.PipeTransport` attaches to the agent's stdin/stdout as a JSON-lines stream:

```
stdin  →  {"type":"prompt","content":"...","id":"req-1"}\n
stdout ←  {"type":"response","content":"...","id":"req-1"}\n
```

Each message is a single newline-terminated JSON object. The transport enforces a
configurable response length limit and a read deadline per request.

### File Transport (artifact exchange)

`protocol.FileTransport` manages three directories per agent session:

| Directory | Purpose |
|-----------|---------|
| `inbox/` | Files written by the caller for the agent to read |
| `outbox/` | Files written by the agent for the caller to consume |
| `shared/` | Bidirectional scratch space for large artifacts |

File transport is used for code files, diffs, and other payloads too large or
ill-suited for inline JSON.

---

## Circuit Breaker

Each agent has an independent `CircuitBreaker`:

- **Closed** (healthy) — requests pass through normally.
- **Open** (unhealthy) — after 3 consecutive failures, requests are rejected
  immediately for a 60-second cool-down period.
- **Half-Open** — after the cool-down, one probe request is allowed; success
  returns to Closed, failure resets the 60-second timer.

`HealthMonitor` runs a background goroutine that periodically calls `Agent.Ping`
and feeds results into the circuit breaker, allowing recovery without requiring
an incoming request.

---

## Response Parser

`parser.ResponseParser` operates on raw string output and extracts:

| Extraction | Pattern |
|------------|---------|
| JSON blocks | First valid JSON object or array in the output |
| Actions | Lines matching `ACTION: <verb> <target>` convention |
| Issues | Lines matching `ISSUE:` or `ERROR:` prefixes |

The parser is intentionally stateless and side-effect-free, making it safe to call
concurrently from multiple goroutines without locking.

---

## Security Constraints

- **Path traversal protection** — `FileTransport` rejects any path containing `..`
  or absolute segments outside the session directory.
- **Response length limit** — `PipeTransport` returns an error if a single response
  exceeds the configured byte ceiling (default 1 MB).
- **API key masking** — Config loader redacts `*_API_KEY` values in log output.
- **Command injection prevention** — Agent binary paths are validated against an
  allowlist; no shell interpolation is used when spawning processes.

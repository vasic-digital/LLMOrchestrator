# LLMOrchestrator Test Coverage Ledger (round-291, vasic-digital)

This ledger maps every exported symbol in `pkg/{adapter,agent,config,parser,protocol}` to the test (unit, integration, fuzz, security, stress) that exercises it and to the round-291 Challenge-runner invariant (`challenges/runner/main.go`) that re-validates the user-visible behaviour at end-user altitude.

It is the visible audit surface that closes the §11.4 anti-bluff covenant:
**every PASS in this submodule MUST correspond to a row below where the Challenge column is non-empty** — otherwise the PASS is a metadata-only check (CONST-035).

---

## How to read this table

| Column | Meaning |
|---|---|
| **Symbol** | Exported Go identifier (`Package.Func` / `Package.Type.Method`). |
| **Source** | File and line where the symbol is declared. |
| **Test(s)** | `*_test.go` files that exercise the symbol with assertions. |
| **Challenge invariant** | Numbered invariant in `challenges/runner/main.go` that re-proves the symbol works end-to-end across the 5-locale fixture matrix. Empty = covered by tests only; non-empty = also re-validated by the round-291 runner. |
| **Bluff risk closed** | Concrete user-visible failure mode the assertion catches. |

---

## `pkg/parser`

| Symbol | Source | Test(s) | Challenge invariant | Bluff risk closed |
|---|---|---|---|---|
| `NewParser` | `parser.go:71` | `parser_test.go` (all `TestParser_*`) | (1) `parser.NewParser.not_nil` | Constructor returns nil but tests don't notice → every downstream method panics for users. |
| `DefaultParser.Parse` | `parser.go:76` | `parser_test.go` (`TestParser_Parse_*`); `parser_fuzz_test.go` (fuzz) | (1) `parser.Parse.<locale>.action`; (2) `parser.Parse.empty_errors` | Bare-stub parser that returns an empty `ParsedResponse{}` would PASS legacy "no-error" tests; runner asserts an Action with matching Type+Target was actually extracted. |
| `DefaultParser.ExtractJSON` | `parser.go:112` | `parser_test.go` (`TestParser_ExtractJSON_*`); `parser_security_test.go` | (1) (via Parse) | JSON parser silently dropping nested actions → users see empty action lists. |
| `DefaultParser.ExtractActions` | `parser.go:150` | `parser_test.go` (`TestParser_ExtractActions_*`) | (1) | Action-keyword extraction regressing to 0-action output. |
| `DefaultParser.ExtractIssues` | `parser.go:200` | `parser_test.go` (`TestParser_ExtractIssues_*`) | (covered indirectly via Parse) | Issue extractor silently swallowing severity field. |
| `ErrEmptyInput` / `ErrNoJSONFound` / `ErrMalformedJSON` / `ErrResponseTooLong` | `parser.go:17-25` | `parser_test.go`; `parser_security_test.go` (length-DoS) | (2) `parser.Parse.empty_errors` (specifically `ErrEmptyInput`) | Defensive boundaries silently weakened — users could DoS the orchestrator with multi-MB junk. |
| `MaxResponseLength` | `parser.go:28` | `parser_security_test.go` | — | Constant raised silently; defense-in-depth disappears. |

## `pkg/protocol`

| Symbol | Source | Test(s) | Challenge invariant | Bluff risk closed |
|---|---|---|---|---|
| `PipeMessage` (struct) | `message.go:25` | `message_test.go`; `pipe_test.go` (round-trip); `protocol_integration_test.go` | (3) `protocol.PipeMessage.roundtrip.<locale>` | JSON tag drift: a future rename of `content` → `Content` in struct tags would silently corrupt every cross-process pipe payload; round-trip with verbatim Content match catches it. |
| `MessageType` constants | `message.go:13-22` | `message_test.go` | (3) (Type field) | Renaming `MessageTypePrompt` value from `"prompt"` to anything else silently breaks every existing agent. |
| `FileMessage` / `FileAttachment` | `message.go:45-60` | `file_test.go`; `protocol_integration_test.go` | (4) `protocol.FileTransport.roundtrip.<locale>` | Stale field — Content not persisted in inbox JSON — users would see empty messages in their session directory. |
| `NewFileTransport` | `file.go:34` | `file_test.go` (`TestFileTransport_*`) | (4) `protocol.NewFileTransport` | Constructor silently returns a transport whose inbox/outbox/shared dirs were not created → writes succeed but reads return nothing. |
| `FileTransport.WriteToInbox` / `ReadFromInbox` | `file.go:56,66` | `file_test.go`; `protocol_integration_test.go` | (4) | Write-then-read drops the message → users' instructions vanish. |
| `FileTransport.WriteToOutbox` / `ReadFromOutbox` | `file.go:61,71` | `file_test.go` | — (inbox covers symmetric path) | — |
| `FileTransport.WriteSharedFile` | `file.go:76` | `file_test.go` (path-traversal) | — | Path-traversal regression allows writing outside session dir. |
| `ErrSessionDirNotExist` / `ErrPathTraversal` | `file.go:17-22` | `file_test.go` (security) | — | Defensive boundary silently weakened. |
| `PipeTransport.*` | `pipe.go` | `pipe_test.go` | — | (Process-level transport — not exercised by runner to keep Challenge dependency-free; tests cover the wire format directly.) |

## `pkg/agent`

| Symbol | Source | Test(s) | Challenge invariant | Bluff risk closed |
|---|---|---|---|---|
| `Agent` (interface) | `agent.go:12` | (every adapter `_test.go` implements & asserts) | (5) (via Register) | Interface contract drift silently breaking all adapter implementations. |
| `Response` / `StreamChunk` / `Attachment` / `AgentCapabilities` / `AgentRequirements` / `HealthStatus` / `Action` / `ParsedResponse` / `Issue` / `ModelInfo` | `agent.go:42-127` | (used throughout the test surface) | (5) | Field renamed/dropped — every adapter response silently truncated. |
| `NewPool` | `pool.go:54` | `pool_test.go`; `pool_stress_test.go` | (5) `agent.Pool.Register` | Constructor returns a non-functional pool that drops every Register on the floor. |
| `Pool.Register` | `pool.go:63` | `pool_test.go` | (5) `agent.Pool.Register` | Duplicate-ID detection silently weakened; one rogue agent shadows another. |
| `Pool.Acquire` | `pool.go:87` | `pool_test.go`; `pool_stress_test.go` (concurrency) | (5) `agent.Pool.Acquire`; (5) `agent.Pool.Acquire.blocks_while_busy` | Acquire returns nil silently (no agent matched, no error) → users wait forever for a response that never starts. Acquire-while-busy not actually blocking → race conditions in production. |
| `Pool.Release` | `pool.go:169` | `pool_test.go` | (5) `agent.Pool.Release.reacquire` | Release silently no-ops; pool exhausts. |
| `Pool.Available` / `Pool.HealthCheck` / `Pool.Shutdown` | `pool.go:180-216` | `pool_test.go`; `health_test.go` | — | Health-check returning stale data; Shutdown not actually stopping subprocesses. |
| `MultiPool.*` | `multi_pool.go` | `pool_test.go` (subset) | — | (Covered by unit tests; not exercised by runner — single-pool path proves the locking contract.) |
| `CircuitBreaker.*` (health) | `health.go` | `health_test.go` | — | 3-failure threshold weakened silently. |
| `ErrAgentAlreadyRegistered` / `ErrNoAvailableAgent` / `ErrPoolShutdown` / `ErrAgentNotFound` | `pool.go:13-21` | `pool_test.go` | (5) (via Register/Acquire error paths) | Error-class collapse — users can't distinguish "no agent matches" from "pool dead". |

## `pkg/adapter`

| Symbol | Source | Test(s) | Challenge invariant | Bluff risk closed |
|---|---|---|---|---|
| `BaseAdapter` | `base.go` | `adapter_test.go`; `adapter_integration_test.go` | (5) (via OpenCode) | Process-management regression silently affects every adapter. |
| `NewOpenCodeAgent` / `OpenCodeAgent.parseOpenCodeResponse` | `opencode.go:20,53` | `adapter_test.go`; `opencode_headless_test.go` | (5) `agent.Pool.Register` (probe is OpenCode) | OpenCode JSON shape drift silently breaks the most-used adapter. |
| `NewClaudeCodeAgent` | `claudecode.go` | `adapter_test.go` | — | Constructor wiring regression. |
| `NewGeminiAgent` | `gemini.go` | `adapter_test.go` | — | Constructor wiring regression. |
| `NewJunieAgent` | `junie.go` | `adapter_test.go` | — | Constructor wiring regression. |
| `NewQwenCodeAgent` | `qwencode.go` | `adapter_test.go` | — | Constructor wiring regression. |
| `OpenCodeHeadlessAgent` | `opencode_headless.go` | `opencode_headless_test.go` | — | Headless flag-list drift. |
| `AdapterConfig` | `base.go` | `adapter_test.go` | (5) | Field rename silently breaks every consumer's config. |

## `pkg/config`

| Symbol | Source | Test(s) | Challenge invariant | Bluff risk closed |
|---|---|---|---|---|
| `Config` / `LoadFromEnv` / `LoadFromEnvironment` / `Validate` | `config.go` | `config_test.go` | — (covered by tests; runner uses no .env) | `.env` parser silently skipping required fields → orchestrator boots with no agents and no warning. |
| `AgentBinaryPath` / `SessionDir` | `config.go` | `config_test.go` | — | Path-traversal in user-supplied SessionDir base. |

---

## Test-type coverage summary

| Type | Where | Anti-bluff guarantee |
|---|---|---|
| **Unit** | `pkg/*/* _test.go` | Direct API exercise; mocks ALLOWED here only per CONST-050(A). |
| **Integration** | `pkg/adapter/adapter_integration_test.go`; `pkg/protocol/protocol_integration_test.go` | Real `os/exec` / real `os.MkdirTemp` / real JSON; no fakes (CONST-050(A)). |
| **Fuzz** | `pkg/parser/parser_fuzz_test.go` | `go test -fuzz=` driven; surfaces panics on adversarial input. |
| **Security** | `pkg/parser/parser_security_test.go` | Length-DoS, traversal, command-injection patterns. |
| **Stress** | `pkg/agent/pool_stress_test.go` | Concurrent Acquire/Release under contention; race detector mandatory. |
| **Challenge (round-291)** | `challenges/runner/main.go` + `challenges/llmorchestrator_describe_challenge.sh` | 5 invariants × 5 locales = 22 PASS rows; paired §1.1 mutation. |
| **Challenge (legacy)** | `challenges/scripts/*.sh` | UI / UX / DDoS / chaos / scaling / stress / no-suspend / host-hardening (legacy bash suite). |
| **Performance / benchmarking** | Not yet wired — tracked as future work (CONST-048). | — |

---

## Anti-bluff guarantees

1. **No metadata-only PASS.** Every Challenge-column entry in the table above corresponds to a runtime invariant in `challenges/runner/main.go` that compares observed values against fixture-pinned expected values byte-for-byte.
2. **5-locale matrix.** Every protocol-level invariant runs across `en`, `sr`, `ja`, `es`, `de` — a single English-only PASS would not survive CONST-046 (no-hardcoded-content) review.
3. **Paired mutation.** `challenges/llmorchestrator_describe_challenge.sh mutate` flips one invariant and asserts the runner exits non-zero. Mutation-undetected = runner is a bluff gate.
4. **Real I/O.** The runner uses `os.MkdirTemp` + `os.WriteFile` (via `protocol.FileTransport`) — never an in-memory shim. The Pool exercises Register/Acquire/Release on a real `adapter.NewOpenCodeAgent` instance.
5. **No mocks past the unit-test boundary.** Per CONST-050(A), the runner's path is real types, real I/O, real concurrency. `pkg/agent/multi_pool.go` etc. are excluded from runner only because their behaviour is a strict superset of `Pool` already covered.

---

## How to run

```bash
cd <repo>/dependencies/vasic-digital/LLMOrchestrator

# Normal mode — every invariant must PASS, exit 0
bash challenges/llmorchestrator_describe_challenge.sh normal

# Mutation mode — one invariant flipped; runner MUST exit non-zero,
# wrapper rewrites to 99 (paired-mutation success)
bash challenges/llmorchestrator_describe_challenge.sh mutate
```

Both invocations are wired into the round-291 evidence capture.

---

## Verbatim 2026-05-19 operator mandate (preserved per CONST-049 §11.4.17)

> "all existing tests and Challenges do work in anti-bluff manner - they MUST confirm that all tested codebase really works as expected! We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completition and full usability by end users of the product!"

<<<<<<< HEAD
# LLMOrchestrator — Symbol→Test Coverage Ledger

**Round:** 275 (deep-doc + Challenge enrichment, mirror round-220 template)
**Generated:** 2026-05-19
**Mandate:** CONST-048 (full-automation coverage) + CONST-050 (no-fakes-beyond-unit-tests) + CONST-035 / Article XI §11.9 (anti-bluff)

> Verbatim 2026-05-19 operator mandate (preserved per CONST-049 §11.4.17):
>
> "all existing tests and Challenges do work in anti-bluff manner — they
> MUST confirm that all tested codebase really works as expected! We had
> been in position that all tests do execute with success and all
> Challenges as well, but in reality the most of the features does not
> work and can't be used! This MUST NOT be the case and execution of
> tests and Challenges MUST guarantee the quality, the completition and
> full usability by end users of the product!"

## Purpose

This ledger maps every exported symbol in the LLMOrchestrator submodule to
the test or Challenge that exercises it with runtime evidence. A row
without runtime evidence is a §11.4 PASS-bluff regardless of how green the
green column reads.

## Convention

- **Layer:** `unit` (mocks allowed per CONST-050(A)), `integration` (real
  collaborators, no mocks), `challenge` (out-of-process bash wrapper, real
  subprocess + real disk + real wire).
- **Evidence:** anti-bluff artefact emitted during the test (file path,
  stdout substring, captured JSON). A green PASS column without an
  evidence column is a critical defect.
- **Mutation:** for every gate, a paired meta-test mutation (§1.1) that
  inverts the polarity of one invariant. The mutation MUST flip green→red.

## Coverage Ledger

| Package                 | Symbol                                     | Layer       | Test / Challenge                                                                   | Evidence                                                       | Paired Mutation                                                |
|-------------------------|--------------------------------------------|-------------|------------------------------------------------------------------------------------|----------------------------------------------------------------|----------------------------------------------------------------|
| `pkg/parser`            | `DefaultParser.Parse`                      | unit        | `pkg/parser/parser_test.go::TestDefaultParser_Parse_*`                             | parsed JSON struct, action slice                               | `parser_security_test.go` size + traversal mutations           |
| `pkg/parser`            | `DefaultParser.Parse` (empty)              | unit        | `pkg/parser/parser_test.go::TestParse_Empty`                                       | `ErrEmptyInput` returned                                       | round-275 runner flips polarity under `LLMORCH_MUTATE_RUNNER=1`|
| `pkg/parser`            | `DefaultParser.Parse` (5-locale prompts)   | challenge   | `challenges/runner/main.go` + `challenges/llmorchestrator_describe_challenge.sh`   | `parser.Parse.<locale>.action  PASS` line + actions=[...] dump | `challenges/llmorchestrator_describe_challenge.sh mutate` → 99 |
| `pkg/parser`            | `DefaultParser.ExtractJSON`                | unit        | `parser_test.go::TestExtractJSON_*`                                                | unmarshalled `map[string]any`                                  | fuzz: `parser_fuzz_test.go`                                    |
| `pkg/parser`            | `DefaultParser.ExtractActions`             | unit        | `parser_test.go::TestExtractActions_*`                                             | non-empty `[]agent.Action`                                     | regex + JSON injection in `parser_security_test.go`            |
| `pkg/parser`            | `DefaultParser.ExtractIssues`              | unit        | `parser_test.go::TestExtractIssues_*`                                              | non-empty `[]agent.Issue`                                      | text/JSON dual-path mutations                                  |
| `pkg/protocol`          | `PipeMessage` JSON encoding                | challenge   | `challenges/runner/main.go::Invariant 3` + `pipe_test.go::TestPipeMessage_*`       | round-trip bytes per locale; `Content == expect_pipe_content`  | mutation strips `Type` field → unmarshal mismatches            |
| `pkg/protocol`          | `MessageTypePrompt` constant               | challenge   | `challenges/runner/main.go::Invariant 3`                                           | `back.Type == prompt` per locale                               | type-rename mutation surfaces via test fail                    |
| `pkg/protocol`          | `NewFileTransport`                         | challenge   | `challenges/runner/main.go::Invariant 4` + `file_test.go::TestNewFileTransport_*`  | inbox/outbox/shared subdirs created at `tmp` path              | empty-sessionDir mutation triggers error path                  |
| `pkg/protocol`          | `FileTransport.WriteToInbox`               | challenge   | `challenges/runner/main.go::Invariant 4` + `file_test.go::TestWriteToInbox_*`      | `<ID>.json` lands in inbox dir                                 | invalid path traversal triggers `ErrPathTraversal`             |
| `pkg/protocol`          | `FileTransport.ReadFromInbox`              | challenge   | `challenges/runner/main.go::Invariant 4` + `file_test.go::TestReadFromInbox_*`     | `messages_in_inbox=1` per locale, content equality             | missing-dir mutation returns sentinel                          |
| `pkg/protocol`          | `FileTransport.WriteSharedFile`            | unit        | `file_test.go::TestSharedFile_*`                                                   | bytes round-trip; path traversal rejected                      | `..` segment mutation surfaces `ErrPathTraversal`              |
| `pkg/protocol`          | `validatePath` (private, exercised)        | unit+integ  | `file_test.go::TestPathTraversal_*` + `protocol_integration_test.go`               | rejection events captured                                      | absolute-path mutation rejected                                |
| `pkg/protocol`          | `PipeTransport.Send` / `Recv`              | unit        | `pkg/protocol/pipe_test.go`                                                        | JSON-lines parsed back to `PipeMessage`                        | stdin/stdout swap mutation surfaces decode error               |
| `pkg/i18n`              | `Translator` interface                     | unit        | `pkg/i18n/translator_test.go::TestTranslatorContract_*`                            | contract assertions on `NoopTranslator`                        | nil-arg mutation                                               |
| `pkg/i18n`              | `NoopTranslator.T`                         | challenge   | `challenges/runner/main.go::Invariant 5`                                           | `got == expectMessageID` per locale                            | strip verbatim contract → returns empty → gate FAIL            |
| `pkg/i18n`              | `NoopTranslator.TPlural`                   | challenge   | `challenges/runner/main.go::Invariant 5 (plural variant)`                          | count-aware id still verbatim per locale                       | count-mutation does not change output                          |
| `pkg/i18n`              | `SetPkgTranslator(nil)` + `Pkg()`          | challenge   | `challenges/runner/main.go::Invariant 5 (Pkg.reset_to_noop)`                       | `pkg.T("round_275_pkg_probe") == "round_275_pkg_probe"`        | nil-handling mutation would yield empty string                 |
| `pkg/agent`             | `Agent` interface                          | unit+integ  | `pkg/agent/agent_test.go` + per-adapter `*_agent_test.go`                          | mock pool round-trip; capability matching                      | capability-bitmask mutations                                   |
| `pkg/agent`             | `AgentPool.Acquire`                        | unit        | `pkg/agent/pool_test.go::TestPool_Acquire_*`                                       | per-requirement match log                                      | stress: `pool_stress_test.go`                                  |
| `pkg/agent`             | `SimplePool` round-robin                   | unit        | `pkg/agent/simple_pool_test.go`                                                    | ordered acquire/release sequence                               | concurrency-race mutation                                      |
| `pkg/agent`             | `HealthMonitor` circuit-breaker            | unit        | `pkg/agent/health_test.go::TestHealthMonitor_Trip`                                 | trip after 3 consecutive failures                              | threshold mutation → trips at 1                                |
| `pkg/agent`             | `ClaudeCodeAgent.Send`                     | unit        | `pkg/agent/claudecode_agent_test.go`                                               | mock-pool stdin/stdout transcript                              | env-not-set mutation                                           |
| `pkg/agent`             | `GeminiAgent.Send`                         | unit        | `pkg/agent/gemini_agent_test.go`                                                   | mock-pool stdin/stdout transcript                              | env-not-set mutation                                           |
| `pkg/agent`             | `JunieAgent.Send`                          | unit        | `pkg/agent/junie_agent_test.go`                                                    | mock-pool stdin/stdout transcript                              | env-not-set mutation                                           |
| `pkg/agent`             | `OpenCodeAgent.Send`                       | unit        | `pkg/agent/opencode_agent_test.go`                                                 | mock-pool stdin/stdout transcript                              | env-not-set mutation                                           |
| `pkg/agent`             | `QwenCodeAgent.Send`                       | unit        | `pkg/agent/qwencode_agent_test.go`                                                 | mock-pool stdin/stdout transcript                              | env-not-set mutation                                           |
| `pkg/agent`             | `MultiPool.Add` / `Pick`                   | unit        | `pkg/agent/mock_pool_test.go`                                                      | capability dispatch table                                      | identity mutation in `Pick`                                    |
| `pkg/adapter`           | `BaseAdapter` lifecycle                    | unit+integ  | `pkg/adapter/adapter_test.go` + `adapter_integration_test.go`                      | process spawn/stop transcript                                  | env-not-set mutation                                           |
| `pkg/adapter`           | `OpenCodeHeadless.Run`                     | unit        | `pkg/adapter/opencode_headless_test.go`                                            | parsed JSON-lines back to `Response`                           | broken-pipe mutation                                           |
| `pkg/config`            | `FromEnv`                                  | unit        | `pkg/config/config_test.go::TestFromEnv_*`                                         | parsed struct values per env scenario                          | missing-env mutation                                           |
| `cmd/orchestrator`      | `main` entry                               | integ       | `automation_test.go::TestOrchestratorCmd_Compile`                                  | compile + smoke run                                            | broken-flag mutation                                           |
| Chaos                   | failure injection across pool              | challenge   | `challenges/scripts/chaos_failure_injection_challenge.sh`                          | circuit-breaker trip + recovery log                            | inject-zero-failures fails the gate                            |
| DDoS                    | health-endpoint flood                      | challenge   | `challenges/scripts/ddos_health_flood_challenge.sh`                                | RPS sustained, no GC death                                     | flood-rate=0 → no failure detected                             |
| Scaling                 | horizontal pool growth                     | challenge   | `challenges/scripts/scaling_horizontal_challenge.sh`                               | N→2N capacity transcript                                       | scale-step=0 → no growth detected                              |
| Stress                  | sustained-load mix                         | challenge   | `challenges/scripts/stress_sustained_load_challenge.sh`                            | p95 latency under cap                                          | load-mix=empty → no measurement                                |
| UI                      | terminal interaction                       | challenge   | `challenges/scripts/ui_terminal_interaction_challenge.sh`                          | stdin/stdout transcript                                        | tty-detached mutation                                          |
| UX                      | end-to-end flow                            | challenge   | `challenges/scripts/ux_end_to_end_flow_challenge.sh`                               | full-cycle log                                                 | step-skip mutation                                             |

## Invariant Floor (per CONST-048)

For every row above, six invariants are asserted:

1. **Anti-bluff posture** — captured runtime evidence per §11.4.
2. **Proof of working capability** — end-to-end on the documented topology.
3. **Implementation matches docs** — README + USER_GUIDE + this ledger reflect actual API.
4. **No open issues** — `docs/Issues.md` empty for this row or row marked `OPERATOR-BLOCKED` per §11.4.21.
5. **Documentation in sync** — `.md` + `.html` + `.pdf` mtimes lockstep per §11.4.12 / §11.4.53.
6. **Four-layer test floor** — pre-build + post-build + runtime + paired mutation.

## How to Re-validate

```bash
# Round-275 Challenge (this round's deliverable)
cd dependencies/HelixDevelopment/LLMOrchestrator
bash challenges/llmorchestrator_describe_challenge.sh normal   # → exit 0
bash challenges/llmorchestrator_describe_challenge.sh mutate   # → exit 99

# Full unit + integration with race detector
go test -race -count=1 ./...
```
=======
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
>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805

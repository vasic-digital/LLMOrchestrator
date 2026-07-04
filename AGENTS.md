## INHERITED FROM Helix Constitution

> Base agent rules live in the Helix Constitution submodule at the
> parent project's `constitution/AGENTS.md` and the universal
> `constitution/Constitution.md` it references. **READ THOSE FIRST.**
> The base file is authoritative for any topic not covered here.
> Module-specific rules below extend them; they never weaken them.

Critical universal rules every CLI agent (Claude Code, Cursor, Aider,
Codex, Gemini CLI) MUST honour while working in this module:

- **No bluffing.** Every PASS carries positive evidence. Constitution §11.4.
- **Mutation-paired gates.** Every new gate has a paired mutation
  proving it catches regressions. Constitution §1.1.
- **No guessing language** (`likely`, `probably`, `maybe`, `seems`).
  Constitution §11.4.6.
- **Credentials never tracked.** `.env` patterns git-ignored; runtime-load
  only. Constitution §11.4.10.
- **Never force-push.** Force-push requires explicit per-session
  authorization AND a green post-op gate. Constitution §9.
- **CONTINUATION.md kept in sync** in every non-trivial commit.
  Constitution §12.10.
- **60% RAM cap.** Heavy work wrapped in bounded execution scope.
  Constitution §12.6.

Canonical reference: <https://github.com/HelixDevelopment/HelixConstitution>

---

# AGENTS.md — LLMOrchestrator

## INHERITED FROM constitution/AGENTS.md

All rules in `constitution/AGENTS.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## Module Identity

LLMOrchestrator (`digital.vasic.llmorchestrator`, Go 1.25) is a
standalone, project-not-aware, fully decoupled Go module for spawning,
managing, and communicating with multiple headless CLI LLM agents
(OpenCode, Claude Code, Gemini, Junie, Qwen Code) via a hybrid
pipe+file communication protocol. It never imports a consuming
project's namespace; project-specific behaviour (translations, agent
binary paths, API keys) is injected at runtime via the `i18n.Translator`
and `config` contracts.

## Responsibilities

- Spawn and supervise headless CLI agent processes and expose a
  thread-safe `Agent`/`AgentPool` abstraction with capability matching
  (vision / streaming / tool-use / token budget) (`pkg/agent`).
- Per-agent health monitoring via a circuit breaker — 3 consecutive
  failures marks an agent unhealthy (`pkg/agent`).
- Provide 5 CLI adapters (OpenCode, Claude Code, Gemini, Junie, Qwen
  Code) on top of a shared `BaseAdapter` process-management layer
  (`pkg/adapter`).
- Communicate over two transports: real-time JSON-lines pipes and
  file-based inbox/outbox/shared directories, with path-traversal
  protection and a 1 MiB response cap (`pkg/protocol`).
- Parse raw LLM stdout into structured actions/issues/JSON
  (`pkg/parser`).
- Load `.env`-based configuration and resolve agent binary paths
  (`pkg/config`).
- Externalise every user-facing string behind a `Translator` interface,
  defaulting to a `NoopTranslator` that returns the message id verbatim
  as a loud fallback (`pkg/i18n`, bundles under `pkg/i18n/bundles`).
- Expose a standalone CLI entry point (`cmd/orchestrator`).
- Keep `docs/ARCHITECTURE.md` honest against the real source tree
  (`internal/archdoc`).

## Testing Boundaries

- Build: `go build ./...`. Vet: `go vet ./...` (zero warnings).
- Unit + integration suite: `go test ./... -race -count=1` — verified
  green across all 9 packages in this checkout (`.`, `cmd/orchestrator`,
  `internal/archdoc`, `pkg/adapter`, `pkg/agent`, `pkg/config`,
  `pkg/i18n`, `pkg/parser`, `pkg/protocol`; `challenges/runner` has no
  test files, it is the Challenge entry point).
- Fuzz targets live in `pkg/parser` (`FuzzParser_Parse`,
  `FuzzParser_ExtractJSON`, `FuzzParser_ExtractActions`); run via
  `make fuzz` (30s per target).
- Mocks/stubs/placeholders are permitted ONLY in `*_test.go` unit
  tests. The `challenges/` tree (runner + wrapper script + chaos/DDoS/
  scaling/stress/UI/UX scripts under `challenges/scripts/`) exercises
  the real, fully-implemented system: real filesystem I/O, real JSON
  encoding, real parser execution — never a mock.
- The Challenge wrapper supports a paired-mutation mode
  (`LLMORCH_MUTATE_RUNNER=1`) that MUST flip a normally-passing
  invariant to a failure, proving the gate itself is not a bluff:
  ```bash
  bash challenges/llmorchestrator_describe_challenge.sh normal   # exit 0
  bash challenges/llmorchestrator_describe_challenge.sh mutate   # exit 99
  ```
- Agents MUST NOT weaken, stub, or skip tests, and MUST NOT introduce a
  dependency on any consuming project's namespace. Every change keeps
  `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1`
  green.

## Integration Points For Consumers

A consuming project wires this module by:

1. Implementing `i18n.Translator` and calling
   `i18n.SetPkgTranslator(...)` at startup (otherwise the built-in
   `NoopTranslator` is used, which is safe but untranslated).
2. Providing `.env` (copied from `.env.example`, mode 0600,
   git-ignored) with agent binary paths / API keys, loaded via
   `config.LoadFromEnv`.
3. Choosing a transport per call site: `protocol.PipeTransport` for
   real-time JSON-lines exchange, or `protocol.FileTransport` for
   inbox/outbox/shared-directory exchange.
4. Acquiring agents through `agent.Pool.Acquire(ctx, requirements)`
   rather than constructing adapters directly, so capability matching
   and circuit-breaker health tracking apply.

## Multi-Remote Distribution

- `upstreams/` (lowercase) holds one recipe script per remote
  (`GitHub.sh`, `VasicDigitalGitHub.sh`, `VasicDigitalGitLab.sh`, plus
  `push-all.sh` / `sync-all.sh` / `setup-remotes.sh`).
- `install_upstreams.sh` reads every `*.sh` recipe under `upstreams/`
  and configures the corresponding git remote.
- `make upstream-push` / `make upstream-sync` invoke
  `upstreams/push-all.sh` / `upstreams/sync-all.sh` directly.
- This module declares no own-org submodule dependencies
  (`helix-deps.yaml`: `deps: []`).

## Anti-Bluff Testing Rules

- Unit tests: mocks OK.
- Integration / Challenge / fuzz / stress / chaos / DDoS / scaling /
  UI / UX suites: real system only, no mocks (see
  `challenges/scripts/` for the supplementary Challenge set).
- Every PASS carries positive evidence — see `README.md`
  "Anti-bluff guarantees" for the exact invariant-to-evidence mapping
  (real disk I/O round trip, real JSON field-preservation check, real
  parser execution against 5-locale fixtures, real defensive-contract
  check for empty input, real i18n verbatim-passthrough check).
- No bare `t.Skip()` without a tracked-ticket marker.

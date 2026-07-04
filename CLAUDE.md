## INHERITED FROM Helix Constitution

This module is a submodule of a consuming project that
includes the Helix Constitution submodule at the parent's
`constitution/` path. All rules in `constitution/CLAUDE.md` and the
`constitution/Constitution.md` it references (universal anti-bluff
covenant §11.4, no-guessing mandate §11.4.6, credentials-handling
mandate §11.4.10, host-session safety §12, data safety §9, mutation-
paired gates §1.1) apply unconditionally to every change landed here.
The module-specific rules below extend them — they never weaken any
universal clause.

When this file disagrees with the constitution submodule, the
constitution wins. Locate the constitution submodule from any
arbitrary nested depth using its `find_constitution.sh` helper.

Canonical reference: <https://github.com/HelixDevelopment/HelixConstitution>

---

# CLAUDE.md — LLMOrchestrator

## INHERITED FROM constitution/CLAUDE.md

All rules in `constitution/CLAUDE.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## Module Overview

LLMOrchestrator is a standalone, project-not-aware, fully decoupled Go
module (`digital.vasic.llmorchestrator`, Go 1.25) for spawning, managing,
and communicating with multiple headless CLI LLM agents (OpenCode, Claude
Code, Gemini, Junie, Qwen Code) through a hybrid pipe+file communication
protocol. It is shared infrastructure meant to be consumed by multiple
independent projects — the module imports no consuming-project namespace,
and all user-facing strings are routed through an injected
`i18n.Translator` so no consumer's specifics leak in (see
`pkg/i18n/translator.go`).

## Build & Test

```bash
go build ./...                        # build all packages + CLI
go build ./cmd/orchestrator           # build the standalone orchestrator binary
go vet ./...                          # static analysis, zero warnings
go test ./... -race -count=1          # unit + integration suite, race detector
```

`make` wraps the common flows (verified against the real `Makefile`):

| Target            | Purpose                                              |
|-------------------|-------------------------------------------------------|
| `make build`      | `go build ./...`                                       |
| `make test`       | `go test ./... -race -count=1`                          |
| `make race`       | same as `test`, verbose                                 |
| `make vet`        | `go vet ./...`                                          |
| `make lint`       | `go vet ./...` (no external linter dependency)          |
| `make fmt`        | `gofmt -w -s .`                                         |
| `make cover`      | race-covered run, emits `coverage.html`                 |
| `make bench`      | `go test ./... -bench=. -benchmem`                      |
| `make fuzz`       | 30s fuzz run for each `pkg/parser` fuzz target          |
| `make check`      | `vet` + `test`                                          |
| `make clean`      | clears the Go build/test cache                          |
| `make upstream-push` / `make upstream-sync` | run `upstreams/push-all.sh` / `upstreams/sync-all.sh` |

Definition-of-Done gates (`scripts/no-silent-skips.sh`,
`scripts/demo-all.sh`) are wired as `make no-silent-skips`,
`make no-silent-skips-warn`, `make demo-all`, `make demo-all-warn`,
`make demo-one MOD=<name>`, and `make ci-validate-all`.

**Challenge runner** (real-system exerciser, no mocks — see `README.md`
"Anti-bluff guarantees" for exactly what each invariant proves):

```bash
LLMORCH_FIXTURES_DIR=challenges/fixtures go run ./challenges/runner/
bash challenges/llmorchestrator_describe_challenge.sh normal   # exits 0 on green
bash challenges/llmorchestrator_describe_challenge.sh mutate   # exits 99 on mutation-detected
```

All of the above were executed against this checkout and passed: `go
build ./...`, `go vet ./...`, `go test ./... -race -count=1` (9/9
packages green), the Challenge runner (29/29 PASS across 5 locales),
and the wrapper script (`normal` → exit 0).

## Package Structure

The `pkg/` layout is flat and shallow on purpose — six packages, plus
the CLI entry point and one internal helper:

| Package             | Purpose                                                                 |
|----------------------|--------------------------------------------------------------------------|
| `pkg/agent`          | `Agent`, `AgentPool` / `SimplePool` / `MultiPool`, `HealthMonitor`, `CircuitBreaker` (3 consecutive failures → unhealthy), per-CLI agent implementations |
| `pkg/adapter`         | `BaseAdapter` + the 5 CLI adapters (OpenCode, Claude Code, Gemini, Junie, Qwen Code) |
| `pkg/protocol`        | `PipeTransport` (real-time JSON-lines), `FileTransport` (inbox/outbox/shared directories), `PipeMessage`, `FileMessage`, path-traversal guard (`validatePath` / `ErrPathTraversal`) |
| `pkg/parser`          | `DefaultParser` / `ResponseParser` — structured extraction of actions, issues, and JSON from raw LLM output |
| `pkg/config`          | `.env` loading, `DefaultConfig()`, agent path resolution |
| `pkg/i18n`            | `Translator` interface, `NoopTranslator` default, `SetPkgTranslator` injection point; `pkg/i18n/bundles/` holds the embedded locale bundles |
| `cmd/orchestrator`    | Standalone CLI entry point (`main.go`, locale wiring in `i18n_msg.go`) |
| `internal/archdoc`    | Verifies `docs/ARCHITECTURE.md` stays factually consistent with the real source tree; generic, no consumer-project knowledge |

## Key Interfaces & Integration Points

- `agent.Agent` / `agent.Pool.Acquire(ctx, requirements)` — capability
  matching (vision / streaming / tool-use / token budget) over a
  thread-safe pool.
- `adapter.BaseAdapter` — process lifecycle shared by all 5 CLI adapters.
- `protocol.PipeTransport` / `protocol.FileTransport` — the two supported
  wire formats between orchestrator and a spawned CLI agent process.
- `parser.DefaultParser.Parse(...)` — turns raw agent stdout into a
  structured `[]agent.Action` (or the sentinel `ErrEmptyInput`).
- `i18n.Translator` — every consumer supplies its own implementation;
  `NoopTranslator` returns the message id verbatim so a missing
  translation surfaces loudly instead of silently as an empty string.
- `config.LoadFromEnv` — reads `.env` (copy from `.env.example`, mode
  0600, git-ignored) for agent binary paths and API keys.

## Language & Dependencies

- **Language**: Go 1.25 (see `go.mod`, module `digital.vasic.llmorchestrator`).
- **Direct dependencies**: `github.com/stretchr/testify` (tests only),
  `gopkg.in/yaml.v3` (Challenge fixture loading). No web framework, no
  database driver, no UI toolkit — this module has no such surfaces.
- **Own-org submodule dependencies**: none (`helix-deps.yaml` —
  `deps: []`, audited against `go.mod`/`go.sum`).

## Submodule Decoupling

- Never import a consuming project's namespace under `pkg/**`,
  `cmd/**`, or `internal/**` — this module must remain reusable by any
  project that wants to orchestrate headless CLI LLM agents.
- All user-facing strings flow through the injected `i18n.Translator`;
  never hardcode English literals in `pkg/`/`cmd/` call sites.
- `.env` is git-ignored and must be `chmod 600`; only `.env.example` is
  committed.
- Multi-remote sync lives in `upstreams/` (lowercase) —
  `install_upstreams.sh` reads each `*.sh` recipe from that directory
  and configures the corresponding git remote; `make upstream-push` /
  `make upstream-sync` call `upstreams/push-all.sh` /
  `upstreams/sync-all.sh` directly.
- Mocks/stubs/placeholders are permitted only in `*_test.go` unit
  tests; `challenges/` exercises the real parser, real disk I/O, real
  JSON encoding, and the real `i18n` surface — see `README.md` for the
  per-invariant evidence.

## Anti-Bluff Notes

This module's tests and Challenges exist to prove the codebase works,
not merely to compile. Guard against regressions of the following
patterns (all previously-resolved classes, kept here so a future change
does not silently reintroduce them):

- A parser change that returns an empty `[]agent.Action` slice instead
  of a real parsed action for a well-formed fixture.
- A transport change that breaks byte-level field preservation across
  a `PipeMessage` / `FileMessage` round trip.
- An `i18n` change that hardcodes an English string instead of routing
  through `Pkg()` / the injected `Translator`.
- A Challenge or test with `simulated`, `for now`, `TODO implement`, or
  `placeholder` in its behaviour — verify with:
  ```bash
  grep -rn "simulated\|for now\|TODO implement\|placeholder" pkg cmd && echo "BLUFF FOUND" || echo "clean"
  ```

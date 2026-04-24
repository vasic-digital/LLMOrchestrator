# CLAUDE.md

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

## Project Overview

**LLMOrchestrator** is a standalone Go module (`digital.vasic.llmorchestrator`) for managing headless CLI agents with hybrid pipe+file communication.

## Build & Test

```bash
go build ./...
go test ./... -race -count=1
go vet ./...
```

## MANDATORY: Never Remove or Disable Tests

NO test may ever be removed, disabled, skipped, or left broken. All issues must be fixed by addressing root causes.

## Architecture

5 packages in `pkg/`:
- `agent/` - Agent interface, AgentPool (thread-safe), CircuitBreaker, HealthMonitor
- `adapter/` - BaseAdapter + 5 CLI adapters (opencode, claudecode, gemini, junie, qwencode)
- `protocol/` - PipeTransport (JSON-lines), FileTransport (inbox/outbox/shared)
- `parser/` - ResponseParser (JSON/action/issue extraction from raw LLM output)
- `config/` - .env loading, agent path resolution, validation

## Key Patterns

- **BaseAdapter**: Shared process management; each adapter only implements parsing
- **AgentPool**: sync.Mutex + sync.Cond for blocking acquire with context cancellation
- **CircuitBreaker**: 3 consecutive failures = open for 60s
- **Security**: Path traversal protection, response length limits, API key masking

## Conventions

- SPDX license headers on all Go files
- Test files: `*_test.go`, `*_stress_test.go`, `*_security_test.go`, `*_fuzz_test.go`
- No TODO/FIXME in production code

## Dependencies

- Go 1.24+
- github.com/stretchr/testify (test only)

## Definition of Done

A change is NOT done because code compiles and tests pass. "Done" requires pasted
terminal output from a real run of the real system, produced in the same session as
the change. Coverage and passing suites measure the LLM's model of the product, not
the product.

1. **No self-certification.** *Verified, tested, working, complete, fixed, passing*
   are forbidden in commits, PRs, and agent replies without accompanying pasted
   output from a same-session real-system run.
2. **Demo before code.** Every task begins with the runnable acceptance demo below.
3. **Real system.** Demos run against real artifacts — built binaries, live
   databases, instrumented devices — not mocks/stubs/in-memory fakes.
4. **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `it.skip` without a trailing
   `SKIP-OK: #<ticket>` annotation fails `make ci-validate-all`.
5. **Contract tests on every seam.** Any change touching a module↔module boundary
   runs one roundtrip test asserting the wire format on both sides.
6. **Evidence in the PR.** PR body contains a fenced `## Demo` block with exact
   command(s) + output.

### Acceptance demo for this module

```bash
# TODO — replace with a 10-line real-system demo. See examples in
# HelixAgent/docs/development/dod-dropin/templates/CLAUDE_md_clause.md
```

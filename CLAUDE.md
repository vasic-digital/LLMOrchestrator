# CLAUDE.md

## MANDATORY: Project-Agnostic / 100% Decoupled

**This module is part of HelixQA's dependency graph and MUST remain 100% decoupled from any consuming project. It is designed for generic use with ANY project, not just ATMOSphere.**

- **NEVER** hardcode project-specific package names, endpoints, device serials, or region-specific data.
- **NEVER** import anything from the consuming project.
- **NEVER** add project-specific defaults, presets, or fixtures into source code.
- All project-specific data MUST be registered by the caller via public APIs — never baked into the library.
- Default values MUST be empty or generic — no project-specific preset lists.

**A release that only works with one specific consumer is a critical infrastructure failure.** Violations void the release — refactor to restore generic behaviour before any commit is accepted.

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


## ⚠️ MANDATORY: NO SUDO OR ROOT EXECUTION

**ALL operations MUST run at local user level ONLY.**

This is a PERMANENT and NON-NEGOTIABLE security constraint:

- **NEVER** use `sudo` in ANY command
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Modify commands to work within user permissions

**VIOLATION OF THIS CONSTRAINT IS STRICTLY PROHIBITED.**


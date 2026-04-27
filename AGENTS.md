# Supported Agents

## MANDATORY: Project-Agnostic / 100% Decoupled

**This module MUST remain 100% decoupled from any consuming project. It is designed for generic use with ANY project, not one specific consumer.**

- NEVER hardcode project-specific package names, endpoints, device serials, or region-specific data
- NEVER import anything from a consuming project
- NEVER add project-specific defaults, presets, or fixtures into source code
- All project-specific data MUST be registered by the caller via public APIs — never baked into the library
- Default values MUST be empty or generic

Violations void the release. Refactor to restore generic behaviour before any commit.

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

## Agent Overview

| Agent | CLI Binary | Headless Mode | Vision | Max Tokens |
|-------|-----------|---------------|--------|------------|
| OpenCode | `opencode` | `--headless --non-interactive` | Via configured LLM | 128K |
| Claude Code | `claude` | `--print --output-format json` | Native (Claude) | 200K |
| Gemini | `gemini` | `--non-interactive` | Native (Gemini) | 1M |
| Junie | `junie` | `--headless` | Via configured LLM | 128K |
| Qwen Code | `qwen-code` | `--headless --non-interactive` | Native (Qwen-VL) | 128K |

## OpenCode

OpenCode is a multi-provider CLI agent. It supports OpenAI, Anthropic, and Google providers. Vision capabilities depend on the configured LLM backend.

**Response format**: JSON with `content`, `tool_use`, `tokens_in`, `tokens_out` fields.

## Claude Code

Anthropic's official CLI for Claude. Uses `--print --output-format json` for headless operation. Native vision through Claude's multimodal capabilities.

**Response format**: JSON with `result`, `usage.input_tokens`, `usage.output_tokens`, `model` fields.

## Gemini

Google's Gemini CLI agent. Supports the largest context window (1M tokens). Native vision through Gemini's multimodal capabilities.

**Response format**: JSON with `text`, `token_count`, `finish_reason` fields.

## Junie

JetBrains' AI coding assistant. Uses `--headless` mode. Vision capabilities depend on the configured backend.

**Response format**: JSON with `response`, `status`, `tokens` fields.

## Qwen Code

Alibaba's Qwen-VL coding assistant. Native vision through Qwen-VL model. Uses `--headless --non-interactive` mode.

**Response format**: JSON with `output`, `token_usage.input`, `token_usage.output`, `model` fields.

## Adding a New Agent

1. Create `pkg/adapter/youragent.go`
2. Embed `*BaseAdapter`
3. Implement `parseYourAgentResponse(raw string) (agent.Response, error)`
4. Set appropriate flags in the constructor
5. Add tests in `pkg/adapter/adapter_test.go`
6. Update this document


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


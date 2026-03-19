# Changelog

All notable changes to LLMOrchestrator will be documented in this file.

## [0.1.0] - 2026-03-19

### Added
- Initial release
- Agent interface and AgentPool with thread-safe acquire/release
- CircuitBreaker with configurable failure threshold and recovery timeout
- HealthMonitor for per-agent circuit breaker management
- BaseAdapter with shared process management (spawn, pipe, shutdown)
- 5 CLI adapters: OpenCode, Claude Code, Gemini, Junie, Qwen Code
- PipeTransport for JSON-lines stdin/stdout communication
- FileTransport for inbox/outbox/shared file-based exchange
- ResponseParser with JSON extraction, action extraction, issue extraction
- Config with .env file loading and OS environment variable support
- Path traversal protection in FileTransport
- Response length limits in ResponseParser
- API key masking for safe logging
- Standalone CLI entry point (cmd/orchestrator)
- 200+ tests: unit, integration, stress, security, fuzz, automation
- Documentation: README, ARCHITECTURE, API_REFERENCE, USER_GUIDE, CONTRIBUTING, AGENTS
- Makefile with test, build, vet, fuzz, cover targets
- Multi-remote Upstreams scripts

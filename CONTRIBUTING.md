# Contributing

## Development Setup

```bash
git clone https://github.com/vasic-digital/LLMOrchestrator.git
cd LLMOrchestrator
go mod tidy
go build ./...
go test ./... -race -count=1
```

## Testing Requirements

- All tests MUST pass with `go test ./... -race -count=1`
- NO test may be removed, disabled, or skipped
- New features MUST include unit tests, integration tests, and security tests
- Target: maintain 200+ test count

## Test Types

- `*_test.go` - Unit tests
- `*_integration_test.go` - Integration tests
- `*_stress_test.go` - Stress/concurrency tests
- `*_security_test.go` - Security tests
- `*_fuzz_test.go` - Fuzz tests

## Code Style

- Go standard formatting (`gofmt`)
- SPDX license headers on all `.go` files
- No TODO/FIXME in production code
- All exported types must have GoDoc comments

## Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Write tests first (TDD)
4. Implement the feature
5. Ensure all tests pass: `make check`
6. Submit PR with clear description

## Commit Messages

```
type: short description

Longer description if needed.

Co-Authored-By: Your Name <email>
```

Types: `feat`, `fix`, `test`, `docs`, `refactor`, `chore`

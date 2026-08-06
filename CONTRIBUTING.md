# Contributing to soroban-budget-core

Thank you for contributing! 

## Local Setup

1. Install Go (1.21+).
2. Clone the repository.
3. Run `go mod download`.
4. Run `go test ./...` to verify your environment.

### Windows Gotchas
If you are developing on Windows, ensure that your git core.autocrlf is configured correctly, as some snapshot diffing tests rely on specific line endings. It is recommended to use WSL2 for testing.

## Submitting a PR
- Ensure all tests pass.
- Link the relevant Drips Wave issue.
- Keep scope bounded to the linked issue to ensure swift maintainer review.

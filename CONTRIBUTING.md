# Contributing to imap-go

Thank you for considering contributing to imap-go! This document provides guidelines and information for contributors.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/imap-go.git`
3. Create a feature branch: `git checkout -b feature/my-feature`
4. Make your changes
5. Run tests: `make test`
6. Commit and push your changes
7. Open a pull request

## Development Requirements

- Go 1.21 or later
- No external dependencies in core packages

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `golangci-lint` for additional linting
- Write tests for all new functionality
- Document exported types and functions

## Testing

Run the full test suite:

```bash
make test          # Run all tests
make test-race     # Run with race detector
make test-cover    # Generate coverage report
make vet           # Run go vet
make lint          # Run golangci-lint
```

## Project Structure

- `wire/` - Wire protocol parser and encoder
- `state/` - Connection state machine
- `server/` - IMAP server implementation
- `client/` - IMAP client implementation
- `extension/` - Extension/plugin system
- `extensions/` - Built-in extension implementations
- `middleware/` - Server middleware
- `auth/` - Authentication mechanisms
- `imaptest/` - Test infrastructure

## Pull Request Guidelines

- Keep PRs focused on a single concern
- Include tests for new functionality
- Update documentation as needed
- Ensure all tests pass before submitting
- Follow existing code patterns and conventions

## Reporting Issues

- Use GitHub Issues for bug reports and feature requests
- Include Go version, OS, and steps to reproduce for bugs
- Search existing issues before creating new ones

## RFC Compliance

When implementing or modifying protocol behavior, reference the relevant RFC section. Key RFCs:

- RFC 9051 - IMAP4rev2
- RFC 3501 - IMAP4rev1
- See `docs/rfcs.md` for the full coverage matrix

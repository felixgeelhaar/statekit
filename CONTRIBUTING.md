# Contributing to Statekit

Thank you for your interest in contributing to Statekit! This document provides guidelines and information for contributors.

## How to Contribute

### Reporting Bugs

Before creating a bug report, please check existing issues to avoid duplicates. When creating a bug report, include:

- A clear, descriptive title
- Steps to reproduce the behavior
- Expected behavior
- Actual behavior
- Go version and OS
- Minimal code example that demonstrates the issue

### Suggesting Features

Feature requests are welcome! Please:

- Check if the feature aligns with our [scope constraints](CLAUDE.md#scope-constraints-v1)
- Describe the use case and expected behavior
- Explain why existing features don't address your need

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Write tests** for any new functionality
3. **Follow the coding style** (run `go fmt` and `go vet`)
4. **Update documentation** if needed
5. **Write clear commit messages** following [Conventional Commits](https://www.conventionalcommits.org/)

## Development Setup

### Prerequisites

- Go 1.21 or later
- Git

### Getting Started

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/statekit.git
cd statekit

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Format code
go fmt ./...

# Run linter
go vet ./...
```

## Coding Guidelines

### Code Style

- Follow standard Go conventions
- Use `go fmt` for formatting
- Use `go vet` for static analysis
- Keep functions focused and small
- Write descriptive variable and function names

### Testing

- Write table-driven tests where appropriate
- Test both success and error cases
- Use meaningful test names that describe behavior
- Aim for high coverage on critical paths

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new feature
fix: resolve bug in interpreter
docs: update README examples
test: add tests for hierarchical states
refactor: simplify transition resolution
```

### Documentation

- Update README.md for user-facing changes
- Update CLAUDE.md for architectural changes
- Add godoc comments for exported types and functions

## Project Structure

```
statekit/
├── types.go              # Public types
├── builder.go            # Fluent builder API
├── interpreter.go        # Runtime execution
├── internal/ir/          # Internal representation
├── export/               # XState exporter
├── examples/             # Usage examples
└── docs/                 # Documentation
```

## Review Process

1. All PRs require at least one review
2. CI must pass (tests, linting)
3. Changes must include appropriate tests
4. Documentation must be updated if needed

## Release Process

Releases are managed using [Relicta](https://github.com/felixgeelhaar/relicta). The maintainers will handle versioning and releases.

## Questions?

Feel free to open an issue for any questions about contributing.

Thank you for contributing!

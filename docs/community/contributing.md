# Contributing to pyproc

First off, thank you for considering contributing to pyproc!

pyproc is a community-driven project, and we welcome contributions of all kinds: bug reports, feature requests, documentation improvements, and code contributions.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
  - [Reporting Bugs](#reporting-bugs)
  - [Suggesting Features](#suggesting-features)
  - [Your First Code Contribution](#your-first-code-contribution)
  - [Pull Requests](#pull-requests)
- [Style Guidelines](#style-guidelines)
  - [Git Commit Messages](#git-commit-messages)
  - [Go Code Style](#go-code-style)
  - [Python Code Style](#python-code-style)
- [Testing](#testing)
- [CI Checks](#ci-checks)
- [Release Process](#release-process)
- [Getting Help](#getting-help)

## Code of Conduct

This project and everyone participating in it is governed by our commitment to providing a welcoming and inclusive environment. Please be respectful and constructive in all interactions.

## Getting Started

pyproc enables Go applications to call Python functions without CGO or microservices. Before contributing, please:

1. Read the [README](https://github.com/YuminosukeSato/pyproc#readme) to understand what pyproc does
2. Check [existing issues](https://github.com/YuminosukeSato/pyproc/issues) to avoid duplicates
3. Review [open PRs](https://github.com/YuminosukeSato/pyproc/pulls) to see what's in progress

## Development Setup

### Prerequisites

- **Go**: 1.24 or later
- **Python**: 3.9 or later (3.12 recommended)
- **uv**: Python package manager (required, pip is not supported)
- **Make**: For running common tasks
- **Git**: For version control

### Clone and Setup

```bash
# Clone the repository
git clone https://github.com/YuminosukeSato/pyproc.git
cd pyproc

# Install Go dependencies
go mod tidy

# Install Python worker dependencies using uv
cd worker/python
uv sync --all-extras --dev
cd ../..

# Verify your setup
make demo
```

### Verify Everything Works

```bash
# Run Go tests
go test ./...

# Run Python tests
cd worker/python && uv run pytest -v && cd ../..

# Run the demo
make demo
```

If all tests pass and the demo runs successfully, you're ready to contribute!

## How to Contribute

### Reporting Bugs

Found a bug? Please help us fix it by [opening an issue](https://github.com/YuminosukeSato/pyproc/issues/new).

**A good bug report includes:**

1. **Clear title**: Summarize the issue in one line
2. **Environment**: Go version, Python version, OS
3. **Steps to reproduce**: Minimal code or commands to trigger the bug
4. **Expected behavior**: What you expected to happen
5. **Actual behavior**: What actually happened
6. **Logs/errors**: Any relevant error messages or stack traces

**Example:**

```markdown
## Bug: Worker crashes on large payload

**Environment:**
- Go 1.24.0
- Python 3.12.0
- macOS 14.0

**Steps to reproduce:**
1. Create a worker with default settings
2. Call with payload > 10MB
3. Worker process exits unexpectedly

**Expected:** Worker handles large payload or returns clear error
**Actual:** Worker crashes without error message

**Logs:**
[paste relevant logs here]
```

### Suggesting Features

Have an idea? We'd love to hear it! Please [open a feature request](https://github.com/YuminosukeSato/pyproc/issues/new).

**Before suggesting:**

- Check if it aligns with pyproc's scope (Go + Python IPC on same host)
- Search existing issues to avoid duplicates

**A good feature request includes:**

- **Use case**: What problem are you trying to solve?
- **Proposed solution**: How do you envision it working?
- **Alternatives considered**: What other approaches did you consider?

### Your First Code Contribution

Not sure where to start? Look for issues labeled:

- [`good first issue`](https://github.com/YuminosukeSato/pyproc/labels/good%20first%20issue) - Great for newcomers
- [`help wanted`](https://github.com/YuminosukeSato/pyproc/labels/help%20wanted) - We'd appreciate help with these
- [`documentation`](https://github.com/YuminosukeSato/pyproc/labels/documentation) - Improve docs (no Go/Python required!)

**First contribution workflow:**

1. Comment on the issue to let us know you're working on it
2. Fork the repository
3. Create a branch (`git checkout -b fix/issue-description`)
4. Make your changes
5. Write or update tests
6. Submit a pull request

### Pull Requests

**Before submitting:**

- [ ] All tests pass (`go test ./...` and `uv run pytest`)
- [ ] Code follows our style guidelines
- [ ] Commit messages follow Conventional Commits format
- [ ] Documentation is updated (if applicable)
- [ ] PR description explains the change

**PR process:**

1. **Fork** the repository
2. **Create a branch** from `main`:

   ```bash
   git checkout -b feature/my-feature
   # or
   git checkout -b fix/bug-description
   ```

3. **Make your changes** with clear, focused commits
4. **Push** to your fork:

   ```bash
   git push origin feature/my-feature
   ```

5. **Open a PR** against `main`
6. **Respond to feedback** - we aim to review within 48 hours

**PR title format:**

```text
<type>(<scope>): <description>

Examples:
feat(pool): add batch call support
fix(worker): handle SIGTERM gracefully
docs(readme): add PyTorch example
```

**PR Labels for Release Notes:**

When your PR is merged, it will be categorized in release notes based on labels:

| Label | Release Notes Category |
|-------|----------------------|
| `breaking-change` | Breaking Changes |
| `enhancement`, `feature` | New Features |
| `bug`, `fix` | Bug Fixes |
| `documentation` | Documentation |
| other | Other Changes |

## Style Guidelines

### Git Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/) for clear, automated changelogs.

**Format:**

```text
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `style` | Formatting, no code change |
| `refactor` | Code restructure, no feature change |
| `perf` | Performance improvement |
| `test` | Adding or updating tests |
| `ci` | CI/CD changes |
| `chore` | Maintenance tasks |

**Examples:**

```bash
# Feature
git commit -m "feat(pool): add support for batch calls"

# Bug fix
git commit -m "fix(worker): prevent panic on nil response"

# Documentation
git commit -m "docs(readme): add scikit-learn example"

# Performance
git commit -m "perf(protocol): reduce JSON serialization overhead by 30%"
```

**Guidelines:**

- Use present tense ("add feature" not "added feature")
- Use imperative mood ("move cursor to..." not "moves cursor to...")
- Keep the subject line under 72 characters
- Reference issues in the footer: `Fixes #123`

### Go Code Style

We follow standard Go conventions with some additions:

```go
// Package pyproc provides Python interop via Unix Domain Sockets.
package pyproc

import (
    // Standard library first
    "context"
    "fmt"

    // External packages second
    "github.com/some/package"

    // Internal packages last
    "github.com/YuminosukeSato/pyproc/internal/protocol"
)

// Pool manages Python worker processes.
// All exported types need doc comments starting with the name.
type Pool struct {
    workers []*Worker
    config  PoolConfig
}

// NewPool creates a new worker pool with the given options.
// All exported functions need doc comments starting with the name.
func NewPool(opts PoolOptions) (*Pool, error) {
    // Validate inputs early
    if opts.Workers <= 0 {
        return nil, fmt.Errorf("workers must be positive, got %d", opts.Workers)
    }

    // Wrap errors with context using %w
    worker, err := newWorker(opts.WorkerConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create worker: %w", err)
    }

    return &Pool{workers: []*Worker{worker}}, nil
}
```

**Key points:**

- Run `go fmt` before committing
- Run `go vet` to catch common issues
- Use `golangci-lint` for comprehensive linting
- Handle all errors explicitly
- Add doc comments to all exported items

### Python Code Style

```python
"""Module docstring explaining the purpose."""

from typing import Any

from pyproc_worker import expose, run_worker


@expose
def predict(req: dict[str, Any]) -> dict[str, Any]:
    """Run prediction on input data.

    Args:
        req: Dictionary containing 'features' key with input array

    Returns:
        Dictionary with 'prediction' and 'confidence' keys

    Raises:
        ValueError: When required keys are missing

    """
    if "features" not in req:
        raise ValueError("missing 'features' key in request")

    result = model.predict(req["features"])
    return {"prediction": result.tolist()}


if __name__ == "__main__":
    run_worker()
```

**Key points:**

- Use type hints for all function signatures
- Write docstrings with Args/Returns/Raises sections
- Follow PEP 8 naming conventions
- Run `uv run ruff check` and `uv run ruff format` before committing

## Testing

### Running Tests

```bash
# All Go tests
go test ./...

# Go tests with verbose output
go test -v ./...

# Go tests with race detection
go test -race ./...

# Go tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Python tests
cd worker/python
uv run pytest -v
uv run pytest --cov=pyproc_worker

# All tests via Makefile
make test
```

### Writing Tests

**Go tests:**

```go
func TestPool_Call(t *testing.T) {
    // Arrange
    pool := setupTestPool(t)
    defer pool.Shutdown(context.Background())

    // Act
    var result map[string]any
    err := pool.Call(context.Background(), "echo", map[string]any{"msg": "hello"}, &result)

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result["msg"] != "hello" {
        t.Errorf("expected 'hello', got %v", result["msg"])
    }
}
```

**Python tests:**

```python
def test_expose_decorator():
    """Test that @expose registers functions correctly."""
    @expose
    def my_func(req):
        return {"result": req["value"] * 2}

    result = my_func({"value": 21})
    assert result == {"result": 42}
```

### Benchmarks

For performance-related changes, include benchmark results:

```bash
# Run benchmarks
go test -bench=. -benchmem ./bench/

# Quick benchmark
make bench-quick

# Full benchmark suite
make bench-full

# Compare before/after
git stash
go test -bench=. -count=5 ./bench/ > old.txt
git stash pop
go test -bench=. -count=5 ./bench/ > new.txt
benchstat old.txt new.txt
```

Include benchmark output in your PR description for performance changes.

## CI Checks

All PRs must pass these automated checks:

| Check | Tool | Description |
|-------|------|-------------|
| Go Build | `go build` | Compiles all packages |
| Go Lint | `golangci-lint` | Static analysis |
| Go Test | `go test` | Unit tests with coverage |
| Go Format | `gofmt` | Code formatting |
| Python Lint | `ruff check` | Static analysis |
| Python Format | `ruff format` | Code formatting |
| Python Type | `mypy` | Type checking |
| Python Test | `pytest` | Unit tests |
| Link Check | `lychee` | Validates documentation links |
| Security | CodeQL | Security vulnerability scanning |

## Release Process

Releases are managed by maintainers. Here's how it works:

1. Changes accumulate on `main` through merged PRs
2. When ready for release, a maintainer creates a version tag:

   ```bash
   git tag v0.4.0
   git push origin v0.4.0
   ```

3. GitHub Actions automatically:
   - Generates release notes from merged PRs
   - Creates a GitHub Release

**Versioning:**

- We follow [Semantic Versioning](https://semver.org/)
- `MAJOR.MINOR.PATCH` (e.g., `v1.2.3`)
- Breaking changes require major version bump

## Getting Help

**Stuck? Here's how to get help:**

- **Questions about contributing**: Open a [Discussion](https://github.com/YuminosukeSato/pyproc/issues)
- **Bug or feature**: Open an [Issue](https://github.com/YuminosukeSato/pyproc/issues)
- **Quick questions**: Comment on relevant issues/PRs

**Response times:**

- Issues: We aim to respond within 48 hours
- PRs: We aim to review within 48 hours
- Discussions: Best effort, usually within a week

---

## Thank You!

Every contribution matters, whether it's:

- Fixing a typo in documentation
- Reporting a bug
- Suggesting an improvement
- Writing code

Your time and effort make pyproc better for everyone. Thank you for being part of this community!

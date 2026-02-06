---
title: Support Matrix - pyproc
description: Supported Go, Python, and OS versions for pyproc
keywords: compatibility, support matrix, go, python, os, container
---

# Support Matrix

A list of runtimes, operating systems, and container configurations officially supported by pyproc.

## Go

| Version | Status | Notes |
|---------|--------|-------|
| 1.24 | Current (minimum) | go.mod minimum: 1.24.4 |
| 1.23 | Planned (v1.0) | CI matrix target for v1.0 |
| 1.22 | Planned (v1.0) | CI matrix target for v1.0 |
| 1.21 and earlier | Unsupported | Requires generics and other features |

Go 1.24 is the current minimum version specified in go.mod. Go 1.22 and 1.23 support is planned for v1.0 when the multi-version CI matrix is established.

## Python

| Version | Status | Notes |
|---------|--------|-------|
| 3.12 | Supported | |
| 3.11 | Supported | |
| 3.10 | Supported | |
| 3.9 | Supported | Minimum supported version |
| 3.8 and earlier | Unsupported | EOL |
| 3.13 and later | Untested | pyproject.toml: `>=3.9,<3.13` |

The `requires-python` field in pyproc-worker is defined as `>=3.9,<3.13`.
Python 3.13 support will be added after validation is complete.

## OS / Architecture

| OS | Architecture | Status |
|----|-------------|--------|
| Linux | amd64 | Supported |
| Linux | arm64 | Supported |
| macOS | amd64 (Intel) | Supported |
| macOS | arm64 (Apple Silicon) | Supported |
| Windows | All | Unsupported |

Windows is unsupported because pyproc relies on Unix Domain Socket (UDS) for IPC.
Windows AF_UNIX support is limited and cannot be guaranteed to work reliably.

## Container

pyproc assumes a single-container pattern.
The Go process and Python workers run within the same Pod and share a UDS.

| Pattern | Status | Notes |
|---------|--------|-------|
| Same Pod / emptyDir sharing | Supported | Recommended configuration |
| Sidecar configuration | Supported | Share socket via emptyDir |
| Separate Pod / network | Unsupported | UDS is local-only communication |

See [Kubernetes Deployment](../deployment/kubernetes.md) for details.

## CI Test Matrix

The following combinations are validated in CI.

### Go

- OS: ubuntu-latest
- Go: Version specified in go.mod (1.24.4)
- Tasks: test, lint, build

### Python

- OS: ubuntu-latest
- Python: Version managed by uv
- Tasks: ruff check, ruff format, ty check, pytest

### Benchmark

- OS: ubuntu-latest
- Runs in Go + Python (uv) environment
- Gates on p50 < 100us, p99 < 500us

Multi-version CI matrix (Go 1.22/1.23/1.24, Python 3.9-3.12) will be set up before v1.0.

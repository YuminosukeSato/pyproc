<!-- OPENSPEC:START -->
# OpenSpec Integration

This project uses OpenSpec for spec-driven development. When creating proposals, refer to:

- `openspec/AGENTS.md` - Workflow and format
- `openspec/project.md` - Project conventions
- `openspec/specs/` - Current specifications
- `openspec/changes/` - In-progress proposals

Before starting spec-driven development, run `openspec list` and `openspec show [item]` to review the current state.
<!-- OPENSPEC:END -->

---

# pyproc - AI Agent Instructions

## Project Core

pyproc's scope is fixed to "fast, stable invocation of Python from Go as if it were a local function, within the same host/Pod via UDS." Cross-host communication is out of scope.

v1.0 "Done" means not adding features, but "meeting the conditions for enterprise adoption":
- API/protocol stabilization
- Documentation and automation of operations, observability, security, compatibility, and release processes

## Public API (subject to SemVer)

1. Go API: exported symbols in `pkg/pyproc` (Pool, WorkerConfig, CallTyped, CodecType, Transport, etc.)
2. Python worker API: `expose`, `run_worker`
3. Wire protocol: compatibility conditions for JSON/MessagePack/Protobuf codecs
4. Config schema & env vars: `PYPROC_POOL_WORKERS`, `PYPROC_SOCKET_DIR`, etc.

## SemVer Policy (0.y.z period)

- Breaking changes bump MINOR (y)
- v1.0.0 defines the Public API
- Released versions are immutable (fixes ship as new versions)
- Go: tags `vX.Y.Z` (Go Modules convention)
- Python: PEP 440 compliant `X.Y.Z`

Current version gap: Go v0.4.0 / pyproc-worker 0.1.0

## Non-Goals

- Cross-host communication (use gRPC/REST services instead)
- Arbitrary user-submitted Python code execution (trusted code assumption)
- GPU cluster management or distributed inference infrastructure

## K8s/Container Strategy

Achieved through "K8s + container distribution completeness," not cloud-specific solutions:
- Reference Dockerfile (Go binary + Python runtime + pyproc-worker)
- K8s manifests (emptyDir for socket write directory)
- Same-container configuration is first-class (sidecar separation is a non-goal for v1.0)

## v1.0 Roadmap

- v0.5.0: High-efficiency IPC enhancements, config schema formalization, Dockerfile stabilization
- v0.6.0: Public API outline freeze, deprecate-then-remove procedure, compatibility table publication
- v0.7.0: Observability (metrics name/label/semantics stabilization)
- v0.8.0: Security hardening (UDS permissions/HMAC auth templating, K8s emptyDir design guidelines)
- v0.9.0: API Freeze, compatibility tests (contract tests) in CI
- v1.0.0: Public API finalized, enterprise-ready release

See `.ssd/` for details.

## Command Reference

```bash
# Go tests (race detector enabled)
go test -v -race ./...

# Python tests
cd worker/python && uv run pytest -v --cov=pyproc_worker

# Go lint
golangci-lint run ./...

# Python lint + format
cd worker/python && uv run ruff check . && uv run ruff format --check .

# Benchmarks
make bench-quick

# OpenSpec
openspec list
openspec show [item]
openspec validate --strict
```

## Subdirectory AGENTS.md

- `worker/python/AGENTS.md` - Python worker development
- `internal/AGENTS.md` - Go internal implementation
- `docs/AGENTS.md` - Documentation
- `bench/AGENTS.md` - Benchmarks

## Documentation

- `README.md` - User-facing overview
- `CONTRIBUTING.md` - Contribution guide
- `docs/design.md` - Design decisions
- `docs/security.md` - Security threat model
- `docs/ops.md` - Operations guide

## Review Guidelines

- Go errors must be wrapped with `fmt.Errorf("context: %w", err)`
- Python functions must have type hints and Google-style docstrings
- All exported Go types/functions must have doc comments
- Channel operations must use `select + context.Done()` for cancellation
- UDS socket permissions must be 0660
- Do not log PII or secrets
- Do not weaken tests, lints, or CI gates to pass checks
- Security-related files (`docs/security.md`, `internal/protocol/`, `.claude/rules/security.md`) require explicit reviewer approval
- Performance-sensitive changes in IPC hot paths must include benchmark results

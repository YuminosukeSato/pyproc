---
title: Versioning Policy - pyproc
description: SemVer rules, Public API definition, and deprecation policy for pyproc
keywords: versioning, semver, compatibility, deprecation, public api
---

# Versioning Policy

Public API definition, SemVer rules, and compatibility guarantees for pyproc.

## Public API Definition

The following constitute the Public API of pyproc.
Changes to the Public API are managed according to SemVer.

### Go

- Exported symbols (types, functions, methods, constants) in the `pkg/pyproc` package
- Fields of configuration structs such as `PoolOptions`

### Python

- The `expose` decorator in the `pyproc_worker` package
- The `run_worker` function in the `pyproc_worker` package
- CLI interface (`pyproc-worker` command)

### Wire Protocol

- 4-byte length-prefix framing (network byte order)
- JSON codec as the default
- MessagePack codec as opt-in

### Config Schema

- YAML configuration file schema

## SemVer Rules

pyproc follows [Semantic Versioning 2.0.0](https://semver.org/).

### 0.y.z Period (Current)

| Change Type | Version Bump |
|-------------|-------------|
| Breaking change | Bump MINOR (y) |
| Feature addition (backward-compatible) | Bump MINOR (y) |
| Bug fix | Bump PATCH (z) |

The 0.y.z period indicates an unstable API.
Breaking changes are made by bumping the MINOR version and documented in the CHANGELOG.

### 1.0.0 and Beyond

| Change Type | Version Bump |
|-------------|-------------|
| Breaking change | Bump MAJOR (x) |
| Feature addition (backward-compatible) | Bump MINOR (y) |
| Bug fix | Bump PATCH (z) |

Release criteria for v1.0.0: Public API finalized, compatibility tests complete, security audit passed.

## Go / Python Version Gap

Current versions:

| Component | Version | Notes |
|-----------|---------|-------|
| Go (pyproc) | v0.4.0 | Development is ahead |
| Python (pyproc-worker) | v0.1.0 | Will align with Go once stable |

Go development proceeds ahead of the Python worker. The Python worker version will be aligned once the API stabilizes.
At v1.0.0, the major versions of Go and Python will be synchronized.

## Wire Protocol Compatibility Rules

Wire protocol compatibility is managed under the following rules.

### Immutable Parts

- 4-byte length-prefix framing will not change
- Frame structure: `[4-byte big-endian length][payload]`

### Codec

- JSON codec is fixed as the default and will not be removed or changed
- MessagePack codec is provided as opt-in and will not become the default
- Adding new codecs is treated as a backward-compatible change

### Field Addition

- Adding JSON fields to request/response is backward-compatible
- Removing existing fields or changing their types is a breaking change

## Deprecation Policy

Deprecation of Public API follows these rules.

### Grace Period

- A minimum of 1 MINOR version grace period is provided
- Example: An API deprecated in v0.5.0 can be removed in v0.6.0 or later

### Notification Methods

- Document the deprecation in the CHANGELOG
- Go: Add `Deprecated:` annotation in godoc comments
- Python: Issue `DeprecationWarning` via `warnings.warn`
- Provide guidance on alternative APIs in documentation

### Removal Procedure

1. Add deprecation marker (MINOR release)
2. Maintain at least 1 MINOR version grace period
3. Remove in the next release containing breaking changes

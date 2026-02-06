# Python Worker

pyproc-worker PyPI package. Python worker implementation called from Go via UDS.

## Commands

```bash
# Install dependencies (pip is not supported)
uv sync --all-extras --dev

# Tests
uv run pytest -v --cov=pyproc_worker --cov-report=term-missing

# Lint & Format
uv run ruff check .
uv run ruff format --check .

# Type check
uv run ty check .
```

## Code Conventions

- Type hints required on all function signatures
- Docstrings: Google style (Args/Returns/Raises)
- Use `@expose` decorator to register public functions
- Raise errors explicitly; minimize catch blocks

## File Structure

```
pyproc_worker/
├── __init__.py       # expose, Worker class, run_worker exports
├── codec.py          # JSONCodec, get_codec()
├── tracing.py        # OpenTelemetry integration
├── cancellation.py   # CancellationError, CancellationManager
└── cli.py            # CLI entry point
tests/
└── test_*.py         # pytest format
```

## Public API (subject to SemVer)

- `expose` decorator
- `run_worker` function
- Their arguments, exceptions, and return types will be frozen at v1.0

## Notes

- GIL is bypassed via process isolation (not threads)
- Load models/heavy libraries at worker startup (not per-request)
- Avoid memory leaks: do not accumulate global state
- orjson is the default codec; msgspec is optional
- Currently v0.1.0. Maintain compatibility table with Go side v0.4.0

## Review Guidelines

- All functions must have type hints and Google-style docstrings
- Do not use pip; uv is the only supported package manager
- Error messages must not leak internal state or PII
- Test coverage must not decrease

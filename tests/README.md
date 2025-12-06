# E2E Integration Tests

End-to-end integration tests for pyproc Python DX improvements.

## Test Structure

- `e2e_schema_test.go` - Schema export → Typed API flow tests
- `e2e_environment_test.go` - Environment detection → Pool startup tests
- `e2e_test.go` - Complete workflow validation tests

## Fixtures

Worker scripts used for testing:

- `fixtures/typed_worker.py` - Worker with type hints for schema testing
- `fixtures/simple_worker.py` - Basic worker for integration testing
- `fixtures/complex_worker.py` - Complex types (nested, dataclasses)

## Running Tests

```bash
# Run all E2E tests
go test -v ./tests/

# Run specific test
go test -v ./tests/ -run TestE2E_SchemaExportToTypedAPI

# Run with verbose output
go test -v ./tests/ -test.v
```

## Test Requirements

- Python 3.9+ installed
- `pyproc-worker` CLI available in PATH (or fallback to basic detection)
- Unix domain sockets support (skips on Windows)

## Test Coverage

These tests validate:

1. **Schema Export**: Python type hints → Go struct generation
2. **Environment Detection**: Auto-detection of Python executables and virtual environments
3. **Worker Validation**: `pyproc-worker check` command catches errors before runtime
4. **Complete Workflow**: Real-world usage from validation to typed API calls

## Performance Benchmarks

See `../bench/python_dx_benchmark_test.go` for performance validation tests.

# Changelog

All notable changes to pyproc will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `PoolConfig.MaxInFlightPerWorker` to cap in-flight requests per worker (default: 1)

### Changed
- `MaxInFlight` now limits total concurrent requests across the pool
- Dispatch now enforces per-worker serialization to prevent head-of-line blocking

### Fixed
- Data race when `Pool.Call` and `Pool.Shutdown` run concurrently

## [0.1.0] - 2025-08-15

### Added

#### Core Features
- **Worker Pool Implementation** - Production-ready pool with round-robin load balancing
- **Process Management** - Robust worker lifecycle management with automatic restarts
- **Connection Pooling** - Persistent connections for improved performance
- **Health Monitoring** - Periodic health checks with automatic recovery
- **Backpressure Control** - Semaphore-based request limiting to prevent overload

#### API
- `NewPool()` - Create and configure worker pools
- `Pool.Start()` - Initialize all workers in the pool
- `Pool.Call()` - Execute Python functions with automatic load balancing
- `Pool.Shutdown()` - Graceful shutdown with resource cleanup
- `Pool.Health()` - Get current health status of all workers

#### Protocol
- JSON-RPC style messaging over Unix Domain Sockets
- 4-byte length header framing for reliable message boundaries
- Request/Response correlation with unique IDs
- Error propagation from Python to Go

#### Python Worker
- `@expose` decorator for function registration
- Automatic health endpoint
- Structured logging support
- Graceful shutdown handling

#### Documentation
- Comprehensive README with quick start guide
- Architecture design document (docs/design.md)
- Operations guide (docs/ops.md)
- Security best practices (docs/security.md)

#### Testing
- Unit tests for all core components
- Integration tests for Go-Python communication
- Pool functionality tests with TDD approach
- Benchmark suite for performance validation

#### Benchmarks
- Single worker vs pool performance comparison
- Parallel request handling benchmarks
- Latency percentile measurements (p50, p95, p99)
- Throughput testing with various payload sizes

#### DevOps
- GitHub Actions CI/CD pipeline
- golangci-lint configuration
- Makefile with common tasks (demo, test, bench, lint)
- Apache 2.0 license

### Performance
- Single worker: ~4,000 req/s
- 4 workers: ~15,000 req/s
- 8 workers: ~22,000 req/s
- Latency p50: 45μs, p95: 89μs, p99: 125μs

### Known Issues
- Workers may take time to stabilize on startup
- Socket cleanup may leave stale files on ungraceful shutdown
- Python GIL still limits single worker performance

### Contributors
- [@YuminosukeSato](https://github.com/YuminosukeSato)

---

## Roadmap

### [0.2.0] - Performance Improvements
- MessagePack protocol support
- Batch request processing
- Metrics collection and export
- Worker recycling after N requests

### [0.3.0] - Advanced Features
- Streaming support
- Bidirectional communication
- Request cancellation
- Circuit breaker pattern

### [0.4.0] - gRPC Support
- gRPC over Unix Domain Sockets
- Protocol buffer schemas
- Service discovery
- Advanced load balancing

### [0.5.0] - Arrow Integration
- Apache Arrow for zero-copy data transfer
- Large dataset optimization
- NumPy/Pandas integration
- Columnar data support

---

## Migration Guide

### From Development to v0.1.0

If you were using the pre-release version, update your code:

#### Before (single worker):
```go
worker := pyproc.NewWorker(cfg, nil)
worker.Start(ctx)
```

#### After (with pool):
```go
pool, _ := pyproc.NewPool(pyproc.PoolOptions{
    Config: pyproc.PoolConfig{
        Workers:              4,
        MaxInFlight:          10,
        MaxInFlightPerWorker: 1,
    },
    WorkerConfig: cfg,
}, nil)
pool.Start(ctx)
```

### Python Worker Updates

No changes required for Python workers. The `@expose` decorator and worker API remain the same.

---

## Acknowledgments

Thanks to all contributors and early adopters who provided valuable feedback to shape this release.

Special thanks to the Go and Python communities for the excellent libraries and tools that make pyproc possible.

[Unreleased]: https://github.com/YuminosukeSato/pyproc/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/YuminosukeSato/pyproc/releases/tag/v0.1.0

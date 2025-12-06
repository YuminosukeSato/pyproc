---
title: pyproc - Run Python like a local function from Go
description: Call Python functions from Go without CGO or microservices. Ultra-low latency, process isolation, and true parallelism.
keywords: go, python, interop, ml, machine learning, ipc, unix domain sockets
---

# pyproc

**Run Python like a local function from Go — no CGO, no microservices.**

[![Go Reference](https://pkg.go.dev/badge/github.com/YuminosukeSato/pyproc.svg)](https://pkg.go.dev/github.com/YuminosukeSato/pyproc)
[![Go Report Card](https://goreportcard.com/badge/github.com/YuminosukeSato/pyproc)](https://goreportcard.com/report/github.com/YuminosukeSato/pyproc)
[![PyPI](https://img.shields.io/pypi/v/pyproc-worker.svg)](https://pypi.org/project/pyproc-worker/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/YuminosukeSato/pyproc/blob/main/LICENSE)
[![CI](https://github.com/YuminosukeSato/pyproc/actions/workflows/ci.yml/badge.svg)](https://github.com/YuminosukeSato/pyproc/actions/workflows/ci.yml)

---

## The Challenge

Go excels at building high-performance web services, but sometimes you need Python:

- **Machine Learning Models**: Your models are trained in PyTorch/TensorFlow
- **Data Science Libraries**: You need pandas, numpy, scikit-learn
- **Legacy Code**: Existing Python code that's too costly to rewrite
- **Python-Only Libraries**: Some libraries only exist in Python ecosystem

Traditional solutions all have major drawbacks:

| Solution | Problems |
|----------|----------|
| **CGO + Python C API** | Complex setup, crashes can take down entire Go service, GIL still limits performance |
| **REST/gRPC Microservice** | Network latency, deployment complexity, service discovery, more infrastructure |
| **Shell exec** | High startup cost (100ms+), no connection pooling, process management nightmare |
| **Embedded Python** | GIL bottleneck, memory leaks, difficult debugging |

---

## The Solution

pyproc lets you call Python functions from Go as if they were local functions:

### :zap: **Zero Network Overhead**
Uses Unix Domain Sockets for IPC — no TCP stack, no serialization overhead

### :shield: **Process Isolation**
Python crashes don't affect your Go service — clean fault boundaries

### :rocket: **True Parallelism**
Multiple Python processes bypass the GIL — scale with CPU cores

### :package: **Simple Deployment**
Just your Go binary + Python scripts — no service mesh, no orchestration

### :bar_chart: **High Performance**
- **45μs** p50 latency
- **200,000+** req/s with 8 workers
- **Process pooling** for connection reuse

---

## Quick Example

=== "Go (Type-Safe API)"

    ```go
    package main

    import (
        "context"
        "github.com/YuminosukeSato/pyproc/pkg/pyproc"
    )

    // Define types for compile-time safety
    type PredictRequest struct {
        Value float64 `json:"value"`
    }

    type PredictResponse struct {
        Result float64 `json:"result"`
    }

    func main() {
        // Create pool of Python workers
        pool, _ := pyproc.NewPool(pyproc.PoolOptions{
            Config: pyproc.PoolConfig{
                Workers:     4,  // 4 Python processes
                MaxInFlight: 10, // Concurrent requests
            },
            WorkerConfig: pyproc.WorkerConfig{
                SocketPath:   "/tmp/pyproc.sock",
                PythonExec:   "python3",
                WorkerScript: "worker.py",
            },
        }, nil)

        ctx := context.Background()
        pool.Start(ctx)
        defer pool.Shutdown(ctx)

        // Call Python with type safety
        result, _ := pyproc.CallTyped[PredictRequest, PredictResponse](
            ctx, pool, "predict", PredictRequest{Value: 42},
        )

        // result.Result == 84.0 (type-safe!)
    }
    ```

=== "Python Worker"

    ```python
    from pyproc_worker import expose, run_worker

    @expose
    def predict(req):
        """Your ML model or Python logic here"""
        return {"result": req["value"] * 2}

    if __name__ == "__main__":
        run_worker()
    ```

That's it! No CGO, no microservices, no network complexity.

[Get Started in 5 Minutes](getting-started/quick-start.md){ .md-button .md-button--primary }
[View on GitHub](https://github.com/YuminosukeSato/pyproc){ .md-button }

---

## Perfect For

**:white_check_mark: Ideal Use Cases**

- Integrating existing Python ML models (PyTorch, TensorFlow, scikit-learn) into Go services
- Processing data with Python libraries (pandas, numpy) from Go applications
- Handling 1-5k RPS with JSON payloads under 100KB
- Deploying on the same host/pod without network complexity
- Migrating gradually from Python microservices to Go

**:x: Not Designed For**

- Cross-host communication → Use gRPC/REST APIs
- Windows support → Unix Domain Sockets only (Linux/macOS)
- GPU cluster management → Use Ray Serve, Triton
- Large-scale ML serving → Consider MLflow, KServe

---

## Why pyproc?

### vs CGO + Python C API
- **Simpler**: No C extension compilation
- **Safer**: Python crashes don't affect Go
- **Faster**: No GIL contention with multiple processes

### vs REST/gRPC Microservices
- **Lower Latency**: ~45μs vs ~1-5ms network overhead
- **Simpler Deployment**: No service discovery, load balancers
- **Fewer Dependencies**: No Kubernetes networking, service mesh

### vs go-plugin
- **Optimized for ML/DS**: Built-in worker pools, health checks
- **Lower Overhead**: Direct socket communication
- **Python-Native**: No protobuf definitions needed

---

## Key Features

- **:lock: Type-Safe API**: Compile-time type checking with Go generics
- **:arrows_counterclockwise: Auto-Restart**: Workers restart on crashes with exponential backoff
- **:heartbeat: Health Checks**: Built-in monitoring and diagnostics
- **:recycle: Connection Pooling**: Reuse connections for high throughput
- **:mag: Observability**: Structured logging, metrics, tracing support
- **:whale: Container-Friendly**: Works in Docker, Kubernetes, any OCI runtime

---

## Compatibility

| Component | Requirements |
|-----------|-------------|
| **Operating System** | Linux, macOS (Unix Domain Sockets required) |
| **Go Version** | 1.22+ |
| **Python Version** | 3.9+ (3.12 recommended) |
| **Architecture** | amd64, arm64 |

---

## Benchmarks

Performance on M1 MacBook Pro (8 workers):

```
BenchmarkPool/workers=1      235μs/op     4,255 req/s
BenchmarkPool/workers=2      124μs/op     8,065 req/s
BenchmarkPool/workers=4       68μs/op    14,706 req/s
BenchmarkPool/workers=8       45μs/op    22,222 req/s

BenchmarkPoolParallel/workers=8    5μs/op    200,000 req/s
```

**Latency Distribution** (8 workers):
- p50: 45μs
- p95: 89μs
- p99: 125μs

See [Performance Tuning Guide](guides/performance-tuning.md) for optimization techniques.

---

## Next Steps

<div class="grid cards" markdown>

-   :material-clock-fast:{ .lg .middle } **Quick Start**

    ---

    Get pyproc running in 5 minutes

    [:octicons-arrow-right-24: Start Here](getting-started/quick-start.md)

-   :material-shield-check:{ .lg .middle } **Type-Safe API**

    ---

    Learn how to use compile-time type checking

    [:octicons-arrow-right-24: Type Safety Guide](guides/type-safe-api.md)

-   :material-rocket-launch:{ .lg .middle } **Deploy to Production**

    ---

    Docker, Kubernetes, and production best practices

    [:octicons-arrow-right-24: Deployment Guide](deployment/docker.md)

-   :material-book-open-variant:{ .lg .middle } **Architecture**

    ---

    Understand how pyproc works under the hood

    [:octicons-arrow-right-24: Design Docs](reference/architecture.md)

</div>

---

## Community & Support

- **GitHub Issues**: [Bug reports and feature requests](https://github.com/YuminosukeSato/pyproc/issues)
- **Discussions**: [Ask questions and share ideas](https://github.com/YuminosukeSato/pyproc/discussions)
- **Contributing**: [Contribution guidelines](community/contributing.md)

---

## License

Apache 2.0 - See [LICENSE](https://github.com/YuminosukeSato/pyproc/blob/main/LICENSE) for details.

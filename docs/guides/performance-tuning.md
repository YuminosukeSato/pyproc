---
title: Performance Tuning - pyproc
description: Optimize pyproc for production workloads
keywords: performance, optimization, tuning
---

# Performance Tuning

!!! info "Coming Soon"
    Detailed performance tuning guide is under development.

## Quick Tips

### 1. Worker Count

Start with 2-8 workers per CPU core:

```go
Workers: runtime.NumCPU() * 2
```

### 2. Connection Pooling

Use `MaxInFlight` to control backpressure:

```go
MaxInFlight: 10  // 10 concurrent requests per worker
```

### 3. Benchmarking

```bash
go test -bench=. ./pkg/pyproc/
```

See [Architecture](../reference/architecture.md) for performance characteristics.

---
title: Failure Behavior - pyproc
description: Timeout hierarchy, retry, backpressure, and SLO definitions
keywords: timeout, retry, backpressure, SLO, failure
---

# Failure Behavior

This document describes the failure handling mechanisms of pyproc, including the timeout hierarchy, retry strategy, backpressure, and SLO definition templates.

## Timeout Hierarchy

pyproc has a timeout system based on the `effectiveDeadline` function, which selects the earliest applicable deadline.

Currently two layers are active:

```
Layer 1: Context deadline (set by the caller via context.WithTimeout)
Layer 2: Transport default (ProtocolConfig.RequestTimeout)
```

The `effectiveDeadline` function also accepts a per-call timeout parameter, but `Call` does not currently expose an option to set it. Per-call timeouts are achieved by wrapping the context with `context.WithTimeout`.

### Priority

The earliest (most restrictive) deadline is applied.

```
                 Context deadline
                     |
                     v
  effectiveDeadline --> select earliest deadline --> TimeoutError{Kind: winner}
                     ^
Transport default ---+
```

### Example

```go
// Layer 1: Context deadline (5 seconds from now)
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Layer 2: Transport default = ProtocolConfig.RequestTimeout (default 60s)

// Result: Context's 5s is the earliest, so timeout occurs at 5 seconds
// The returned TimeoutError.Kind is TimeoutKindContext
err := pool.Call(ctx, "predict", req, &resp)
```

### Configuration for Each Layer

| Layer | Configuration Method | Default Value |
|-------|---------------------|---------------|
| Context | `context.WithTimeout` / `context.WithDeadline` | None |
| Transport | `ProtocolConfig.RequestTimeout` | 60 seconds |

## Worker Crash Behavior

When a worker process terminates abnormally, the pool marks that worker as unhealthy and routes subsequent requests to other healthy workers.

### Current Behavior

```
Worker crash detected
        |
        v
  Worker marked unhealthy
        |
        v
  Traffic routed to remaining healthy workers
```

Automatic restart is not yet implemented. The `RestartConfig` struct exists in the configuration but is not wired into the pool health loop. Callers should monitor `Pool.Health()` and recreate the pool if too many workers become unhealthy.

### Planned: Automatic Restart (not yet implemented)

`RestartConfig` defines parameters for automatic worker restart with exponential backoff. This functionality is planned but not yet active.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `MaxAttempts` | `int` | 5 | Maximum number of retry attempts |
| `InitialBackoff` | `time.Duration` | 1000ms | Initial wait time |
| `MaxBackoff` | `time.Duration` | 30000ms | Maximum wait time |
| `Multiplier` | `float64` | 2.0 | Backoff multiplier |

## Backpressure

The Pool uses a semaphore to limit concurrency.

### Semaphore Mechanism

```go
semaphore := make(chan struct{}, Workers*MaxInFlight)
```

- `Workers`: Number of worker processes (default: 4)
- `MaxInFlight`: Maximum concurrent requests per worker (default: 10)
- Semaphore size = `Workers * MaxInFlight` (default: 40)

### Behavior When Semaphore Is Full

When the semaphore is full, `Pool.Call` blocks until the context is cancelled.

```go
select {
case p.semaphore <- struct{}{}:
    // Semaphore acquired, execute request
    defer func() { <-p.semaphore }()
case <-ctx.Done():
    // Context cancelled (including timeout)
    return ctx.Err()
}
```

Callers can control the maximum wait time for backpressure by setting a timeout on the context.

### Capacity Planning

| Workers | MaxInFlight | Max Concurrent Requests |
|---------|-------------|------------------------|
| 4 | 10 | 40 |
| 8 | 10 | 80 |
| 4 | 20 | 80 |

Requests exceeding `Workers * MaxInFlight` are queued until capacity becomes available.

## SLO Definition Template

A template for defining SLOs in pyproc-based services.

### Availability

```
healthy_workers / total_workers >= X%
```

Measured using the `HealthStatus` returned by `Pool.Health()`.

```go
status := pool.Health()
availability := float64(status.HealthyWorkers) / float64(status.TotalWorkers)
// Example: availability >= 0.75 (75%)
```

### Latency

```
p99 < Y ms
```

Measured as the execution time of `Pool.Call`. Benchmark targets:

- p50: < 100us
- p99: < 500us
- Payload: < 100KB JSON, 8 worker configuration

### Error Rate

```
failed_calls / total_calls < Z%
```

Error categories counted toward SLO (see [Error Categories](error-categories.md)):

- Timeout errors: Counted
- Connection errors: Counted
- Worker errors: Counted
- Protocol errors: Not counted (bug)
- Pool lifecycle errors: Not counted (operational error)

### Example Definition

```yaml
slo:
  availability:
    target: 99.9%
    window: 30d
    metric: healthy_workers / total_workers
  latency:
    target_p99: 500us
    target_p50: 100us
    window: 30d
  error_rate:
    target: 0.1%
    window: 30d
    excluded:
      - protocol_errors
      - pool_lifecycle_errors
```

Related documentation:

- [Error Categories](error-categories.md) - Error category classification and retry eligibility
- [Error Handling Guide](../guides/error-handling.md) - Best practices for error handling

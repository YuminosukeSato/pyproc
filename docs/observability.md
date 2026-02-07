---
title: Observability - pyproc
description: Distributed tracing, metrics, and logging for pyproc applications
keywords: observability, tracing, metrics, logging, opentelemetry, prometheus
---

# Observability

pyproc provides built-in observability features for monitoring your Go-Python IPC workloads. This guide shows you how to enable distributed tracing, collect metrics, and correlate logs with traces.

## Overview

Observability in pyproc consists of three pillars:

- Distributed Tracing: Track requests across Go and Python boundaries using OpenTelemetry
- Metrics: Collect performance metrics exposed via Prometheus
- Structured Logging: JSON logs with trace correlation using Go's slog

All three integrate seamlessly to help you debug latency issues, track error rates, and understand system behavior in production.

## Quick Start

Enable observability with minimal configuration:

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "github.com/YuminosukeSato/pyproc/pkg/pyproc"
    "github.com/YuminosukeSato/pyproc/pkg/pyproc/telemetry"
)

func main() {
    // Step 1: Create telemetry provider
    provider, shutdown := telemetry.NewProvider(telemetry.Config{
        ServiceName:  "my-service",
        Enabled:      true,
        ExporterType: "stdout",  // Options: "stdout", "jaeger", "otlp"
        SamplingRate: 1.0,
    })
    defer shutdown(context.Background())

    // Step 2: Create logger
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    // Step 3: Create pool with standard configuration
    opts := pyproc.PoolOptions{
        Config: pyproc.PoolConfig{
            Workers:     4,
            MaxInFlight: 10,
        },
        WorkerConfig: pyproc.WorkerConfig{
            SocketPath:   "/tmp/pyproc.sock",
            PythonExec:   "python3",
            WorkerScript: "worker.py",
        },
    }
    pool, _ := pyproc.NewPool(opts, logger)

    // Step 4: Attach tracer to pool
    pool.WithTracer(provider.Tracer("my-service"))

    pool.Start(context.Background())
    defer pool.Shutdown(context.Background())

    // Tracing is automatic - each Call() creates a span
    ctx := context.Background()
    var result map[string]interface{}
    _ = pool.Call(ctx, "predict", map[string]interface{}{"value": 42}, &result)
}
```

Access metrics at your configured Prometheus endpoint (typically `:9090/metrics` or `:8080/metrics` depending on your setup).

## Configuration

### Telemetry Config Options

The `telemetry.Config` structure controls observability behavior:

```go
type Config struct {
    // ServiceName identifies this service in traces
    ServiceName string

    // Enabled controls whether telemetry is active
    Enabled bool

    // SamplingRate controls trace sampling (0.0-1.0)
    // 1.0 = sample all requests, 0.1 = sample 10%
    SamplingRate float64

    // ExporterType specifies the OpenTelemetry exporter
    // Options: "stdout", "jaeger", "otlp"
    ExporterType string
}
```

### Initialization Pattern

Telemetry is initialized separately from the pool:

```go
import (
    "context"
    "log/slog"
    "os"
    "github.com/YuminosukeSato/pyproc/pkg/pyproc"
    "github.com/YuminosukeSato/pyproc/pkg/pyproc/telemetry"
)

// Step 1: Create telemetry provider
provider, shutdown := telemetry.NewProvider(telemetry.Config{
    ServiceName:  "my-service",
    Enabled:      true,
    ExporterType: "otlp",
    SamplingRate: 1.0,
})
defer shutdown(context.Background())

// Step 2: Create logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

// Step 3: Create pool
opts := pyproc.PoolOptions{
    Config: pyproc.PoolConfig{
        Workers:     4,
        MaxInFlight: 10,
    },
    WorkerConfig: pyproc.WorkerConfig{
        SocketPath:   "/tmp/pyproc.sock",
        PythonExec:   "python3",
        WorkerScript: "worker.py",
    },
}
pool, _ := pyproc.NewPool(opts, logger)

// Step 4: Attach tracer
pool.WithTracer(provider.Tracer("my-service"))
```

### Environment Variables

Configure telemetry via environment variables:

```bash
export PYPROC_TELEMETRY_SERVICE_NAME="my-service"
export PYPROC_TELEMETRY_ENABLED="true"
export PYPROC_TELEMETRY_EXPORTER_TYPE="otlp"
export PYPROC_TELEMETRY_SAMPLING_RATE="1.0"
```

## Backward Compatibility

### Protocol Changes

The observability integration (v0.7.1+) adds a `headers` field to the internal `Request` structure for W3C Trace Context propagation:

```go
type Request struct {
    // ... existing fields ...
    Headers map[string]string `json:"headers,omitempty"`  // v0.7.1+
}
```

**Compatibility guarantees:**

- **Old Python workers (< v0.7.1)**: Will ignore the `headers` field due to `omitempty` JSON tag. All existing functionality continues to work.
- **Old Go clients (< v0.7.1)**: Will not send trace context headers. Python workers will function normally without tracing.
- **Full tracing**: Requires both Go pool and Python worker to be v0.7.1 or later.

### Opt-In Design

Observability features are opt-in and do not affect existing code:

- Tracing requires explicit `pool.WithTracer()` call
- Without tracer attachment, Pool operates with zero overhead (no-op mode)
- Metrics collection is passive and does not modify request/response flow
- Logging remains unchanged for existing applications

### Migration Path

1. **Phase 1**: Update Go pool to v0.7.1 (tracing disabled by default)
2. **Phase 2**: Update Python workers to v0.7.1 when ready
3. **Phase 3**: Enable tracing by calling `pool.WithTracer()` after testing

No breaking changes to existing APIs or protocols.

## Distributed Tracing

### How Tracing Works

pyproc automatically creates OpenTelemetry spans for every `Call()` operation. The trace context propagates from Go to Python over the Unix Domain Socket using W3C Trace Context headers.

```
┌─────────────────────────────────────────────────────┐
│ Go Application                                       │
│                                                      │
│  pool.Call(ctx, "predict", req, &resp)              │
│    │                                                 │
│    ├─ Span: "pyproc.pool.call"                      │
│    │   ├─ Attributes:                               │
│    │   │   - function_name: "predict"               │
│    │   │   - worker_id: 3                           │
│    │   │                                             │
│    │   └─ UDS Request with trace context            │
│    │                                                 │
└────┼─────────────────────────────────────────────────┘
     │
     │ Unix Domain Socket
     │
┌────▼─────────────────────────────────────────────────┐
│ Python Worker                                        │
│                                                      │
│  @expose                                             │
│  def predict(req):                                   │
│    │                                                 │
│    ├─ Span: "pyproc.worker.execute"                 │
│    │   ├─ Parent: Go span                           │
│    │   ├─ Attributes:                               │
│    │   │   - function_name: "predict"               │
│    │   │                                             │
│    │   └─ User function execution                   │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Exporter Setup

The telemetry package supports three exporter types via the `ExporterType` field.

#### Stdout Exporter (Development)

Print traces to console for debugging:

```go
import (
    "context"
    "github.com/YuminosukeSato/pyproc/pkg/pyproc/telemetry"
)

provider, shutdown := telemetry.NewProvider(telemetry.Config{
    ServiceName:  "my-service",
    Enabled:      true,
    ExporterType: "stdout",
    SamplingRate: 1.0,
})
defer shutdown(context.Background())

// Use provider.Tracer("my-service") with pool.WithTracer()
```

#### Jaeger Exporter (Production)

Export traces to Jaeger for visualization:

```go
import (
    "context"
    "github.com/YuminosukeSato/pyproc/pkg/pyproc/telemetry"
)

provider, shutdown := telemetry.NewProvider(telemetry.Config{
    ServiceName:  "my-service",
    Enabled:      true,
    ExporterType: "jaeger",
    SamplingRate: 1.0,
})
defer shutdown(context.Background())

// Configure Jaeger endpoint via environment:
// export OTEL_EXPORTER_JAEGER_ENDPOINT=http://jaeger:14268/api/traces
```

#### OTLP Exporter (OpenTelemetry Collector)

Use the OpenTelemetry Protocol for vendor-neutral export:

```go
import (
    "context"
    "github.com/YuminosukeSato/pyproc/pkg/pyproc/telemetry"
)

provider, shutdown := telemetry.NewProvider(telemetry.Config{
    ServiceName:  "my-service",
    Enabled:      true,
    ExporterType: "otlp",
    SamplingRate: 1.0,
})
defer shutdown(context.Background())

// Configure OTLP endpoint via environment:
// export OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
```

### Custom Spans

Add custom spans to track specific operations:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
)

func processRequest(ctx context.Context, pool *pyproc.Pool, req Request) error {
    tracer := otel.Tracer("my-app")

    // Create a parent span
    ctx, span := tracer.Start(ctx, "process_request")
    defer span.End()

    // Add attributes
    span.SetAttributes(
        attribute.String("request.id", req.ID),
        attribute.Int("request.priority", req.Priority),
    )

    // Child span is automatic
    var result Response
    err := pool.Call(ctx, "predict", req.Data, &result)
    if err != nil {
        span.RecordError(err)
        return err
    }

    return nil
}
```

### Python Worker Tracing

The Python worker automatically extracts trace context from incoming requests. No code changes are required if you use the standard `@expose` decorator.

For custom instrumentation inside Python functions:

```python
from pyproc_worker import expose, run_worker
from opentelemetry import trace

tracer = trace.get_tracer(__name__)

@expose
def predict(req):
    # The parent span is already active
    with tracer.start_as_current_span("model_inference") as span:
        span.set_attribute("model.version", "v2.1")
        result = model.predict(req["features"])
        span.set_attribute("result.confidence", result["confidence"])
        return result
```

## Metrics

pyproc exposes metrics in Prometheus format at the configured endpoint.

### Available Metrics

#### Request Metrics

- `pyproc_requests_total` (Counter): Total number of requests
  - Labels: `function_name`, `status` (success/error)
- `pyproc_request_duration_seconds` (Histogram): Request latency distribution
  - Labels: `function_name`
  - Buckets: 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0
- `pyproc_requests_in_flight` (Gauge): Current number of active requests
  - Labels: `worker_id`

#### Worker Metrics

- `pyproc_workers_total` (Gauge): Number of worker processes
  - Labels: `status` (active/crashed/restarting)
- `pyproc_worker_restarts_total` (Counter): Worker restart count
  - Labels: `worker_id`, `reason` (crash/health_check)

#### Pool Metrics

- `pyproc_pool_queue_length` (Gauge): Number of requests waiting for workers
- `pyproc_pool_capacity` (Gauge): Maximum number of workers

### Querying Metrics

#### Request Rate (QPS)

```promql
rate(pyproc_requests_total[5m])
```

#### Error Rate

```promql
rate(pyproc_requests_total{status="error"}[5m])
  / rate(pyproc_requests_total[5m])
```

#### Latency Percentiles

```promql
histogram_quantile(0.50, rate(pyproc_request_duration_seconds_bucket[5m]))
histogram_quantile(0.95, rate(pyproc_request_duration_seconds_bucket[5m]))
histogram_quantile(0.99, rate(pyproc_request_duration_seconds_bucket[5m]))
```

#### Worker Health

```promql
pyproc_workers_total{status="active"}
```

### Grafana Dashboard

Import the prebuilt Grafana dashboard from the repository:

```bash
curl -o pyproc-dashboard.json \
  https://raw.githubusercontent.com/YuminosukeSato/pyproc/main/examples/monitoring/grafana-dashboard.json
```

The dashboard includes:

- Request rate and error rate over time
- Latency percentiles (p50, p95, p99)
- Worker health and restart events
- Queue depth and saturation

## Structured Logging

pyproc uses Go's `slog` package for structured logging. All logs are JSON-formatted by default.

### Log Levels

Configure log verbosity:

```go
config := pyproc.Config{
    Logging: pyproc.LoggingConfig{
        Level:  "debug", // debug, info, warn, error
        Format: "json",  // json or text
    },
}
```

### Trace Correlation

When `TraceEnabled` is true, every log entry includes trace context:

```json
{
  "time": "2024-01-15T10:30:45Z",
  "level": "INFO",
  "msg": "request completed",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "function_name": "predict",
  "duration_ms": 45,
  "status": "success"
}
```

This allows you to filter logs by trace ID when debugging specific requests.

### Request IDs

Add request IDs to correlate logs across services:

```go
import "github.com/YuminosukeSato/pyproc/internal/logging"

// Create logger with request ID
logger := logging.NewLogger(config.Logging).
    WithRequestID(requestID)

// Pass context with logger
ctx := logging.WithLogger(ctx, logger)

// Logs from pool.Call() will include the request ID
pool.Call(ctx, "predict", req, &result)
```

### Custom Log Fields

Add custom fields to all logs for a request:

```go
import (
    "log/slog"
    "github.com/YuminosukeSato/pyproc/internal/logging"
)

logger := logging.FromContext(ctx).With(
    slog.String("user_id", userID),
    slog.String("tenant", tenant),
)

ctx = logging.WithLogger(ctx, logger)
pool.Call(ctx, "predict", req, &result)
```

### Python Worker Logs

Python workers write logs to stderr. Configure Python logging to match the JSON format:

```python
import logging
import json
from pythonjsonlogger import jsonlogger

logger = logging.getLogger()
handler = logging.StreamHandler()
formatter = jsonlogger.JsonFormatter()
handler.setFormatter(formatter)
logger.addHandler(handler)
logger.setLevel(logging.INFO)

@expose
def predict(req):
    logger.info("prediction started", extra={"request_id": req.get("request_id")})
    result = model.predict(req["features"])
    logger.info("prediction completed", extra={"confidence": result["confidence"]})
    return result
```

Python logs are captured by the Go pool and forwarded with trace context.

## Performance Considerations

Observability features introduce overhead. Understand the tradeoffs before enabling in production.

### Tracing Overhead

Tracing adds latency to each request:

- Span creation: ~1-2μs per span
- Context propagation: ~500ns per boundary
- Export batching: amortized cost, negligible with batching

For high-throughput workloads (over 10k RPS), use sampling to reduce overhead:

```go
provider, shutdown := telemetry.NewProvider(telemetry.Config{
    ServiceName:  "my-service",
    Enabled:      true,
    ExporterType: "otlp",
    SamplingRate: 0.1, // Sample 10% of requests
})
defer shutdown(context.Background())
```

### Metrics Overhead

Metrics collection is lightweight:

- Counter increment: ~50ns
- Histogram observation: ~200ns
- Prometheus scrape: no impact on request path

Metrics are safe to enable in all environments.

### Logging Overhead

JSON logging adds CPU cost:

- Structured log call: ~1-2μs per log
- JSON serialization: ~500ns per field

For high-throughput workloads, use `info` or `warn` level to reduce log volume. Avoid `debug` in production.

### Benchmarking

Compare performance with and without observability:

```bash
# Baseline (no observability)
go test -bench=BenchmarkPool ./bench/ -benchtime=10s

# With tracing enabled
PYPROC_TELEMETRY_ENABLED=true go test -bench=BenchmarkPool ./bench/ -benchtime=10s
```

Expect 5-10% latency increase with full observability enabled at 100% sampling.

## Troubleshooting

### Missing Traces

If traces do not appear in your backend:

- Verify the exporter endpoint is reachable
- Check logs for export errors: `journalctl -u myapp | grep "trace export"`
- Confirm the OpenTelemetry Collector is running: `curl http://otel-collector:13133`
- Ensure telemetry is enabled: `Enabled: true` in telemetry.Config
- Verify pool.WithTracer() was called with a valid tracer

### High Cardinality Metrics

Avoid adding high-cardinality labels to metrics. Labels with many unique values (like request IDs or user IDs) cause memory growth in Prometheus.

Bad practice:

```go
// DO NOT: request_id has unbounded cardinality
span.SetAttributes(attribute.String("request.id", requestID))
```

Good practice:

```go
// Use request ID in logs, not metrics
logger.Info("request started", "request_id", requestID)
```

### Trace Context Loss

If Python spans do not appear as children of Go spans, check:

- The Python worker uses the correct trace extraction logic
- UDS message framing includes trace context headers
- OpenTelemetry SDK versions are compatible (use v1.x on both sides)

### Log Correlation Failures

If logs lack trace IDs:

- Verify telemetry provider is initialized before creating the pool
- Confirm pool.WithTracer() was called with a valid tracer
- Check that the context passed to `Call()` contains an active span
- Ensure the logger is configured to extract trace context from context.Context

### Performance Degradation

If observability causes unacceptable latency:

- Lower sampling rate: Set `SamplingRate: 0.01` in telemetry.Config (1% sampling)
- Use asynchronous exporters with batching (default for OTLP)
- Reduce log level to `warn` or `error`
- Consider disabling telemetry entirely for low-latency endpoints

## References

- OpenTelemetry Go SDK: [https://opentelemetry.io/docs/languages/go/](https://opentelemetry.io/docs/languages/go/)
- OpenTelemetry Python SDK: [https://opentelemetry.io/docs/languages/python/](https://opentelemetry.io/docs/languages/python/)
- Prometheus Querying: [https://prometheus.io/docs/prometheus/latest/querying/basics/](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- W3C Trace Context Specification: [https://www.w3.org/TR/trace-context/](https://www.w3.org/TR/trace-context/)
- Go slog Package: [https://pkg.go.dev/log/slog](https://pkg.go.dev/log/slog)

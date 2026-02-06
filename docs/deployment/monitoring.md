---
title: Monitoring Guide - pyproc
description: Monitor pyproc in production with Prometheus metrics
keywords: monitoring, observability, metrics, prometheus, grafana
---

# Monitoring

pyproc exposes Prometheus-compatible metrics via `MetricsHandler` and pool health checks via the `Health()` method. No external dependencies are required.

## Metrics Endpoint Setup

Use `MetricsHandler` to serve metrics in Prometheus text exposition format:

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/YuminosukeSato/pyproc/pkg/pyproc"
)

func main() {
    pool, err := pyproc.NewPoolWithMetrics(pyproc.PoolOptions{
        Config: pyproc.PoolConfig{Workers: 4, MaxInFlight: 10, MaxInFlightPerWorker: 1},
        WorkerConfig: pyproc.WorkerConfig{
            PythonExec:   "python3",
            WorkerScript: "worker.py",
            SocketPath:   "/tmp/pyproc",
        },
    }, nil)
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    if err := pool.Start(ctx); err != nil {
        log.Fatal(err)
    }

    http.Handle("/metrics", pyproc.MetricsHandler(pool))
    log.Fatal(http.ListenAndServe(":9090", nil))
}
```

## Available Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `pyproc_requests_total` | counter | `status` (success, failed, timeout) | Total number of requests by outcome |
| `pyproc_request_duration_seconds` | gauge | `quantile` (0.5, 0.95, 0.99) | Request latency percentiles in seconds |
| `pyproc_workers_total` | gauge | | Total number of workers in the pool |
| `pyproc_workers_healthy` | gauge | | Number of healthy workers |
| `pyproc_inflight_requests` | gauge | | Number of in-flight requests |
| `pyproc_worker_restarts_total` | counter | | Total worker restarts |

## Health Checks

Use `pool.Health()` to get a snapshot of pool health:

```go
health := pool.Health()
fmt.Printf("Total: %d, Healthy: %d, LastCheck: %s\n",
    health.TotalWorkers, health.HealthyWorkers, health.LastCheck)
```

Expose as an HTTP health endpoint:

```go
http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
    health := pool.Health()
    if health.HealthyWorkers == 0 {
        w.WriteHeader(http.StatusServiceUnavailable)
        fmt.Fprintf(w, "unhealthy: 0/%d workers healthy\n", health.TotalWorkers)
        return
    }
    fmt.Fprintf(w, "ok: %d/%d workers healthy\n",
        health.HealthyWorkers, health.TotalWorkers)
})
```

## Prometheus Configuration

Add a scrape target in `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: "pyproc"
    scrape_interval: 15s
    static_configs:
      - targets: ["localhost:9090"]
```

## Grafana Dashboard

Import the following JSON as a Grafana dashboard panel to visualize request rates and latency:

```json
{
  "panels": [
    {
      "title": "Request Rate",
      "type": "timeseries",
      "targets": [
        {
          "expr": "rate(pyproc_requests_total[1m])",
          "legendFormat": "{{status}}"
        }
      ]
    },
    {
      "title": "Latency Percentiles",
      "type": "timeseries",
      "targets": [
        {
          "expr": "pyproc_request_duration_seconds{quantile=\"0.5\"}",
          "legendFormat": "p50"
        },
        {
          "expr": "pyproc_request_duration_seconds{quantile=\"0.95\"}",
          "legendFormat": "p95"
        },
        {
          "expr": "pyproc_request_duration_seconds{quantile=\"0.99\"}",
          "legendFormat": "p99"
        }
      ]
    },
    {
      "title": "Worker Health",
      "type": "gauge",
      "targets": [
        {
          "expr": "pyproc_workers_healthy / pyproc_workers_total",
          "legendFormat": "health ratio"
        }
      ]
    },
    {
      "title": "In-Flight Requests",
      "type": "timeseries",
      "targets": [
        {
          "expr": "pyproc_inflight_requests",
          "legendFormat": "inflight"
        }
      ]
    }
  ]
}
```

## Alerting Rules

Example Prometheus alerting rules for pyproc:

```yaml
groups:
  - name: pyproc
    rules:
      - alert: PyProcNoHealthyWorkers
        expr: pyproc_workers_healthy == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "No healthy pyproc workers"
          description: "All pyproc workers are unhealthy for more than 1 minute."

      - alert: PyProcHighErrorRate
        expr: sum(rate(pyproc_requests_total{status="failed"}[5m])) / sum(rate(pyproc_requests_total[5m])) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "pyproc error rate above 5%"
          description: "Request failure rate is {{ $value | humanizePercentage }}."

      - alert: PyProcHighLatency
        expr: pyproc_request_duration_seconds{quantile="0.99"} > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "pyproc p99 latency above 500ms"
          description: "p99 latency is {{ $value }}s."

      - alert: PyProcWorkerRestarts
        expr: rate(pyproc_worker_restarts_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "pyproc workers restarting frequently"
          description: "Worker restart rate is {{ $value }}/s."
```

See [Operations Guide](../reference/operations.md) for deployment and runtime configuration details.

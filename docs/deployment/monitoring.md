---
title: Monitoring Guide - pyproc
description: Monitor pyproc in production
keywords: monitoring, observability, metrics
---

# Monitoring

!!! info "Coming Soon"
    Detailed monitoring guide is under development.

## Health Checks

```go
health := pool.Health()
fmt.Printf("Workers: %d, Active: %d\n",
    health.Workers, health.ActiveRequests)
```

## Metrics

Enable logging:

```go
logger := pyproc.NewLogger(pyproc.LoggingConfig{
    Level:  "info",
    Format: "json",
})
```

See [Operations Guide](../reference/operations.md) for details.

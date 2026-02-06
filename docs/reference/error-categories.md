---
title: Error Categories - pyproc
description: Classification of all error types in pyproc with retry and SLO guidance
keywords: errors, categories, timeout, connection, protocol, worker, pool
---

# Error Categories

Errors returned by pyproc are classified into five categories. Retry eligibility and SLO accounting guidance are provided for each category.

## 1. Timeout Errors

Occur when a call does not complete within the time limit. Returned as `*pyproc.TimeoutError`, with the `Kind` field identifying the source of the timeout.

| Kind | Meaning |
|------|---------|
| `TimeoutKindContext` | The deadline set on the caller's `context.Context` was exceeded |
| `TimeoutKindPerCall` | The per-call timeout specified at `Call` time was exceeded |
| `TimeoutKindTransport` | The default transport-layer timeout set via `ProtocolConfig.RequestTimeout` was exceeded |

Detection:

```go
var te *pyproc.TimeoutError
if errors.As(err, &te) {
    switch te.Kind {
    case pyproc.TimeoutKindContext:
        // Caller context deadline exceeded
    case pyproc.TimeoutKindPerCall:
        // Per-call timeout exceeded
    case pyproc.TimeoutKindTransport:
        // Transport layer default timeout exceeded
    }
}
```

- Retry: Yes (idempotent operations only)
- SLO: Counted

## 2. Connection Errors

Occur when UDS connection to a worker fails. Returned in situations such as a missing socket file or a stopped worker process.

Typical error messages:

- `"failed to connect to worker at <path> after <duration>"` (timeout in `ConnectToWorker`)
- `"failed to connect to <path>: <cause>"` (connection failure in `Pool.connect`)
- `"failed to connect: <cause>"` (connection failure within `Pool.Call`)

Detection:

```go
// Connection errors are detected via string matching
if strings.Contains(err.Error(), "failed to connect") {
    // Connection error
}
```

Note: String-based error detection is fragile. Wrapping detection in a helper function is recommended so that call sites can be updated in one place when typed errors become available.

```go
// Recommended: wrap detection in a helper
func IsConnectionError(err error) bool {
    return err != nil && strings.Contains(err.Error(), "failed to connect")
}
```

Typed/sentinel errors for connection and protocol categories are planned for v1.0.

- Retry: Yes (after worker restart)
- SLO: Counted

## 3. Protocol Errors

Errors occurring at the wire protocol layer. Include framing failures (reading/writing message lengths) and JSON decode failures.

Typical error messages:

- `"failed to unmarshal response: <cause>"` (JSON decode failure for response)
- `"response body is nil"` (empty response body)
- Framing errors (invalid message length, EOF during read)

Detection:

```go
if strings.Contains(err.Error(), "unmarshal") ||
    strings.Contains(err.Error(), "response body is nil") {
    // Protocol error
}
```

- Retry: No (possible data corruption)
- SLO: Not counted (treat as a bug)

## 4. Worker Errors

Errors originating from the Python worker side. Correspond to the error returned by `Response.Error()` when the `OK` field of `protocol.Response` is `false`.

Typical cases:

- Python-side exceptions (`Response.OK == false`, message in `Response.ErrorMsg`)
- Worker process crash (connection is severed)

Detection:

```go
// Python-side exceptions are returned via resp.Error()
// Pool.Call returns this error as-is
err := pool.Call(ctx, "predict", req, &resp)
if err != nil {
    // If it is neither a TimeoutError nor a connection error,
    // it is likely a worker error
    var te *pyproc.TimeoutError
    if !errors.As(err, &te) && !strings.Contains(err.Error(), "failed to connect") {
        // Worker error
    }
}
```

- Retry: Case-by-case (business logic errors: no, transient errors: yes)
- SLO: Counted

## 5. Pool Lifecycle Errors

Errors related to Pool lifecycle. Occur when the Pool has been shut down or when no healthy workers are available.

Typical error messages:

- `"pool is shut down"` (`Call` after `Pool.Shutdown` has been invoked)
- `"no healthy workers available"` (all workers are unhealthy)

Detection:

```go
// Use strings.Contains to handle wrapped errors
if strings.Contains(err.Error(), "pool is shut down") {
    // Pool needs to be recreated
}
if strings.Contains(err.Error(), "no healthy workers available") {
    // Wait for recovery via health check, or recreate Pool
}
```

- Retry: No (Pool recreation required)
- SLO: Not counted (treat as an operational error)

## Summary

| Category | Type / Detection | Retry | SLO |
|----------|-----------------|-------|-----|
| Timeout | `errors.As(*TimeoutError)` | Yes (idempotent only) | Counted |
| Connection | `"failed to connect"` string | Yes (after restart) | Counted |
| Protocol | `"unmarshal"` etc. string | No | Not counted |
| Worker | Errors not matching the above | Case-by-case | Counted |
| Pool lifecycle | `"pool is shut down"` etc. | No | Not counted |

Related documentation:

- [Failure Behavior](failure-behavior.md) - Details on timeout hierarchy, retry, and backpressure
- [Error Handling Guide](../guides/error-handling.md) - Best practices for error handling

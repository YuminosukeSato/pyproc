---
title: Error Handling Guide - pyproc
description: Handle errors gracefully in pyproc
keywords: errors, exceptions, handling
---

# Error Handling

Best practices for error handling in pyproc. This guide covers error classification, detection methods, and handling approaches for both the Go and Python sides.

## Error Detection Basics

Errors returned by pyproc are detected using `errors.Is` / `errors.As`. See [Error Categories](../reference/error-categories.md) for detailed category information.

### Go Side: Branching with errors.As

```go
result, err := pyproc.CallTyped[Req, Resp](ctx, pool, "predict", req)
if err != nil {
    var te *pyproc.TimeoutError
    if errors.As(err, &te) {
        // Timeout: identify the source via Kind
        log.Warn("timeout", "kind", te.Kind, "duration", te.Timeout)
        // Retry is possible for idempotent operations
        return handleTimeout(te)
    }

    if strings.Contains(err.Error(), "pool is shut down") {
        // Pool needs to be recreated
        return ErrPoolShutdown
    }

    if strings.Contains(err.Error(), "no healthy workers available") {
        // All workers are unhealthy: wait for recovery
        return ErrNoHealthyWorkers
    }

    if strings.Contains(err.Error(), "failed to connect") {
        // Connection error: retry possible after worker restart
        log.Warn("connection error", "error", err)
        return handleConnectionError(err)
    }

    // Worker error or protocol error
    return fmt.Errorf("call failed: %w", err)
}
// Use the result
log.Info("prediction completed", "prediction", result.Prediction)
```

### Go Side: Setting Timeouts

```go
// Control timeout via Context
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := pool.Call(ctx, "predict", req, &resp)
if err != nil {
    var te *pyproc.TimeoutError
    if errors.As(err, &te) && te.Kind == pyproc.TimeoutKindContext {
        // Context timeout
    }
}
```

See [Failure Behavior](../reference/failure-behavior.md) for details on the timeout hierarchy.

### Python Side: Exception Handling

Exceptions raised in the Python worker are returned to the Go side as `Response.OK = false`. Distinguish errors explicitly on the worker side.

```python
@expose
def predict(req: dict[str, Any]) -> dict[str, Any]:
    """Run prediction.

    Args:
        req: Input dictionary containing a features key

    Returns:
        Dictionary with prediction and confidence

    Raises:
        ValueError: When required keys are missing
    """
    if "features" not in req:
        raise ValueError("features key is required")

    try:
        result = model.predict(req["features"])
        return {"prediction": result, "confidence": 0.95}
    except Exception as e:
        # Log unexpected errors and re-raise
        logger.error("prediction failed", error=str(e))
        raise
```

Exceptions raised on the Python side are returned to Go as errors stored in `Response.ErrorMsg`.

### Python Side: Error Classification

Distinguishing between business logic errors and transient errors makes it easier for the Go side to make retry decisions.

```python
@expose
def process(req: dict[str, Any]) -> dict[str, Any]:
    """Process a request.

    Args:
        req: Data to process

    Returns:
        Processing result

    Raises:
        ValueError: When input data is invalid
        RuntimeError: When a transient failure occurs
    """
    if not validate(req):
        # Business logic error: not retryable
        raise ValueError("invalid input")

    try:
        return call_external_service(req)
    except ConnectionError:
        # Transient error: retryable
        raise RuntimeError("temporary failure, retry later")
```

## Health Checks

Use `Pool.Health()` to check worker availability.

```go
status := pool.Health()
if status.HealthyWorkers < status.TotalWorkers {
    log.Warn("degraded",
        "healthy", status.HealthyWorkers,
        "total", status.TotalWorkers,
    )
}
```

## Related Documentation

- [Error Categories](../reference/error-categories.md) - Error category classification, retry eligibility, and SLO accounting guidance
- [Failure Behavior](../reference/failure-behavior.md) - Timeout hierarchy, retry, backpressure, and SLO definition templates
- [Troubleshooting Guide](../deployment/troubleshooting.md) - Deployment troubleshooting

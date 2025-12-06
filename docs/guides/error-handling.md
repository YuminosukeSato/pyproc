---
title: Error Handling Guide - pyproc
description: Handle errors gracefully in pyproc
keywords: errors, exceptions, handling
---

# Error Handling

!!! info "Coming Soon"
    Detailed error handling guide is under development.

## Quick Reference

### Go Side

```go
result, err := pyproc.CallTyped[Req, Resp](ctx, pool, "predict", req)
if err != nil {
    // Handle transport/communication error
    return fmt.Errorf("call failed: %w", err)
}

// Process result
fmt.Println(result)
```

### Python Side

```python
@expose
def predict(req):
    try:
        return {"result": process(req)}
    except ValueError as e:
        return {"error": str(e)}
```

For more examples, see [Troubleshooting Guide](../deployment/troubleshooting.md).

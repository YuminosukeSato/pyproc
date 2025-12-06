---
title: Testing Guide - pyproc
description: Test pyproc integrations
keywords: testing, unit tests, integration tests
---

# Testing

!!! info "Coming Soon"
    Detailed testing guide is under development.

## Quick Reference

### Go Tests

```go
func TestPyproc(t *testing.T) {
    pool, _ := pyproc.NewPool(opts, nil)
    defer pool.Shutdown(context.Background())

    result, err := pool.Call(ctx, "predict", req, &resp)
    assert.NoError(t, err)
    assert.Equal(t, expected, resp)
}
```

### Python Tests

```python
import pytest
from worker import predict

def test_predict():
    result = predict({"value": 42})
    assert result["result"] == 84
```

See [CONTRIBUTING.md](../community/contributing.md) for test requirements.

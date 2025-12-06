---
title: Worker Development Guide - pyproc
description: Build production-ready Python workers
keywords: python, worker, development
---

# Worker Development

!!! info "Coming Soon"
    Detailed worker development guide is under development.

## Quick Reference

### Basic Worker

```python
from pyproc_worker import expose, run_worker

@expose
def my_function(req):
    """Process request and return response"""
    return {"result": req["value"] * 2}

if __name__ == "__main__":
    run_worker()
```

For more examples, see [pyproc repository](https://github.com/YuminosukeSato/pyproc/tree/main/examples).

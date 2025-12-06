---
title: Your First Integration - pyproc
description: Build a complete Go-Python integration step-by-step
keywords: tutorial, integration, example
---

# Your First Integration

Build a production-ready Python ML integration from scratch.

!!! info "Coming Soon"
    This guide is under development. For now, see:

    - **[Quick Start](quick-start.md)**: Get started quickly
    - **[Type-Safe API](../guides/type-safe-api.md)**: Learn CallTyped
    - **[Examples](https://github.com/YuminosukeSato/pyproc/tree/main/examples)**: Working code examples

## Example Pattern

For a complete example, see [`examples/basic/`](https://github.com/YuminosukeSato/pyproc/tree/main/examples/basic):

- `main.go`: Go side with Pool setup
- `worker.py`: Python side with exposed functions

Run it with:
```bash
make demo
```

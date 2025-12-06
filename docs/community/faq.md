---
title: FAQ - pyproc
description: Frequently asked questions about pyproc
keywords: faq, questions, answers
---

# Frequently Asked Questions

Common questions about pyproc.

## General

### What is pyproc?

pyproc is a Go library that lets you call Python functions from Go without CGO or microservices, using Unix Domain Sockets for low-latency IPC.

### Why not use CGO?

CGO has several drawbacks:
- Complex setup and compilation
- Python crashes can take down the Go process
- GIL still limits performance
- Difficult debugging

pyproc provides process isolation and bypasses the GIL with multiple workers.

### Why not use gRPC/REST microservices?

For same-host communication, microservices add unnecessary overhead:
- Network latency (~1-5ms vs ~45μs with pyproc)
- Service discovery complexity
- More infrastructure (load balancers, service mesh)

Use microservices when you need cross-host communication or language-agnostic APIs.

### Does pyproc support Windows?

No. pyproc requires Unix Domain Sockets, which are not fully supported on Windows.

**Alternatives**:
- Use Windows Subsystem for Linux (WSL)
- Use Docker with Linux containers

---

## Performance

### What's the latency overhead?

- p50: ~45μs
- p95: ~89μs
- p99: ~125μs

This includes socket communication, JSON serialization, and Python execution.

### How many workers should I use?

Start with **2-8 workers per CPU core**. Profile your workload to find the optimal number.

### Can I use pyproc for high-throughput scenarios?

Yes! pyproc handles **200,000+ req/s** with 8 workers in parallel benchmarks. See [Performance Tuning](../guides/performance-tuning.md).

---

## Python Integration

### Can I use existing Python packages?

Yes! You can use any Python package (PyTorch, TensorFlow, pandas, etc.) in your workers.

### How do I pass complex data structures?

Use JSON-serializable types. See [Type-Safe API Guide](../guides/type-safe-api.md) for details.

### Can I use async Python code?

Yes, but pyproc doesn't provide special async support. Use Python's `asyncio` normally within your worker functions.

---

## Deployment

### Can I use pyproc in Docker?

Yes! See [Docker Deployment Guide](../deployment/docker.md).

### Can I use pyproc in Kubernetes?

Yes! Use same-pod deployments with shared volumes for Unix Domain Sockets. See [Kubernetes Guide](../deployment/kubernetes.md).

### How do I handle worker crashes?

pyproc automatically restarts crashed workers with exponential backoff. See [Architecture](../reference/architecture.md).

---

## Development

### How do I debug Python workers?

Use Python's standard debugging tools:

```python
@expose
def my_function(req):
    import pdb; pdb.set_trace()  # Breakpoint
    # ... your code
```

See [Troubleshooting Guide](../deployment/troubleshooting.md).

### Can I hot-reload Python code?

Not directly. You need to restart the worker pool to reload Python code.

---

## Security

### Is pyproc secure?

pyproc uses Unix socket permissions for access control. It's designed for **trusted code execution**, not for running untrusted Python scripts.

See [Security Guide](../reference/security.md) for details.

### Can I sandbox Python workers?

pyproc doesn't provide sandboxing. Use system-level tools like gVisor or Firecracker if you need isolation.

---

## Still Have Questions?

- **Ask in Discussions**: [GitHub Discussions](https://github.com/YuminosukeSato/pyproc/discussions)
- **Report Issues**: [GitHub Issues](https://github.com/YuminosukeSato/pyproc/issues)

---
title: Troubleshooting Guide - pyproc
description: Solutions to common problems when using pyproc
keywords: troubleshooting, debugging, errors, problems
---

# Troubleshooting Guide

Quick solutions to common problems when using pyproc.

## Quick Checklist

Before diving into specific issues, verify these basics:

- [ ] **Python dependencies installed**: `pip list | grep pyproc-worker`
- [ ] **Socket path exists and writable**: `ls -la /tmp/pyproc.sock`
- [ ] **Worker script has no syntax errors**: `python3 worker.py`
- [ ] **Go version is 1.22+**: `go version`
- [ ] **Python version is 3.9+**: `python3 --version`
- [ ] **No firewall/SELinux blocking**: Check system logs

---

## Worker Won't Start

### Symptom

```
failed to start worker: context deadline exceeded
```

### Common Causes & Solutions

#### 1. Python Not Found

**Diagnosis**:
```bash
which python3
# If empty, Python not in PATH
```

**Solution**:
```go
WorkerConfig{
    PythonExec: "/usr/bin/python3",  // Use absolute path
    // or
    PythonExec: "/path/to/venv/bin/python",  // Virtual env
}
```

#### 2. Missing pyproc-worker Package

**Diagnosis**:
```bash
python3 -c "from pyproc_worker import run_worker"
# ModuleNotFoundError if missing
```

**Solution**:
```bash
pip install pyproc-worker
# or in virtual env
/path/to/venv/bin/pip install pyproc-worker
```

#### 3. Worker Script Syntax Error

**Diagnosis**:
```bash
python3 worker.py
# Should not error immediately (waits for socket connection)
```

**Solution**: Fix Python syntax errors
```python
# Check for common issues:
# - Indentation errors
# - Missing imports
# - Undefined functions
```

#### 4. Socket Path Not Writable

**Diagnosis**:
```bash
touch /tmp/test.sock
# Permission denied if not writable
```

**Solution**:
```bash
# Ensure directory is writable
mkdir -p /tmp
chmod 755 /tmp

# Or use user-specific path
export XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR:-/tmp}
SocketPath: "$XDG_RUNTIME_DIR/pyproc.sock"
```

#### 5. Stale Socket File

**Diagnosis**:
```bash
ls -la /tmp/pyproc.sock*
# Old socket files exist
```

**Solution**:
```bash
# Clean up old sockets
rm -f /tmp/pyproc.sock*
```

---

## High Latency

### Symptom

```
p99 latency > 500ms (expected < 100ms)
```

### Diagnosis

Enable metrics to identify bottleneck:

```go
pool, _ := pyproc.NewPool(pyproc.PoolOptions{
    Config: pyproc.PoolConfig{
        Workers:              4,
        MaxInFlight:          10,
        MaxInFlightPerWorker: 1,
    },
    // ...
}, logger)

// Check pool health
health := pool.Health()
fmt.Printf("Workers: %d, Active: %d\n", health.Workers, health.ActiveRequests)
```

### Common Causes & Solutions

#### 1. Too Few Workers

**Symptom**: All workers constantly busy

**Diagnosis**:
```go
health := pool.Health()
if health.ActiveRequests >= health.Workers * maxInFlight {
    // Pool is saturated
}
```

**Solution**: Increase worker count
```go
Config: pyproc.PoolConfig{
    Workers: runtime.NumCPU() * 2,  // Start with 2x CPU cores
}
```

#### 2. Python GIL Contention

**Symptom**: Single worker performing poorly

**Solution**: Use multiple processes (already default in pyproc)
```go
// pyproc automatically bypasses GIL with multiple processes
Workers: 4,  // Each is a separate Python process
```

#### 3. Large Payloads

**Diagnosis**: Log request/response sizes

**Solution**: Consider MessagePack for large payloads
```go
// Switch to MessagePack codec (requires Python msgpack)
import "github.com/YuminosukeSato/pyproc/pkg/pyproc/codec"

transport := pyproc.NewUnixTransport(cfg)
transport.SetCodec(codec.NewMsgpackCodec())
```

See [Codec Performance Reference](../reference/codec-performance.md) for benchmarks.

#### 4. Slow Python Logic

**Diagnosis**: Profile Python worker
```python
import time

@expose
def predict(req):
    start = time.time()

    # Your logic
    result = expensive_operation(req)

    elapsed = time.time() - start
    print(f"Processing took {elapsed*1000:.2f}ms")

    return {"result": result}
```

**Solution**: Optimize Python code
- Cache model loading
- Use numpy vectorization
- Consider multiprocessing for CPU-bound tasks

---

## Memory Leaks

### Symptom

Worker memory grows unbounded over time.

### Diagnosis

#### Go Side

```bash
# Enable Go profiling
go tool pprof http://localhost:6060/debug/pprof/heap
```

```go
import _ "net/http/pprof"

// In main()
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

#### Python Side

```python
import tracemalloc

tracemalloc.start()

@expose
def predict(req):
    # Your logic
    result = process(req)

    # Check memory
    current, peak = tracemalloc.get_traced_memory()
    print(f"Current memory: {current / 10**6:.2f} MB, Peak: {peak / 10**6:.2f} MB")

    return result
```

### Common Causes & Solutions

#### 1. Model Not Cached in Python

**Problem**: Loading model on every request
```python
@expose
def predict(req):
    model = load_model("model.pkl")  # ❌ Reloads every time!
    return model.predict(req)
```

**Solution**: Load once at module level
```python
# Load model once when worker starts
MODEL = load_model("model.pkl")

@expose
def predict(req):
    return MODEL.predict(req)  # ✅ Reuse cached model
```

#### 2. Large Response Accumulation

**Problem**: Keeping references to responses

**Solution**: Let Python GC clean up
```python
@expose
def batch_predict(req):
    samples = req["samples"]

    # Process in batches to avoid accumulation
    results = []
    for batch in chunks(samples, 100):
        batch_results = process_batch(batch)
        results.extend(batch_results)
        # batch_results is freed by GC here

    return {"results": results}
```

#### 3. Go Connection Leaks

**Problem**: Not closing connections

**Solution**: Always defer pool shutdown
```go
pool, _ := pyproc.NewPool(opts, logger)
defer pool.Shutdown(ctx)  // ✅ Ensures cleanup
```

---

## Connection Errors

### Symptom

```
dial unix /tmp/pyproc.sock: connect: connection refused
```

### Causes & Solutions

#### 1. Worker Not Started

**Diagnosis**: Check worker process
```bash
ps aux | grep worker.py
```

**Solution**: Ensure `pool.Start()` is called
```go
if err := pool.Start(ctx); err != nil {
    log.Fatal(err)
}
```

#### 2. Socket Path Mismatch

**Diagnosis**: Check actual socket path
```bash
lsof | grep pyproc.sock
```

**Solution**: Ensure paths match
```go
// Go side
SocketPath: "/tmp/pyproc.sock"

// Python side (via env var)
export PYPROC_SOCKET_PATH=/tmp/pyproc.sock
```

#### 3. SELinux/AppArmor Blocking

**Diagnosis**: Check audit logs
```bash
# SELinux
sudo ausearch -m avc -ts recent | grep pyproc

# AppArmor
sudo dmesg | grep DENIED | grep python
```

**Solution**: Add SELinux/AppArmor rules or use permissive mode (dev only)

---

## Type Errors

### Symptom

```
json: cannot unmarshal string into Go struct field .result of type float64
```

### Cause

Python returns wrong type for field.

### Solution

**Check Python return types**:
```python
@expose
def predict(req):
    # ❌ Bad: Returns string
    return {"result": "84.0"}

    # ✅ Good: Returns float
    return {"result": 84.0}
```

**Use type hints in Python**:
```python
from typing import TypedDict

class PredictRequest(TypedDict):
    value: float

class PredictResponse(TypedDict):
    result: float
    model: str

@expose
def predict(req: PredictRequest) -> PredictResponse:
    return {"result": req["value"] * 2.0, "model": "test"}
```

---

## Worker Crashes

### Symptom

Worker process exits unexpectedly.

### Diagnosis

#### Check Worker Logs

```go
logger := pyproc.NewLogger(pyproc.LoggingConfig{
    Level:        "debug",  // Enable debug logs
    Format:       "json",
    TraceEnabled: true,
})

pool, _ := pyproc.NewPool(opts, logger)
```

#### Check Python Stderr

```python
import sys
import traceback

@expose
def predict(req):
    try:
        return process(req)
    except Exception as e:
        # Log full traceback
        traceback.print_exc(file=sys.stderr)
        raise
```

### Common Causes

#### 1. Uncaught Python Exception

**Solution**: Add error handling in Python
```python
@expose
def predict(req):
    try:
        return {"result": req["value"] * 2}
    except KeyError as e:
        # Return error response instead of crashing
        return {"error": f"Missing field: {e}"}
    except Exception as e:
        return {"error": f"Unexpected error: {e}"}
```

#### 2. Out of Memory

**Diagnosis**: Check system logs
```bash
dmesg | grep -i "out of memory"
```

**Solution**: Configure memory limits
```go
WorkerConfig{
    Env: map[string]string{
        "PYTHONMALLOC": "malloc",  // Use system allocator
    },
}
```

#### 3. Segfault in Native Library

**Symptom**: Worker exits with signal 11 (SIGSEGV)

**Solution**: Update native dependencies
```bash
pip install --upgrade numpy torch tensorflow
```

---

## Debugging Techniques

### Enable Verbose Logging

```go
logger := pyproc.NewLogger(pyproc.LoggingConfig{
    Level:        "debug",
    Format:       "json",
    TraceEnabled: true,
})
```

### Test Worker Manually

```bash
# Start worker manually
export PYPROC_SOCKET_PATH=/tmp/test.sock
python3 worker.py

# In another terminal, test with netcat
echo '{"id": 1, "method": "predict", "body": {"value": 42}}' | \
  nc -U /tmp/test.sock
```

### Use Python pdb Debugger

```python
@expose
def predict(req):
    import pdb; pdb.set_trace()  # Breakpoint
    result = req["value"] * 2
    return {"result": result}
```

### Check Health Endpoint

```go
health := pool.Health()
log.Printf("Workers: %d, Active: %d, Total Requests: %d",
    health.Workers, health.ActiveRequests, health.TotalRequests)
```

---

## Getting Help

If you're still stuck:

1. **Search existing issues**: [GitHub Issues](https://github.com/YuminosukeSato/pyproc/issues)
2. **Ask in Discussions**: [GitHub Discussions](https://github.com/YuminosukeSato/pyproc/issues)
3. **File a bug report**: [New Issue](https://github.com/YuminosukeSato/pyproc/issues/new)

### Bug Report Template

When filing an issue, include:

```
**Environment**:
- OS: Linux/macOS/Windows
- Go version: go version
- Python version: python3 --version
- pyproc version: go list -m github.com/YuminosukeSato/pyproc

**Config**:
- Workers: 4
- MaxInFlight: 10
- MaxInFlightPerWorker: 1
- Socket path: /tmp/pyproc.sock

**Code**:
[Minimal reproducible example]

**Error**:
[Full error message and logs]

**Steps to Reproduce**:
1. ...
2. ...
```

---

## Next Steps

- **[Performance Tuning](../guides/performance-tuning.md)**: Optimize for production
- **[Monitoring Guide](monitoring.md)**: Set up observability
- **[Architecture Reference](../reference/architecture.md)**: Understand internals

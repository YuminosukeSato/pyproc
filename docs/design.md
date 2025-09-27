# pyproc Design Document

## Overview

pyproc is a Go library that enables calling Python functions from Go without CGO or microservices. It uses Unix Domain Sockets (UDS) for low-latency inter-process communication and prefork workers to bypass Python's Global Interpreter Lock (GIL).

## Goals

- Run Python on the same host as Go without CGO
- Keep failure isolation with a separate Python process  
- Provide a small, predictable IPC surface over Unix domain sockets
- Scale Python execution by bypassing the GIL through multiple processes
- Offer a simple, function-like API for Go developers

## Architecture

### Core Components

```
┌─────────────────────────────────────────────────────┐
│                   Go Application                      │
├─────────────────────────────────────────────────────┤
│                    pyproc API                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────┐   │
│  │ Worker Pool  │  │ Connection   │  │  Config  │   │
│  │              │  │   Manager    │  │          │   │
│  └──────────────┘  └──────────────┘  └──────────┘   │
├─────────────────────────────────────────────────────┤
│                  Framing Layer                        │
│         [4-byte length] + [JSON payload]              │
└─────────────────────────────────────────────────────┘
                           │
                    Unix Domain Socket
                           │
┌─────────────────────────────────────────────────────┐
│                  Python Workers                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │ Worker 1 │  │ Worker 2 │  │ Worker N │          │
│  └──────────┘  └──────────┘  └──────────┘          │
│                                                      │
│              @expose decorated functions             │
└─────────────────────────────────────────────────────┘
```

### Message Flow

1. Go process spawns long-lived Python workers
2. Requests and responses are framed with a 4-byte big-endian length header
3. Payloads are JSON objects with `id`, `method`, `body` fields
4. Python registers callable functions via `@expose` decorator
5. Each worker serves requests over a single UDS

## Worker Pool Design

### Load Balancing Strategy

The pool implements round-robin load balancing for fair work distribution:

```go
// Select next worker in rotation
idx := p.nextIdx.Add(1) - 1
worker := p.workers[idx % uint64(len(p.workers))]
```

### Backpressure Mechanism

Prevents overwhelming workers using a semaphore-based approach:

```go
// Limit total in-flight requests
semaphore := make(chan struct{}, workers * maxInFlight)
```

### Health Monitoring

- Periodic health checks via dedicated goroutine
- Connection testing to verify worker responsiveness
- Process liveness verification
- Automatic restart on failure (configurable)

### IO-Bound Concurrency

- Python workers now accept multiple socket connections in parallel.
- `WorkerConcurrency` controls the number of threads servicing requests per process.
- Each connection maintains request ordering while enabling overlapping IO waits.
- Go pools require no API change; set the concurrency field in `WorkerConfig`.
- Use the IO-bound worker example to validate throughput before production rollout.

## Protocol Specification

### Framing Protocol

```
[4 bytes: message length (big-endian)]
[N bytes: JSON message]
```

### Request Format

```json
{
  "id": 12345,        // Unique request ID
  "method": "predict", // Function to call
  "body": {           // Function arguments
    "value": 42
  }
}
```

### Response Format

Success:
```json
{
  "id": 12345,
  "ok": true,
  "body": {           // Function return value
    "result": 84
  }
}
```

Error:
```json
{
  "id": 12345,
  "ok": false,
  "error": "Method not found: unknown_method"
}
```

## Reliability Features

- **Process Supervision**: Go supervises worker processes and restarts on exit
- **Health Endpoint**: `health` function always exposed by Python worker
- **Socket Cleanup**: Socket paths cleaned up on restarts
- **Connection Pooling**: Reuse connections to reduce overhead
- **Graceful Shutdown**: Proper cleanup of resources

## Performance Optimizations

### Current Optimizations

1. **Connection Reuse**: Persistent connections per worker
2. **Process Prefork**: Workers start once, handle many requests
3. **Parallel Processing**: Multiple workers for concurrent execution
4. **Buffer Management**: Efficient buffer allocation and reuse

### Benchmarked Performance

- Single worker: ~4,000 req/s
- 4 workers: ~15,000 req/s  
- 8 workers: ~22,000 req/s
- Latency p50: 45μs, p95: 89μs, p99: 125μs

## Extensibility

### Protocol Flexibility

- Protocol decouples framing and payload encoding
- Alternative encodings (MessagePack, Arrow) can be added
- Same API surface maintained across protocols

### Future Protocol Support

1. **MessagePack** (v0.2): Binary serialization for efficiency
2. **gRPC** (v0.4): Industry-standard RPC over UDS
3. **Arrow IPC** (v0.5): Zero-copy for large datasets

## Key Design Decisions

### Why Unix Domain Sockets?

**Pros:**
- Lower latency than TCP (no network stack)
- Better security (filesystem permissions)
- No port management
- Ideal for same-host scenarios

**Cons:**
- Limited to same host
- Platform-specific (Unix-like systems)

### Why Process-based Parallelism?

**Pros:**
- Complete GIL bypass
- True parallel execution
- Process isolation
- Better multi-core utilization

**Cons:**
- Higher memory usage
- Process startup overhead
- IPC complexity

### Why JSON Protocol?

**Pros:**
- Human-readable for debugging
- Native support in Go and Python
- Flexible schema evolution
- Wide ecosystem support

**Cons:**
- Serialization overhead
- Larger message sizes
- Type conversion complexity

## Error Handling Strategy

### Error Categories

1. **Connection Errors**: Socket/network failures
2. **Protocol Errors**: Malformed messages
3. **Worker Errors**: Python exceptions
4. **System Errors**: Process crashes, resource exhaustion

### Recovery Mechanisms

1. **Automatic Retry**: For transient failures
2. **Worker Restart**: On process crashes
3. **Circuit Breaking**: After repeated failures
4. **Graceful Degradation**: Fallback behavior

## Security Considerations

- Unix socket permissions for access control
- Process isolation for failure containment
- No network exposure by default
- Input validation in Python workers

See [security.md](security.md) for detailed security analysis.

## Operations Guide

See [ops.md](ops.md) for deployment and monitoring guidance.


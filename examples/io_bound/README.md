# IO-Bound Tasks Example

This example demonstrates how to use pyproc with multi-threaded Python workers to efficiently handle IO-bound tasks.

## Overview

When dealing with IO-bound operations (network requests, database queries, file I/O), Python's GIL (Global Interpreter Lock) doesn't significantly impact performance since threads release the GIL during I/O operations. This makes threading an effective solution for concurrent IO-bound tasks.

## Features

- **Multi-threaded Workers**: Each Python worker process can handle multiple requests concurrently using threads
- **Configurable Concurrency**: Set the number of threads per worker via `WorkerConcurrency` field
- **Efficient Resource Usage**: Better utilization of worker processes for IO-bound workloads

## Configuration

Key configuration for IO-bound tasks:

```go
WorkerConfig: pyproc.WorkerConfig{
    WorkerConcurrency: 4, // 4 threads per worker process
}
```

## Running the Example

1. Make sure you have Python 3 installed
2. Run the example:

```bash
cd examples/io_bound
go run main.go
```

## What This Example Demonstrates

1. **Concurrent API Fetches**: Multiple simulated HTTP requests running in parallel
2. **Batch Operations**: Processing multiple URLs in a single request
3. **Mixed IO Operations**: Different types of IO operations running concurrently
4. **Performance Comparison**: Shows the speedup achieved with threading

## When to Use Threading

Use multi-threading (`WorkerConcurrency > 1`) when:
- Your workload is IO-bound (network, database, file operations)
- Requests spend significant time waiting for external resources
- You want to maximize throughput for concurrent requests

Keep single-threaded (`WorkerConcurrency = 1`) when:
- Your workload is CPU-bound (heavy computation)
- Operations don't involve waiting for external resources
- You need predictable, sequential processing

## Performance Benefits

For IO-bound tasks, threading can provide significant performance improvements:
- 8 requests × 1 second each = 8 seconds sequential → ~2 seconds concurrent (4x speedup)
- Better resource utilization as threads wait for I/O
- Reduced overall latency for batch operations

## Architecture

```
┌─────────────┐
│   Go App    │
└──────┬──────┘
       │
   ┌───▼───┐
   │ Pool  │
   └───┬───┘
       │
   ┌───▼────────────────┐
   │ Worker Process 1   │
   │ ┌────────────────┐ │
   │ │ Thread Pool    │ │
   │ │ (4 threads)    │ │
   │ └────────────────┘ │
   └────────────────────┘
```

Each worker process maintains a thread pool, allowing multiple requests to be processed concurrently within the same process.
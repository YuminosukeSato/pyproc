# pyproc

*Run Python like a local function from Go — no CGO, no microservices.*

[![Go Reference](https://pkg.go.dev/badge/github.com/YuminosukeSato/pyproc.svg)](https://pkg.go.dev/github.com/YuminosukeSato/pyproc)
[![Go Report Card](https://goreportcard.com/badge/github.com/YuminosukeSato/pyproc)](https://goreportcard.com/report/github.com/YuminosukeSato/pyproc)
[![Go Coverage](https://github.com/YuminosukeSato/pyproc/wiki/coverage.svg)](https://raw.githack.com/wiki/YuminosukeSato/pyproc/coverage.html)
[![codecov](https://codecov.io/gh/YuminosukeSato/pyproc/graph/badge.svg)](https://codecov.io/gh/YuminosukeSato/pyproc)
[![PyPI](https://img.shields.io/pypi/v/pyproc-worker.svg)](https://pypi.org/project/pyproc-worker/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![CI](https://github.com/YuminosukeSato/pyproc/actions/workflows/ci.yml/badge.svg)](https://github.com/YuminosukeSato/pyproc/actions/workflows/ci.yml)

## 🎯 Purpose & Problem Solved

### The Challenge

Go excels at building high-performance web services, but sometimes you need Python:
- **Machine Learning Models**: Your models are trained in PyTorch/TensorFlow
- **Data Science Libraries**: You need pandas, numpy, scikit-learn
- **Legacy Code**: Existing Python code that's too costly to rewrite
- **Python-Only Libraries**: Some libraries only exist in Python ecosystem

Traditional solutions all have major drawbacks:

| Solution | Problems |
|----------|----------|
| **CGO + Python C API** | Complex setup, crashes can take down entire Go service, GIL still limits performance |
| **REST/gRPC Microservice** | Network latency, deployment complexity, service discovery, more infrastructure |
| **Shell exec** | High startup cost (100ms+), no connection pooling, process management nightmare |
| **Embedded Python** | GIL bottleneck, memory leaks, difficult debugging |

### The Solution: pyproc

pyproc lets you call Python functions from Go as if they were local functions, with:
- **Zero network overhead** - Uses Unix Domain Sockets for IPC
- **Process isolation** - Python crashes don't affect your Go service
- **True parallelism** - Multiple Python processes bypass the GIL
- **Simple deployment** - Just your Go binary + Python scripts
- **Connection pooling** - Reuse connections for high throughput

## 🎯 Target Audience & Use Cases

**Perfect for teams who need to:**
- Integrate existing Python ML models (PyTorch, TensorFlow, scikit-learn) into Go services
- Process data with Python libraries (pandas, numpy) from Go applications
- Handle 1-5k RPS with JSON payloads under 100KB
- Deploy on the same host/pod without network complexity
- Migrate gradually from Python microservices to Go while preserving Python logic

**Ideal deployment scenarios:**
- Kubernetes same-pod deployments with shared volume for UDS
- Docker containers with shared socket volumes
- Traditional server deployments on Linux/macOS

## ❌ Non-Goals

pyproc is **NOT** designed for:
- **Cross-host communication** - Use gRPC/REST APIs for distributed systems
- **Windows UDS support** - Windows named pipes are not supported
- **GPU management** - Use dedicated ML serving frameworks (TensorRT, Triton)
- **Large-scale ML serving** - Consider Ray Serve, MLflow, or KServe for enterprise ML
- **Real-time streaming** - Use Apache Kafka or similar for high-throughput streams
- **Database operations** - Use native Go database drivers directly

## 🔄 Alternatives & Comparison

pyproc is a **dedicated IPC engine for integrating Python ML/DS code into Go services on the same host**. It differs from general-purpose plugin systems and embedded runtimes in design philosophy.

| Solution | Pros | Cons | Best For |
|----------|------|------|----------|
| **go-embed-python** | ✅ Python runtime embedded / No Python installation required on host | ❌ Increased binary size / Python operations are DIY | Tools distributed as a single binary |
| **go-plugin (HashiCorp)** | ✅ Multi-language plugin support / Proven in Terraform, Vault | ❌ Requires gRPC proto definitions / Not optimized for Python | Language-agnostic plugin architecture |
| **pyproc** | ✅ Optimized for ML/DS workloads / Built-in worker pool, health checks, auto-restart / Ultra-low latency (~45µs p50) | ❌ Python-only / Same-host only | Integrating Python ML/DS into Go services |

### When to Choose What

**Choose go-embed-python if:**

- You want to distribute a single binary (no Python required on host)
- Increased binary size is acceptable

**Choose go-plugin if:**

- You need multi-language support (Rust, Ruby, etc.) beyond Python
- You're integrating with HashiCorp ecosystem

**Choose pyproc if:**

- You're calling Python ML models (PyTorch, TensorFlow) or DS libraries (pandas, NumPy) from Go
- You need low latency (<100µs) on the same host
- You want built-in worker pool management, health checks, and auto-restart

### Non-Goals (Recap)

pyproc is **NOT** designed for:

- ❌ **General-purpose plugin system** → Use go-plugin
- ❌ **Embedded Python runtime** → Consider go-embed-python
- ❌ **Cross-host communication** → Use gRPC/REST microservices
- ❌ **GPU cluster management** → Use Ray Serve, Triton

## 🔐 Trust Model & Security Considerations

### pyproc is designed for trusted code execution

pyproc is **NOT a sandbox environment**. It operates under the following assumptions:

- ✅ **Target**: Python code developed and managed by your organization (ML models, data processing logic)
- ✅ **Process isolation**: Python crashes do not affect the Go service
- ❌ **No security isolation**: Python workers can access the same filesystem and network as the parent Go process

### Intended Use Cases

**✅ Recommended:**

- Running your own trained PyTorch/TensorFlow models for inference
- Data transformation pipelines using pandas/NumPy
- Integrating scikit-learn models into Go recommendation engines

**❌ Not Recommended:**

- Executing arbitrary user-submitted Python scripts
- Dynamically loading third-party plugins
- Running untrusted code

### Security Details

For detailed threat model, security architecture, and best practices, see [SECURITY.md](docs/security.md).

**Key Guarantees:**

- OS-level access control via Unix Domain Socket filesystem permissions
- Fault tolerance through process isolation
- Configurable resource limits (memory, CPU)

**Limitations:**

- Inter-process communication on the same host only (cross-host is out of scope)
- Does not provide sandbox environment (use gVisor, Firecracker if needed)

## 📋 Compatibility Matrix

| Component | Requirements |
|-----------|-------------|
| **Operating System** | Linux, macOS (Unix Domain Sockets required) |
| **Go Version** | 1.22+ |
| **Python Version** | 3.9+ (3.12 recommended) |
| **Deployment** | Same host/pod only |
| **Container Runtime** | Docker, containerd, any OCI-compatible |
| **Orchestration** | Kubernetes (same-pod), Docker Compose, systemd |
| **Architecture** | amd64, arm64 |

## ✨ Features

- **No CGO Required** - Pure Go implementation using Unix Domain Sockets
- **Bypass Python GIL** - Run multiple Python processes in parallel
- **Function-like API** - Call Python functions as easily as `pool.Call(ctx, "predict", input, &output)`
- **Minimal Overhead** - 45μs p50 latency, 200,000+ req/s with 8 workers
- **Production Ready** - Health checks, graceful shutdown, automatic restarts
- **Easy Deployment** - Single binary + Python scripts, no service mesh needed

## 🚀 Quick Start (5 minutes)

### 1. Install

**Go side:**
```bash
go get github.com/YuminosukeSato/pyproc@latest
```

**Python side:**
```bash
pip install pyproc-worker
```

### 2. Create a Python Worker

```python
# worker.py
from pyproc_worker import expose, run_worker

@expose
def predict(req):
    """Your ML model or Python logic here"""
    return {"result": req["value"] * 2}

if __name__ == "__main__":
    run_worker()
```

### 3. Call from Go

```go
package main

import (
    "context"
    "fmt"
    "log"
    "github.com/YuminosukeSato/pyproc/pkg/pyproc"
)

func main() {
    // Create a pool of Python workers
    pool, err := pyproc.NewPool(pyproc.PoolOptions{
        Config: pyproc.PoolConfig{
            Workers:     4,  // Run 4 Python processes
            MaxInFlight: 10, // Max concurrent requests per worker
        },
        WorkerConfig: pyproc.WorkerConfig{
            SocketPath:   "/tmp/pyproc.sock",
            PythonExec:   "python3",
            WorkerScript: "worker.py",
        },
    }, nil)
    if err != nil {
        log.Fatal(err)
    }
    
    // Start all workers
    ctx := context.Background()
    if err := pool.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer pool.Shutdown(ctx)
    
    // Call Python function (automatically load-balanced)
    input := map[string]interface{}{"value": 42}
    var output map[string]interface{}
    
    if err := pool.Call(ctx, "predict", input, &output); err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Result: %v\n", output["result"]) // Result: 84
}
```

### 4. Run

```bash
go run main.go
```

That's it! You're now calling Python from Go without CGO or microservices.

### Try the demo in this repo

If you cloned this repository, you can run a working end to end example without installing a Python package by using the bundled worker module.

```bash
make demo
```

This starts a Python worker from examples/basic/worker.py and calls it from Go. The example adjusts PYTHONPATH to import the local worker/python/pyproc_worker package.

## 📚 Detailed Usage Guide

### Installation

#### Go Application
```bash
go get github.com/YuminosukeSato/pyproc@latest
```

#### Python Worker
```bash
# Install from PyPI
pip install pyproc-worker

# Or install from source
cd worker/python
pip install -e .
```

### Configuration

#### Basic Configuration
```go
cfg := pyproc.WorkerConfig{
    ID:           "worker-1",
    SocketPath:   "/tmp/pyproc.sock",
    PythonExec:   "python3",           // or path to virtual env
    WorkerScript: "path/to/worker.py",
    StartTimeout: 30 * time.Second,
    Env: map[string]string{
        "PYTHONUNBUFFERED": "1",
        "MODEL_PATH": "/models/latest",
    },
}
```

#### Pool Configuration
```go
poolCfg := pyproc.PoolConfig{
    Workers:        4,                    // Number of Python processes
    MaxInFlight:    10,                   // Max concurrent requests per worker
    HealthInterval: 30 * time.Second,     // Health check frequency
}
```

### Python Worker Development

#### Basic Worker
```python
from pyproc_worker import expose, run_worker

@expose
def add(req):
    """Simple addition function"""
    return {"result": req["a"] + req["b"]}

@expose
def multiply(req):
    """Simple multiplication"""
    return {"result": req["x"] * req["y"]}

if __name__ == "__main__":
    run_worker()
```

#### ML Model Worker
```python
import pickle
from pyproc_worker import expose, run_worker

# Load model once at startup
with open("model.pkl", "rb") as f:
    model = pickle.load(f)

@expose
def predict(req):
    """Run inference on the model"""
    features = req["features"]
    prediction = model.predict([features])[0]
    confidence = model.predict_proba([features])[0].max()
    
    return {
        "prediction": int(prediction),
        "confidence": float(confidence)
    }

@expose
def batch_predict(req):
    """Batch prediction for efficiency"""
    features_list = req["batch"]
    predictions = model.predict(features_list)
    
    return {
        "predictions": predictions.tolist()
    }

if __name__ == "__main__":
    run_worker()
```

#### Data Processing Worker
```python
import pandas as pd
from pyproc_worker import expose, run_worker

@expose
def analyze_csv(req):
    """Analyze CSV data using pandas"""
    df = pd.DataFrame(req["data"])
    
    return {
        "mean": df.mean().to_dict(),
        "std": df.std().to_dict(),
        "correlation": df.corr().to_dict(),
        "summary": df.describe().to_dict()
    }

@expose
def aggregate_timeseries(req):
    """Aggregate time series data"""
    df = pd.DataFrame(req["data"])
    df['timestamp'] = pd.to_datetime(df['timestamp'])
    df.set_index('timestamp', inplace=True)
    
    # Resample to hourly
    hourly = df.resample('H').agg({
        'value': ['mean', 'max', 'min'],
        'count': 'sum'
    })
    
    return hourly.to_dict()

if __name__ == "__main__":
    run_worker()
```

### Go Integration Patterns

#### Simple Request-Response
```go
func callPythonFunction(pool *pyproc.Pool) error {
    input := map[string]interface{}{
        "a": 10,
        "b": 20,
    }
    
    var output map[string]interface{}
    if err := pool.Call(context.Background(), "add", input, &output); err != nil {
        return fmt.Errorf("failed to call Python: %w", err)
    }
    
    fmt.Printf("Result: %v\n", output["result"])
    return nil
}
```

#### With Timeout
```go
func callWithTimeout(pool *pyproc.Pool) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    input := map[string]interface{}{"value": 42}
    var output map[string]interface{}
    
    if err := pool.Call(ctx, "slow_process", input, &output); err != nil {
        if err == context.DeadlineExceeded {
            return fmt.Errorf("Python function timed out")
        }
        return err
    }
    
    return nil
}
```

#### Batch Processing
```go
func processBatch(pool *pyproc.Pool, items []Item) ([]Result, error) {
    input := map[string]interface{}{
        "batch": items,
    }
    
    var output struct {
        Predictions []float64 `json:"predictions"`
    }
    
    if err := pool.Call(context.Background(), "batch_predict", input, &output); err != nil {
        return nil, err
    }
    
    results := make([]Result, len(output.Predictions))
    for i, pred := range output.Predictions {
        results[i] = Result{Value: pred}
    }
    
    return results, nil
}
```

#### Error Handling
```go
func robustCall(pool *pyproc.Pool) {
    for retries := 0; retries < 3; retries++ {
        var output map[string]interface{}
        err := pool.Call(context.Background(), "predict", input, &output)
        
        if err == nil {
            // Success
            return
        }
        
        // Check if it's a Python error
        if strings.Contains(err.Error(), "ValueError") {
            // Invalid input, don't retry
            log.Printf("Invalid input: %v", err)
            return
        }
        
        // Transient error, retry with backoff
        time.Sleep(time.Duration(retries+1) * time.Second)
    }
}
```

### Deployment

#### Docker
```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o myapp .

FROM python:3.11-slim
RUN pip install pyproc-worker numpy pandas scikit-learn
COPY --from=builder /app/myapp /app/myapp
COPY worker.py /app/
WORKDIR /app
CMD ["./myapp"]
```

#### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        image: myapp:latest
        env:
        - name: PYPROC_POOL_WORKERS
          value: "4"
        - name: PYPROC_SOCKET_DIR
          value: "/var/run/pyproc"
        volumeMounts:
        - name: sockets
          mountPath: /var/run/pyproc
      volumes:
      - name: sockets
        emptyDir: {}
```

### Monitoring & Debugging

#### Enable Debug Logging
```go
logger := pyproc.NewLogger(pyproc.LoggingConfig{
    Level: "debug",
    Format: "json",
})

pool, _ := pyproc.NewPool(opts, logger)
```

#### Health Checks
```go
health := pool.Health()
fmt.Printf("Workers: %d healthy, %d total\n", 
    health.HealthyWorkers, health.TotalWorkers)
```

#### Metrics Collection
```go
// Expose Prometheus metrics
http.Handle("/metrics", promhttp.Handler())
http.ListenAndServe(":9090", nil)
```

### Common Issues & Solutions

#### Issue: Worker won't start
```bash
# Check Python dependencies
python3 -c "from pyproc_worker import run_worker"

# Check socket permissions
ls -la /tmp/pyproc.sock

# Enable debug logging
export PYPROC_LOG_LEVEL=debug
```

#### Issue: High latency
```go
// Increase worker count
poolCfg.Workers = runtime.NumCPU() * 2

// Pre-warm connections
pool.Start(ctx)
time.Sleep(1 * time.Second) // Let workers stabilize
```

#### Issue: Memory growth
```python
# Add memory profiling to worker
import tracemalloc
tracemalloc.start()

@expose
def get_memory_usage(req):
    current, peak = tracemalloc.get_traced_memory()
    return {
        "current_mb": current / 1024 / 1024,
        "peak_mb": peak / 1024 / 1024
    }
```

## Use Cases

### Machine Learning Inference

```python
@expose
def predict(req):
    model = load_model()  # Cached after first load
    features = req["features"]
    return {"prediction": model.predict(features)}
```

### Data Processing

```python
@expose
def process_dataframe(req):
    import pandas as pd
    df = pd.DataFrame(req["data"])
    result = df.groupby("category").sum()
    return result.to_dict()
```

### Document Processing

```python
@expose
def extract_pdf_text(req):
    import PyPDF2
    # Process PDF and return text
    return {"text": extracted_text}
```

## Architecture

```
┌─────────────┐           UDS            ┌──────────────┐
│   Go App    │ ◄──────────────────────► │ Python Worker│
│             │    Low-latency IPC        │              │
│  - HTTP API │                           │  - Models    │
│  - Business │                           │  - Libraries │
│  - Logic    │                           │  - Data Proc │
└─────────────┘                           └──────────────┘
     ▲                                           ▲
     │                                           │
     └──────────── Same Host/Pod ────────────────┘
```

## Benchmarks

Run benchmarks locally:

```bash
# Quick benchmark
make bench

# Full benchmark suite with memory profiling
make bench-full
```

Example results on M1 MacBook Pro:

```
BenchmarkPool/workers=1-10         10    235µs/op     4255 req/s
BenchmarkPool/workers=2-10         10    124µs/op     8065 req/s  
BenchmarkPool/workers=4-10         10     68µs/op    14706 req/s
BenchmarkPool/workers=8-10         10     45µs/op    22222 req/s

BenchmarkPoolParallel/workers=2-10   100    18µs/op    55556 req/s
BenchmarkPoolParallel/workers=4-10   100     9µs/op   111111 req/s
BenchmarkPoolParallel/workers=8-10   100     5µs/op   200000 req/s

BenchmarkPoolLatency-10            100   p50: 45µs  p95: 89µs  p99: 125µs
```

The benchmarks show near-linear scaling with worker count, demonstrating the effectiveness of bypassing Python's GIL through process-based parallelism.

## Advanced Features

### Worker Pool

```go
pool, _ := pyproc.NewPool(pyproc.PoolOptions{
    Config: pyproc.PoolConfig{
        Workers:     4,
        MaxInFlight: 10,
    },
    WorkerConfig: pyproc.WorkerConfig{
        SocketPath:   "/tmp/pyproc.sock",
        PythonExec:   "python3",
        WorkerScript: "worker.py",
    },
}, nil)

ctx := context.Background()
pool.Start(ctx)
defer pool.Shutdown(ctx)

var result map[string]interface{}
pool.Call(ctx, "predict", input, &result)
```

### gRPC Mode (coming in v0.4)

```go
pool, _ := pyproc.NewPool(ctx, pyproc.PoolOptions{
    Protocol: pyproc.ProtocolGRPC(),
    // Unix domain socket with gRPC
})
```

### Arrow IPC for Large Data (coming in v0.5)

```go
pool, _ := pyproc.NewPool(ctx, pyproc.PoolOptions{
    Protocol: pyproc.ProtocolArrow(),
    // Zero-copy data transfer
})
```

## 🚀 Operational Standards

### Performance Targets

| Metric | Target | Notes |
|--------|--------|-------|
| **Latency (p50)** | < 100μs | Simple function calls |
| **Latency (p99)** | < 500μs | Including GC and process overhead |
| **Throughput** | 1-5k RPS | Per service instance |
| **Payload Size** | < 100KB | JSON request/response |
| **Worker Count** | 2-8 per CPU core | Based on workload type |

### Health & Monitoring

**Required Metrics:**
- Request latency (p50, p95, p99)
- Request rate and error rate
- Worker health status
- Connection pool utilization
- Python process memory usage

**Health Check Endpoints:**
```go
// Built-in health check
health := pool.Health()
if health.HealthyWorkers < health.TotalWorkers/2 {
    log.Warn("majority of workers unhealthy")
}
```

**Alerting Thresholds:**
- Worker failure rate > 5%
- p99 latency > 1s
- Memory growth > 500MB/hour
- Connection pool exhaustion

### Deployment Best Practices

**Resource Limits:**
```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "200m"
  limits:
    memory: "1Gi" 
    cpu: "1000m"
```

**Restart Policies:**
- Python worker restart on OOM or crash
- Exponential backoff for failed restarts
- Maximum 3 restart attempts per minute
- Circuit breaker after 10 consecutive failures

**Socket Management:**
- Use `/tmp/sockets/` or shared volume in K8s
- Set socket permissions 0660
- Clean up sockets on graceful shutdown
- Monitor socket file descriptors

## Production Checklist

- [ ] Set appropriate worker count based on CPU cores
- [ ] Configure health checks and alerting
- [ ] Set up monitoring (metrics exposed at `:9090/metrics`)
- [ ] Configure restart policies and circuit breakers
- [ ] Set resource limits (memory, CPU)
- [ ] Handle worker failures gracefully
- [ ] Test failover scenarios
- [ ] Configure socket permissions and cleanup
- [ ] Set up log aggregation for Python workers
- [ ] Document runbook for common issues

## Documentation

- [Design Document](docs/design.md)
- [Operations Guide](docs/ops.md)
- [Security Guide](docs/security.md)
- [API Reference](https://pkg.go.dev/github.com/YuminosukeSato/pyproc)

## Contributing

We welcome contributions! Check out our ["help wanted"](https://github.com/YuminosukeSato/pyproc/labels/help%20wanted) issues to get started.
Issues and PRs receive an initial response within 14 days; stable releases keep open bug reports under 6 months.
PR descriptions must include links to pkg.go.dev, Go Report Card, and Coverage.

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.

## References

- [Python GIL Documentation](https://docs.python.org/3/library/threading.html)
- [Unix Domain Sockets](https://man7.org/linux/man-pages/man7/unix.7.html)
- [gRPC Unix Sockets](https://grpc.github.io/grpc/cpp/md_doc_naming.html)
- [Apache Arrow IPC](https://arrow.apache.org/docs/python/ipc.html)

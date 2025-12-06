# pyproc Operations Guide

## Deployment Models

### Single Host Deployment

```yaml
# Standard deployment on a single machine
pool:
  workers: 4           # Number of Python processes
  max_in_flight: 10    # Max concurrent requests per worker
  health_interval: 30s # Health check frequency

python:
  executable: python3
  worker_script: /app/worker.py
  env:
    PYTHONUNBUFFERED: "1"
    
socket:
  dir: /tmp
  prefix: pyproc
  permissions: 0600
```

### Kubernetes Deployment

Place Go app and Python workers in the same pod for UDS communication:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  template:
    spec:
      volumes:
      - name: pyproc-sockets
        emptyDir: {}
      
      containers:
      - name: app
        image: myapp:latest
        volumeMounts:
        - name: pyproc-sockets
          mountPath: /var/run/pyproc
        env:
        - name: PYPROC_SOCKET_DIR
          value: /var/run/pyproc
        - name: PYPROC_POOL_WORKERS
          value: "4"
```

### Docker Compose

```yaml
version: '3.8'
services:
  app:
    build: .
    volumes:
      - sockets:/var/run/pyproc
    environment:
      PYPROC_SOCKET_DIR: /var/run/pyproc
      PYPROC_POOL_WORKERS: 4
      
volumes:
  sockets:
    driver: local
```

## Process Model

- **One Go process** manages one or more Python workers
- **Each worker** listens on a dedicated Unix domain socket
- **Workers are isolated** - crash of one doesn't affect others
- **Automatic restart** on worker failure (configurable)

## Configuration

### Worker Configuration

```go
cfg := pyproc.WorkerConfig{
    ID:           "worker-1",
    SocketPath:   "/tmp/pyproc.sock",
    PythonExec:   "python3",
    WorkerScript: "worker.py",
    StartTimeout: 30 * time.Second,
    Env: map[string]string{
        "PYTHONUNBUFFERED": "1",
        "MODEL_PATH": "/models/latest",
    },
}
```

### Pool Configuration

```go
poolCfg := pyproc.PoolConfig{
    Workers:        4,               // Number of workers
    MaxInFlight:    10,              // Per-worker concurrency
    HealthInterval: 30 * time.Second, // Health check frequency
    Restart: pyproc.RestartConfig{
        MaxAttempts:    5,
        InitialBackoff: 1 * time.Second,
        MaxBackoff:     30 * time.Second,
        Multiplier:     2.0,
    },
}
```

## Health and Monitoring

### Health Checks

Python workers automatically expose a `health` endpoint:

```python
# Automatically registered by pyproc_worker
def health(req):
    return {
        "status": "healthy",
        "pid": os.getpid(),
        "uptime": time.time() - start_time,
        "requests_handled": request_count
    }
```

### Metrics Collection

Export Prometheus metrics from Go:

```go
// Recommended metrics endpoint
http.Handle("/metrics", promhttp.Handler())
http.ListenAndServe(":9090", nil)
```

Key metrics to track:
- `pyproc_worker_requests_total` - Total requests per worker
- `pyproc_worker_request_duration_seconds` - Request latency
- `pyproc_worker_errors_total` - Error count by type
- `pyproc_worker_restarts_total` - Worker restart count
- `pyproc_pool_inflight_requests` - Current in-flight requests

### Logging

Structured logging with trace IDs:

```go
logger := pyproc.NewLogger(pyproc.LoggingConfig{
    Level:        "info",
    Format:       "json",
    TraceEnabled: true,
})
```

Log aggregation recommendations:
- Use structured JSON logging
- Include trace IDs for request correlation
- Ship logs to centralized system (ELK, Datadog, etc.)

## Lifecycle Management

### Startup Sequence

1. Go application starts
2. Worker pool initialized
3. Python workers spawned
4. Socket connections established
5. Health checks begin
6. Ready to serve requests

### Graceful Shutdown

```go
// Handle shutdown signals
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

<-sigCh
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := pool.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

### Worker Restart Strategy

Configure automatic restart with exponential backoff:

```yaml
restart:
  max_attempts: 5
  initial_backoff: 1s
  max_backoff: 30s
  multiplier: 2.0
```

## Resource Management

### Memory Considerations

- **Python processes** can consume significant memory
- **Monitor RSS** (Resident Set Size) per worker
- **Set memory limits** in container deployments
- **Consider worker recycling** after N requests

### CPU Allocation

```yaml
# Kubernetes resource limits
resources:
  requests:
    cpu: "2"
    memory: "4Gi"
  limits:
    cpu: "4"
    memory: "8Gi"
```

### File Descriptors

Ensure sufficient file descriptors:

```bash
# Check current limit
ulimit -n

# Increase limit (add to systemd service or container)
ulimit -n 65536
```

## Performance Tuning

### Worker Count

```python
# Optimal worker count formula
workers = min(
    cpu_cores * 2,  # CPU-bound workloads
    cpu_cores * 4   # I/O-bound workloads
)
```

### Socket Buffer Sizes

```go
// Tune socket buffers for large payloads
conn.SetReadBuffer(1024 * 1024)  // 1MB
conn.SetWriteBuffer(1024 * 1024) // 1MB
```

### Connection Pool Size

```go
// Match MaxInFlight to expected concurrency
MaxInFlight: runtime.NumCPU() * 2
```

## Troubleshooting

### Common Issues

1. **Worker won't start**
   - Check Python path and dependencies
   - Verify socket permissions
   - Review worker script syntax

2. **High latency**
   - Monitor worker CPU usage
   - Check for GIL contention
   - Increase worker count

3. **Connection refused**
   - Verify socket path exists
   - Check filesystem permissions
   - Ensure worker is running

4. **Memory leaks**
   - Monitor Python process memory
   - Implement worker recycling
   - Profile Python code

### Debug Mode

Enable debug logging:

```go
logger := pyproc.NewLogger(pyproc.LoggingConfig{
    Level: "debug",
})
```

### Health Check Failures

Check worker health status:

```bash
# Manual health check
echo '{"id":1,"method":"health","body":{}}' | \
  nc -U /tmp/pyproc.sock
```

## Security Best Practices

- **Run workers with least privilege** user
- **Set restrictive socket permissions** (0600)
- **Validate input** in Python functions
- **Use separate Python virtual environments**
- **Regular dependency updates**
- **Monitor for anomalous behavior**

## Production Checklist

- [ ] Configure appropriate worker count
- [ ] Set up health checks and monitoring
- [ ] Implement graceful shutdown
- [ ] Configure restart policies
- [ ] Set resource limits
- [ ] Enable structured logging
- [ ] Set up metrics collection
- [ ] Test failure scenarios
- [ ] Document worker dependencies
- [ ] Create runbooks for common issues


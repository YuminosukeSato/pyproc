---
title: Installation Guide - pyproc
description: Detailed installation instructions for pyproc
keywords: installation, setup, go, python
---

# Installation

Comprehensive installation guide for pyproc in various environments.

## Prerequisites

| Requirement | Minimum Version | Recommended |
|-------------|----------------|-------------|
| **Go** | 1.22 | Latest stable |
| **Python** | 3.9 | 3.12 |
| **OS** | Linux/macOS | Linux (production) |

---

## Go Installation

### Option 1: Go Modules (Recommended)

Add pyproc to your `go.mod`:

```bash
go get github.com/YuminosukeSato/pyproc@latest
```

### Option 2: Specific Version

```bash
go get github.com/YuminosukeSato/pyproc@v0.3.0
```

### Option 3: Development Version

```bash
go get github.com/YuminosukeSato/pyproc@main
```

### Verify Installation

```bash
go list -m github.com/YuminosukeSato/pyproc
```

---

## Python Installation

### Option 1: pip (Simple)

```bash
pip install pyproc-worker
```

### Option 2: Virtual Environment (Recommended)

```bash
# Create virtual environment
python3 -m venv venv

# Activate (Linux/macOS)
source venv/bin/activate

# Activate (Windows)
venv\Scripts\activate

# Install pyproc-worker
pip install pyproc-worker
```

### Option 3: Poetry

```bash
poetry add pyproc-worker
```

### Option 4: uv (Fast)

```bash
uv pip install pyproc-worker
```

### Verify Installation

```bash
python3 -c "from pyproc_worker import run_worker; print('OK')"
```

---

## Development Environment Setup

### 1. Clone Repository (for examples)

```bash
git clone https://github.com/YuminosukeSato/pyproc.git
cd pyproc
```

### 2. Install Dependencies

**Go side**:
```bash
go mod download
```

**Python side**:
```bash
cd worker/python
pip install -e .
pip install -e ".[dev]"  # Include dev dependencies
```

### 3. Run Examples

```bash
make demo
```

---

## Production Environment Setup

### Docker

Create `Dockerfile`:

```dockerfile
# Multi-stage build
FROM golang:1.22-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/server ./cmd/server

FROM python:3.12-slim
WORKDIR /app

# Install Python dependencies
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# Copy Go binary
COPY --from=go-builder /app/server /app/server
COPY worker.py ./

# Run
CMD ["/app/server"]
```

### Kubernetes

See [Kubernetes Deployment Guide](../deployment/kubernetes.md) for details.

---

## Platform-Specific Notes

### Linux

**Ubuntu/Debian**:
```bash
sudo apt-get update
sudo apt-get install -y python3 python3-pip golang-go
```

**CentOS/RHEL**:
```bash
sudo yum install -y python3 python3-pip golang
```

### macOS

**Using Homebrew**:
```bash
brew install go python@3.12
```

### Windows

⚠️ **Not Supported**: pyproc requires Unix Domain Sockets, which are not available on Windows.

**Alternatives**:
- Use Windows Subsystem for Linux (WSL)
- Use Docker Desktop with Linux containers

---

## Configuration

### Environment Variables

```bash
# Python executable (if not in PATH)
export PYPROC_PYTHON_EXEC=/path/to/python3

# Socket path (default: /tmp/pyproc.sock)
export PYPROC_SOCKET_PATH=/var/run/pyproc.sock

# Log level
export PYPROC_LOG_LEVEL=debug
```

### Virtual Environment

If using a Python virtual environment, configure Go to use it:

```go
WorkerConfig{
    PythonExec: "/path/to/venv/bin/python",  // Use venv Python
}
```

---

## Troubleshooting

### Python Not Found

**Symptom**: `exec: "python3": executable file not found in $PATH`

**Solution**:
```bash
# Find Python location
which python3

# Use absolute path in config
WorkerConfig{
    PythonExec: "/usr/bin/python3",
}
```

### Permission Denied on Socket

**Symptom**: `permission denied` when creating socket

**Solution**:
```bash
# Use user-writable directory
mkdir -p ~/tmp
chmod 755 ~/tmp

# Configure socket path
SocketPath: os.Getenv("HOME") + "/tmp/pyproc.sock"
```

### Module Not Found

**Symptom**: `ModuleNotFoundError: No module named 'pyproc_worker'`

**Solution**:
```bash
# Verify installation
pip list | grep pyproc-worker

# Reinstall if missing
pip install --upgrade pyproc-worker
```

---

## Next Steps

Now that pyproc is installed:

1. **[Quick Start](quick-start.md)**: Get pyproc running in 5 minutes
2. **[First Integration](first-integration.md)**: Build a real integration
3. **[Type-Safe API](../guides/type-safe-api.md)**: Learn type-safe calling

---

## See Also

- [PyPI Package](https://pypi.org/project/pyproc-worker/)
- [GitHub Repository](https://github.com/YuminosukeSato/pyproc)
- [Troubleshooting Guide](../deployment/troubleshooting.md)

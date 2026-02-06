---
title: Docker Deployment - pyproc
description: Deploy pyproc with Docker
keywords: docker, container, deployment
---

# Docker Deployment

pyproc provides two Docker images for different deployment patterns.

## Images

| Image | Purpose | Use Case |
|-------|---------|----------|
| `Dockerfile` | Go app + Python runtime | Single-container deployment |
| `Dockerfile.worker` | Python worker only | Sidecar deployment in Kubernetes |

## Single-Container Image

The main Dockerfile builds a multi-stage image containing both the Go binary and the Python runtime with pyproc-worker installed.

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/pyproc

FROM python:3.12-slim
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv
RUN uv pip install --system pyproc-worker
RUN groupadd -g 1000 pyproc && \
    useradd -u 1000 -g pyproc -m -s /bin/sh pyproc
WORKDIR /app
COPY --from=builder /app/server /app/server
USER 1000:1000
EXPOSE 8080
ENTRYPOINT ["/app/server"]
```

Build and run:

```bash
docker build -t myapp .
docker run -p 8080:8080 myapp
```

## Sidecar Worker Image

The worker-only image is used in the sidecar pattern where Go and Python run as separate containers in the same Pod. They communicate via a shared UDS volume.

```dockerfile
FROM python:3.12-slim
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv
RUN uv pip install --system pyproc-worker
RUN groupadd -g 1000 pyproc && \
    useradd -u 1000 -g pyproc -m -s /bin/sh pyproc
USER 1000:1000
ENTRYPOINT ["pyproc-worker"]
```

Build:

```bash
docker build -f Dockerfile.worker -t myapp-worker .
```

## Adding Custom Worker Code

Copy your Python worker files into the image:

```dockerfile
FROM python:3.12-slim
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv
RUN uv pip install --system pyproc-worker
WORKDIR /app
COPY worker.py /app/worker.py
COPY requirements.txt /app/requirements.txt
RUN uv pip install --system -r /app/requirements.txt
RUN groupadd -g 1000 pyproc && \
    useradd -u 1000 -g pyproc -m -s /bin/sh pyproc
USER 1000:1000
ENTRYPOINT ["pyproc-worker", "/app/worker.py"]
```

## Security

Both images follow these security practices:

- Run as non-root user (UID 1000)
- Use `uv` instead of `pip` for package management
- Minimize image layers and installed packages
- No secrets baked into the image

For Kubernetes security context settings, see [Kubernetes Deployment](kubernetes.md).

## Related Documentation

- [Kubernetes Deployment](kubernetes.md): Pod configuration and manifests
- [Monitoring](monitoring.md): Metrics and observability

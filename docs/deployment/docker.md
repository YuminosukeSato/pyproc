---
title: Docker Deployment - pyproc
description: Deploy pyproc with Docker
keywords: docker, container, deployment
---

# Docker Deployment

!!! info "Coming Soon"
    Detailed Docker deployment guide is under development.

## Quick Start

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server ./cmd/server

FROM python:3.12-slim
RUN pip install pyproc-worker
COPY --from=builder /app/server /app/
COPY worker.py /app/
CMD ["/app/server"]
```

Build and run:
```bash
docker build -t myapp .
docker run -p 8080:8080 myapp
```

See README for more examples.

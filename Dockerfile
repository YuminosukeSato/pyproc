# Stage 1: Build Go binary
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/pyproc

# Stage 2: Runtime with Python + Go binary
FROM python:3.12-slim

# Install uv for Python package management
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# Install pyproc-worker
RUN uv pip install --system pyproc-worker

# Create non-root user
RUN groupadd -g 1000 pyproc && \
    useradd -u 1000 -g pyproc -m -s /bin/sh pyproc

WORKDIR /app

# Copy Go binary from builder stage
COPY --from=builder /app/server /app/server

# Copy Python worker files (users should add their worker.py here)
# COPY worker.py /app/worker.py

# Create directories for UDS and tmp with correct ownership
RUN mkdir -p /var/run/pyproc /tmp/pyproc && \
    chown -R pyproc:pyproc /app /var/run/pyproc /tmp/pyproc

USER 1000:1000

EXPOSE 8080

ENTRYPOINT ["/app/server"]

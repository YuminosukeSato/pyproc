# Helm Sidecar Example

A Helm values override for sidecar (ExternalMode) pyproc deployment.

## Overview

This example configures a two-container deployment where the Go application and Python worker run as separate containers in the same Pod. They communicate via a shared UDS volume mounted at `/var/run/pyproc`.

This pattern is useful when:

- Go and Python have different scaling or resource requirements
- Python worker images are built and versioned independently
- You want to update the worker without rebuilding the Go image

## Usage

```bash
helm install myapp ./charts/pyproc -f examples/helm-sidecar/values.yaml
```

## What This Configures

- 2 replicas with sidecar enabled
- Separate resource limits for Go app and Python worker containers
- Shared emptyDir volume for UDS at `/var/run/pyproc`
- Non-root security context (UID 1000) applied to both containers
- Read-only root filesystem for both containers

## Container Layout

```
Pod
├── app (Go)          → myapp-go:latest
│   └── /var/run/pyproc (emptyDir, shared)
└── worker (Python)   → myapp-worker:latest
    └── /var/run/pyproc (emptyDir, shared)
```

## Customization

- `sidecar.image`: Worker container image
- `sidecar.args`: Worker startup arguments (socket path, worker script)
- `sidecar.resources`: Worker-specific CPU and memory limits

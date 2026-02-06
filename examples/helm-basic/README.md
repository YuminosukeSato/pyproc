# Helm Basic Example

A basic Helm values override for single-container pyproc deployment.

## Overview

This example configures a standard deployment where the Go application manages Python workers internally within the same container. UDS communication happens inside the container via `/var/run/pyproc`.

## Usage

```bash
helm install myapp ./charts/pyproc -f examples/helm-basic/values.yaml
```

## What This Configures

- 2 replicas
- Resource requests and limits for the application container
- Liveness and readiness probes on `/healthz` and `/readyz`
- Non-root security context (UID 1000)
- Read-only root filesystem
- All Linux capabilities dropped

## Customization

Edit `values.yaml` to adjust:

- `replicaCount`: Number of Pod replicas
- `resources`: CPU and memory requests/limits
- `image.repository` and `image.tag`: Container image reference
- Probe timings: `initialDelaySeconds`, `periodSeconds`, `failureThreshold`

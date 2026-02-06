---
title: Kubernetes Resource and Security Recommendations - pyproc
description: Resource sizing, Pod Security Standards, and securityContext guidelines
keywords: kubernetes, resources, security, pod security standards
---

# Kubernetes Resource and Security Recommendations

This guide provides resource sizing guidelines and Pod Security Standards configuration for pyproc workloads.

## Resource Sizing by Workload Type

### Low-Latency Inference (p99 < 1ms)

| Component | CPU request | CPU limit | Memory request | Memory limit |
|-----------|------------|-----------|---------------|-------------|
| Go application | 500m | 1000m | 256Mi | 512Mi |
| Python worker | 250m | 500m | 256Mi | 512Mi |

Characteristics:

- Small payload sizes (< 1KB JSON)
- High request rate (1k-5k RPS)
- CPU-bound Python functions

### Batch Processing

| Component | CPU request | CPU limit | Memory request | Memory limit |
|-----------|------------|-----------|---------------|-------------|
| Go application | 250m | 500m | 128Mi | 256Mi |
| Python worker | 500m | 2000m | 512Mi | 2Gi |

Characteristics:

- Large payload sizes (10KB-100KB)
- Lower request rate (10-100 RPS)
- Memory-intensive Python computation

### General Purpose

| Component | CPU request | CPU limit | Memory request | Memory limit |
|-----------|------------|-----------|---------------|-------------|
| Go application | 250m | 1000m | 256Mi | 512Mi |
| Python worker | 100m | 500m | 128Mi | 256Mi |

This is the default starting point. Adjust based on observed metrics.

## Sizing Methodology

1. Start with General Purpose values
2. Run the application under expected load
3. Monitor actual CPU and memory usage via Prometheus or `kubectl top`
4. Set requests to the observed p95 usage
5. Set limits to 2x the request value (or observed peak)

Key metrics to watch:

- `container_cpu_usage_seconds_total`: Actual CPU consumption
- `container_memory_working_set_bytes`: Actual memory usage
- `kube_pod_container_status_restarts_total`: OOMKill restarts

## Pod Security Standards

Kubernetes defines three Pod Security Standards levels. pyproc supports both Baseline and Restricted.

### Baseline (minimum recommended)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pyproc-app
spec:
  securityContext:
    runAsNonRoot: true
  containers:
    - name: app
      securityContext:
        allowPrivilegeEscalation: false
```

### Restricted (recommended for production)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pyproc-app
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: app
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        runAsNonRoot: true
        capabilities:
          drop:
            - ALL
```

### Comparison

| Field | Baseline | Restricted |
|-------|----------|------------|
| `runAsNonRoot` | Required | Required |
| `allowPrivilegeEscalation` | false | false |
| `readOnlyRootFilesystem` | Optional | true |
| `capabilities.drop` | Optional | ALL |
| `seccompProfile` | Optional | RuntimeDefault |
| `runAsUser` / `runAsGroup` | Optional | Explicit UID/GID |

## Volume Sizing

### UDS Socket Volume

```yaml
volumes:
  - name: socket-dir
    emptyDir:
      medium: Memory
      sizeLimit: 16Mi
```

- `medium: Memory` mounts as tmpfs for lower latency
- 16Mi is sufficient for socket files
- Using Memory medium counts against the container memory limit

### Tmp Volume

```yaml
volumes:
  - name: tmp
    emptyDir:
      sizeLimit: 64Mi
```

Required when `readOnlyRootFilesystem: true` is set. Python may need `/tmp` for temporary files.

## Namespace-Level Enforcement

Apply Pod Security Standards at the namespace level:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: pyproc-apps
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

## Related Documentation

- [Kubernetes Deployment](kubernetes.md): Pod configuration and manifests
- [Docker Deployment](docker.md): Container image building

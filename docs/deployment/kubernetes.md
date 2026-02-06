---
title: Kubernetes Deployment - pyproc
description: Deploy pyproc on Kubernetes
keywords: kubernetes, k8s, deployment, uds, pod
---

# Kubernetes Deployment

pyproc uses Unix Domain Socket (UDS) for communication between Go processes and Python workers.
Containers within the same Pod share an emptyDir volume where the UDS is placed.

## Prerequisites

- pyproc requires deployment within the same Pod
- UDS is not network communication, so it cannot be used for inter-Pod RPC
- The Go container and Python workers must reference the same volume

## Basic Configuration: Single-Container

A configuration where Python worker processes are launched from within the Go application.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pyproc-app
  labels:
    app: pyproc-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: pyproc-app
  template:
    metadata:
      labels:
        app: pyproc-app
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 1000
      containers:
        - name: app
          image: myapp:latest
          resources:
            requests:
              cpu: "250m"
              memory: "256Mi"
            limits:
              cpu: "1000m"
              memory: "512Mi"
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
          volumeMounts:
            - name: socket-dir
              mountPath: /var/run/pyproc
            - name: tmp
              mountPath: /tmp
          env:
            - name: PYPROC_SOCKET_DIR
              value: /var/run/pyproc
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 2
      volumes:
        - name: socket-dir
          emptyDir:
            medium: Memory
            sizeLimit: 16Mi
        - name: tmp
          emptyDir:
            sizeLimit: 64Mi
```

## Sidecar Configuration

A configuration where the Go container and Python worker are deployed as separate containers.
They share the `socket-dir` volume and communicate via UDS.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pyproc-sidecar
  labels:
    app: pyproc-sidecar
spec:
  replicas: 2
  selector:
    matchLabels:
      app: pyproc-sidecar
  template:
    metadata:
      labels:
        app: pyproc-sidecar
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 1000
      containers:
        - name: app
          image: myapp-go:latest
          resources:
            requests:
              cpu: "250m"
              memory: "256Mi"
            limits:
              cpu: "1000m"
              memory: "512Mi"
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
          volumeMounts:
            - name: socket-dir
              mountPath: /var/run/pyproc
          env:
            - name: PYPROC_SOCKET_DIR
              value: /var/run/pyproc
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 15
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
        - name: worker
          image: myapp-python:latest
          command: ["pyproc-worker"]
          args: ["/app/worker.py", "--socket-path", "/var/run/pyproc/worker.sock"]
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "500m"
              memory: "256Mi"
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
          volumeMounts:
            - name: socket-dir
              mountPath: /var/run/pyproc
            - name: tmp
              mountPath: /tmp
      volumes:
        - name: socket-dir
          emptyDir:
            medium: Memory
            sizeLimit: 16Mi
        - name: tmp
          emptyDir:
            sizeLimit: 64Mi
```

## Security Context

Apply the following security settings in production environments.

### Pod Level

- `runAsNonRoot: true`: Prohibit execution as root
- `runAsUser` / `runAsGroup`: Run as a non-privileged user
- `fsGroup`: Unify file group ownership for volumes

### Container Level

- `allowPrivilegeEscalation: false`: Prevent privilege escalation
- `readOnlyRootFilesystem: true`: Make root filesystem read-only
- `capabilities.drop: [ALL]`: Drop all Linux capabilities

### UDS Volume

- `medium: Memory`: Mount as tmpfs to avoid disk I/O
- `sizeLimit: 16Mi`: Minimum size required for socket files

UDS file permissions (0660) are automatically set by pyproc.
Setting `fsGroup` allows both the Go container and Python container to access the socket.

## Health Checks

### Liveness Probe

Checks whether the application process is running normally.
On failure, the Pod is restarted.

- `initialDelaySeconds`: Time to wait for startup completion
- `failureThreshold`: Number of consecutive failures before restart

### Readiness Probe

Checks whether the application is ready to accept requests.
On failure, the Pod is removed from Service endpoints.

- It is recommended to include Python worker pool initialization completion as a readiness condition

## Resource Planning Guidelines

| Component | CPU request | CPU limit | Memory request | Memory limit |
|-----------|------------|-----------|---------------|-------------|
| Go application | 250m | 1000m | 256Mi | 512Mi |
| Python worker (sidecar) | 100m | 500m | 128Mi | 256Mi |

Actual values depend on the workload. Adjust based on benchmark results.

## Related Documentation

- [Support Matrix](../compatibility/support-matrix.md): Supported OS / runtime list
- [Docker Deployment](docker.md): Container image building
- [Monitoring](monitoring.md): Metrics and observability

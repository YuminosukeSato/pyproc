---
title: Kubernetes Troubleshooting - pyproc
description: Diagnose and fix common pyproc issues on Kubernetes
keywords: kubernetes, troubleshooting, debug, crashloopbackoff, probes
---

# Kubernetes Troubleshooting

This guide covers common issues when running pyproc on Kubernetes and how to diagnose them.

## CrashLoopBackOff

### Symptoms

```
NAME                          READY   STATUS             RESTARTS   AGE
pyproc-app-5d8f9b7c4-x2j9k   0/1     CrashLoopBackOff   5          3m
```

### Diagnosis

Check container logs:

```bash
kubectl logs <pod-name> -c app --previous
kubectl logs <pod-name> -c worker --previous
```

Check events:

```bash
kubectl describe pod <pod-name>
```

### Common Causes

1. Missing socket directory: The container cannot create the UDS socket.

```bash
# Check if the volume mount exists
kubectl exec <pod-name> -c app -- ls -la /var/run/pyproc
```

Fix: Verify the emptyDir volume is mounted at the socket path.

2. Permission denied on socket: The container user cannot write to the socket directory.

```bash
kubectl exec <pod-name> -c app -- id
kubectl exec <pod-name> -c app -- ls -la /var/run/
```

Fix: Set `fsGroup` in the Pod securityContext to match the container user group.

3. OOMKilled: The container exceeds its memory limit.

```bash
kubectl describe pod <pod-name> | grep -A 5 "Last State"
```

Fix: Increase `resources.limits.memory` for the affected container.

4. Python worker binary not found: `pyproc-worker` is not installed or not in PATH.

```bash
kubectl exec <pod-name> -c worker -- which pyproc-worker
```

Fix: Verify the Docker image installs pyproc-worker correctly.

## Probe Failures

### Liveness Probe Failure

Symptoms: Pod keeps restarting. Events show `Liveness probe failed`.

```bash
kubectl describe pod <pod-name> | grep -A 3 "Liveness"
```

Diagnosis:

```bash
# Test the health endpoint from inside the container
kubectl exec <pod-name> -c app -- wget -qO- http://localhost:8080/healthz
```

Common fixes:

- Increase `initialDelaySeconds` if the app needs more startup time
- Increase `timeoutSeconds` if the health check is slow under load
- Increase `failureThreshold` to tolerate transient failures

### Readiness Probe Failure

Symptoms: Pod is Running but not Ready. Traffic is not routed to the Pod.

```bash
kubectl get pods -o wide
kubectl describe pod <pod-name> | grep -A 3 "Readiness"
```

Diagnosis:

```bash
kubectl exec <pod-name> -c app -- wget -qO- http://localhost:8080/readyz
```

Common causes:

- Python worker pool is still initializing
- Worker processes failed to connect to UDS
- Insufficient resources causing slow startup

## UDS Permission Issues

### Socket Not Found

Symptoms: Go application logs show connection refused or socket not found errors.

```bash
kubectl exec <pod-name> -c app -- ls -la /var/run/pyproc/
```

Diagnosis checklist:

- Both containers mount the same volume at the same path
- `PYPROC_SOCKET_DIR` environment variable matches the mount path
- In sidecar mode, the worker container creates the socket before the Go app connects

### Permission Denied on Socket

Symptoms: `permission denied` errors when connecting to the UDS.

```bash
# Check socket permissions
kubectl exec <pod-name> -c app -- ls -la /var/run/pyproc/worker.sock

# Check user identity in each container
kubectl exec <pod-name> -c app -- id
kubectl exec <pod-name> -c worker -- id
```

Fix: Both containers must run as the same user/group, or `fsGroup` must be set in the Pod securityContext:

```yaml
spec:
  securityContext:
    fsGroup: 1000
```

## Debug Commands

### Pod Status and Events

```bash
# Overview
kubectl get pods -l app=pyproc-app -o wide

# Detailed status
kubectl describe pod <pod-name>

# Events for the namespace
kubectl get events --sort-by=.metadata.creationTimestamp
```

### Container Logs

```bash
# Current logs
kubectl logs <pod-name> -c app
kubectl logs <pod-name> -c worker

# Previous container logs (after restart)
kubectl logs <pod-name> -c app --previous

# Follow logs
kubectl logs <pod-name> -c app -f

# Last 100 lines
kubectl logs <pod-name> -c app --tail=100
```

### Interactive Debugging

```bash
# Shell into the container
kubectl exec -it <pod-name> -c app -- /bin/sh

# Check processes
kubectl exec <pod-name> -c app -- ps aux

# Check network (UDS)
kubectl exec <pod-name> -c app -- ls -la /var/run/pyproc/

# Check resource usage
kubectl top pod <pod-name> --containers
```

### Ephemeral Debug Container

When the container has a read-only filesystem or minimal tooling:

```bash
kubectl debug -it <pod-name> --image=busybox --target=app
```

## Resource Issues

### Throttling (CPU)

Symptoms: High latency, slow responses, but no crashes.

```bash
kubectl top pod <pod-name> --containers
```

Check if CPU usage is near the limit. If throttled, increase `resources.limits.cpu`.

### OOMKilled (Memory)

Symptoms: Container restarts with reason `OOMKilled`.

```bash
kubectl describe pod <pod-name> | grep -A 5 "Last State"
```

Fix:

- Increase `resources.limits.memory`
- Investigate if the Python worker has a memory leak
- Check payload sizes (large JSON payloads consume memory)

## Related Documentation

- [Kubernetes Deployment](kubernetes.md): Pod configuration and manifests
- [Resource and Security Recommendations](kubernetes-resources.md): Sizing guidelines
- [Docker Deployment](docker.md): Container image building
- [Monitoring](monitoring.md): Metrics and observability

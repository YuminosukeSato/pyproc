---
title: Kubernetes Deployment - pyproc
description: Deploy pyproc on Kubernetes
keywords: kubernetes, k8s, deployment
---

# Kubernetes Deployment

!!! info "Coming Soon"
    Detailed Kubernetes deployment guide is under development.

## Key Requirement

pyproc requires **same-pod deployment** with shared volume for Unix Domain Sockets:

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: myapp:latest
    volumeMounts:
    - name: socket-dir
      mountPath: /tmp
  volumes:
  - name: socket-dir
    emptyDir: {}
```

See README for complete example.

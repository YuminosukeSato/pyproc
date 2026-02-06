---
title: Secrets Management - pyproc
description: Mount cloud secrets into pyproc pods using the Secrets Store CSI Driver
keywords: secrets, csi driver, secret manager, key vault, secrets manager
---

# Secrets Management

This guide covers mounting cloud secrets into pyproc pods using the Secrets Store CSI Driver. This approach avoids storing sensitive values in Kubernetes Secrets and provides automatic rotation support.

## Overview

The Secrets Store CSI Driver mounts secrets from external secret managers (GCP Secret Manager, AWS Secrets Manager, Azure Key Vault) as files into pods. pyproc can then read these secrets from the filesystem at runtime.

> [!NOTE]
> The example directories referenced below (e.g. `examples/helm-secrets-store-csi/`) are introduced by companion PRs in the v0.6.0 release. They must be merged before this documentation.

## Prerequisites

- Kubernetes cluster with the [Secrets Store CSI Driver](https://secrets-store-csi-driver.sigs.k8s.io/) installed
- Cloud-specific provider installed:
    - GCP: `gcp-provider`
    - AWS: `aws-provider`
    - Azure: `azure-provider`
- Workload Identity configured for the service account (see [Cloud Authentication](cloud-authentication.md))

## Architecture

```
Pod
+-- app container
|   +-- /var/run/pyproc  (UDS socket)
|   +-- /mnt/secrets     (CSI volume, read-only)
|       +-- pyproc-secret
+-- worker container (sidecar)
    +-- /var/run/pyproc  (UDS socket)
```

## Helm Configuration

```bash
helm install myapp ./charts/pyproc \
  -f examples/helm-secrets-store-csi/values.yaml
```

See [examples/helm-secrets-store-csi/](https://github.com/YuminosukeSato/pyproc/tree/main/examples/helm-secrets-store-csi) for the full values file.

## Kustomize Configuration

```bash
kustomize build deploy/kustomize/overlays/secrets-store-csi
```

See [deploy/kustomize/overlays/secrets-store-csi/](https://github.com/YuminosukeSato/pyproc/tree/main/deploy/kustomize/overlays/secrets-store-csi) for the overlay.

## Cloud-Specific SecretProviderClass Examples

### GCP Secret Manager

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: pyproc-secrets
spec:
  provider: gcp
  parameters:
    secrets: |
      - resourceName: "projects/PROJECT_ID/secrets/PYPROC_SECRET/versions/latest"
        path: "pyproc-secret"
```

### AWS Secrets Manager

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: pyproc-secrets
spec:
  provider: aws
  parameters:
    objects: |
      - objectName: "pyproc/secret"
        objectType: "secretsmanager"
```

### Azure Key Vault

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: pyproc-secrets
spec:
  provider: azure
  parameters:
    usePodIdentity: "false"
    useVMManagedIdentity: "true"
    keyvaultName: "KEYVAULT_NAME"
    objects: |
      array:
        - |
          objectName: pyproc-secret
          objectType: secret
    tenantId: "TENANT_ID"
```

## Reading Secrets in Application Code

Once mounted, secrets are available as files:

```go
secret, err := os.ReadFile("/mnt/secrets/pyproc-secret")
if err != nil {
    return fmt.Errorf("failed to read secret: %w", err)
}
```

## Related Documents

- [Cloud Authentication](cloud-authentication.md)
- [Kubernetes Deployment](kubernetes.md)
- [Security Reference](../reference/security.md)

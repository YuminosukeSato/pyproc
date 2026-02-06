# Helm Secrets Store CSI Driver Example

A Helm values override for mounting cloud secrets via the Secrets Store CSI Driver with pyproc.

## Overview

This example configures a pyproc deployment to mount secrets from cloud secret managers (GCP Secret Manager, AWS Secrets Manager, Azure Key Vault) as files via the Secrets Store CSI Driver.

## Prerequisites

- Kubernetes cluster with the Secrets Store CSI Driver installed
- Cloud-specific provider installed (GCP, AWS, or Azure)
- Workload Identity or equivalent configured for the service account

## Usage

```bash
helm install myapp ./charts/pyproc -f examples/helm-secrets-store-csi/values.yaml
```

## What This Configures

- Extra volume mount for secrets at `/mnt/secrets`
- Secrets Store CSI volume referencing a SecretProviderClass

## Kustomize Overlays

The `deploy/kustomize/overlays/secrets-store-csi/` directory contains:

- `secret-provider-class-gcp.yaml`: GCP Secret Manager SecretProviderClass
- `secret-provider-class-aws.yaml`: AWS Secrets Manager SecretProviderClass
- `secret-provider-class-azure.yaml`: Azure Key Vault SecretProviderClass
- `volume-patch.yaml`: Deployment patch to mount the CSI volume

## Customization

- Modify the SecretProviderClass for your cloud provider
- Update secret names and versions in the provider-specific files
- Adjust mount path in `volume-patch.yaml` as needed

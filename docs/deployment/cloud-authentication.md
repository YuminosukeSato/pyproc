---
title: Cloud Authentication - pyproc
description: Configure pyproc with cloud-native workload identity for GKE, EKS, and AKS
keywords: authentication, workload identity, gke, eks, aks, irsa, entra
---

# Cloud Authentication

This guide covers configuring pyproc to authenticate with cloud services using workload identity mechanisms. These approaches eliminate static credentials and follow cloud-native security best practices.

## Overview

When running pyproc on managed Kubernetes, you can leverage cloud-native workload identity to grant your pods access to cloud APIs (e.g. object storage, databases, secret managers) without managing service account keys or access keys.

> [!NOTE]
> The example directories referenced below (e.g. `examples/helm-gke-workload-identity/`) are introduced by companion PRs in the v0.6.0 release. They must be merged before this documentation.

| Cloud | Mechanism | Annotation |
|-------|-----------|------------|
| GCP (GKE) | Workload Identity Federation | `iam.gke.io/gcp-service-account` |
| AWS (EKS) | IAM Roles for Service Accounts (IRSA) | `eks.amazonaws.com/role-arn` |
| Azure (AKS) | Entra Workload Identity | `azure.workload.identity/client-id` |

## GKE Workload Identity Federation

GKE Workload Identity Federation links a Kubernetes ServiceAccount to a Google Cloud service account, allowing pods to authenticate as that GCP identity.

### Prerequisites

- GKE cluster with Workload Identity enabled
- Google Cloud service account with required permissions
- IAM binding between the Kubernetes SA and the GCP SA

### Helm Configuration

```bash
helm install myapp ./charts/pyproc \
  -f examples/helm-gke-workload-identity/values.yaml
```

See [examples/helm-gke-workload-identity/](https://github.com/YuminosukeSato/pyproc/tree/main/examples/helm-gke-workload-identity) for the full values file.

### Kustomize Configuration

```bash
kustomize build deploy/kustomize/overlays/gke-workload-identity
```

See [deploy/kustomize/overlays/gke-workload-identity/](https://github.com/YuminosukeSato/pyproc/tree/main/deploy/kustomize/overlays/gke-workload-identity) for the overlay.

## EKS IAM Roles for Service Accounts (IRSA)

EKS IRSA associates a Kubernetes ServiceAccount with an IAM role via an OIDC provider, granting pods AWS API access.

### Prerequisites

- EKS cluster with OIDC provider configured
- IAM role with a trust policy referencing the EKS OIDC provider

### Helm Configuration

```bash
helm install myapp ./charts/pyproc \
  -f examples/helm-eks-irsa/values.yaml
```

See [examples/helm-eks-irsa/](https://github.com/YuminosukeSato/pyproc/tree/main/examples/helm-eks-irsa) for the full values file.

### Kustomize Configuration

```bash
kustomize build deploy/kustomize/overlays/eks-irsa
```

See [deploy/kustomize/overlays/eks-irsa/](https://github.com/YuminosukeSato/pyproc/tree/main/deploy/kustomize/overlays/eks-irsa) for the overlay.

## AKS Entra Workload Identity

AKS Entra Workload Identity uses Azure AD federated identity credentials to link a Kubernetes ServiceAccount to an Azure managed identity. It requires both a ServiceAccount annotation and a pod label.

### Prerequisites

- AKS cluster with Workload Identity enabled
- Azure managed identity with a federated identity credential

### Helm Configuration

```bash
helm install myapp ./charts/pyproc \
  -f examples/helm-aks-workload-identity/values.yaml
```

See [examples/helm-aks-workload-identity/](https://github.com/YuminosukeSato/pyproc/tree/main/examples/helm-aks-workload-identity) for the full values file.

### Kustomize Configuration

```bash
kustomize build deploy/kustomize/overlays/aks-workload-identity
```

See [deploy/kustomize/overlays/aks-workload-identity/](https://github.com/YuminosukeSato/pyproc/tree/main/deploy/kustomize/overlays/aks-workload-identity) for the overlay.

## Related Documents

- [Kubernetes Deployment](kubernetes.md)
- [Secrets Management](secrets-management.md)
- [Security Reference](../reference/security.md)

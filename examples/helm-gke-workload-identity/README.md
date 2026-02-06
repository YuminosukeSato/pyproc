# Helm GKE Workload Identity Federation Example

A Helm values override for GKE Workload Identity Federation with pyproc.

## Overview

This example configures a pyproc deployment to use GKE Workload Identity Federation for authenticating to Google Cloud services without static credentials.

## Prerequisites

- GKE cluster with Workload Identity enabled
- Google Cloud service account with required permissions
- IAM binding between Kubernetes SA and Google Cloud SA:
  ```bash
  gcloud iam service-accounts add-iam-policy-binding SA_NAME@PROJECT_ID.iam.gserviceaccount.com \
    --role roles/iam.workloadIdentityUser \
    --member "serviceAccount:PROJECT_ID.svc.id.goog[NAMESPACE/RELEASE_NAME-pyproc]"
  ```
  Replace RELEASE_NAME with your Helm release name (e.g., myapp). Alternatively, set `serviceAccount.name` in values.yaml to use a fixed name.

## Usage

```bash
helm install myapp ./charts/pyproc -f examples/helm-gke-workload-identity/values.yaml
```

## What This Configures

- ServiceAccount with `iam.gke.io/gcp-service-account` annotation
- Links the Kubernetes SA to a Google Cloud SA for Workload Identity

## Token Mount Note

The base Helm chart sets `automountServiceAccountToken: false` on the ServiceAccount. GKE Workload Identity Federation requires mounting a projected service account token. The GKE metadata server intercepts token requests independently, but the Kustomize overlay sets `automountServiceAccountToken: true` explicitly to ensure the token is available in all configurations.

## Customization

- `serviceAccount.annotations.iam.gke.io/gcp-service-account`: Target Google Cloud service account email

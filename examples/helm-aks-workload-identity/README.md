# Helm AKS Entra Workload ID Example

A Helm values override for Azure AKS Entra Workload Identity with pyproc.

## Overview

This example configures a pyproc deployment to use AKS Entra Workload Identity for authenticating to Azure services without static credentials.

## Prerequisites

- AKS cluster with Workload Identity enabled
- Azure managed identity with required permissions
- Federated identity credential linking the Kubernetes SA to the managed identity:
  ```bash
  az identity federated-credential create \
    --name pyproc-federated \
    --identity-name MANAGED_IDENTITY_NAME \
    --resource-group RESOURCE_GROUP \
    --issuer AKS_OIDC_ISSUER \
    --subject system:serviceaccount:NAMESPACE:RELEASE_NAME-pyproc
  ```
  Replace RELEASE_NAME with your Helm release name (e.g., myapp). Alternatively, set `serviceAccount.name` in values.yaml to use a fixed name.

## Usage

```bash
helm install myapp ./charts/pyproc -f examples/helm-aks-workload-identity/values.yaml
```

## What This Configures

- ServiceAccount with `azure.workload.identity/client-id` annotation
- Pod label `azure.workload.identity/use: "true"` (required by the AKS Workload Identity webhook)

## Customization

- `serviceAccount.annotations.azure.workload.identity/client-id`: Managed identity client ID
- `podLabels.azure.workload.identity/use`: Must be `"true"` for the webhook to inject tokens

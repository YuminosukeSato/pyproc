# Helm EKS IRSA Example

A Helm values override for AWS EKS IAM Roles for Service Accounts (IRSA) with pyproc.

## Overview

This example configures a pyproc deployment to use EKS IRSA for authenticating to AWS services without static credentials.

## Prerequisites

- EKS cluster with OIDC provider configured
- IAM role with trust policy for the EKS OIDC provider:
  ```bash
  aws iam create-role --role-name pyproc-role \
    --assume-role-policy-document file://trust-policy.json
  ```
- Trust policy must reference the EKS OIDC provider and the Kubernetes service account

## Usage

```bash
helm install myapp ./charts/pyproc -f examples/helm-eks-irsa/values.yaml
```

## What This Configures

- ServiceAccount with `eks.amazonaws.com/role-arn` annotation
- Links the Kubernetes SA to an IAM role via IRSA

## Customization

- `serviceAccount.annotations.eks.amazonaws.com/role-arn`: Target IAM role ARN

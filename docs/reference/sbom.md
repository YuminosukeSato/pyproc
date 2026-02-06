---
title: SBOM (Software Bill of Materials)
description: SBOM generation methods and formats for pyproc
keywords: sbom, cyclonedx, syft, supply chain, dependencies
---

# SBOM (Software Bill of Materials)

## Overview

An SBOM (Software Bill of Materials) is a manifest that lists all dependencies included in software.
pyproc generates SBOMs for both Go modules and Python packages.

SBOMs enable the following:

- Tracking vulnerabilities in dependent libraries
- Verifying license compliance
- Ensuring supply chain transparency

## Generation Tool

pyproc uses [syft](https://github.com/anchore/syft) to generate SBOMs.
syft is an open-source tool developed by Anchore that can generate SBOMs from container images and filesystems.

## Format

CycloneDX JSON is used as the output format.

- Specification: [CycloneDX](https://cyclonedx.org/)
- File extension: `.cdx.json`
- Generated files:
    - `sbom-go.cdx.json`: Go module dependencies
    - `sbom-python.cdx.json`: Python package dependencies

## CI Automated Generation

The `sbom` job in `.github/workflows/ci.yml` automatically generates SBOMs for each PR.
Generated SBOMs are available for download as GitHub Actions Artifacts.

## Release Artifacts

In `.github/workflows/release.yml`, SBOMs are generated on tag push and attached as GitHub Release artifacts.
`sbom-go.cdx.json` and `sbom-python.cdx.json` can be downloaded from the release page.

## Local Generation

Install syft to generate SBOMs locally.

```bash
# Install syft (macOS)
brew install syft

# Generate Go SBOM
syft dir:. --exclude ./worker/python -o cyclonedx-json=sbom-go.cdx.json

# Generate Python SBOM
syft dir:worker/python -o cyclonedx-json=sbom-python.cdx.json
```

Generated SBOMs can be used for vulnerability scanning with [grype](https://github.com/anchore/grype).

```bash
# Vulnerability scanning with grype
grype sbom:sbom-go.cdx.json
grype sbom:sbom-python.cdx.json
```

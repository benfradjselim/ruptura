# Getting Started

Get Ruptura up and running in minutes.

## Installation paths

| Method | Best for |
|--------|----------|
| [Kubernetes / Helm](installation.md#kubernetes) | Production, GitOps workflows |
| [Docker](installation.md#docker) | Local dev, single-host deployments |
| [Binary](installation.md#binary) | Edge, air-gapped, minimal footprint |

## Recommended flow

1. **[Install →](installation.md)** — choose Docker, Kubernetes, or binary
2. **[Quickstart →](quickstart.md)** — ingest your first metrics and see the Rupture Index in 5 minutes
3. **[Configuration →](configuration.md)** — tune `ruptura.yaml` for your environment

## Minimum requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 0.5 core | 1 core |
| RAM | 64 MB | 256 MB |
| Disk | 1 GB | 20 GB (400-day retention) |
| Go | 1.18+ | — (build only) |
| Kubernetes | 1.24+ | — (operator only) |

# Ruptura — Predictive AIOps for Kubernetes

<p align="center">
  <img src="assets/logo/ruptura-icon-256.png" alt="Ruptura" width="120" />
</p>

<p align="center">
  <img alt="Go version" src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go">
  <img alt="License" src="https://img.shields.io/badge/License-Apache%202.0-blue.svg">
  <img alt="CNCF Sandbox" src="https://img.shields.io/badge/CNCF-Sandbox%20Applicant-0086FF">
  <img alt="GitHub release" src="https://img.shields.io/github/v/release/benfradjselim/ruptura">
</p>

Ruptura is a predictive AIOps engine for Kubernetes that detects workload failures before they become outages. It ingests telemetry via OTLP and Prometheus remote-write, computes 10 composite KPI signals per workload with adaptive per-workload baselines, fuses them into the **Fused Rupture Index (FRI)** using a self-calibrating 5-model ML ensemble, and drives an action engine that applies automated Kubernetes remediations — scaling, restarts, cordons — behind configurable safety gates.

---

## Key Features

- **Fused Rupture Index (FRI)** — weighted fusion of metric, log, and trace anomaly scores into a single actionable severity value
- **5-model ML ensemble** — CA-ILR, ARIMA, Holt-Winters, MAD, and EWMA; models are re-weighted every 60 s based on live prediction error
- **10 KPI signals** — Stress, Fatigue, Mood, Pressure, Humidity, Contagion, Resilience, Entropy, Velocity, Throughput; each with adaptive per-workload baselines
- **OTLP ingest** — native receivers for OTLP metrics, logs, and traces plus Prometheus remote-write on a dedicated NodePort
- **Helm / OCI deploy** — single `helm install` from `ghcr.io/benfradjselim/charts/ruptura`
- **ruptura-ctl CLI** — inspect workloads, trigger actions, and query the engine from the terminal
- **Automated K8s remediation** — Tier-1 (auto), Tier-2 (human-approved), Tier-3 (alert-only); emergency stop endpoint
- **Rupture fingerprinting** — cosine-similarity pattern matching against historical rupture vectors for instant suggested fixes
- **HealthScore forecast** — +15 and +30 minute projections with "Critical in ~Nm" warnings
- **Svelte 4 dashboard** — light/dark, SSE live stream, Fleet grid, Topology map, per-workload history and predictions

---

## Quick Start

```bash
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system \
  --create-namespace \
  --set apiKey=$(openssl rand -hex 32)

# Dashboard:   http://<node-ip>:31469/
# Engine API:  http://<node-ip>:31468/api/v2/health
# OTLP ingest: http://<node-ip>:31470/otlp/v1/metrics
```

That's it. Ruptura starts calibrating per-workload baselines immediately once telemetry flows in. Send synthetic workloads to see it in action:

```bash
python3 scripts/simulate.py
```

---

## Architecture

Ruptura v7 ships as two Kubernetes pods behind a shared Helm chart in the `ruptura-system` namespace. The **ruptura-engine** pod (Go binary) handles all telemetry ingest on port 4317/31470, exposes the REST API on port 8080/31468, and runs the analyzer, ensemble, action engine, and BadgerDB storage. The **ruptura-ui** pod (Svelte 4 + nginx) serves the dashboard on port 80/31469 and reverse-proxies `/api/` calls to the engine, injecting the Bearer token automatically. Both images are published to `ghcr.io/benfradjselim/ruptura`.

```
ruptura-system
  ruptura-engine  :8080 REST API | :4317 OTLP     NodePort 31468 / 31470
  ruptura-ui      :80  dashboard  (proxies /api/)  NodePort 31469
```

---

## Documentation

| Resource | Link |
|----------|------|
| Full technical docs & API reference | [workdir/README.md](workdir/README.md) |
| Governance | [GOVERNANCE.md](GOVERNANCE.md) |
| Contributing | [CONTRIBUTING.md](CONTRIBUTING.md) |
| Security policy | [SECURITY.md](SECURITY.md) |
| OpenAPI spec | [docs/openapi.yaml](docs/openapi.yaml) |
| Website | <https://ruptura.dev> |

---

## Project Status

| Version | Status |
|---------|--------|
| v7.1.0 | Current stable release |
| v7.0.x | Maintained |

Active branch: `main` — Module: `github.com/benfradjselim/ruptura`

---

## Community

Questions, ideas, and discussion live in [GitHub Discussions](https://github.com/benfradjselim/ruptura/discussions). Bug reports and feature requests go to [GitHub Issues](https://github.com/benfradjselim/ruptura/issues).

Ruptura is applying to the [CNCF Sandbox](https://www.cncf.io/sandbox-projects/) program. Governance details, maintainer list, and contribution process are documented in the files above.

---

## License

Copyright 2024-2026 Selim Benfradj and the Ruptura Authors.

Licensed under the [Apache License, Version 2.0](LICENSE).

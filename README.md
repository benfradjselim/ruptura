# Ruptura — Predictive AIOps for Kubernetes

<p align="center">
  <img src="assets/logo/ruptura-icon-256.png" alt="Ruptura" width="120" />
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.18-00ADD8?logo=go">
  <img alt="License" src="https://img.shields.io/badge/License-Apache%202.0-blue.svg">
  <img alt="Version" src="https://img.shields.io/github/v/release/benfradjselim/ruptura">
  <img alt="CNCF Sandbox" src="https://img.shields.io/badge/CNCF-Sandbox%20Applicant-0086FF">
  <a href="https://codecov.io/gh/benfradjselim/ruptura"><img alt="Coverage" src="https://codecov.io/gh/benfradjselim/ruptura/branch/main/graph/badge.svg"></a>
</p>

<p align="center">
  <b>Ruptura detects Kubernetes workload failures before they become outages.</b>
</p>

It ingests telemetry via OTLP and Prometheus remote-write, computes 10 composite KPI signals per workload, fuses them into the **Fused Rupture Index (FRI)** using a 5-model adaptive ML ensemble, and drives a 3-tier action engine that applies automated Kubernetes remediations behind configurable safety gates.

---

## Why Ruptura?

| Traditional observability | Ruptura |
|--------------------------|---------|
| Alerts fire *after* the fact | FRI detects divergence **hours early** |
| Global thresholds — batch jobs always "stressed" | **Adaptive per-workload baselines** after ~45 min |
| 5+ tools: Prom + Grafana + AM + Loki + PD | **One `helm install`**, two pods, no external database |
| "host-123 CPU 78%" — what does it mean? | "payment-api has 28 minutes before cascade failure" |
| Manual incident response | Tier-1 actions (scale, restart, rollback) fire automatically |

---

## Quick Start

One command, no Helm required — a single manifest with namespace, RBAC,
Deployment, Services, and a Job that auto-generates your API key on first
apply:

```bash
kubectl apply -f https://raw.githubusercontent.com/benfradjselim/ruptura/main/install/ruptura.yaml
kubectl -n ruptura-system get pods
kubectl -n ruptura-system logs job/ruptura-init-apikey   # prints the generated API key
```

Prefer Helm (production installs, upgrades, values overrides)? Same result,
more control:

```bash
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system \
  --create-namespace \
  --set apiKey=$(openssl rand -hex 32) \
  --set ui.enabled=true
```

```bash
kubectl -n ruptura-system port-forward svc/ruptura 8080:80 &      # engine API
kubectl -n ruptura-system port-forward svc/ruptura-ui 8081:80 &   # dashboard
```

| Endpoint | URL |
|----------|-----|
| Dashboard | `http://localhost:8081/` |
| Engine API | `http://localhost:8080/api/v2/health` |
| OTLP ingest | `<node-ip>:31470` (fixed NodePort) |

Send synthetic workloads to see all 6 failure modes immediately:

```bash
python3 scripts/simulate.py --host <node-ip> --port 31470
```

### No Kubernetes? Try it locally

```bash
docker compose up --build
```

Starts the engine in demo mode (7 days of synthetic baseline data across 12
workloads, no calibration wait) plus the dashboard — nothing else required.
Dashboard: `http://localhost:8081/` (log in with any username, password
`ruptura-local-dev`). Engine API: `http://localhost:8080/api/v2/health`.

For Go development without Docker at all:

```bash
cd workdir && go run ./cmd/ruptura --demo
```

---

## Key Features

- **Fused Rupture Index** — fuses metric, log, and trace anomaly scores into one actionable severity value per workload
- **5-model adaptive ensemble** — CA-ILR, ARIMA, Holt-Winters, MAD, EWMA; re-weighted every 60s on live prediction error
- **10 KPI signals** — Stress, Fatigue, Mood, Pressure, Humidity, Contagion, Resilience, Entropy, Velocity, Throughput
- **SRE-friendly labels** — signals display as Risk Score, Memory Pressure, Blast Radius, etc. in the UI
- **Adaptive per-workload baselines** — Welford online stats, active after ~45 min of telemetry
- **OTLP + Prometheus ingest** — native receivers, no sidecar required
- **Kubernetes-native** — groups signals by `namespace/kind/name`, not by host IP
- **Helm OCI deploy** — single `helm install`, two pods, no external database
- **ruptura-ctl** — full CLI with watch mode, context windows, emergency stop, server version check
- **3-tier action engine** — Tier-1 auto, Tier-2 human-approved, Tier-3 alert-only; per-target rate limits
- **Rupture fingerprinting** — cosine-similarity pattern matching against historical rupture vectors
- **HealthScore forecast** — +15 and +30 minute projections with time-to-critical warnings
- **Synthetic test lab** — 6 pre-built workload simulators covering all failure scenarios (stable, degraded, at-risk, spike, calibrating)

---

## Architecture

```
ruptura-system
  ruptura-engine  :8080 REST API | :4317 OTLP     NodePort 31468 / 31470
  ruptura-ui      :80  dashboard  (proxies /api/)  NodePort 31469
```

---

## Lab Setup (Civo / k3s)

A ready-to-deploy lab environment with Prometheus, OTel Collector, and 6 synthetic test apps:

```bash
export KUBECONFIG=~/your-kubeconfig
bash lab-setup/setup.sh
```

Deploys: Prometheus + kube-state-metrics + OTel Collector + Ruptura + 6 apps covering all failure scenarios.

---

## CLI — ruptura-ctl

```bash
ruptura-ctl status              # fleet overview
ruptura-ctl status -w 5         # watch mode, refresh every 5s
ruptura-ctl get ruptures        # active rupture events (server-side filtered)
ruptura-ctl describe workload <ns/kind/name>
ruptura-ctl actions             # pending Tier-2 actions
ruptura-ctl context add "production/*" --type maintenance --duration 2h
ruptura-ctl suppress create "staging/*" 30m
ruptura-ctl emergency-stop      # halt all Tier-1 actions (requires confirmation)
```

---

## Security

| Property | Status |
|----------|--------|
| Auth fail-closed | `RUPTURA_API_KEY` required — no silent open access |
| `/api/v2/metrics` | Public (Prometheus scraping without token) |
| `/api/v2/health` `/ready` | Public (k8s probes) |
| All other routes | Bearer token required |
| Constant-time comparison | `crypto/subtle` on all auth checks |
| Atomic storage | Crash-safe compaction — no double-averaging on restart |

---

## Documentation

| Resource | Link |
|----------|------|
| Technical docs & API reference | [docs/REFERENCE.md](docs/REFERENCE.md) |
| Engine & concepts | [workdir/README.md](workdir/README.md) |
| CLI reference | [site/docs/cli/rupturactl.md](site/docs/cli/rupturactl.md) |
| Website | <https://benfradjselim.github.io/ruptura/> |
| CNCF Sandbox proposal | [docs/cncf-sandbox-proposal.md](docs/cncf-sandbox-proposal.md) |

---

## Project Status

Pre-1.0-adoption: the engine is feature-complete and released continuously, and the project is actively seeking early production adopters and external contributors. The latest stable version is always the newest [GitHub Release](https://github.com/benfradjselim/ruptura/releases) — the version badge above tracks it automatically.

Active branch: `main` — Module: `github.com/benfradjselim/ruptura`

---

## Community

- **Discussions** — [github.com/benfradjselim/ruptura/discussions](https://github.com/benfradjselim/ruptura/discussions)
- **Issues** — [github.com/benfradjselim/ruptura/issues](https://github.com/benfradjselim/ruptura/issues)
- **CNCF Sandbox** — proposal in preparation

---

## License

Copyright 2024–2026 Selim Benfradj and the Ruptura Authors.  
Licensed under the [Apache License, Version 2.0](LICENSE).

# Ruptura

<p align="center">
  <img src="ruptura-icon.png" alt="Ruptura" width="100" />
</p>

**The Predictive Action Layer for Cloud-Native Infrastructure.**

Ruptura detects Kubernetes workload failures before they become outages — using the Fused Rupture Index, 10 composite KPI signals with adaptive per-workload baselines, and an action engine that responds automatically with configurable safety gates.

→ **[Getting Started →](getting-started/installation.md)** · **[GitHub](https://github.com/benfradjselim/ruptura)** · **[CLI Reference →](cli/rupturactl.md)**

---

## Why Ruptura?

| Traditional Observability | Ruptura |
|--------------------------|---------| 
| Threshold alerts fire *after* the fact | Fused Rupture Index detects divergence **hours early** |
| Global thresholds — batch jobs always "stressed" | **Adaptive per-workload baselines** after ~45 min |
| "host-123 CPU 78%" — what does it mean? | "payment-api is exhausted — fatigue accumulation, cascade from db" |
| Manual incident response | Tier-1 actions (scale, restart, rollback) with safety gates |
| 5+ tools: Prom + Grafana + AM + Loki + PD | **One `helm install`**, two pods, no external database |
| Numbers, no reasoning | **Narrative explain** — structured causal chain |

---

## v7 Architecture

```
┌────────────────────────────────────────────────────────────┐
│                      ruptura-system                         │
│                                                             │
│  ┌───────────────────────┐    ┌────────────────────────┐   │
│  │    ruptura-engine     │    │      ruptura-ui         │   │
│  │    (Go binary)        │    │  (Svelte 4 + nginx)     │   │
│  │                       │    │                          │   │
│  │  :8080  REST API      │◄───│  nginx proxies /api/    │   │
│  │  :4317  OTLP ingest   │    │  injects Bearer token   │   │
│  │                       │    │  :80   dashboard UI      │   │
│  └───────────────────────┘    └────────────────────────┘   │
│          NodePort 31468               NodePort 31469         │
│          NodePort 31470 (OTLP)                               │
└────────────────────────────────────────────────────────────┘
```

| Port | Purpose |
|------|---------|
| 31468 | Engine REST API (`/api/v2/*`) |
| 31469 | Svelte dashboard |
| 31470 | OTLP ingest + Prometheus remote-write |

---

## Core Concepts

### Fused Rupture Index

```
FusedR = weighted_average(metricR, logR, traceR)
         requires ≥ 2 sources — one noisy signal cannot trigger critical
```

| FusedR | State | Action |
|--------|-------|--------|
| < 1.5 | Stable | None |
| 1.5 – 3.0 | Warning | Tier-3 alert |
| 3.0 – 5.0 | Critical | Tier-2 suggested |
| ≥ 5.0 | Emergency | Tier-1 automated |

### 10 Composite KPI Signals

| Signal | Display Name | Measures |
|--------|-------------|----------|
| stress | CPU Pressure | CPU + latency burst |
| fatigue | Memory Pressure | Cumulative baseline deviation |
| mood | Trend | Log error/warn sentiment |
| pressure | Load Index | Memory + disk saturation |
| humidity | Saturation | Forecast variance |
| contagion | Blast Radius | Error propagation from upstream |
| resilience | Resilience | Recovery speed after spikes |
| entropy | Entropy | Signal disorder |
| velocity | Velocity | Request rate acceleration |
| throughput | Throughput | Data volume per cycle |

### Adaptive Ensemble — 5 models, no configuration

| Model | Strengths |
|-------|-----------|
| **CA-ILR** (dual-scale) | O(1) update · detects acceleration |
| **ARIMA** | Stationary series with trends |
| **Holt-Winters** | Seasonal patterns |
| **MAD** | Robust to outliers |
| **EWMA** | Reacts quickly to recent shifts |

Weights recomputed every 60s from live prediction error — no tuning needed.

### Action Engine

| Tier | Trigger | Mode |
|------|---------|------|
| Tier-1 | FusedR ≥ 5.0 + confidence ≥ 0.85 | Auto (scale/restart/cordon) |
| Tier-2 | FusedR ≥ 3.0 + confidence ≥ 0.60 | Suggested — approve via API or CLI |
| Tier-3 | FusedR ≥ 1.5 | Alert only (Slack / PagerDuty / webhook) |

Safety gates: per-target rate limit (6/hour), 300s cooldown, namespace allowlist, emergency stop.

---

## Current Release

**v8.2.3** — Dual-axis infra collector layer, complete UI redesign, HealthScore fixes.

| Change | Detail |
|--------|--------|
| Infra collector layer (v8.0) | 8 collectors (node, control-plane, MCP, networking, storage, admission, operator, tenancy) feeding SDI/EBI/GNI/CGPM signals into HealthScore + eFRI |
| `/api/v2/infra/*`, `/api/v2/propagation/*` | New endpoints exposing the infra signal graph and cross-workload propagation |
| UI redesign (v8.1) | Complete Müller-Brockmann grid redesign, dark/light mode, infrastructure visibility panels, calibration progress bar |
| HealthScore fix (v8.2) | Analyzer now stores `1 - mood` in the adaptive baseline, fixing a bug that pinned `health_score` at 0.80 for healthy workloads |
| Demo mode (unreleased, `main`) | `--demo` / `RUPTURA_DEMO_MODE=true` seeds 7 days of synthetic data across 12 workloads — no cluster, no calibration wait |
| One-command install (unreleased, `main`) | `kubectl apply -f install/ruptura.yaml` — namespace, RBAC, and an auto-generated API key, no Helm required |

Previous release: **v7.1.0** — auth fail-closed, atomic compaction, Fleet UX, ruptura-ctl v1.2.0. [Full history →](community/roadmap.md)

[Full changelog →](community/roadmap.md) · [Getting Started →](getting-started/installation.md) · [CLI Reference →](cli/rupturactl.md)


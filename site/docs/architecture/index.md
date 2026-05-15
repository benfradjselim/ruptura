# Architecture

Ruptura v7 ships as **two separate Kubernetes pods** behind a shared Helm chart — the engine binary and the Svelte dashboard — with BadgerDB embedded in the engine. No external database, no sidecar, no agent fleet required.

## System diagram

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
| 31469 | Svelte dashboard (light/dark, SSE live events) |
| 31470 | OTLP ingest (`/api/v2/write`, `/otlp/v1/metrics`, `/otlp/v1/logs`, `/otlp/v1/traces`) |

## Data flow

```
Prometheus remote_write ──┐
OTLP metrics/logs/traces ─┼─► Ingest receivers (port 31470 / 4317)
                           │
              WorkloadRef grouping
         (namespace / kind / name — pods merged)
                           │
              Adaptive per-workload baselines
           (Welford online stats · active after 24h)
                           │
              10 Composite KPI signals  (every 15 s)
      stress · fatigue · mood · pressure · humidity
      contagion · resilience · entropy · velocity · throughput
                           │
              5-model adaptive ensemble
         CA-ILR · ARIMA · Holt-Winters · MAD · EWMA
               online MAE-based weights · 60s update
                           │
           Fused Rupture Index™  (FusedR)
       metricR + logR + traceR  ─ requires ≥ 2 sources
                           │
           ┌───────────────┼───────────────┐
           │               │               │
       Tier-3 alert    Tier-2 suggest  Tier-1 auto
     (FusedR ≥ 1.5)  (FusedR ≥ 3.0)  (FusedR ≥ 5.0)
           │               │               │
     AM / PagerDuty    approve via API   K8s / webhook
```

## Engine packages

| Package | Responsibility |
|---------|---------------|
| `cmd/ruptura` | Binary entry point, flag parsing, graceful shutdown |
| `cmd/ruptura-ctl` | CLI companion — status, health, workload queries |
| `internal/ingest` | OTLP/HTTP, Prometheus remote-write receivers |
| `internal/pipeline` | Metric / log / trace pipelines, workload keying |
| `internal/analyzer` | 10-signal KPI computation, calibration, adaptive baselines |
| `internal/fusion` | FusedR compositor (metricR + logR + traceR) |
| `internal/pipeline` (ensemble) | 5-model anomaly ensemble, MAE weight rebalancing |
| `internal/predictor` | HealthScore forecast (+15m, +30m) |
| `internal/actions` | Action execution, K8s actuator, safety gates |
| `internal/api` | REST API v2 handlers (44 endpoints) |
| `internal/correlator` | Burst detector, topology graph builder |
| `internal/explain` | Narrative engine, rupture fingerprinting |
| `internal/history` | Time-series history manager |
| `internal/storage` | BadgerDB wrapper, TTL GC |

## UI architecture

The `ruptura-ui` container is a **Svelte 4 SPA** built at CI time and served by nginx:

- nginx proxies all `/api/*` requests to `ruptura-engine:8080` and injects the `Authorization: Bearer` header from an environment variable
- Dashboard state is driven by the REST API and by **SSE** (`GET /api/v2/events`) for live rupture/recovery events
- Light/dark mode persisted via `localStorage`
- Chart.js 4.5 for time-series and forecast views; Cytoscape.js for the topology map

## Detailed pages

- [Pipelines →](pipelines.md)
- [Fusion Engine →](fusion-engine.md)
- [Dashboard →](dashboard.md)
- [Kubernetes Operator →](operator.md)

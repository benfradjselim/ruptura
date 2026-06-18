# Ruptura — Technical Reference
**Version:** 7.1.0 · **Updated:** 2026-06-18

This document is the single authoritative reference for contributors and operators.
For the whitepaper and formal specs, see `docs/v6.1.0/SPECS.md`.

---

## Architecture Overview

```
                        ┌─────────────────────────────┐
  OTLP / Prometheus ──► │     ingest (port 4317)      │
                        └────────────┬────────────────┘
                                     │
                        ┌────────────▼────────────────┐
                        │   pipeline/metrics           │
                        │   CA-ILR + ARIMA + HW + MAD │ ◄── ensemble weighting (MSE)
                        └────────────┬────────────────┘
                                     │
              ┌──────────────────────┼──────────────────────┐
              │                      │                       │
   ┌──────────▼──────┐   ┌──────────▼──────┐   ┌──────────▼──────┐
   │ pipeline/logs   │   │  fusion engine  │   │ pipeline/traces │
   │ error rate      │   │  0.6M+0.2L+0.2T │   │ cascade index   │
   └──────────┬──────┘   └──────────┬──────┘   └──────────┬──────┘
              └──────────────────────┴──────────────────────┘
                                     │ FusedR
                        ┌────────────▼────────────────┐
                        │   analyzer (KPI engine)      │
                        │   stress, fatigue, contagion  │
                        │   health_score, HealthScore   │
                        └────────────┬────────────────┘
                                     │
                        ┌────────────▼────────────────┐
                        │   action engine              │
                        │   Tier-1 / Tier-2 / Tier-3   │
                        └────────────┬────────────────┘
                                     │
                        ┌────────────▼────────────────┐
                        │   REST API  (port 8080)      │
                        │   + Svelte UI (workdir/ui/)  │
                        └─────────────────────────────┘
```

---

## Key Formulas

### CA-ILR Dual-Scale Tracker
```
stable tracker: λ = 0.995  → N_eff ≈ 200 samples (~50 min @ 15s)
burst  tracker: λ = 0.80   → N_eff ≈ 5 samples  (~75s @ 15s)

Rupture Index: R(t) = α_burst / α_stable
```

### Rupture Classification
| R | State | Action Tier |
|---|-------|-------------|
| < 1.0 | Stable | None |
| 1.0–1.5 | Elevated | None |
| 1.5–3.0 | Warning | Tier-3 |
| 3.0–5.0 | Critical | Tier-2 |
| ≥ 5.0 | Emergency | Tier-1 |

### Fused Rupture Index
```
FusedR = 0.6 × metricR + 0.2 × logR + 0.2 × traceR
         (when all three signals available)

If only 2 signals:  FusedR = 0.75 × metricR + 0.25 × other
```

### HealthScore
```
HealthScore = 100 × ∏_{k ∈ {stress, fatigue, pressure, contagion}} min(1, max(0, 1 − w_k × s_k))
```

### Time-to-Failure
```
TTF = (θ_critical − m(t)) / α_burst
      clamped to [0, 3600] seconds
```

---

## Signal Scales

All signals are returned as raw values from the API. The UI applies a multiply factor:

| Signal | API range | Display multiply | SRE label |
|--------|-----------|-----------------|-----------|
| health_score | 0–1 | ×100 → % | Reliability |
| fused_rupture_index | 0–10 | ×10 → 0–100 | Risk Score |
| stress | 0–1 | ×100 → % | CPU Pressure |
| fatigue | 0–1 | ×100 → % | Memory Pressure |
| mood | 0–1 | ×100 → % | Trend |
| contagion | 0–1 | ×100 → % | Blast Radius |
| pressure | 0–1 | ×100 → % | Load Index |
| resilience | 0–1 | ×100 → % | Resilience |
| calibration_pct | 0–100 | ×1 | Calibration |
| forecast mean/upper/lower | 0–1 | ×100 → % | — |

---

## Storage Key Schema

```
m:{host}:{metric}:{ts_ns}       raw metrics      TTL 7d
kpi:{name}:{host}:{ts_ns}       raw KPIs         TTL 7d
r5:{host}:{metric}:{ts_ns}      5m metric rollup TTL 35d
kr5:{name}:{host}:{ts_ns}       5m KPI rollup    TTL 35d
r1h:{host}:{metric}:{ts_ns}     1h metric rollup TTL 400d
kr1h:{name}:{host}:{ts_ns}      1h KPI rollup    TTL 400d
r:{id}                          rupture events
r:{host}:history:{ts}           rupture history
ac:{id}                         action records
ctx:{id}                        context entries
sup:{id}                        suppression windows
sp:{traceID}:{spanID}           spans
l:{service}:{ts}                log output
```

**CRITICAL:** Never change prefixes without updating `internal/storage/retention.go`.

---

## API Reference

### Authentication
All `/api/v2/` routes (except `/health`, `/ready`, `/version`, `/auth/*`) require:
```
Authorization: Bearer <RUPTURA_API_KEY>
```

### Core Endpoints

```
GET  /api/v2/health                          probe (no auth)
GET  /api/v2/ready                           probe (no auth)
GET  /api/v2/fleet                           all workloads + KPI snapshots
GET  /api/v2/ruptures                        active ruptures
GET  /api/v2/rupture/{host}                  rupture state for a workload
GET  /api/v2/rupture/{host}/history          rupture timeline
GET  /api/v2/rupture/{host}/profile          surge profile + fingerprint match
GET  /api/v2/kpi/{name}/{host}               single KPI value
GET  /api/v2/kpi/{name}/{host}/history       KPI timeline
GET  /api/v2/forecast/{metric}/{host}        ensemble forecast
GET  /api/v2/history/{workload}              KPI snapshot history
GET  /api/v2/alerts                          active anomaly alerts
GET  /api/v2/topology                        service dependency graph
GET  /api/v2/nodes                           node inventory (requires metrics-server)
GET  /api/v2/explain/{rupture_id}            XAI trace + narrative
GET  /api/v2/explain/{rupture_id}/formula    formula audit trail
GET  /api/v2/engine/status                   engine internals
GET  /api/v2/engine/storage                  BadgerDB stats
GET  /api/v2/metrics                         Prometheus self-metrics

POST /api/v2/write                           Prometheus remote-write ingest
POST /api/v2/forecast                        POST-body forecast request
POST /api/v2/context                         add context entry (maintenance, load-test)
DELETE /api/v2/context/{id}                  remove context entry
POST /api/v2/suppressions                    create suppression window
DELETE /api/v2/suppressions/{id}             remove suppression
DELETE /api/v2/ingest/purge                  purge stored data (type=signals|kpis|all)

GET  /api/v2/actions                         action recommendations
POST /api/v2/actions/{id}/approve            approve Tier-2 action (autopilot only → 402 in community)
POST /api/v2/actions/{id}/rollback           rollback action (autopilot only → 402 in community)
POST /api/v2/actions/emergency-stop          stop all auto-actions immediately
```

### OTLP Ingest (separate port 4317)
```
POST /otlp/v1/metrics    OTLP/HTTP metrics (protobuf or JSON)
POST /otlp/v1/logs       OTLP/HTTP logs
POST /otlp/v1/traces     OTLP/HTTP traces
```

---

## Edition Gate

Community edition returns HTTP 402 for paid features:
```json
{
  "error": "action execution requires the Autopilot edition",
  "upgrade": "set RUPTURA_EDITION=autopilot to enable automated and manual action approval"
}
```

Current community limitations:
- `POST /api/v2/actions/{id}/approve` → 402
- `POST /api/v2/actions/{id}/rollback` → 402
- Tier-1 auto-execution goroutine disabled

---

## Configuration

Key environment variables:
```
RUPTURA_API_KEY        Bearer token for API auth (required in production)
RUPTURA_EDITION        "community" (default) | "autopilot"
RUPTURA_DEMO_MODE      "true" to inject synthetic workloads
KAFKA_BROKERS          Kafka brokers for event bus (optional)
RUPTURA_WORKLOAD_WEIGHTS  JSON array of SignalWeights for per-workload overrides
```

CLI flags:
```
--port        int    HTTP API port (default 8080)
--otlp-port   int    OTLP ingest port (default 4317)
--storage     string BadgerDB path (default /var/lib/ruptura/data)
--api-key     string API bearer token
--version            print version and exit
```

---

## Helm Quick Start

```bash
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system \
  --create-namespace \
  --set apiKey=$(openssl rand -hex 32)
```

NodePorts: API=31468 · Dashboard=31469 · OTLP=31470

---

## Development

```bash
# Build engine
cd workdir && go build ./...

# Run tests
cd workdir && go test -race ./...

# Build UI
cd workdir/ui && npm ci && npm run build

# Simulate workloads (replace with your cluster IP)
python3 scripts/simulate.py --host <YOUR_NODE_IP> --port 31470
```

**Important:** The active UI is `workdir/ui/`. The directory `ui/` at repo root is archived.

---

## Project Files

```
CLAUDE.md                     Claude Code instructions (English-only, rules)
workdir/                      Go engine source
  cmd/ruptura/                main binary entry point
  internal/pipeline/metrics/  CA-ILR ensemble (core engine)
  internal/analyzer/          KPI computation
  internal/fusion/            multi-signal fusion
  internal/storage/           BadgerDB + compaction
  internal/api/               REST handlers
  internal/actions/           action engine + K8s actuator
  ui/                         ACTIVE Svelte 4 UI (workdir/ui/)
docs/                         internal planning + specs
  REFERENCE.md                this file
  v6.1.0/SPECS.md             formal technical specifications
  judgment.md                 design conscience document
helm/                         Helm chart for community edition
site/                         MkDocs source → GitHub Pages
scripts/simulate.py           workload simulator for demos
```

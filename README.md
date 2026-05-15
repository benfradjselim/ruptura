# Ruptura

<p align="center">
  <img src="assets/logo/ruptura-icon-256.png" alt="Ruptura" width="120" />
</p>

**The Predictive Action Layer for Cloud-Native Infrastructure.**

Ruptura detects workload ruptures before they cause outages — using the Fused Rupture Index™, 10 composite KPI signals with adaptive per-workload baselines, and an action engine that responds automatically with safety gates.

→ **[Technical documentation & quickstart](workdir/README.md)**
→ **[Website & full docs](https://benfradjselim.github.io/ruptura/)**
→ **[API Specification](docs/openapi.yaml)**
→ **[Live dashboard](http://185.229.225.115:31469/)**

---

## Project Status

| Version | Date | Status |
|---------|------|--------|
| v7.0.4 | 2026-05-15 | ✅ Released — OTLP NodePort 31470 exposed, workload simulator |
| v7.0.3 | 2026-05-15 | ✅ Released — JSON crash fix, real PNG logo, topology overhaul, health scores per workload |
| v7.0.2 | 2026-05-15 | ✅ Released — 10-signal bars, light/dark mode, dataflow stats, all backend APIs wired |
| v7.0.1 | 2026-05-15 | ✅ Released — ruptura-ui pod, logo, calibrating state, Settings & Alerts pages |
| v7.0.0 | 2026-05-15 | ✅ Released — v7 architecture: separate UI pod, SSE, k8s metadata, node health |
| v6.8.13 | 2026-05-13 | ✅ Released — log/trace ingest counters, Live Data Flow, ruptura-ctl v1.0.0 |

**Operator:**

| Version | Date | Status |
|---------|------|--------|
| ruptura-operator v0.6.9 | 2026-05-07 | 🔄 Submitted to Red Hat OperatorHub |
| ruptura-operator v0.6.8 | 2026-05-07 | ✅ Merged into OperatorHub community-operators |

**Active branch:** `v7` · **Module:** `github.com/benfradjselim/ruptura`

---

## v7 Architecture

v7 ships as **two separate Kubernetes pods** behind a shared Helm chart:

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
| 31470 | OTLP ingest (`/api/v2/write`, `/otlp/v1/metrics`, `/otlp/v1/logs`, `/otlp/v1/traces`) |

---

## How Ruptura Works

### 1 — Telemetry ingestion (port 31470)

```
Prometheus remote-write  →  /api/v2/write          →  metric pipeline
OTLP metrics             →  /otlp/v1/metrics        →  metric pipeline
OTLP logs                →  /otlp/v1/logs           →  burst detector → logR
OTLP traces              →  /otlp/v1/traces         →  topology graph + traceR
```

Workload identity is derived from OTLP resource attributes (`k8s.deployment.name`, `k8s.namespace.name`). Metrics via `/api/v2/write` must include a `host` label set to `namespace/Kind/name`.

### 2 — Signal computation (10 KPIs)

Every 15 seconds, the analyzer computes 10 composite KPI signals per workload:

| Signal | Measures |
|--------|----------|
| **Stress** | CPU + latency burst |
| **Fatigue** | Cumulative baseline deviation (long-term wear) |
| **Mood** | Log error/warn sentiment ratio |
| **Pressure** | Memory + disk saturation |
| **Humidity** | Forecast variance — how predictable behavior is |
| **Contagion** | Error propagation from upstream services |
| **Resilience** | Recovery speed after spikes |
| **Entropy** | Internal signal disorder |
| **Velocity** | Request rate acceleration |
| **Throughput** | Data volume processed per cycle |

Each signal carries `value`, `state` (ok / warning / critical), and `trend` (rising / falling / stable).

### 3 — Fused Rupture Index (FusedR)

```
FusedR = weighted_average(metricR, logR, traceR)
```

| FusedR | State | Default action |
|--------|-------|----------------|
| < 1.5 | Stable / Elevated | None |
| 1.5 – 3.0 | Warning | Tier-3 — human alert |
| 3.0 – 5.0 | Critical | Tier-2 — suggested action |
| ≥ 5.0 | Emergency | Tier-1 — automated action |

### 4 — Adaptive 5-model ensemble

| Model | Strength |
|-------|----------|
| CA-ILR (dual-scale) | O(1) update, detects acceleration |
| ARIMA | Stationary series with trends |
| Holt-Winters | Seasonal / periodic patterns |
| MAD | Outlier-robust |
| EWMA | Fast reaction to recent shifts |

Models are re-weighted every 60s based on actual prediction error — no config needed.

### 5 — HealthScore forecast

Projects HealthScore +15 and +30 minutes forward. `critical_eta_minutes` appears on the card when the projected score is heading toward critical — shows "⚠ Critical in ~12m" in the UI.

### 6 — Rupture fingerprinting & pattern matching

At every confirmed rupture (FusedR ≥ 3.0), an 11-dimensional KPI vector is stored. Future queries run cosine similarity — a match ≥ 0.85 surfaces as `pattern_match` with the prior resolution note. Operators can immediately apply a known fix.

### 7 — Business signals

| Signal | Description |
|--------|-------------|
| `slo_burn_velocity` | Error budget burn rate (multiples) |
| `blast_radius` | Downstream service count |
| `recovery_debt` | Near-miss count in 7 days |

### 8 — SSE live event stream

`GET /api/v2/events` — Server-Sent Events. Every rupture and recovery fires in real time. The Fleet dashboard shows a live rupture counter that updates without polling.

### 9 — Action engine

```
FusedR ≥ threshold → safety gates → K8s (scale / restart / cordon) or webhook
POST /api/v2/actions/emergency-stop   →  halt all pending actions immediately
```

### 10 — Narrative explain

```
GET /api/v2/explain/{id}/narrative
→ "payment-api fatigue 0.81 + contagion wave from payment-db pushed
   FusedR from 1.8 to 4.2 in 18 minutes. Cascade rupture, not an isolated spike."

GET /api/v2/explain/{id}/formula
→ raw KPI breakdown
```

---

## Dashboard (v7)

Svelte 4 SPA with nginx reverse proxy — light/dark mode toggle, full SSE integration.

| View | What you see |
|------|-------------|
| **Fleet** | Workload grid. Per-card: health ring (actual KPI value), 10 signal mini-bars, calibration progress, rupture warning |
| **Fleet → Signals** | 10 KPI cells, PatternMatch warning, BusinessSignals, explain panel |
| **Fleet → History** | Time-series — toggle any of 12 signals; Chart.js |
| **Fleet → Forecast** | HealthScore projection chart |
| **Fleet → Predictions** | Per-metric ensemble predictions |
| **Fleet → Events** | SSE live rupture/recovery log |
| **Fleet → Logs** | Last 200 log lines for the workload |
| **Fleet → Actions** | Approve / reject Tier-2 actions |
| **Fleet → Kubernetes** | Pod list, replicas, resources, labels |
| **Topology** | Service dependency graph from OTLP traces. Click node → health bar + FusedR. Click edge → call rate, error rate, P99 latency |
| **Engine** | Runtime stats, analyzer state, ingest rates, cumulative data flow, BadgerDB storage |
| **Alerts** | Active / resolved alert feed |
| **Nodes** | K8s node health — CPU, memory, disk pressure |
| **Settings** | Data sources, Ingest Stats (live totals), preferences |

---

## Install

**Kubernetes (Helm — OCI):**

```bash
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system \
  --create-namespace \
  --set apiKey=$(openssl rand -hex 32)

# Dashboard:   http://<node-ip>:31469/
# Engine API:  http://<node-ip>:31468/api/v2/health
# OTLP ingest: http://<node-ip>:31470/api/v2/write
```

**Inject synthetic workloads immediately:**

```bash
python3 scripts/simulate.py
# Sends 5 workloads every 5s:
#   gateway        — stable/healthy
#   order-service  — slow-burn CPU stress (45→90% over 10 min)
#   payment-api    — error bursts every 2 min (8→43%)
#   cache-worker   — traffic spikes every 3 min (1200 req/s)
#   ml-inference   — noisy/calibrating new workload
```

**Send OTLP metrics directly:**

```bash
# Remote-write format
curl -X POST http://<node-ip>:31470/api/v2/write \
  -H "Content-Type: application/json" \
  -d '{
    "timeseries": [{
      "Labels": [
        {"Name": "__name__",   "Value": "cpu_percent"},
        {"Name": "host",       "Value": "default/Deployment/my-app"},
        {"Name": "namespace",  "Value": "default"},
        {"Name": "deployment", "Value": "my-app"}
      ],
      "Samples": [{"Value": 72.5, "Timestamp": '$(date +%s%3N)'}]
    }]
  }'

# OTLP JSON
curl -X POST http://<node-ip>:31470/otlp/v1/metrics \
  -H "Content-Type: application/json" \
  -d '{"resourceMetrics":[{"resource":{"attributes":[
    {"key":"k8s.deployment.name","value":{"stringValue":"my-app"}},
    {"key":"k8s.namespace.name","value":{"stringValue":"default"}}
  ]},"scopeMetrics":[{"metrics":[
    {"name":"process.cpu.utilization","gauge":{"dataPoints":[
      {"timeUnixNano":"'$(date +%s%N)'","asDouble":0.72}
    ]}}
  ]}]}]}'
```

---

## Repository Layout

```
workdir/                  Go source (v7.0.4)
  cmd/ruptura/            Engine binary
  cmd/ruptura-ctl/        CLI tool
  internal/
    analyzer/             10-signal KPI computation + calibration
    api/                  REST API (44 endpoints)
    actions/              Action engine + K8s actuator + safety gates
    correlator/           Burst detector + topology builder
    explain/              Narrative engine + fingerprinting
    fusion/               FusedR compositor (metricR + logR + traceR)
    history/              Time-series history manager
    ingest/               OTLP + Prometheus remote-write receivers
    pipeline/             5-model anomaly ensemble
    predictor/            HealthScore forecast
    storage/              BadgerDB + TTL GC

ui/                       Svelte 4 dashboard (v7.0.4)
  src/routes/             Fleet, Map, Engine, Alerts, Nodes, Settings
  src/components/         WorkloadCard, TopologyMap, NavBar, modals

helm/                     Helm chart (OCI: ghcr.io/benfradjselim/charts/ruptura)
scripts/
  simulate.py             Workload simulator (5 behavioral profiles)
operator/                 Kubernetes operator (ruptura-operator v0.6.9)
```

---

## Roadmap

```
v7.0.4  ✅  OTLP NodePort 31470 · workload simulator
v7.0.3  ✅  JSON crash fix · PNG logo · topology edge click · per-workload health scores
v7.0.2  ✅  10-signal bars · light/dark mode · dataflow stats · all backend APIs wired
v7.0.1  ✅  ruptura-ui pod · calibration per workload · Settings + Alerts pages
v7.0.0  ✅  Separate UI pod · SSE stream · k8s metadata · node health view
v6.8.x  ✅  Log/trace counters · BadgerDB GC · embedded dashboard (pre-v7)

v7.1.0  ⏳  SLO config UI · dashboard layout customization · multi-tenant namespaces
v7.2.0  ⏳  Python SDK v2 · Grafana data source plugin
```

---

## CNCF

Ruptura targets alignment with CNCF sandbox criteria: Apache 2.0 license, open governance ([GOVERNANCE.md](GOVERNANCE.md)), documented security policy ([SECURITY.md](SECURITY.md)), public roadmap.

---

## License

Apache 2.0

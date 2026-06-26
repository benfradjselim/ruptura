# Ruptura — Technical Reference
**Version:** 8.0.0 · **Updated:** 2026-06-26

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
v7 (3-signal, default when infraR absent):
  FusedR = 0.6 × metricR + 0.2 × logR + 0.2 × traceR
  2-signal fallback: FusedR = 0.75 × metricR + 0.25 × other

v8 (4-signal, when infraR is available from infra collector):
  Base weights: metric=0.42, log=0.14, trace=0.14, infra=0.30
  Active subset renormalised to sum to 1.0.
  When infraR is absent or stale: automatically falls back to v7 split —
  the 0.30 infra weight is never left dangling.
```

### HealthScore
```
penalty = w_stress·stress + w_fatigue·fatigue + w_mood·(1−mood)
        + w_pressure·pressure + w_humidity·humidity + w_contagion·contagion
        + w_infraStress·infraStress + w_networkHealth·(1−networkHealth)

HealthScore = 100 × clamp(1 − penalty, 0, 1)

Weights are per-workload overrides (SignalWeights); defaults sum to 1.0.
infraStress and networkHealth default to weight=0 when not configured,
preserving bit-for-bit v7 behavior on clusters without the infra collector.
```

### Extended Contagion (v8)
```
contagion = max(trace_topology_contagion, cgpm_prop_pressure)
            clamped to [0, 1]

PropPressure is the CGPM-computed downstream pressure delivered to
grp.workload for the workload's namespace. It can only increase the
existing contagion — never decrease it.
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

## v8 Infrastructure & Cluster Intelligence Layer

### Dual-Axis Object Identity

Every watched Kubernetes object is identified on two orthogonal axes:

| Axis | Dimension | Values |
|------|-----------|--------|
| A — topology | `Scope` | `cluster` · `namespace` · `workload` · `pod` |
| B — domain   | `Group` | `grp.controlplane` · `grp.operators` · `grp.admission` · `grp.tenancy` · `grp.storage` · `grp.network` · `grp.workload` |

Signals aggregate: object → `GroupSnapshot` (per group per namespace) → `NamespaceSnapshot` → workload KPI.

### Infra Collectors

| Collector | Kubernetes resources watched | Probe guard |
|-----------|------------------------------|-------------|
| `node` | `api/v1/nodes` | always active |
| `co` | `apis/config.openshift.io/v1/clusteroperators` | OpenShift only |
| `mcp` | `apis/machineconfiguration.openshift.io/v1/machineconfigpools` | OpenShift only |
| `networking` | Services, Endpoints, NetworkPolicy + Routes/Ingresses | Routes probed at startup |
| `storage` | PVC, PV, StorageClass | always active |
| `admission` | PolicyReport (wgpolicyk8s.io/v1alpha2 or kyverno.io/v1), ValidatingWebhookConfiguration | Kyverno only — silently skipped if absent |
| `operator` | Subscription, CSV, InstallPlan, CRD | OLM only — silently skipped if absent |
| `tenancy` | ResourceQuota, LimitRange, Namespace | always active |

Non-in-cluster deployments: all constructors return an error → zero collectors added → registry is a safe no-op. All API endpoints return empty results with HTTP 200.

### Cross-Group Propagation Model (CGPM)

```
PropPressure(t) = clamp( max over upstream edges g→t of:
    effectiveA(g) · ω(g→t) · (1 + κ · GNI(g))
, 0, 1 )

effectiveA(g) = max( activation(g), PropPressure(g) )   — multi-hop
activation(g) = 1 − GroupHealth(g)
κ (noise amplification) = 0.5
θ_blast (blast radius threshold) = 0.2
```

CGPM edge graph (canonical, read-only):
```
controlplane → workload  ω=1.0   node/CO failure breaks hosted pods
controlplane → network   ω=0.9   network-operator CO failure breaks routing
network      → workload  ω=0.9   endpoint/route failure makes service unreachable
storage      → workload  ω=0.8   PVC stall blocks pod start
admission    → workload  ω=0.7   admission denial blocks pod creation
admission    → network   ω=0.6   admission blocks route/service creation
operators    → storage   ω=0.6   CSI operator failure breaks provisioning
operators    → network   ω=0.6   network operator failure breaks routes
operators    → workload  ω=0.5   operator-managed workload reconcile loop
```

Processing order: controlplane → operators → admission → tenancy → storage → network → workload.
Tick rate: 30 seconds. GNI=0 in v8.0 (Phase 6); full GNI wiring in v8.1.

### Infra Signal Catalog (v8)

**GroupSnapshot fields** (per group per namespace):

| Field | Formula | Meaning |
|-------|---------|---------|
| `health` | `1 − max(object signals)` | Group health in [0,1]; 1.0 = all healthy |
| `spread` | `mean(object signals)` | Localized vs. widespread fault |
| `gni` | `0.5·StateChurn + 0.5·EventBurst` | Group Noise Index (v8.0: always 0; v8.1: wired) |
| `agitated` | `GNI elevated ∧ health still green` | Pre-rupture warning |

**NamespaceSnapshot fields** (consumed by workload analyzer):

| Field | Source | Default (nil registry) |
|-------|--------|----------------------|
| `infraStress` | `max(1 − GroupHealth)` across all groups | 0.0 |
| `networkHealth` | `GroupHealth(grp.network)` | 1.0 |
| `storageRisk` | `1 − GroupHealth(grp.storage)` | 0.0 |
| `admissionPressure` | `1 − GroupHealth(grp.admission)` | 0.0 |
| `propPressure` | CGPM result for `grp.workload` in namespace | 0.0 |

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

v8 additive prefixes (infra collector — not yet written to storage in v8.0; reserved for v8.1):
```
is:{group}:{ns}:{ts_ns}         infra signal raw       TTL 7d
is5:{group}:{ns}:{ts_ns}        5m infra rollup        TTL 35d
is1h:{group}:{ns}:{ts_ns}       1h infra rollup        TTL 400d
gh:{group}:{ns}:{ts_ns}         GroupHealth history    TTL 35d
gni:{group}:{ns}:{ts_ns}        GNI history            TTL 35d
prop:{ns}:{ts_ns}               PropPressure history   TTL 35d
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

### v8 Infrastructure Endpoints

```
GET  /api/v2/infra/groups      all GroupSnapshots (all groups, all namespaces)
GET  /api/v2/infra/nodes       Node signals (Kind=Node, grouped by ObjectID)
GET  /api/v2/infra/mcp         MachineConfigPool signals (OpenShift)
GET  /api/v2/infra/operators   ClusterOperator signals (OpenShift)
GET  /api/v2/infra/network     Network health per namespace (grp.network)
GET  /api/v2/infra/storage     Storage health per namespace (grp.storage)
GET  /api/v2/infra/admission   Admission health per namespace (grp.admission)
GET  /api/v2/infra/tenancy     Tenancy health per namespace (grp.tenancy)
GET  /api/v2/propagation       CGPM PropPressure results per namespace
```

All infra endpoints return empty results (`{"groups":[]}` or `{"signals":[]}`) with HTTP 200 when the infra collector registry has no active collectors (non-in-cluster deployments).

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
  internal/collector/infra/   v8 infra collectors (node, co, mcp, networking, storage, admission, operator, tenancy)
  internal/collector/infra/dag/  Registry + CGPM Propagator
  ui/                         ACTIVE Svelte 4 UI (workdir/ui/)
docs/                         internal planning + specs
  REFERENCE.md                this file
  v6.1.0/SPECS.md             formal technical specifications
  judgment.md                 design conscience document
helm/                         Helm chart for community edition
site/                         MkDocs source → GitHub Pages
scripts/simulate.py           workload simulator for demos
```

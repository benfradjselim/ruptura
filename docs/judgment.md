# Ruptura — Solution Judgment

> This file is the design conscience of the project.
> Read it before planning any new version. Update it after every version ships.
> It captures what we decided, what we discovered, and what must be resolved.
> The habit: **build → optimize → judge → rebuild based on judgment**.

---

## The User Contract — What a Production User Expects

This section defines the experience a user must have after installing Ruptura. Every engineering decision is measured against it. If a feature does not contribute to one of these outcomes, it should not be built.

---

### Who installs Ruptura

An SRE or platform engineer at a company with a Kubernetes cluster of 20–300 microservices. They already have Prometheus and Grafana. They may have Datadog. They have had at least one incident where a service degraded slowly over days and the Prometheus threshold alert only fired when users were already affected. They are tired of being paged for things that were visible for hours before they broke. They want to act before the break, not after.

---

### What they expect on Day 1

```
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --set autodiscovery.enabled=true
```

Within 15 minutes, without configuring a single workload manually, they should see:

- A list of every Deployment and StatefulSet in their cluster
- A HealthScore per workload (0–100)
- A state label per workload: excellent / good / fair / poor / critical
- Which 3 workloads need attention right now

They should NOT be asked to annotate pods, write OTel configs per service, or define alert rules. Auto-discovery is the entry ticket.

---

### What they expect on Day 3 (baselines established)

After 24–48h of observation per workload, the signals become relative to each workload's normal. Now:

- A batch job that always runs at 0.9 CPU is no longer "stressed"
- An API server that normally runs at 0.1 CPU and is now at 0.4 gets flagged
- Fatigue starts to accumulate for workloads that have been under sustained stress

The user opens the dashboard in the morning and sees:

> **payment-api** — HealthScore 61 (declining)
> Fatigue: 0.67 (exhausted) — building for 3 days
> Velocity: accelerating
> **Estimated time to rupture: 4–8 hours at current rate**
> Recommended: rolling restart to relieve fatigue

They have not been paged. They have not had an incident. They are acting on a prediction, not a post-mortem.

---

### What they expect during a real incident

An alert fires: **checkout-service ruptured (R = 4.8, critical)**

Ruptura tells them — without them having to open Grafana or look at a single raw metric:

> **Root cause chain:**
> 1. `payment-db` entered epidemic state at 14:32 (timeout rate 8× baseline)
> 2. `payment-api` received contagion from `payment-db` via the `payment-api → payment-db` call edge at 14:45 (weighted propagation R=2.1)
> 3. `payment-api` fatigue was already at 0.71 (3-day accumulation). The contagion pushed FusedR to 4.8 at 15:00.
> 4. `checkout-service` depends on `payment-api` and entered critical state at 15:04.
>
> **This is a cascade, not an isolated failure. Fix `payment-db` first.**
>
> T1 action queued: restart 1 pod of `payment-api` (auto-executing in 30s)
> T2 action pending your approval: scale `payment-db` by +2 replicas

The SRE immediately understands what happened, in what order, and what the right first action is. They did not need to correlate 5 dashboards. They did not need to ask "is this related to that?"

---

### What they expect on an ordinary Tuesday deploy

They deploy a new version of `order-processor` at 14:30.

Ruptura sees the deploy event (context entry), enters a 10-minute suppression window for that workload, and does not fire any alarms during the rollout. After the window, it compares the new baseline against the pre-deploy baseline and reports:

> **order-processor** post-deploy health: HealthScore 88 (+3 vs pre-deploy)
> Fatigue reset by rolling restart. Pressure normalized.
> No action required.

No false alarms. No alert fatigue.

---

### What they expect after 1 month

- 3 ruptures predicted and avoided (T1 auto-restart, no incident)
- 1 real incident with full causality chain in the explain report
- 0 false alarms during planned maintenance windows
- The on-call rotation has fewer 3am pages

The business case is: Ruptura pays for itself in avoided incidents and reduced on-call burden.

---

### What Ruptura must never do

- Page for something healthy under normal load (false positive from global thresholds)
- Stay silent while a workload slowly degrades toward rupture
- Show "host-123 CPU 78%" instead of "payment-api is exhausted"
- Require 2 weeks of manual OTel instrumentation before showing anything useful
- Produce an unexplained number without telling the user what caused it

---

### The single sentence that defines success

> Ruptura tells you which workload will break next, why it is breaking, and what to do about it — before your users notice.

Every feature either contributes to this sentence or it should not be built.

---

## How to Use This File

Before starting work on any version:
1. Read every open judgment under the current version section.
2. Resolve or explicitly defer each one before adding new features.
3. After shipping, add a new version section and document what changed and what new gaps emerged.

---

## Treatment Unit — Fundamental Modeling Decision

### The Problem

Ruptura currently treats `host` (a node/machine name) as the atomic unit of observation. Every model, every KPI, every rupture index, every API route is keyed by `host`:

```go
// pkg/models/models.go
type Metric struct { Host string }
type KPI    struct { Host string }
type KPISnapshot struct { Host string }

// internal/analyzer/analyzer.go
hosts map[string]*hostState   // key = node hostname

// internal/ingest/engine.go
host := rm.Resource.GetAttr("host.name")  // only attribute extracted
```

In a Kubernetes or OpenShift cluster this is the **wrong unit** for the user. A single `payment-api` Deployment with 5 replicas spreads across 5 nodes. Ruptura produces 5 disconnected host-level health scores. The user asking "is my payment-api workload healthy?" gets nothing useful.

### The Right Treatment Hierarchy

```
Cluster
  └── Namespace          (tenant / team boundary)
        └── Workload     (Deployment / StatefulSet / DaemonSet) ← PRIMARY user unit
              └── Pod    (instance, mostly for debugging)
                    └── Node/Host  (infra layer, secondary)
```

The **primary treatment unit must be Workload** (or Service when traces are the source). Node/host remains relevant for infrastructure-level signals (node CPU, node memory, disk pressure) but must not be the key for application health.

### Root Cause in Code

OTLP resource attributes already carry all the information needed. The OTel semantic conventions define:

| Attribute | Meaning |
|-----------|---------|
| `k8s.namespace.name` | Namespace |
| `k8s.deployment.name` | Deployment name |
| `k8s.statefulset.name` | StatefulSet name |
| `k8s.daemonset.name` | DaemonSet name |
| `k8s.pod.name` | Pod instance |
| `k8s.node.name` | Node (same as host.name in K8s) |
| `service.name` | Logical service (used in traces) |

The ingest engine discards all of these except `host.name` and `service.name`. The namespace, workload name, and workload kind are thrown away at parse time and can never be recovered downstream.

### GAP-10 — Treatment Unit is Infra-Only (CRITICAL, blocks v6.2 usefulness)

**What must change:**

#### 1. New core type: `WorkloadRef`

```go
// pkg/models/workload.go
type WorkloadRef struct {
    Cluster   string // optional, defaults to "default"
    Namespace string
    Kind      string // Deployment | StatefulSet | DaemonSet | Job | bare-pod
    Name      string // workload name
    Node      string // infra node — still tracked, different dimension
}

func (w WorkloadRef) Key() string {
    // canonical key used everywhere as the treatment unit
    return w.Namespace + "/" + w.Kind + "/" + w.Name
}

func (w WorkloadRef) NodeKey() string {
    return w.Node
}
```

#### 2. Enrich `Metric`, `KPI`, `KPISnapshot`

```go
type Metric struct {
    Name      string
    Value     float64
    Timestamp time.Time
    Labels    map[string]string
    Host      string       // kept: infra node
    Workload  WorkloadRef  // NEW: logical workload identity
}

type KPISnapshot struct {
    Workload  WorkloadRef  // replaces Host as primary key
    Host      string       // still present for node-level drill-down
    // ... all signals unchanged
}
```

#### 3. OTLP ingest must extract K8s attributes

```go
// ingest/engine.go — current
host := rm.Resource.GetAttr("host.name")

// required
ref := WorkloadRef{
    Namespace: rm.Resource.GetAttr("k8s.namespace.name"),
    Name:      firstNonEmpty(
        rm.Resource.GetAttr("k8s.deployment.name"),
        rm.Resource.GetAttr("k8s.statefulset.name"),
        rm.Resource.GetAttr("k8s.daemonset.name"),
        rm.Resource.GetAttr("service.name"),   // fallback for non-K8s
    ),
    Kind: inferKind(rm.Resource),
    Node: firstNonEmpty(rm.Resource.GetAttr("k8s.node.name"), rm.Resource.GetAttr("host.name")),
}
```

#### 4. Analyzer keyed by WorkloadRef, not host string

The analyzer's `map[string]*hostState` becomes `map[string]*workloadState` where the key is `WorkloadRef.Key()`. When multiple pods from the same Deployment send metrics, they are merged into a single workload snapshot using these aggregation rules:

| Signal | Aggregation across pods | Rationale |
|--------|------------------------|-----------|
| Stress | max | worst pod defines workload stress |
| Fatigue | max | accumulated burden follows the most fatigued pod |
| Mood | min | workload mood is as low as the saddest pod |
| Pressure | max | highest pressure pod sets the alarm |
| Humidity | mean | spread errors across all pods |
| Contagion | max | if any pod is contagious, the workload is |
| Resilience | min | weakest pod limits overall resilience |
| Entropy | mean | disorder is a workload-wide property |
| Velocity | mean | aggregate rate of change |
| HealthScore | min | weakest pod governs workload health |

#### 5. API routes must reflect the hierarchy

Current (wrong):
```
GET /api/v2/rupture/{host}
```

Required:
```
GET /api/v2/rupture/{namespace}/{workload}          ← primary
GET /api/v2/rupture/{namespace}                     ← namespace rollup
GET /api/v2/rupture                                 ← cluster-wide
GET /api/v2/infra/node/{node}                       ← infra layer, separate
```

#### 6. Backward compatibility for non-K8s users

When OTLP attributes carry no K8s labels (bare-metal, VMs, standalone Docker), `WorkloadRef` degrades gracefully:
```
Namespace = "default"
Kind      = "host"
Name      = host.name value
Node      = host.name value
```
This means existing integrations continue working with the same host-level view — they just live under `default/host/{hostname}` instead of the top level.

### Impact on Roadmap

This gap **must be resolved before v6.2 ships**. The web dashboard v2, the multi-tenant feature (X-Org-ID maps to Namespace), and the CLI all depend on meaningful workload-level output. Building a dashboard over host-level signals in a K8s context produces a tool that no Kubernetes user will adopt.

---

## Signal Inventory

### Analyzer KPIs (internal/analyzer — 10 signals)

| Signal | Formula basis | State labels | Wired to HealthScore |
|--------|--------------|--------------|----------------------|
| Stress | weighted cpu+ram+latency+errors+timeouts | calm/nervous/stressed/panic | YES (0.25) |
| Fatigue | dissipative accumulation of stress | rested/tired/exhausted/burnout | YES (0.20) |
| Mood | log-normalized uptime×throughput / errors×timeouts×restarts | happy/content/neutral/sad/depressed | YES (0.20) |
| Pressure | dS/dt + ∫errors dt | improving/stable/rising/storm_approaching | YES (0.15) |
| Humidity | (errors × timeouts) / throughput | dry/humid/very_humid/storm | YES (0.10) |
| Contagion | errors × cpu (proxy, not graph-based) | low/moderate/epidemic/pandemic | YES (0.10) |
| Resilience | mood × (1−fatigue) × (1−contagion) | robust/stable/fragile/critical | NOT in HealthScore |
| Entropy | MAD of rolling HealthScore history | ordered/fluctuating/chaotic/turbulent | NOT in HealthScore |
| Velocity | rate of change of HealthScore | steady/shifting/accelerating/volatile | NOT in HealthScore |
| HealthScore | weighted composite of above | excellent/good/fair/poor/critical | IS the output |

### Composites Package (pkg/composites + internal/composites/engine — 7 signals)

A second, parallel set of composites with different formulas:

| Signal | Key difference vs Analyzer |
|--------|---------------------------|
| Stress | same 5 factors, same weights |
| Fatigue | simpler — no dt scaling |
| Pressure | z-scored EWMA of latency+error, not derivative |
| Contagion | graph-based (edges between services) — more accurate |
| Resilience | ring buffer mean stress inversion |
| Entropy | Shannon entropy of variance slice |
| Sentiment | log(nPos+1) − log(nNeg+1) |

**HealthScore in composites only uses: stress(0.35), fatigue(0.25), pressure(0.25), contagion(0.15).**
Resilience, Entropy, Sentiment are computed but excluded from the formula.

---

## Architecture Gaps

### GAP-01 — Dual Composite Engine (CRITICAL)

Two independent engines compute overlapping signals:
- `internal/analyzer/analyzer.go` — 10 KPIs, richer formulas, stateful
- `internal/composites/engine.go` — 7 signals, graph-based contagion, EWMA pressure

Neither is clearly the canonical one. The API (`handlers_extra.go`) uses neither directly — all rupture/KPI endpoints return empty stubs. Before v6.2, one engine must be designated authoritative and the other either merged or removed.

**Judgment**: `internal/analyzer` should be the canonical KPI engine. `internal/composites` has superior Contagion (graph-based) and Pressure (EWMA z-score) formulas — those specific formulas should be ported into the analyzer and the composites engine retired.

---

### GAP-02 — API Stubs (HIGH)

The following handlers return empty responses and are not implemented:

| Handler | Route(s) | What it should return |
|---------|---------|----------------------|
| handleRupture | GET /api/v2/rupture/{host} | KPI snapshot + rupture index |
| handleForecast | POST /api/v2/forecast, GET /api/v2/forecast/{metric}/{host} | Prediction result |
| handleActions | GET/POST /api/v2/actions, approve/reject/rollback | Action queue |
| handleSuppressions | GET/POST /api/v2/suppressions | Suppression list |
| handleExplain | GET /api/v2/explain/{id}/formula, /pipeline | Explanation chain |
| handleOTLP | POST /api/v2/v1/metrics, logs, traces | 204 (but real handler in ingest engine not wired) |
| handleKPI | GET /api/v2/kpi/{name}/{host} | Only accepts "stress", returns empty |

The web dashboard v2 and any external consumer cannot function until these are implemented.

---

### GAP-03 — Fusion Layer Never Wired (HIGH)

`internal/fusion/fusion.go` implements `FusedR` — combining metric R + log R + trace R into a single rupture index per host. The architecture is sound. But:

- Nothing calls `SetLogR()` or `SetTraceR()` in production flow.
- The BurstDetector in `internal/correlator` fires `BurstEvent` on log error/warn surges but its output channel is never consumed by the fusion engine.
- Trace spans are ingested but span latency/error status is never converted to a trace R value.

**Judgment**: The activation chain must be explicit:
1. Log ingestion → BurstDetector.Observe() → BurstEvent → logR derivation → fusion.SetLogR()
2. Trace ingestion → span error rate + P99 latency → traceR derivation → fusion.SetTraceR()
3. Metric pipeline → metric R from predictor/CAILR → fusion.SetMetricR()
4. fusion.FusedR() → canonical rupture index exposed by handleRupture

---

### GAP-04 — Anomaly Engine Not Wired to Actions (MEDIUM)

`internal/predictor/anomaly_engine.go` and `internal/pipeline/metrics/` both contain `AnomalyEngine` implementations (duplicated). Anomalies are detected but:

- They are not stored in the AnomalyStore in the production flow.
- They do not trigger the ActionEngine.
- The consensus scoring (≥2 methods = critical) is correct but never surfaced.

---

### GAP-05 — Throughput Collapse Blind Spot (MEDIUM)

None of the 10 signals captures a sudden drop in request rate. A service going silent (e.g., circuit broken upstream, crash-loop) will show low Stress and high Mood — both misleadingly healthy. A `Throughput` signal based on rate-of-change of `request_rate` (negative velocity) must be added.

---

### GAP-06 — In-Memory Only Storage (MEDIUM)

All KPI state, anomaly events, rupture history, and action queues live in memory. A pod restart loses all history. Before v6.3 at the latest:

- KPI snapshots must be persisted (at minimum to a local BoltDB/SQLite, ideally to object storage for cluster mode).
- Anomaly event ring buffer must have a durable overflow path.

---

### GAP-07 — No Grafana Dashboard Template (LOW)

Prometheus metrics are exported at `/api/v2/metrics` but there is no provisioned Grafana dashboard. Users cannot visualize out of the box. A dashboard JSON template (ruptura_overview.json) with the following panels is needed:

- HealthScore gauge per host (0–100, green/yellow/red bands)
- Stress + Fatigue time series (overlay)
- Rupture Index heatmap (hosts × time, colored by fused R)
- Anomaly event annotations
- Active action queue table

---

### GAP-08 — handleOTLP Route Disconnect (HIGH)

`api/router.go` mounts OTLP routes under `/api/v2/v1/{metrics,logs,traces}` pointing to `handleOTLP` in `handlers_extra.go`, which is a no-op stub. The real implementation exists in `internal/ingest/engine.go` (`handleOTLPMetrics`, `handleOTLPLogs`, `handleOTLPTraces`) mounted at `/otlp/v1/*`. These are two separate HTTP servers on different ports, which is fine, but the router stub misleads callers who POST to `/api/v2/v1/*` and get 204 with no processing.

**Judgment**: Either remove the `/api/v2/v1/*` stubs from the API router and document that OTLP goes to the ingest port, or wire the stubs to the real ingest engine. Do not maintain two routes that appear equivalent but behave differently.

---

### GAP-09 — Sentiment Signal Disconnected from Log Pipeline (MEDIUM)

`pkg/composites.Sentiment()` computes `log(nPos+1) − log(nNeg+1)` and is correct. But `nPos` and `nNeg` are only updated via `engine.UpdateSentiment()`, which is never called by the log ingest path. Log entries are parsed (level: error/warn/info) but their counts never flow into Sentiment.

**Activation**: `IngestLog()` in the sink implementations must call `UpdateSentiment()` after counting positive (info/debug) and negative (error/warn) lines per host per scrape window.

---

## Functional Requirements

These define what Ruptura must do. Validate against every version.

| ID | Requirement | Status |
|----|-------------|--------|
| FR-01 | Accept Prometheus remote-write, OTLP metrics/logs/traces, DogStatsD | Implemented |
| FR-02 | Compute 10 KPI signals per host in real time | Partial (analyzer exists, not wired to API) |
| FR-03 | Detect anomalies with ≥2-method consensus | Implemented (not wired to actions) |
| FR-04 | Forecast any metric at configurable horizon with confidence bands | Implemented (API stub) |
| FR-05 | Fuse metric + log + trace signals into a single Rupture Index per host | Partial (fusion engine exists, not wired) |
| FR-06 | Recommend and execute tiered actions (T1 auto, T2 suggest, T3 human) | Partial (engine exists, not wired to rupture events) |
| FR-07 | Expose all signals as Prometheus metrics for Grafana scraping | Partial (registry exists, signal registration incomplete) |
| FR-08 | Provide REST API for rupture index, forecasts, actions, suppressions | Partial (routes declared, most return stubs) |
| FR-09 | Emit structured explanations of why a rupture was scored | Stub |
| FR-10 | Support per-tenant isolation via X-Org-ID header | Not started (v6.2 target) |

---

## Non-Functional Requirements

| ID | Requirement | Target | Status |
|----|-------------|--------|--------|
| NFR-01 | Ingest throughput | ≥50,000 active series | Cardinality cap at 50k in ingest engine |
| NFR-02 | KPI computation latency | <100ms per host update | Not measured |
| NFR-03 | API response time (p99) | <200ms for GET endpoints | Not benchmarked |
| NFR-04 | Memory usage under max load | <512MB per pod | Not profiled |
| NFR-05 | No data loss on graceful shutdown | WAL or flush on SIGTERM | Not implemented |
| NFR-06 | API authentication | Bearer token (per-deployment) | Implemented |
| NFR-07 | Rate limiting on ingest | Max req/s configurable | Not implemented |
| NFR-08 | TLS on all external endpoints | Configurable cert/key | Partial (tls_test.go exists) |
| NFR-09 | Multi-arch Docker image | linux/amd64 + linux/arm64 | Implemented in release.yml |
| NFR-10 | Helm chart lint-clean | helm lint passes | Implemented |

---

## OT Signal Activation Contract

When a log or trace is ingested, it must follow this path to become an active input:

```
OTLP Log/Trace  ──► ingest/engine.go (IngestLog / IngestSpan)
                         │
                    ┌────▼──────────────────────────────┐
                    │  Correlator (BurstDetector)        │
                    │  - error/warn counts per service   │
                    │  - fires BurstEvent when σ > 3     │
                    └────────────────┬──────────────────┘
                                     │ BurstEvent
                    ┌────────────────▼──────────────────┐
                    │  Log R derivation                  │
                    │  r_log = burst.Rate / baseline     │
                    └────────────────┬──────────────────┘
                                     │
                    ┌────────────────▼──────────────────┐
                    │  fusion.SetLogR(host, r, ts)       │
                    │  fusion.FusedR(host) → R_fused     │
                    └────────────────┬──────────────────┘
                                     │ R_fused
                    ┌────────────────▼──────────────────┐
                    │  analyzer.UpdateMetrics / Sentiment│
                    │  actionEngine.Recommend(event)     │
                    └───────────────────────────────────┘
```

A log or trace is considered **active input** the moment it crosses the BurstDetector threshold and produces a non-zero `r_log` or `r_trace`. Below threshold, it is stored but passive.

---

## Visualization Contract

### Grafana (available now)

Scrape target: `GET /api/v2/metrics` (Prometheus format)

Expected metric names (to be confirmed once signal registration is complete):

| Metric name | Type | Labels |
|-------------|------|--------|
| `ruptura_health_score` | gauge | host |
| `ruptura_stress` | gauge | host |
| `ruptura_fatigue` | gauge | host |
| `ruptura_mood` | gauge | host |
| `ruptura_pressure` | gauge | host |
| `ruptura_contagion` | gauge | host |
| `ruptura_resilience` | gauge | host |
| `ruptura_entropy` | gauge | host |
| `ruptura_velocity` | gauge | host |
| `ruptura_rupture_index` | gauge | host |
| `ruptura_anomaly_total` | counter | host, method, severity |
| `ruptura_ingest_total` | counter | source |

### Web Dashboard v2 (v6.2 target — Svelte)

The dashboard consumes the REST API directly. Key views:

**Ruptura Index Heatmap** — Primary screen
- Grid: rows = hosts, columns = time (last 30m, 1h buckets)
- Cell color: fused R value (green=0–1, yellow=1–3, red=3+)
- Click cell → signal breakdown sidebar

**Host Detail Panel** — On host click
- HealthScore headline (0–100, large digit)
- 10 signals as horizontal bar gauges (value + state label)
- Anomaly timeline
- Forecast chart for top 3 degrading metrics

**Action Queue** — Side panel
- T1 actions (executed) with timestamps
- T2 actions (pending approval) with approve/reject buttons
- T3 actions (human-only, informational)

---

## Honest Completeness Judgment — "Is Ruptura truly useful after fixing the GAPs?"

### Short answer

No. Fixing all 10 GAPs makes Ruptura **correct** but not yet **useful**. There are four more things that determine whether a user adopts it or abandons it within a week. They are not implementation gaps — they are product gaps that no amount of wiring will solve without deliberate design decisions.

---

### What fixing the GAPs actually gives you

After GAP-01 through GAP-10 are resolved, you have:

- A connected pipeline from ingest → analyze → fuse → rupture index
- Workload-level signals (not node-level noise)
- A working REST API
- Anomaly detection that informs the action engine
- Log/trace signals contributing to FusedR
- An explain engine that returns metric contributions

This is the engine. It is technically sound. The ML ensemble (ILR + HoltWinters + ARIMA) is real, not fake. The CAILR dual-scale rupture detector is sophisticated. The fusion concept is architecturally correct. The action tier system with cooldown arbitration is well-designed.

But an engine with no drivetrain does not move.

---

### MISSING-01 — Adaptive Per-Workload Baselines (BLOCKS practical accuracy)

The alerter fires `stress_panic` at threshold `0.8` for every workload in the cluster. This is a global constant. In practice:

- A batch job runs at 0.9 CPU for 8 hours and is healthy.
- A latency-sensitive API server at 0.35 CPU is already degraded.
- A message queue at 0.6 memory is normal. A microservice at 0.6 memory is a leak.

Without learning what "normal" looks like per workload over its first 24-48h of operation, Ruptura produces:
- **False positives** for heavy workloads → alert fatigue → teams disable it
- **False negatives** for normally-light workloads degrading subtly → misses what matters

This is the primary reason observability tools get abandoned. The signals can be architecturally perfect and still be useless if the thresholds are global constants.

**What's needed**: After each workload's first observation window (configurable, default 24h), the analyzer should compute a rolling baseline per signal per workload. All thresholds become relative deviations from that baseline, not absolute values. This is the same principle Datadog uses for anomaly monitors.

---

### MISSING-02 — Narrative Explain, Not Just Numbers (DEFINES the differentiation)

The explain engine returns this:

```json
{
  "rupture_id": "r1",
  "r": 4.2,
  "contributions": [
    {"metric": "fatigue", "weight": 0.31, "pipeline": "metric"},
    {"metric": "contagion", "weight": 0.28, "pipeline": "trace"}
  ],
  "first_pipeline": "log"
}
```

This is data, not insight. The person receiving a PagerDuty alert at 3am gets a JSON blob with numbers. They still need to open Grafana, understand what fatigue=0.81 means, correlate with the deploy that happened Tuesday, and realize the contagion came from payment-db.

Ruptura's entire premise — modeling system health as fatigue, mood, pressure, velocity — is wasted if the output is a number dashboard. The differentiation only becomes real as a **narrative**:

> "payment-api has been accumulating fatigue for 72h (fatigue 0.81, burnout threshold 0.80). The Tuesday 14:30 deploy increased pressure to 0.74 (storm_approaching). At 16:45, a contagion wave from payment-db — which entered epidemic state — propagated via the payment-api→payment-db edge and pushed FusedR from 1.8 to 4.2 in 18 minutes. This is a cascade rupture, not an isolated spike. Recommended action: scale payment-api by 2 replicas and circuit-break the payment-db dependency."

That text is already computable from the data Ruptura holds. The fatigue history is there. The deploy timestamp can come from a context entry. The contagion edge will be there once topology is wired. The TTF is already in `FormulaAuditResponse`. This does not require an LLM — it is a structured template filled from the rupture record.

Without this, Ruptura is a prettier Prometheus. With it, Ruptura is genuinely different.

---

### MISSING-03 — Real Contagion from Trace Topology (MAKES contagion signal honest)

Contagion in the analyzer is `errors × cpu`. This is not contagion — it is a proxy that happens to correlate weakly with propagation events. The `TopologyGraph` and `ServiceEdge` models already exist in `pkg/models/trace.go`. The trace spans carry parent/child relationships that build the real dependency graph. But nothing connects them.

A contagion signal built on `errors × cpu` will fire on every high-traffic deploy. A contagion signal built on actual service edges will fire when payment-db's error rate propagates to payment-api through a real call path. The difference between noise and signal.

The composites engine already has the right formula (graph-based, weighted by edge weight). It just needs the topology populated from the trace correlator.

---

### MISSING-04 — Maintenance Windows / Suppressions (PRODUCTION ADOPTION GATE)

The suppression API exists as a stub. In practice: every deploy generates a rupture alarm. Teams trying Ruptura in production will see 50 false alarms during their first Tuesday deploy, decide the signal-to-noise ratio is unacceptable, and remove it. Suppression is not a feature — it is the gate between "evaluated in staging" and "adopted in production."

A suppression entry needs: workload ref + time window + optional signal filter. During the window, ruptures are recorded but not dispatched to the action engine or notifier.

---

### The HealthScore Formula Sensitivity Problem

A mathematical note, independent of the above. The multiplicative HealthScore formula collapses aggressively. A workload with 6 signals each at a moderate 0.4 (not alarming by any individual measure) yields:

```
100 × (1−0.25×0.4) × (1−0.20×0.4) × (1−0.20×0.4) × (1−0.15×0.4) × (1−0.10×0.4) × (1−0.10×0.4)
≈ 64.5
```

That reads as "fair" for a service that is genuinely fine. When stress is 0.6 and fatigue is 0.5 simultaneously (still below the "stressed" threshold), HealthScore drops to ~43 ("poor"). The formula amplifies co-occurring moderate signals in a way that does not match human intuition about system health. This needs empirical calibration against real workloads before the dashboard shows this number to users.

---

### What Ruptura's raison d'être actually requires

Ruptura was founded on the idea that systems have a lifecycle of degradation — they get tired, stressed, and eventually rupture — and that you can model this temporally and act before the break happens. The ML ensemble and the composite signals are the right tools for that. No existing commercial tool models fatigue accumulation over time the way Ruptura does.

For that premise to deliver value, these three things must be true simultaneously:

1. **The signals must be relative to each workload's normal** (MISSING-01)
2. **The rupture explanation must be a narrative, not a JSON** (MISSING-02)
3. **The contagion must reflect real service topology** (MISSING-03)

If those three are in place alongside the GAP fixes, Ruptura answers the question "why is my payment-api degrading, what is it doing to its dependencies, and what should I do?" in a way no other self-hosted tool does today. That is a genuine, defensible reason to exist.

---

### Revised priority order for v6.2

| Priority | Work item | Why this position |
|----------|-----------|-------------------|
| 1 | GAP-10: WorkloadRef treatment unit | Everything else depends on this model change |
| 2 | MISSING-01: Adaptive baselines | Without this, all signals are noise for diverse workloads |
| 3 | GAP-03 + GAP-08: Fusion wiring + OTLP route fix | Data must flow before you can display anything honest |
| 4 | GAP-02: Implement handleRupture, handleKPI, handleForecast | Users need to query state |
| 5 | MISSING-03: Topology-based contagion | Makes contagion a real signal |
| 6 | MISSING-04: Maintenance windows | Production adoption gate |
| 7 | MISSING-02: Narrative explain | The differentiator — save for when the data is correct |
| 8 | Web dashboard v2, ruptura-ctl, Python SDK | Surface layer — build on top of a correct foundation |

Building the dashboard before items 1–6 produces a UI that looks impressive and generates bad signal. Build the correct engine first. The dashboard is 2 weeks of work once the API is honest.

---

## Version Judgment Log

### v6.1.3 (shipped 2026-04-28)

**What shipped**: Full rename Kairo→Ruptura. Dockerfile fixed. Helm chart fixed. Docker image published to GHCR. SDK tagged at sdk/go/v6.1.3.

**What was NOT done that should have been**:
- API handlers remain stubs. Any external integration is broken.
- Fusion engine not wired.
- Dual composite engine problem unresolved.
- No Grafana dashboard template shipped.

**Judgment before v6.2 starts**:
1. **[GAP-10] Migrate treatment unit from host → WorkloadRef** — without this, no K8s user can make sense of the output. This is the highest-priority change.
2. Designate one composite engine as canonical (Analyzer + ported Contagion/Pressure from composites).
3. Wire fusion: metric R + log R + trace R → FusedR.
4. Implement at minimum: `handleRupture`, `handleKPI`, `handleForecast` (non-stub, using WorkloadRef routes).
5. Fix the OTLP route disconnect (GAP-08).
6. Add Throughput signal (GAP-05).

---

## Pre-Version Checklist

Before cutting any release tag, verify:

- [ ] GAP-10 resolved: WorkloadRef is the treatment unit, host is a secondary dimension
- [ ] All other GAPs in this file are resolved or explicitly deferred with a reason
- [ ] `go test -race ./...` passes clean
- [ ] `helm lint deploy/helm/ruptura/` passes
- [ ] API handler coverage: no stub returning `[]` or `{}` for a route that v6.2+ consumers depend on
- [ ] Prometheus metrics endpoint exports all 12 signal metric names
- [ ] At least one integration test exercises the full path: ingest → analyze → rupture API response
- [ ] This file updated with a new version judgment section

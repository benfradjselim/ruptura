# Ruptura — Living Reference

> Update this file at the end of every sprint block, before context limit.
> It is the source of truth for current component versions, key file locations,
> API contracts added in v7, and architectural decisions.

---

## Current Component Versions (v7 branch)

| Component | Version | Image / Binary |
|-----------|---------|----------------|
| ruptura (core engine) | v7.0.0 (in progress) | `ghcr.io/benfradjselim/ruptura:v7.0.0` |
| ruptura-ui (dashboard) | v1.0.0 (new — not yet built) | `ghcr.io/benfradjselim/ruptura-ui:v1.0.0` |
| ruptura-ctl (CLI) | v1.0.0 → v1.1.0 target | binary `ruptura-ctl-{os}-{arch}` |
| ruptura-operator | v0.6.9 → v0.7.0 target | `ghcr.io/benfradjselim/ruptura-operator:v0.7.0` |
| Helm chart | 0.7.6 → 0.8.0 target | `oci://ghcr.io/benfradjselim/charts/ruptura` |

Last updated: 2026-05-14 (Sprint 2 done — fusion state API, topology map, engine self-health page)

---

## Key File Map

### Core Engine (`workdir/`)

| Path | Purpose |
|------|---------|
| `cmd/ruptura/main.go` | Entry point, version constant, server setup |
| `cmd/ruptura-ctl/cmd/root.go` | CLI root, `CTLVersion` constant |
| `internal/analyzer/analyzer.go` | 10 KPI signals, 15s tick, adaptive baselines |
| `internal/analyzer/fingerprint.go` | Rupture fingerprinting (cosine similarity) |
| `internal/analyzer/business.go` | SLO burn, blast radius, recovery debt |
| `internal/ingest/engine.go` | OTLP/DogStatsD/Prometheus ingest, `ingestHook` |
| `internal/fusion/fusion.go` | metricR + logR + traceR → FusedR |
| `internal/correlator/` | BurstDetector, TopologyBuilder (service edges from traces) |
| `internal/api/router.go` | All HTTP route registrations |
| `internal/api/handlers_rupture.go` | GET /rupture, GET /kpi, forecast |
| `internal/api/handlers_actions.go` | Action approve/reject/emergency-stop, edition gate |
| `internal/api/handlers_engine.go` | (v7 new) fusion state, engine status, storage stats |
| `internal/storage/badger.go` | BadgerDB persistence layer |
| `internal/sim/` | ruptura-sim simulation patterns |
| `internal/telemetry/registry.go` | Prometheus metrics registry, `IncIngestTotal` |
| `internal/ui/static/index.html` | Embedded Alpine.js dashboard (v6 — to be removed in v7) |
| `internal/discovery/client.go` | In-cluster SA token + CA cert reader |
| `internal/discovery/watcher.go` | LIST+WATCH loop, backoff, 410 Gone re-list |
| `internal/discovery/informer.go` | `Informer.Run()` — 3 goroutines (Deployments/StatefulSets/DaemonSets) |
| `pkg/client/client.go` | Go SDK HTTP client |
| `pkg/models/models.go` | Core types: KPISnapshot, WorkloadRef, FusedRuptureIndex, WorkloadStatus constants |

### ruptura-ui (`ui/` — v7 new)

| Path | Purpose |
|------|---------|
| `ui/package.json` | Svelte + Vite project |
| `ui/src/lib/api.ts` | Typed API client for all REST endpoints |
| `ui/src/routes/` | SvelteKit pages: /, /map, /engine, /nodes |
| `ui/src/components/` | WorkloadCard, TopologyMap, EnginePanel, etc. |
| `ui/Dockerfile` | nginx multi-stage build |
| `ui/public/version.json` | `{"version":"1.0.0"}` — read by health checks |

### Infrastructure

| Path | Purpose |
|------|---------|
| `helm/Chart.yaml` | Chart version + appVersion |
| `helm/values.yaml` | All Helm defaults including `autodiscovery.enabled`, `ui.enabled`, `goMemLimit` |
| `helm/templates/deployment.yaml` | Core engine Deployment |
| `helm/templates/deployment-ui.yaml` | (v7 new) ruptura-ui Deployment |
| `helm/templates/rbac.yaml` | ServiceAccount + ClusterRole (autodiscovery RBAC added in v7) |
| `.github/workflows/release.yml` | Main release pipeline |
| `.github/workflows/release-ui.yml` | (v7 new) ruptura-ui build + push |
| `.github/workflows/operator-bump.yml` | (v7 new) auto-update operator bundle on app release |
| `.github/workflows/operator-smoke.yml` | (v7 new) operator smoke test CI |
| `operator/` | Kubernetes operator source |
| `bundle/` | OLM bundle (OperatorHub submission) |

---

## API Contract — v7 New Endpoints

All new endpoints require `Authorization: Bearer <api-key>` unless noted.

### Engine internals

```
GET /api/v2/engine/fusion/{namespace}/{kind}/{name}
→ { metric_r, log_r, trace_r, fused_r, dominant_pipeline, last_updated }

GET /api/v2/engine/status
→ { analyzer: {tick_interval_ms, last_tick_ago_ms, active_workloads, calibrating_workloads},
    ingest: {metrics_per_sec, logs_per_sec, traces_per_sec},
    actions: {pending_tier1, pending_tier2, executed_last_hour},
    version, edition, uptime_seconds }

GET /api/v2/engine/storage
→ { badger: {disk_bytes, keys, vlog_size_bytes, reads_last_min, writes_last_min} }
```

### Topology

```
GET /api/v2/topology
→ { nodes: [{id, health_score, fused_r, state}],
    edges: [{source, target, call_rate, error_rate, p99_latency_ms}] }
```

### Nodes

```
GET /api/v2/nodes
→ [{ name, cpu_pct, memory_pct, disk_pressure, workload_count, worst_fused_r }]

GET /api/v2/nodes/{node}
→ { name, workloads: [{ref, health_score, fused_r}] }
```

### Kubernetes metadata (requires autodiscovery)

```
GET /api/v2/workloads/{namespace}/{kind}/{name}/k8s
→ { replicas: {desired, ready, available}, image, resources, pods, labels, last_deploy }
```

### Auto-discovery workload status values

```
"pending_telemetry"  — known from k8s API, no metrics received yet
"calibrating"        — metrics received, baseline not yet established
"active"             — baseline established, rupture detection enabled
"removed"            — workload deleted from k8s (historical only)
```

### Event streaming (SSE)

```
GET /api/v2/events?namespace=production&min_fused_r=1.5
Content-Type: text/event-stream

data: {"type":"rupture","workload":"production/Deployment/payment-api","fused_r":2.8,"state":"warning","ts":"..."}
data: {"type":"recovery","workload":"...","fused_r":1.1,"ts":"..."}
data: {"type":"heartbeat","ts":"..."}   (every 30s)
```

### Forecast change

```
GET /api/v2/forecast/health_score/{namespace}/{workload}
→ { ... existing fields ..., "confidence_window": 45 }
```
`confidence_window` = number of observations the OLS regression is based on.
When < 60, the UI must label the forecast "low confidence" and suppress ETAs beyond 30 min.

---

## v7 Sprint Status

| Sprint | Item | Status |
|--------|------|--------|
| S1 | GAP-V7-04 Auto-discovery | [x] **done** — `internal/discovery/`, `analyzer.RegisterWorkload`, `handleFleet` merge, Helm env |
| S1 | S1-2 ruptura-ui scaffold | [x] **done** — `ui/` Svelte 4+Vite, nginx proxy, Helm `ui.enabled`, CI `release-ui.yml` |
| S1 | MISSING-05 Read-write dashboard | [x] **done** — SuppressionModal (create/list/delete), WeightsModal (inline-edit all rows + save), wired into Fleet toolbar |
| S1 | MISSING-06 HealthScore/FusedR UX | [x] **done** — `confidence_window` in `HealthForecast`; `fused_r`+`health_forecast` in fleet response; rupture-warning banner in `WorkloadCard` |
| S2 | MISSING-07 Fusion state API | [x] **done** — `fusion.StateByWorkload`, `GET /api/v2/engine/fusion/{ns}/{kind}/{name}`, wired in main; CI matrix workflow `ui-components.yml` |
| S2 | GAP-V7-01 Topology map | [x] **done** — `GET /api/v2/topology` (nodes+edges from TopologyBuilder); `TopologyMap.svelte` Cytoscape.js force-directed, side panel, rupture highlight |
| S2 | MISSING-08 Engine self-health | [x] **done** — `GET /api/v2/engine/status` + `GET /api/v2/engine/storage`; Engine.svelte with runtime, analyzer, ingest bars, action queue, BadgerDB cards + footer |
| S3 | GAP-V7-02 K8s workload metadata | [ ] not started |
| S3 | GAP-V7-03 Node health view | [ ] not started |
| S3 | GAP-OP-01 Operator bundle CI | [ ] not started |
| S3 | GAP-OP-02 Operator smoke test | [ ] not started |
| S4 | MISSING-09 SSE + SDK Watch/Wait | [ ] not started |
| S4 | FR-10 Multi-tenant (deferred) | [ ] deferred |

---

## Architectural Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-05-13 | Split core + ruptura-ui into separate pods | Embedded 71KB file blocks all UX improvements |
| 2026-05-13 | Auto-discovery via k8s informer, not annotation-based | Zero-config Day 1 is the entry ticket |
| 2026-05-13 | Topology map uses Cytoscape.js | Better graph layout control than D3 for this data shape |
| 2026-05-13 | SSE not WebSocket for event streaming | Simpler, HTTP/1.1 compatible, unidirectional is sufficient |
| 2026-05-13 | `pending_telemetry` as third workload state | Distinct from calibrating — no data ≠ learning |
| 2026-05-09 | GOMEMLIMIT uses MiB/GiB suffix strings | Raw bytes rendered as scientific notation by Helm → Go panic |
| 2026-04-30 | ruptura-ctl versioned independently from server | Operator/CLI lifecycle is different from engine |

---

## Lab / Deployment Info

| Item | Value |
|------|-------|
| Kamatera k3s node | 185.229.225.115 |
| Namespace | ruptura-system |
| API NodePort | 31468 |
| OTLP ClusterIP | 10.43.118.33:4317 |
| Dashboard | http://185.229.225.115:31468/ui/ |
| GitHub Pages docs | https://benfradjselim.github.io/ruptura/ |
| GHCR | ghcr.io/benfradjselim/ruptura |

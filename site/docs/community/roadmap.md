# Roadmap

## Released

### v7.0.4 — 2026-05-15 ✅

| Item | Detail |
|------|--------|
| **OTLP NodePort** | OTLP ingest service exposed as NodePort 31470 — send telemetry directly without port-forwarding |
| **Workload simulator** | `scripts/simulate.py` — 5 behavioral profiles (stable, CPU stress, error bursts, traffic spikes, calibrating) injected every 5s via `/api/v2/write` |

### v7.0.3 — 2026-05-15 ✅

| Item | Detail |
|------|--------|
| **JSON crash fix** | `get()` helper in api.ts reads text before parsing — empty body no longer crashes Fleet/Topology |
| **Real PNG logo** | SVG gradient replaced with actual PNG in NavBar — renders correctly in all browsers |
| **Topology overhaul** | Explanation banner, edge click panel (call rate / error rate / P99 latency), better empty state |
| **Per-workload health scores** | WorkloadCard now reads per-snapshot KPI values — each card shows its own health ring |

### v7.0.2 — 2026-05-15 ✅

| Item | Detail |
|------|--------|
| **10-signal mini-bars** | WorkloadCard shows all 10 KPI signals (stress → throughput) as color-coded bars |
| **Light/dark mode** | Theme toggle in NavBar, persisted to localStorage, CSS variables across all components |
| **Live Data Flow** | Engine view shows cumulative log/metric/trace counters with proportional stacked bar |
| **All backend APIs wired** | TopologyMap, Settings ingest stats, NodeHealth all pulling from real endpoints |

### v7.0.1 — 2026-05-15 ✅

| Item | Detail |
|------|--------|
| **ruptura-ui pod** | Svelte 4 dashboard deployed as separate pod, nginx proxies `/api/` to engine, injects Bearer token |
| **Logo** | Ruptura logo in NavBar and Settings About section |
| **Calibrating state** | WorkloadCard shows calibration progress bar and "calibrating" badge |
| **Settings & Alerts pages** | Ingest Stats, data source config, active/resolved alert feed |

### v7.0.0 — 2026-05-15 ✅

| Item | Detail |
|------|--------|
| **v7 architecture** | Two-pod model: ruptura-engine (Go binary) + ruptura-ui (Svelte 4 + nginx) |
| **SSE live event stream** | `GET /api/v2/events` — real-time rupture/recovery events, live counter in Fleet |
| **K8s workload metadata** | Pod list, replicas, resources, labels under Fleet → Kubernetes tab |
| **Node health view** | Nodes page showing CPU, memory, disk pressure per K8s node |

### v6.8.13 — 2026-05-13 ✅

| Item | Detail |
|------|--------|
| **Log/trace ingest counters** | `/api/v2/dataflow` endpoint exposes cumulative metrics/logs/traces totals |
| **Live Data Flow** | Engine dashboard section showing ingest throughput |
| **ruptura-ctl v1.0.0** | CLI companion — health, status, workload queries, kubectl plugin support |

### v6.7.0 — 2026-05-06 ✅

| Item | Detail |
|------|--------|
| **Embedded web dashboard** | Self-contained Svelte UI served by the Go binary — no external dependencies. Chart.js and Alpine.js vendored locally; font loading non-blocking with noscript fallback. Works fully in air-gapped environments. |
| **Dashboard panels** | Fused Rupture Index heatmap, per-workload signal timelines, action log with approve/reject/emergency-stop, narrative explain panel, SLO widget, health forecast. |
| **Air-gap safe** | All assets (`vendor/alpine.min.js`, `vendor/chart.min.js`, embedded PNG logo) served from the binary via `go:embed`. No CDN required. |

### ruptura-operator v0.6.9 — 2026-05-07 🔄

Red Hat OperatorHub certification pipeline running.

| Item | Detail |
|------|--------|
| **UBI9 base image** | Both `ruptura` and `ruptura-operator` images switched from `gcr.io/distroless/static-debian12:nonroot` to `registry.access.redhat.com/ubi9/ubi-micro` — satisfies Red Hat preflight `BasedOnUBI` check. |
| **Required Red Hat labels** | `name`, `vendor`, `version`, `release`, `summary`, `description` labels added to both images — satisfies `HasRequiredLabel` preflight checks. |
| **Default app image bump** | CSV default app image updated to `ruptura:v6.7.0`. |
| **Build arg wiring** | CI workflows now pass `VERSION` build-arg so the `version` label reflects the actual image tag at build time. |

### ruptura-operator v0.6.8 — 2026-05-07 ✅

OperatorHub PR merged: https://github.com/k8s-operatorhub/community-operators/pull/8070

| Item | Detail |
|------|--------|
| **Fix: ServiceAccount never created** | Operator used `serviceAccountName: ruptura-instance` in the Deployment but never created the SA. Every Pod would fail to schedule. Fixed: `reconcileServiceAccount()` added to the reconcile loop; SA deleted in `cleanup()`. |
| **Fix: RBAC missing `serviceaccounts` verb** | ClusterRole now grants `create/update/patch/delete` on `serviceaccounts`. |
| **OLM upgrade graph** | `replaces: ruptura-operator.v0.6.7` added to CSV — existing installations upgrade cleanly. |
| **Prometheus metrics** | `/metrics` + `/healthz` on `:9090`; `ruptura_instances_total` + `ruptura_reconcile_errors_total` gauges. |

### ruptura-operator v0.6.7 — 2026-05-07 ✅

First OperatorHub release, merged into community-operators.

| Item | Detail |
|------|--------|
| `RupturaInstance` CRD | Manages Deployment + Service + PVC + ServiceAccount per instance |
| OpenShift support | Route with edge TLS termination when running on OpenShift |
| Finalizer cleanup | `ruptura.io/cleanup` finalizer ensures owned resources are deleted before CR removal |
| OLM bundle | Correct dot-notation annotation keys; `stable` and `alpha` channels |

### v6.6.3 — 2026-05-06 ✅

| Item | Detail |
|------|--------|
| Security: timing-safe auth | Bearer token comparison uses `crypto/subtle.ConstantTimeCompare` — eliminates timing-oracle on the API key. |
| Security: auth warning | Server logs `WARNING` at startup when `RUPTURA_API_KEY` is unset. |
| Emergency stop wired | `POST /api/v2/actions/emergency-stop` now calls `engine.EmergencyStop()` (was a no-op). |
| Forecast signal fix | Warm-up stub returns the requested signal's current value via `signalValue()`; nil-guard on `h.store`. |
| `RUPTURA_API_KEY` env var | Server reads the API key from the environment when `--api-key` flag is absent. |
| Slowloris protection | `http.Server` sets `ReadHeaderTimeout: 5s`. |
| Horizon + limit caps | `?horizon=` capped at 10 080 min (1 week); `?limit=` capped at 1 000. |
| Sim robustness | Injector uses `http.Client{Timeout: 10s}`; `math/rand` seeded at `Run()` start. |
| `reject` 404 | `POST /api/v2/actions/{id}/reject` returns 404 for unknown IDs. |
| `ruptura-ctl status` | `Actions()` error surfaced as a dim warning. |

### v6.6.1 — 2026-05-06 ✅

| Item | Detail |
|------|--------|
| `sim inject` fixed | CLI was sending `{pattern}` payload; server expects `{workload, metrics}`. Rewired to `sim.Run()` — real metric ticks per pattern. |
| `sim.send()` auth | `APIKey` added to `sim.Config`; every tick sends `Authorization: Bearer` header. |
| 3-segment workload refs | `describe workload ns/Kind/name` was 404 — added `/rupture/{namespace}/{kind}/{workload}` route. |
| Suppressions field mismatch | Handler now matches `workload`/`start`/`end` fields sent by the CLI. |
| Health port label | `ruptura-ctl health` now shows `traces (OTLP :4317)`. |

### v6.6.0 — 2026-05-05 ✅

| Item | Detail |
|------|--------|
| IMPROVE-07: Per-workload signal weight tuning | `POST /api/v2/config/weights` + `GET /api/v2/config/weights` for runtime override. `RUPTURA_WORKLOAD_WEIGHTS` JSON env var for Helm/K8s bootstrap. Selector syntax: exact, `ns/*` prefix, or `*` wildcard. Weights normalised to 1.0 on load. Helm `workloadWeights:` array in `values.yaml`. |

### v6.5.0 — 2026-05-05 ✅

| Item | Detail |
|------|--------|
| IMPROVE-06: Edition gate | `RUPTURA_EDITION` env var (`community` \| `autopilot`). `POST .../approve` returns 402 in `community` — recommendations stay visible read-only. Full execution in `autopilot`. Helm `edition: community` in `values.yaml`. |

### v6.4.0 — 2026-05-05 ✅

| Item | Detail |
|------|--------|
| IMPROVE-04: Rupture fingerprinting | 11-dimensional KPI vector per confirmed rupture (FusedR > 3.0). Cosine similarity ≥ 0.85 surfaced as `pattern_match` in every rupture response. |
| IMPROVE-05: Business signal layer | `slo_burn_velocity`, `blast_radius`, `recovery_debt` in every snapshot's `business` block. SLO contracts in Helm `values.yaml`. |

### v6.3.0 — 2026-05-04 ✅

| Item | Detail |
|------|--------|
| IMPROVE-01: Calibration warm-up | `status` + `calibration_progress` + `calibration_eta_minutes` in every snapshot. |
| IMPROVE-02: HealthScore trend forecast | `health_forecast` block — OLS slope → `in_15min`, `in_30min`, `critical_eta_minutes`. |
| IMPROVE-03: `ruptura-sim` binary | Four simulation patterns (`memory-leak`, `cascade-failure`, `traffic-surge`, `slow-burn`) via `POST /api/v2/sim/inject`. |

### v6.2.2 — 2026-04-30 ✅

| Item | Detail |
|------|--------|
| GAP-04 closed | Anomaly REST endpoints: `GET /api/v2/anomalies`, `GET /api/v2/anomalies/{host}` |
| Dead code removed | Duplicate `internal/predictor/anomaly_engine.go` removed; `MetricPipeline` interface extended |
| Docs updated | Correct API key env var (`RUPTURA_API_KEY`), accurate port references, all 10 KPI signal names |
| Release workflow | `HELM_CHART` path corrected; docker dependency wired for image-tag output |

**All v6.x engineering gaps closed as of v6.2.2.**

### v6.2.1 — 2026-04-30 ✅

| Item | Detail |
|------|--------|
| FusedR in API | `fused_rupture_index` field added to every rupture response; integration test verifies non-zero |
| Grafana dashboard | Panel 3 now queries `ruptura_kpi{signal="fused_rupture_index"}`; added Panel 4 (Pressure + Contagion), Panel 6 (Throughput Collapse), workload template variable, 15s auto-refresh |

### v6.2.0 — 2026-04-30 ✅

| Item | Detail |
|------|--------|
| WorkloadRef treatment unit | OTLP extracts `k8s.namespace.name`, `k8s.deployment.name`, etc. Multiple pods merged into one workload view. API routes: `/api/v2/rupture/{namespace}/{workload}` |
| Adaptive per-workload baselines | After 24h, thresholds become z-score deviations from Welford baseline. Batch jobs stop generating false alarms. |
| Narrative explain | `GET /api/v2/explain/{id}/narrative` — structured English causal chain, not raw JSON |
| Topology-based contagion | Real service edges from trace spans. Falls back to `errors×cpu` proxy when no trace data. |
| Maintenance windows | `POST/GET/DELETE /api/v2/suppressions` — suppress alerts during planned deploys |
| Fusion end-to-end | metricR (CA-ILR 15s ticker) + logR (burst detector) + traceR (OTLP span error rate) → FusedR |
| HealthScore formula | Switched from multiplicative (collapsed aggressively) to additive-penalty model |
| All 10 KPI signals | stress · fatigue · mood · pressure · humidity · contagion · resilience · entropy · velocity · health_score |
| Action engine | Bounded 256-entry pending queue; `Approve()` / `Reject()` API endpoints live |
| BadgerDB flush on SIGTERM | `FlushSnapshots()` called on graceful shutdown — no data loss |
| Token-bucket rate limiter | Default 1000 req/s on ingest; configurable via `RUPTURA_INGEST_RPS` |
| Integration test | Full-stack: `analyzer.Update()` → `store.StoreSnapshot()` → REST API response |

### v6.1.0 — 2026-04-27 ✅

| § | Feature | Detail |
|---|---------|--------|
| §23 | gRPC ingest | Real gRPC server (:9090), 4 MB max, RESOURCE_EXHAUSTED back-pressure |
| §24 | NATS / Kafka eventbus | JetStream at-least-once + franz-go exactly-once; topics: `ruptura.rupture.*`, `ruptura.actions.tier1` |
| §25 | Adaptive ensemble weighting | Online MAE-based weights, 1-hour sliding window, 60 s update cycle |
| §26 | Kubernetes operator | `RupturaInstance` CRD, controller-runtime reconcile, creates Deployment + Service + PVC |
| — | Go SDK (`sdk/go`) | Full v2 API coverage, typed client |

### v6.0.0 — 2026-04-25 ✅

Clean-room rewrite from OHE v5.1 as `github.com/benfradjselim/ruptura`:

- CA-ILR dual-scale ELS engine
- 5-model ensemble (CA-ILR, ARIMA, Holt-Winters, MAD, EWMA)
- 44-endpoint REST API v2 with XAI explainability
- Action engine (K8s / Webhook / Alertmanager / PagerDuty) with 3-tier safety gates
- OTLP + Prometheus remote_write + DogStatsD ingest
- BadgerDB embedded storage
- ≥ 70% test coverage across all packages

---

## Planned

### v7.1.0 — Q3 2026

| Feature | Detail |
|---------|--------|
| SLO config UI | Configure SLO targets and error budgets directly from the dashboard |
| Dashboard layout customization | Drag-and-drop card arrangement, pinned signals |
| Multi-tenant namespaces | X-Org-ID header → namespace filter on all queries; per-org storage namespacing |

### v7.2.0 — Q4 2026

| Feature | Detail |
|---------|--------|
| Python SDK v2 | async support (`httpx`), type stubs, full v2 API parity with Go SDK |
| Grafana data source plugin | Native Grafana plugin for Ruptura — query KPIs and FusedR directly in Grafana panels |
| Cluster mode (WAL + S3) | Raft-based replication, S3-compatible snapshot target (MinIO / AWS / GCS) |

---

## Engineering Gap Closure Log

All gaps from `docs/judgment.md` resolved in v6.2.x:

| GAP / MISSING | Status | Closed in |
|---------------|--------|-----------|
| GAP-01 Dual composite engine | ✅ | v6.2.0 |
| GAP-02 API stubs | ✅ | v6.2.0 |
| GAP-03 Fusion wiring + FusedR in API | ✅ | v6.2.0 + v6.2.1 |
| GAP-04 AnomalyStore not wired to actions | ✅ | v6.2.2 |
| GAP-05 Throughput collapse blind spot | ✅ | v6.2.0 |
| GAP-06 In-memory only storage | ✅ | v6.2.0 (BadgerDB + SIGTERM flush) |
| GAP-07 No Grafana dashboard | ✅ | v6.2.1 (6 panels, workload labels) |
| GAP-08 OTLP route disconnect | ✅ | v6.2.0 (421 Misdirected with port guidance) |
| GAP-09 Sentiment disconnected from log pipeline | ✅ | v6.2.0 |
| GAP-10 Treatment unit infra-only | ✅ | v6.2.0 (WorkloadRef) |
| MISSING-01 Adaptive per-workload baselines | ✅ | v6.2.0 |
| MISSING-02 Narrative explain | ✅ | v6.2.0 |
| MISSING-03 Real contagion from trace topology | ✅ | v6.2.0 |
| MISSING-04 Maintenance windows / suppressions | ✅ | v6.2.0 |

---

## CNCF

Ruptura is an independent open-source project targeting alignment with CNCF sandbox criteria. The project follows CNCF principles: Apache 2.0 license, open governance (`GOVERNANCE.md`), documented security policy (`SECURITY.md`), and a public roadmap.

A CNCF sandbox application requires demonstrable production adoption and a committed maintainer community. Achieving that is a **long-term goal**, not a current status. Production feedback and contributions from the community are the path there.

---

## Changelog

### v5.1.0 (OHE) — 2026-04-19

Go + Python SDK, Prometheus remote_write, gRPC agent, Vault integration, plugin system.

### v5.0.0 (OHE) — 2026-04-12

CA-ILR dual-scale predictor, dissipative fatigue (λ recovery), METRICS.md XAI standard, BadgerDB tiered storage.

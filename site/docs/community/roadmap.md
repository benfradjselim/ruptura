# Roadmap

## Released

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

### v7.0.0 — Q3 2026

| Feature | Detail |
|---------|--------|
| `ruptura-ctl` CLI | `ruptura-ctl status`, `ruptura-ctl explain <id>`, `ruptura-ctl suppress <workload> 30m` |
| Web dashboard v2 | Embedded Svelte UI: Fused Rupture Index heatmap, signal timelines, action log, narrative explain panel |
| Multi-tenant opt-in (FR-10) | X-Org-ID header → namespace filter on all queries; per-org storage namespacing |
| Python SDK v2 | async support (`httpx`), type stubs, full v2 parity with Go SDK |

### v7.1.0 — Q4 2026

| Feature | Detail |
|---------|--------|
| Cluster mode (WAL + S3) | Raft-based replication, S3-compatible snapshot target (MinIO / AWS / GCS) |
| FFT cycle detection | Replace manual seasonality buckets with frequency-domain analysis |
| Confidence intervals | Residual-based uncertainty quantification on predictions |

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

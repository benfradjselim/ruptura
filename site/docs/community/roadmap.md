# Roadmap

## Released

### v6.1.1 — 2026-04-28 ✅

Bug fixes, documentation site launch, all 8 composite signal formulas published.

### v6.1.0 — 2026-04-27 ✅

| § | Feature | Detail |
|---|---------|--------|
| §23 | gRPC ingest | Real gRPC server (:9090), 4 MB max, back-pressure via RESOURCE_EXHAUSTED |
| §24 | NATS / Kafka eventbus | JetStream at-least-once + franz-go exactly-once; topics: `ruptura.rupture.*`, `ruptura.actions.tier1` |
| §25 | Adaptive ensemble weighting | Online MAE-based weights, 1-hour sliding window, 60 s update cycle |
| §26 | Kubernetes operator | `RupturaInstance` CRD, controller-runtime reconcile, creates Deployment + Service + PVC |
| — | Go SDK (`sdk/go`) | Full v2 API coverage, typed client, `ohe.WithAPIKey` / `ohe.WithToken` |

### v6.0.0 — 2026-04-25 ✅

Clean-room rewrite from OHE v5.1 as `github.com/benfradjselim/ruptura`:

- CA-ILR dual-scale ELS engine
- 5-model ensemble (CA-ILR, ARIMA, Holt-Winters, MAD, EWMA)
- 8 composite signals with published formulas
- 44-endpoint REST API v2 with XAI explainability
- Action engine (K8s / Webhook / Alertmanager / PagerDuty) with 3-tier safety gates
- OTLP + Prometheus remote_write + DogStatsD ingest
- BadgerDB embedded storage, 400-day KPI retention
- ≥ 70% test coverage across all packages

---

## Planned

### v6.2.0 — Q2 2026

| Feature | Detail |
|---------|--------|
| `kairoctl` CLI | Command-line client: `kairoctl rupture list`, `kairoctl kpi get`, `kairoctl actions approve` |
| Web dashboard v2 | Embedded Svelte UI showing Rupture Index heat map, signal timeline, action log |
| Multi-tenant opt-in | Organisation isolation via `X-Org-ID` header, per-org storage namespacing |
| Python SDK v2 | async support (`httpx`), type stubs, full v2 parity with Go SDK |

### v6.3.0 — Q3 2026

| Feature | Detail |
|---------|--------|
| SaaS self-serve | Hosted Ruptura at `ruptura.io` — managed instance, usage billing |
| Cluster mode (WAL + S3) | Raft-based replication, S3-compatible snapshot target (MinIO / AWS / GCS) |
| Median pre-filter for ILR | Outlier robustness before slope computation |

### v7.0.0 — Q4 2026

| Feature | Detail |
|---------|--------|
| FFT cycle detection | Replace manual seasonality buckets with frequency-domain analysis |
| Confidence intervals | Residual-based uncertainty quantification on predictions |
| Auto-remediation webhooks | Closed-loop healing: Ruptura triggers and verifies its own remediation |

---

## CNCF

Ruptura is an independent open-source project targeting alignment with CNCF sandbox criteria. The project follows CNCF principles: Apache 2.0 license, open governance (`GOVERNANCE.md`), documented security policy (`SECURITY.md`), and a public roadmap.

A CNCF sandbox application requires demonstrable production adoption and a committed maintainer community. Achieving that is a **long-term goal**, not a current status. Contributions and production feedback from the community are the path there.

---

## Changelog

### v5.1.0 (OHE) — 2026-04-19

Go + Python SDK, Prometheus remote_write, gRPC agent, Vault integration, plugin system.

### v5.0.0 (OHE) — 2026-04-12

CA-ILR dual-scale predictor, dissipative fatigue (λ recovery), METRICS.md XAI standard, BadgerDB tiered storage.

[Full OHE v5 changelog in docs/v5.0.0/](https://github.com/benfradjselim/ruptura/tree/v6.1/workdir/docs/v5.0.0)

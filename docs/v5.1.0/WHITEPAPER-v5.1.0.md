# Observability Holistic Engine (OHE) v5.1.0
## Technical White Paper — Strategic & Implementation Backbone

**Document ID:** OHE-WP-006
**Version:** 5.1.0 (Supersedes v5.0.0)
**Status:** Canonical Specification — Source of Truth for Implementation
**Date:** April 2026
**Author:** Selim Benfradj, Founding Architect
**Branch:** v5.1.0

---

## Table of Contents

1. Executive Summary
2. What Changed in v5.1.0
3. Context and Problem Statement
4. The Observability Trilemma
5. Vision: Holistic Observability
6. Technical Architecture
7. The CA-ILR Predictive Engine
8. Mathematical Formalization
9. Storage and Multi-Tenancy
10. Security and Compliance
11. SDK Ecosystem
12. Deployment Guide
13. Benchmarks
14. Roadmap
15. Changelog v5.0.0 to v5.1.0

---

## 1. Executive Summary

### 1.1 The Problem

Current observability splits along two failing axes. Open-source stacks (Prometheus + Grafana + Loki) demand 5+ services, 8 GB+ RAM, and weeks of integration — producing reactive alerts from static thresholds. Enterprise SaaS (Datadog, Dynatrace) deliver black-box AI at prohibitive cost with opaque logic. Both answer "What is broken?" — neither answers "When will it break, and why?"

### 1.2 The OHE v5.1 Solution

OHE treats infrastructure as a living organism, measuring vital signs, behaviors, emotions, and social interactions through auditable composite KPIs. v5.1 ships the first complete SDK ecosystem (Go + Python), closing the integration loop from platform to developer tooling.

### 1.3 Headline Differentiators

| Dimension | v4.4.0 | v5.0.0 | v5.1.0 |
|---|---|---|---|
| Predictive Model | Single ILR | Dual-Scale CA-ILR | CA-ILR + YAML-configurable |
| Fatigue Function | Cumulative | Dissipative (lambda recovery) | Config-driven (r_threshold, lambda) |
| SDK | None | None | Go SDK + Python SDK |
| API Endpoints | 40 | 60 | 60 + Prometheus remote_write |
| Transparency | Implicit | METRICS.md | METRICS.md + /explain/{kpi} XAI |
| RAM Footprint | under 100 MB | 22 MB typical | 22 MB typical |
| Test Coverage | ~40% | at least 60% all 18 packages | at least 65% api, at least 77% orchestrator |

---

## 2. What Changed in v5.1.0

### 2.1 Go SDK

Module: `github.com/benfradjselim/ohe-sdk-go`
Location: `sdk/go/`
Tests: 15 httptest-based tests, race-detector clean

```go
c := ohe.New("https://ohe.example.com",
    ohe.WithAPIKey("ohe_abc123"),
    ohe.WithOrgID("acme"),
)
health, err := c.Health(ctx)
snap, err   := c.KPIGet(ctx, "web-01")
pred, err   := c.KPIPredict(ctx, "stress", "web-01", 60)
expl, err   := c.Explain(ctx, "stress", "web-01")
```

Resources covered: health, auth (login/logout/refresh), metrics (list/get/range/aggregate), KPIs (get/predict/multi), explain (XAI), alerts (list/get/ack/silence/delete), alert rules (CRUD), dashboards (CRUD), SLOs (CRUD + status), orgs (CRUD + members), API keys (create/list/delete), ingest, logs query, traces (search/get), QQL query.

### 2.2 Python SDK

Package: `ohe-sdk` (`pip install ohe-sdk`)
Location: `sdk/python/`
Tests: 20 tests using `responses` mock library

```python
from ohe import OHEClient, OHEError

c = OHEClient("https://ohe.example.com", api_key="ohe_abc123", org_id="acme")
c.login("admin", "password")        # stores token automatically
snap   = c.kpi_get("web-01")
pred   = c.kpi_predict("stress", "web-01", horizon=60)
alerts = c.alert_list(status="active")
```

Error handling:

```python
try:
    c.health()
except OHEError as e:
    print(e.status_code, e.code, e.message)
```

### 2.3 Predictor and Fatigue Fully Config-Driven

All v5.0 prediction knobs now flow through `orchestrator.Config` YAML:

```yaml
predictor:
  stable_window: 60m
  burst_window: 5m
  rupture_threshold: 3.0

fatigue:
  r_threshold: 0.3
  lambda: 0.05
```

### 2.4 Router Bug Fix

`/kpis/multi` was shadowed by `/kpis/{name}` in the gorilla/mux registration order. Fixed: `multi` is now registered before the `{name}` wildcard in `internal/api/router.go`.

### 2.5 Coverage Improvements

| Package | v5.0.0 | v5.1.0 |
|---|---|---|
| internal/api | 61.2% | 65.3% |
| internal/orchestrator | 64.7% | 77.6% |
| All 18 packages | at least 60% | at least 65% |

New test files added:
- `internal/api/internal_helpers_test.go` — white-box OTLP helpers, isGood, otlpSanitize
- `internal/api/api_coverage_boost_test.go` — KPIMulti, LogStream SSE, OTLP JSON bodies, Loki, DD, ES
- `internal/orchestrator/collect_boost_test.go` — collectLocally, runGC, runCompaction via short-interval run

---

## 3. Context and Problem Statement

### 3.1 The Evolution of Observability

| Era | Focus | Core Question |
|---|---|---|
| 2000-2010 | Monitoring | Is the server up? |
| 2010-2020 | Observability | Why is the server slow? |
| 2020-2025 | AIOps / MLOps | What will go wrong? |
| 2025+ | Holistic Observability (OHE) | When, how, and why will it go wrong? |

### 3.2 The Gap No Current Solution Fills

No current solution simultaneously provides:

1. A holistic view of infrastructure as a living organism
2. Composite KPIs reflecting overall health (observability ETFs)
3. Contextual predictions with business reasoning
4. Behavioral analysis: habits, rhythms, recovery cycles
5. Emotional state detection: stress, fatigue, mood
6. Social dynamics: error propagation, dependency contagion
7. Radical transparency — every KPI auditable via published formulas
8. Edge-native deployment in a single binary under 25 MB

---

## 4. The Observability Trilemma

OHE resolves three properties that other tools treat as mutually exclusive:

| Solution | Predictive Accuracy | Operational Simplicity | Cost Efficiency |
|---|---|---|---|
| Prometheus/Grafana/Loki | Static thresholds only | 5+ services to operate | Open source, high human cost |
| Datadog / Dynatrace | Black-box AI | SaaS agent install | Unpredictable per-host billing |
| OHE v5.1 | Transparent AI (XAI) | Single binary | Zero marginal cost |

---

## 5. Vision: Holistic Observability

### 5.1 The Biometric Metaphor

| Infrastructure | Human Analog |
|---|---|
| CPU / RAM / Disk | Temperature / Blood pressure / Heart rate |
| Network throughput | Blood circulation |
| Logs | Symptoms |
| Errors | Pain signals |
| Timeouts | Fatigue signals |
| Restarts | Fever |
| Latency | Reflexes |

### 5.2 The Four Pillars

1. Vital Signs — raw metrics: CPU, RAM, disk, network
2. Behavioral Patterns — rhythms, cycles, habits captured by ILR trending
3. Emotional State — Stress, Fatigue, Mood, Pressure, Contagion
4. Social Dynamics — dependency topology, alert grouping, contagion propagation

### 5.3 Philosophy

Prevention is better than cure. Shift from "CPU is at 85%" to "CPU will reach 90% in 3 hours — fatigue index rising, rupture index 4.2 (threshold 3.0), recommend preventive restart."

---

## 6. Technical Architecture

### 6.1 Component Map

```
cmd/ohe (main binary ~22 MB)
  |
  +-- orchestrator (Config YAML, wires all subsystems)
       |
       +-- api (HTTP :8080, Gin router, 60 endpoints)
       +-- grpcserver (gRPC :9090, AgentService/Ingest)
       +-- receiver (DogStatsD UDP :8125, OTLP HTTP)
       +-- predictor (CA-ILR, ARIMA, HoltWinters, MAD, Ensemble)
       +-- analyzer (fatigue topology, ILR clusters)
       +-- alerter (rule engine, grouping, silencing)
       +-- correlator (burst-to-KPI correlation)
       +-- eventbus (pub/sub)
       +-- billing (UsageEvent ring buffer, webhook flush)
       +-- plugin (sandboxed plugin system)
       +-- vault (HashiCorp Vault integration)
       +-- storage (BadgerDB KV, OrgStore multi-tenant isolation)

SDK Layer (external):
  sdk/go/     github.com/benfradjselim/ohe-sdk-go
  sdk/python/ pip install ohe-sdk
```

### 6.2 Data Flow (Central Mode)

```
System metrics (collector) -------\
DogStatsD UDP :8125 (receiver) ----+-> processor -> analyzer -> predictor
OTLP HTTP (receiver) --------------+                         -> alerter
gRPC ingest (grpcserver) ----------/                         -> storage
                                                             -> api (HTTP)
                                                             -> SDK clients
```

### 6.3 Multi-Tenancy

Every storage key is prefixed `o:{orgID}:`. A BadgerDB prefix scan for org `acme` is physically incapable of returning data from org `beta`. Isolation is enforced at the storage iteration level, not in application logic.

---

## 7. The CA-ILR Predictive Engine

### 7.1 Dual-Scale Architecture

For each metric stream, two ELS (Exponential Least Squares) trackers run in parallel:

```
Stable tracker:  lambda_stable = 0.95   (~60-minute memory)
Burst tracker:   lambda_burst  = 0.80   (~5-minute memory)
```

Each tracker maintains: alpha (slope/acceleration), beta (intercept/level), P (Kalman gain).

### 7.2 Rupture Index

```
R = alpha_burst / alpha_stable
```

When R > rupture_threshold (configurable, default 3.0), a RuptureEvent is emitted containing host, metric, rupture index, alpha_stable, alpha_burst, and timestamp. This signals exponential acceleration — the classic signature of cascading failures.

### 7.3 Forecast Production

- Short horizon (0-15 min): burst tracker slope
- Medium horizon (15-120 min): stable tracker slope
- Ensemble: EWMA-weighted combination of ILR, ARIMA(1,1,1), Holt-Winters (damped trend phi=0.98), MAD anomaly guard

### 7.4 Configuration

```yaml
predictor:
  stable_window: 60m      # ELS forgetting factor window for stable tracker
  burst_window: 5m        # ELS forgetting factor window for burst tracker
  rupture_threshold: 3.0  # R above this value triggers a RuptureEvent
```

---

## 8. Mathematical Formalization

See `METRICS.md` for canonical formulas and thresholds. High-level summary:

| KPI | Formula Type |
|---|---|
| Stress | Weighted linear combination of 5 normalized inputs |
| Fatigue | Dissipative accumulator with lambda recovery term |
| Mood | Ratio of positive to negative signals |
| Pressure | Trend-weighted latency/error composite |
| Humidity | Network and inter-service coupling factor |
| Contagion | Graph propagation coefficient from topology |
| Resilience | Inverse of sustained stress duration |
| Entropy | Shannon-style disorder of metric variance |
| Velocity | EWMA first derivative (rate of change) |
| HealthScore | Weighted composite [0-100], ETF-style |

---

## 9. Storage and Multi-Tenancy

### 9.1 Key Schema

```
o:{orgID}:m:{host}:{metric}:{ts_ns}     raw metrics      (MetricsTTL)
o:{orgID}:k:{host}:{kpi}:{ts_ns}        computed KPIs    (KPIsTTL)
o:{orgID}:a:{id}                         alerts           (AlertsTTL)
o:{orgID}:d:{id}                         dashboards       (permanent)
o:{orgID}:ds:{id}                        datasources      (permanent)
o:{orgID}:nc:{id}                        notif. channels  (permanent)
o:{orgID}:slo:{id}                       SLOs             (permanent)
o:{orgID}:ak:{id}                        API keys         (permanent)
o:{orgID}:ar:{name}                      alert rules      (permanent)
o:{orgID}:l:{service}:{ts_ns}            logs             (LogsTTL)
o:{orgID}:sp:{traceID}:{spanID}          spans            (LogsTTL)
```

### 9.2 High Availability

Litestream sidecar replicates the BadgerDB data directory to any S3-compatible store in real-time. See `deploy/central-deployment.yaml` for the K8s manifest including the sidecar spec and replica URL configuration.

### 9.3 Retention and Compaction

The compaction goroutine runs every 30 minutes and downsamples raw time-series into 5-minute and 1-hour rollups. This bounds storage growth while preserving trend visibility at all time scales.

---

## 10. Security and Compliance

### 10.1 Authentication Mechanisms

**JWT (session tokens)**
- Standard `Authorization: Bearer <token>` header
- Tokens revocable via JTI blocklist persisted in BadgerDB with TTL
- Logout endpoint invalidates token immediately

**API Keys (long-lived programmatic access)**
- Format: `ohe_` prefix + random bytes (e.g. `ohe_a1b2c3d4e5f6`)
- bcrypt-hashed at rest — server cannot reconstruct plaintext
- Plaintext returned once at creation; must be stored by client
- Role-bound: viewer, operator, or admin
- CRUD via `/api/v1/api-keys`

### 10.2 Role-Based Access Control

| Role | Capabilities |
|---|---|
| viewer | Read-only: metrics, KPIs, alerts, dashboards, SLOs, logs |
| operator | viewer + write: dashboards, SLOs, alert rules, datasources, ingest, notification channels |
| admin | operator + user management, org management, audit log, reload |

### 10.3 Audit Log

Every write operation emits an AuditEntry stored with 2-year TTL. Fields: timestamp, org_id, username, IP, action, resource, resource_id, details. Accessible via `GET /api/v1/audit` (admin only).

### 10.4 TLS

`--tls-cert` and `--tls-key` flags enable HTTPS. Both must be provided together — a half-config is rejected at startup with a clear error.

### 10.5 Per-Org Quotas

Configurable limits enforced at storage layer:
- max_dashboards, max_datasources, max_api_keys
- max_alert_rules, max_slos
- ingest_rate_rpm (requests per minute)

Default (free tier): 10 dashboards, 5 datasources, 5 API keys, 20 alert rules, 5 SLOs, 300 req/min ingest.

---

## 11. SDK Ecosystem

### 11.1 Go SDK

**Module:** `github.com/benfradjselim/ohe-sdk-go`
**Location in repo:** `sdk/go/`
**Go version:** 1.22+

Client options:

| Option | Description |
|---|---|
| `WithToken(token)` | JWT bearer token |
| `WithAPIKey(key)` | Long-lived API key (ohe_*) |
| `WithOrgID(id)` | Multi-tenant org scope |
| `WithTimeout(d)` | HTTP request timeout (default 30s) |
| `WithHTTPClient(hc)` | Custom http.Client |

Methods by resource:

| Resource | Methods |
|---|---|
| Health | Health, Liveness, Readiness, Fleet |
| Auth | Login, Logout, Refresh |
| Metrics | MetricsList, MetricGet, MetricRange, MetricAggregate |
| KPIs | KPIGet, KPIPredict, KPIMulti, Explain, Query |
| Alerts | AlertList, AlertGet, AlertAcknowledge, AlertSilence, AlertDelete |
| Alert Rules | AlertRuleList, AlertRuleCreate, AlertRuleUpdate, AlertRuleDelete |
| Dashboards | DashboardList, DashboardGet, DashboardCreate, DashboardUpdate, DashboardDelete |
| SLOs | SLOList, SLOGet, SLOCreate, SLOUpdate, SLODelete, SLOStatus, SLOAllStatus |
| Orgs | OrgList, OrgGet, OrgCreate, OrgUpdate, OrgDelete, OrgMemberList, OrgInvite |
| API Keys | APIKeyList, APIKeyCreate, APIKeyDelete |
| Ingest | Ingest |
| Logs | LogQuery, TraceSearch, TraceGet |

### 11.2 Python SDK

**Package:** `ohe-sdk`
**Install:** `pip install ohe-sdk`
**Location in repo:** `sdk/python/`
**Python version:** 3.9+

`OHEClient(base_url, *, token, api_key, org_id, timeout, session)`

Methods mirror Go SDK with snake_case naming:
`health()`, `login()`, `logout()`, `kpi_get()`, `kpi_predict()`, `explain()`, `alert_list()`, `alert_acknowledge()`, `slo_create()`, `slo_status()`, `ingest()`, `apikey_create()`, etc.

`OHEError(status_code, code, message)` raised on all non-2xx responses.

---

## 12. Deployment Guide

### 12.1 Minimal Start (Development)

```bash
./ohe --storage-path ./data --port 8080
```

### 12.2 Production Config (YAML)

```yaml
mode: central
host: web-01
port: 8080
storage_path: /data
collect_interval: 15s
auth_enabled: true
jwt_secret: "${OHE_JWT_SECRET}"

predictor:
  stable_window: 60m
  burst_window: 5m
  rupture_threshold: 3.0

fatigue:
  r_threshold: 0.3
  lambda: 0.05

billing_webhook_url: "${OHE_BILLING_WEBHOOK}"
replica_url: "${OHE_REPLICA_URL}"
grpc_addr: ":9090"
dogstatsd_addr: ":8125"
```

### 12.3 Key Environment Variables

| Variable | Purpose |
|---|---|
| OHE_JWT_SECRET | JWT signing key (at least 32 chars) |
| OHE_ADMIN_PASSWORD | Force-reset admin password at boot |
| OHE_REPLICA_URL | S3/GCS/Azure URL for Litestream replication |
| OHE_TLS_CERT / OHE_TLS_KEY | TLS certificate file paths |

### 12.4 Agent Mode

```yaml
mode: agent
central_url: https://ohe-central:8080
host: web-01
collect_interval: 15s
```

### 12.5 gRPC Agent Ingest

Agents can push metrics over gRPC using the `ohe.v1.AgentService/Ingest` unary RPC. No protoc-generated code is required — the JSON codec is used. Send `org-id` in gRPC metadata for multi-tenant routing.

---

## 13. Benchmarks

| Operation | Throughput | Latency p99 |
|---|---|---|
| Metric ingest (HTTP) | over 10,000 req/s | under 2 ms |
| KPI computation (15s cycle, 50 metrics) | 3,500 KPI/s | under 1 ms |
| Ensemble prediction (4 models) | 1,550x faster than ARIMA alone | under 500 us |
| Storage writes (BadgerDB, NVMe) | over 100,000/s | under 1 ms |
| Memory footprint (idle) | 22 MB | — |
| Binary size | ~24 MB | — |

---

## 14. Roadmap

### v5.1.0 (Current — April 2026)

- Go SDK with 15 tests
- Python SDK with 20 tests
- CA-ILR config fully wired through YAML
- Router bug fix (KPIMulti handler shadowing)
- Coverage: api 65%, orchestrator 78%
- CLAUDE.md workspace guide + gofmt hook

### v5.2.0 (Q2 2026)

- NATS/Kafka event streaming (item 18 from roadmap)
- Coverage: api to 70%, orchestrator to 80%
- Go SDK: publish to pkg.go.dev
- Python SDK: publish to PyPI

### v6.0.0 (Horizon)

- S3-native storage backend (replace BadgerDB)
- Distributed multi-region topology
- ML-assisted anomaly threshold auto-tuning
- Grafana datasource plugin for OHE

---

## 15. Changelog v5.0.0 to v5.1.0

```
feat(sdk):      Go SDK — typed client, 15 tests, all 60 API resources covered
feat(sdk):      Python SDK — OHEClient + OHEError, 20 tests, pip-installable
fix(router):    /kpis/multi shadowed by /kpis/{name} — moved before wildcard
feat(config):   predictor/fatigue knobs wired through orchestrator YAML
fix(coverage):  api 61% to 65%, orchestrator 64% to 78%
chore:          CLAUDE.md workspace guide for session continuity
chore:          gofmt PostToolUse hook in ~/.claude/settings.json
docs:           WHITEPAPER-v5.1.0.md (this file)
docs:           METRICS.md — canonical KPI formulas and thresholds
docs:           ARCHITECTURE.md — component map and data flows
docs:           API-REFERENCE.md — all 60 endpoints with request/response shapes
docs:           SDK-GUIDE.md — Go and Python SDK usage examples
```

# OHE v5.1.0 — Architecture Reference

## 1. Overview

OHE (Observability Holistic Engine) is a **self-hosted, single-binary** observability platform written in Go. It replaces the Prometheus + Grafana + Loki + Jaeger + Alertmanager stack with a unified daemon backed by embedded BadgerDB.

```
┌─────────────────────────────────────────────────────────────────┐
│                        OHE Binary (ohe)                          │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────┐   │
│  │ HTTP API │  │  gRPC    │  │DogStatsD │  │  OTLP HTTP/   │   │
│  │ :8080    │  │ :4317    │  │ UDP:8125 │  │  gRPC :4318   │   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └───────┬───────┘   │
│       │              │              │                │            │
│  ┌────▼──────────────▼──────────────▼────────────────▼───────┐  │
│  │                     Orchestrator                            │  │
│  │  tick loop · collectLocally · logAlerts · runGC            │  │
│  └────┬────────────┬────────────┬─────────────┬──────────────┘  │
│       │            │            │              │                  │
│  ┌────▼──┐  ┌─────▼──┐  ┌─────▼──┐  ┌───────▼──┐              │
│  │Predictor│ │Analyzer│  │Alerter │  │ Storage  │              │
│  │CA-ILR  │  │Topology│  │Rules   │  │BadgerDB  │              │
│  │ARIMA   │  │ILR clus│  │Groups  │  │Multi-org │              │
│  └────────┘  └────────┘  └────────┘  └──────────┘              │
│                                                                   │
│  ┌─────────────┐  ┌────────────┐  ┌────────────┐               │
│  │  EventBus   │  │ Correlator │  │  Billing   │               │
│  │  pub/sub    │  │  metrics   │  │  metering  │               │
│  └─────────────┘  └────────────┘  └────────────┘               │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Package Layout

```
workdir/
├── cmd/ohe/              Main binary — wires all subsystems
├── internal/
│   ├── api/              HTTP handlers, middleware, router (gorilla/mux)
│   ├── alerter/          Alert rules, firing, grouping, suppression
│   ├── analyzer/         Topology graph, dissipative fatigue ILR
│   ├── billing/          UsageEvent ring buffer + webhook flush
│   ├── correlator/       Metric correlation engine
│   ├── eventbus/         In-process pub/sub
│   ├── grpcserver/       Agent gRPC ingest (ohe.v1.AgentService)
│   ├── notifier/         Channel dispatch (webhook, Slack, PagerDuty)
│   ├── orchestrator/     Engine: Config, Run(), tick loop
│   ├── plugin/           Plugin sandbox (WASM-ready)
│   ├── predictor/        CA-ILR, ARIMA, HoltWinters, MAD, Ensemble
│   ├── processor/        Metric fan-out, validation, enrichment
│   ├── receiver/         DogStatsD UDP, OTLP HTTP/gRPC receivers
│   ├── storage/          BadgerDB wrapper, OrgStore, retention
│   └── vault/            HashiCorp Vault integration
├── operator/             Kubernetes operator controller
├── pkg/
│   ├── logger/           Zero-dep structured JSON logger
│   └── models/           Shared domain types
└── sdk/
    ├── go/               Typed Go client (github.com/benfradjselim/ohe-sdk-go)
    └── python/           Python client (pip install ohe-sdk)
```

---

## 3. Data Flow

### 3.1 Metric Ingestion

```
Agent / Prometheus / OTLP / DogStatsD
         │
         ▼
    Receiver (UDP/HTTP/gRPC)
         │  validates & normalises
         ▼
    Processor (fan-out)
         │  enriches: host, org, timestamp
         ▼
    Storage.SaveMetric()
         │  writes BadgerDB key: o:{orgID}:m:{host}:{metric}:{ts}
         ▼
    EventBus.Publish("metric.*")
         │
    ┌────┴──────────┐
    ▼               ▼
Correlator       Orchestrator tick
```

### 3.2 KPI Computation (Orchestrator tick, default 60s)

```
Storage.GetRecentMetrics(host, window=5m)
         │
         ▼
buildMetricsMap()  → normalised [0,1] values
         │
         ▼
computeStress()    → S ∈ [0,1]
computeFatigue()   → F ∈ [0,1]
computeOtherKPIs() → Mood, Pressure, Resilience …
         │
         ▼
Storage.ForOrg(org).SaveKPI(host, kpiName, value, ts)
         │
         ▼
Predictor.Update(kpiName, value)
Alerter.Evaluate(rules, kpiValues)
```

### 3.3 Alert Lifecycle

```
Alerter.Evaluate()
    → rule matches
         │
         ▼
GroupingEngine.Add(alert)
    → deduplication window (default 5m)
         │
    if group fires:
         ▼
Notifier.Fire(channels, alert)
    → webhook POST / Slack / PagerDuty
         │
         ▼
Storage.SaveAlert(alert)
EventBus.Publish("alert.fired")
```

---

## 4. Storage Model

OHE uses **BadgerDB** (embedded, no external process).

### 4.1 Multi-Tenancy

Every key is prefixed with `o:{orgID}:` — tenants are fully isolated at the key level. The `OrgStore` wrapper adds the prefix automatically.

### 4.2 Key Schema

```
o:{orgID}:m:{host}:{metric}:{ts}    raw metric timeseries
o:{orgID}:k:{host}:{kpi}:{ts}       KPI timeseries
o:{orgID}:a:{id}                    alerts
o:{orgID}:d:{id}                    dashboards
o:{orgID}:ds:{id}                   datasources
o:{orgID}:nc:{id}                   notification channels
o:{orgID}:slo:{id}                  SLOs
o:{orgID}:ak:{id}                   API keys
o:{orgID}:l:{service}:{ts}          logs
o:{orgID}:sp:{traceID}:{spanID}     spans
o:{orgID}:audit:{ts}:{id}           audit log entries
jti:{jti}                           revoked JWT (global)
```

### 4.3 Retention & Downsampling

| Tier | Window | Resolution | Action |
|------|--------|-----------|--------|
| Hot  | 0–7d   | raw ticks | kept as-is |
| Warm | 7–30d  | 5-min     | compacted on write |
| Cold | 30d+   | 1-hour    | background compaction |

---

## 5. API Layer

All HTTP endpoints are registered in `internal/api/router.go` and served by `gorilla/mux`.

### 5.1 Middleware Stack (outermost first)

1. `SecurityHeadersMiddleware` — adds `X-Frame-Options`, `X-Content-Type-Options`, CSP
2. `LoggingMiddleware` — structured JSON request log (pkg/logger)
3. `CORSMiddleware` — configurable origin allowlist
4. `AuthMiddleware` — JWT HS256 / API key validation, sets `claims` in context
5. `RateLimitLogin` — per-IP token bucket on `/auth/login`

### 5.2 Role Model

| Role | Capabilities |
|------|-------------|
| `viewer` | Read all data, no mutations |
| `operator` | All viewer rights + create/update/delete most resources |
| `admin` | Full access including user management and audit log |

### 5.3 Auth Flow

```
POST /api/v1/auth/login  →  JWT (HS256, configurable secret)
    │
    ├── Authorization: Bearer <jwt>    for subsequent calls
    └── X-API-Key: <key>              for CI/CD / SDK usage
```

---

## 6. Predictor Architecture

```
┌─────────────────────────────────────────┐
│              Ensemble                    │
│  weight(CA-ILR)=0.4  weight(ARIMA)=0.3  │
│  weight(HoltWinters)=0.2  weight(MAD)=0.1│
└────────┬────────────────────────────────┘
         │  weighted average
    ┌────▼──────┐ ┌────────┐ ┌────────────┐ ┌──────┐
    │  CA-ILR   │ │ ARIMA  │ │HoltWinters │ │ MAD  │
    │  dual     │ │(2,1,2) │ │  seasonal  │ │outlier│
    │  scale    │ │        │ │            │ │detect │
    └───────────┘ └────────┘ └────────────┘ └──────┘
```

The ensemble selects the model with the lowest RMSE over the last 100 predictions.

---

## 7. gRPC Agent Protocol

Agents use `ohe.v1.AgentService/Ingest` (streaming RPC) to push metrics.

```protobuf
service AgentService {
  rpc Ingest(stream MetricBatch) returns (IngestAck);
}

message MetricBatch {
  string host    = 1;
  string org_id  = 2;
  repeated MetricPoint points = 3;
}
```

TLS is mutual (mTLS) when `grpc.tls_enabled = true` in config.

---

## 8. EventBus

Internal pub/sub (`internal/eventbus`) decouples producers from consumers.

| Topic | Producer | Consumers |
|-------|----------|-----------|
| `metric.raw` | Receiver | Processor, Correlator |
| `kpi.updated` | Orchestrator | Alerter, Predictor |
| `alert.fired` | Alerter | Notifier, Billing |
| `alert.resolved` | Alerter | Notifier |

---

## 9. Deployment Models

### 9.1 Standalone (single binary)

```bash
ohe --config ohe.yaml
```
All subsystems in one process. Suitable for single-host or small fleet (<50 hosts).

### 9.2 Central + Agent (distributed)

```
┌─────────┐   gRPC stream   ┌───────────────┐
│  Agent  │ ──────────────► │  Central OHE  │
│ (edge)  │                 │  (aggregator) │
└─────────┘                 └───────────────┘
```

Edge agents collect metrics locally; central aggregator stores and analyses. Configured via `mode: agent` / `mode: central`.

### 9.3 Kubernetes (Operator)

The included Kubernetes operator (`operator/`) manages `OHEInstance` CRDs and handles rolling upgrades, PVC sizing, and TLS cert rotation.

---

## 10. Security Model

| Control | Implementation |
|---------|---------------|
| JWT authentication | HS256, configurable secret, 1h expiry |
| API keys | Hashed (SHA-256) stored in BadgerDB |
| Token revocation | JTI blocklist in BadgerDB |
| RBAC | Per-route `RequireRole()` middleware |
| Rate limiting | Token bucket per IP on auth endpoints |
| TLS | Configurable on HTTP and gRPC listeners |
| mTLS | Supported on gRPC agent channel |
| Secret management | HashiCorp Vault integration (`internal/vault`) |
| Audit logging | All write operations recorded with actor + timestamp |

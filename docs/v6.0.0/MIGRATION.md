# MIGRATION.md — OHE v5.1 → Kairo Core v6.0.0

Document ID: KC-MIG-001
Date: April 2026
Status: Canonical — Phase -1 Output
Produced by: Orchestrator (Claude Code)

Reference: WP §26 "Migration from OHE v5.1"

---

## 1. Binary & Module Renaming

| v5.1 | v6.0 | Notes |
|------|------|-------|
| Binary: `ohe` | Binary: `kairo-core` | `cmd/ohe/` → `cmd/kairo-core/` |
| Module: `github.com/benfradjselim/ohe` | Module: `github.com/benfradjselim/kairo-core` | All import paths change |
| Config file: `ohe.yaml` / `config.yaml` | Config file: `kairo.yaml` | New schema (WP §17) |
| Version const | `"6.0.0"` | Update `const version` in handlers.go |

---

## 2. Package Path Mapping

| v5.1 Import Path | v6.0 Import Path | Action |
|-----------------|-----------------|--------|
| `.../ohe/internal/predictor` | `.../kairo-core/internal/pipeline/metrics` | RÉUTILISER — move files |
| `.../ohe/internal/analyzer` | `.../kairo-core/internal/pipeline/traces` | RÉÉCRIRE |
| `.../ohe/internal/alerter` | `.../kairo-core/internal/rupture` | RÉÉCRIRE |
| `.../ohe/internal/api` | `.../kairo-core/internal/api` | RÉUTILISER — major edit |
| `.../ohe/internal/storage` | `.../kairo-core/internal/storage` | RÉUTILISER — strip org layer |
| `.../ohe/internal/receiver` | `.../kairo-core/internal/ingest` | RÉUTILISER — merge + rename |
| `.../ohe/internal/grpcserver` | `.../kairo-core/internal/ingest` | RÉUTILISER — merge into ingest |
| `.../ohe/internal/correlator` | `.../kairo-core/internal/fusion` | RÉÉCRIRE |
| `.../ohe/internal/notifier` | `.../kairo-core/internal/actions/providers` | RÉÉCRIRE |
| `.../ohe/internal/orchestrator` | `.../kairo-core/cmd/kairo-core` | RÉÉCRIRE |
| `.../ohe/internal/billing` | `.../kairo-core/internal/telemetry` | JETER — rewrite purpose |
| `.../ohe/internal/eventbus` | `.../kairo-core/internal/eventbus` | RÉUTILISER as-is |
| `.../ohe/internal/plugin` | `.../kairo-core/internal/actions/providers` | RÉUTILISER |
| `.../ohe/internal/vault` | `.../kairo-core/internal/vault` | RÉUTILISER as-is |
| `.../ohe/internal/processor` | `.../kairo-core/internal/pipeline/metrics` | RÉUTILISER |
| `.../ohe/internal/collector` | — | JETER |
| `.../ohe/internal/cost` | — | JETER |
| `.../ohe/pkg/logger` | `.../kairo-core/pkg/logger` | RÉUTILISER as-is |
| `.../ohe/pkg/models` | `.../kairo-core/pkg/models` | RÉÉCRIRE |
| `.../ohe/pkg/utils` | `.../kairo-core/pkg/utils` | RÉUTILISER as-is |

---

## 3. API Migration: /api/v1 → /api/v2

### 3.1 Deprecated v1 Endpoints

Removed from default routing. Accessible behind `--compat-ohe-v5` flag for 6-month window.

| v5.1 Endpoint | Status in v6 |
|--------------|-------------|
| `GET /api/v1/orgs` | REMOVED (multi-tenant gone) |
| `POST /api/v1/orgs` | REMOVED |
| `GET /api/v1/metrics` | SUPERSEDED → `GET /api/v2/rupture/{host}` |
| `GET /api/v1/kpi/mood/{host}` | SUPERSEDED → `GET /api/v2/kpi/sentiment/{host}` |
| `GET /api/v1/kpi/velocity/{host}` | REMOVED (absorbed into CA-ILR) |
| `GET /api/v1/dashboards` | REMOVED (not a dashboard platform) |
| `GET /api/v1/datasources` | REMOVED |
| `GET /api/v1/alerts` | SUPERSEDED → `GET /api/v2/ruptures` |
| `POST /api/v1/alerts` | SUPERSEDED → automatic via rupture detection |
| `GET /api/v1/slos` | REMOVED |
| `GET /api/v1/apikeys` | SIMPLIFIED → single-operator auth |
| `POST /api/v1/notifications` | SUPERSEDED → `GET /api/v2/actions` |
| `POST /api/v1/write` | SUPERSEDED → `POST /api/v2/write` |
| `GET /api/v1/openapi.yaml` | SUPERSEDED → `GET /api/v2/openapi.yaml` |

### 3.2 New v2 Endpoints

| v6 Endpoint | Purpose | v5.1 Equivalent |
|------------|---------|----------------|
| `GET /api/v2/rupture/{host}` | Current rupture state + R(t) | None |
| `GET /api/v2/rupture/{host}/history` | Rupture Index timeline | None |
| `GET /api/v2/rupture/{host}/profile` | Surge profile classification | None |
| `GET /api/v2/ruptures` | All active ruptures | None |
| `POST /api/v2/forecast` | Batch forecast request | Partial |
| `GET /api/v2/forecast/{metric}/{host}` | Single metric forecast | Partial |
| `GET /api/v2/kpi/{name}/{host}` | Composite signal value | Partial (different names) |
| `GET /api/v2/actions` | List recent actions | None |
| `POST /api/v2/actions/{id}/approve` | Approve Tier-2 action | None |
| `POST /api/v2/actions/emergency-stop` | Kill switch | None |
| `POST /api/v2/suppressions` | Create suppression window | None |
| `POST /api/v2/context` | Set manual context | None |
| `GET /api/v2/explain/{rupture_id}` | Full XAI trace | Partial |
| `GET /api/v2/explain/{rupture_id}/formula` | Formula audit | None |
| `POST /api/v2/write` | Prometheus remote_write (primary) | `POST /api/v1/write` |
| `POST /api/v2/v1/logs` | OTLP/HTTP logs | `POST /api/v1/otlp/v1/logs` |
| `POST /api/v2/v1/traces` | OTLP/HTTP traces | None |
| `GET /timeline` | Native HTML prediction timeline | None |

---

## 4. KPI / Composite Signal Renaming

| v5.1 KPI Name | v6.0 Name | Change | WP Section |
|--------------|-----------|--------|------------|
| `mood` | `sentiment` | RENAMED — remove anthropomorphic language | §11.9 |
| `velocity` | — | REMOVED — absorbed into CA-ILR burst tracker | — |
| `stress` | `stress` | Unchanged | §11.3 |
| `fatigue` | `fatigue` | Unchanged | §11.4 |
| `pressure` | `pressure` | Unchanged | §11.5 |
| `contagion` | `contagion` | Improved (graph now from traces, not correlations) | §11.6 |
| `resilience` | `resilience` | Unchanged | §11.7 |
| `entropy` | `entropy` | Unchanged | §11.8 |
| `healthscore` | `healthscore` | Unchanged | §11.10 |

API path: `/api/v1/kpi/mood/{host}` → `/api/v2/kpi/sentiment/{host}`

---

## 5. Configuration YAML Migration

### 5.1 Key Mapping

| v5.1 Config Key | v6.0 Config Key | Change |
|----------------|----------------|--------|
| `server.port` | `ingest.http_port` | Renamed |
| `storage.path` | `storage.path` | Unchanged |
| `predictor.stable_window` | `predictor.stable_window` | Unchanged (60m default) |
| `predictor.burst_window` | `predictor.burst_window` | Unchanged (5m default) |
| `predictor.rupture_threshold` | `predictor.rupture_threshold` | Unchanged (3.0 default) |
| `fatigue.r_threshold` | `composites.fatigue.r_threshold` | Moved under composites |
| `fatigue.lambda` | `composites.fatigue.lambda` | Moved under composites |
| `auth.jwt_secret` | `auth.jwt_secret` | Unchanged |
| `orgs.*` | — | REMOVED (single-tenant) |
| `billing.*` | — | REMOVED |
| — | `mode` | NEW: `connected \| stateless \| shadow` |
| — | `actions.*` | NEW: action engine config |
| — | `context.*` | NEW: context awareness config |
| — | `fusion.*` | NEW: Bayesian fusion config |
| — | `outputs.*` | NEW: Grafana, k8s events, webhook |
| — | `telemetry.*` | NEW: self-observability config |

---

## 6. Storage Key Schema Migration

### 6.1 v5.1 Schema (multi-tenant)

```
o:{orgID}:m:{host}:{metric}:{ts}
o:{orgID}:k:{host}:{kpi}:{ts}
o:{orgID}:a:{id}
o:{orgID}:d:{id}
o:{orgID}:ds:{id}
o:{orgID}:nc:{id}
o:{orgID}:slo:{id}
o:{orgID}:ak:{id}
o:{orgID}:l:{service}:{ts}
o:{orgID}:sp:{traceID}:{spanID}
```

### 6.2 v6.0 Schema (single-tenant)

```
m:{host}:{metric}:{ts}
r:{id}
r:{host}:history:{ts}
kpi:{name}:{host}:{ts}
fc:{metric}:{host}:{ts}
ac:{id}
sp:{traceID}:{spanID}
l:{service}:{ts}
ctx:{id}
sup:{id}
```

### 6.3 Migration Strategy

On first v6 boot with existing v5.1 BadgerDB:
1. Detect `o:` prefixed keys → v5.1 schema
2. Strip `o:{defaultOrgID}:` prefix from all keys
3. Drop deprecated keys: dashboards, datasources, SLOs, notification channels, org API keys
4. Log migration summary to stderr
5. Continue normal operation

---

## 7. SDK Migration

| v5.1 SDK | v6.0 SDK | Migration Path |
|---------|---------|---------------|
| `ohe-sdk-go` (Go) | `kairo-client-go` | New module path; endpoints → v2 |
| `ohe-sdk` (Python, pip) | `kairo-client` | New package name; endpoints → v2 |

6-month overlap: v5.1 SDKs work against v6 via `--compat-ohe-v5` flag.

---

## 8. Symbol Renaming (Global Find-Replace)

| Old Symbol | New Symbol | Scope |
|-----------|-----------|-------|
| `OHE` | `Kairo` | Exported types, constants |
| `ohe` | `kairo` | Package names, config keys, string literals |
| `OheConfig` | `KairoConfig` | Config struct |
| `NewOHE(` | `NewKairo(` | Constructors |
| `"ohe"` | `"kairo-core"` | Binary name, logger name |
| `version = "5.1.x"` | `version = "6.0.0"` | Version constant |
| `AgentService` (gRPC) | `KairoAgentService` | proto service name |
| `KPI.Mood` | `KPI.Sentiment` | Model field |
| `KPI.Velocity` | — | Delete field |
| `/api/v1/` | `/api/v2/` | Router paths (except compat) |
| `org_id` | — | Remove from context/middleware |

---

## 9. Removed Concepts

| OHE v5.1 Concept | Reason Removed |
|-----------------|---------------|
| Multi-tenancy (Orgs) | Kairo is single-tenant sidecar — WP §3.1 |
| Dashboards | Not a dashboard platform — WP §1.3 |
| Datasources | Not a data routing layer |
| SLOs | Out of scope |
| Alert rules (manual CRUD) | Replaced by automatic rupture detection |
| Notification channels (CRUD) | Replaced by action providers in kairo.yaml |
| Billing metering hooks | Replaced by /metrics self-observability |
| Quota enforcement (HTTP 402) | No longer multi-tenant |
| RBAC (per-org roles) | Simplified to single-operator auth |

---

## 10. Files to Delete (Phase 0)

```
internal/collector/            (JETER — entire package)
internal/billing/              (JETER — entire package)
internal/cost/                 (JETER — empty package)
internal/api/handlers_audit.go (JETER — org-scoped audit)
internal/api/handlers_rbac.go  (JETER — per-org RBAC)
internal/api/handlers_proxy.go (REVIEW — likely JETER)
```

---

Produced: 2026-04-24
Next: SPECS.md (extract all formulas, endpoints, schemas, metrics from whitepaper)

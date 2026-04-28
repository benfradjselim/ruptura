# AUDIT.md — OHE v5.1 → Kairo Core v6.0.0

Document ID: KC-AUDIT-001  
Date: April 2026  
Status: Canonical — Phase -1 Output  
Produced by: Orchestrator (Claude Code)

---

## 1. Executive Summary

OHE v5.1 contains **~8,000 LOC of Go** across 17 internal packages, 3 pkg packages, and SDK stubs.
The key architectural asset is the **predictor package** (~2,300 LOC): CA-ILR, ARIMA, HoltWinters,
Ensemble — this is the mathematical core and is fully reusable.

The **storage**, **receiver**, **grpcserver**, and **api** packages are reusable with adaptation.
The **notifier**, **analyzer**, and **alerter** packages need rewriting.
The **cost**, **billing** (webhook-flush metering), and **collector** packages are deprecated or repurposed.

OHE was multi-tenant (per-org isolation). Kairo Core v6 is single-tenant (sidecar per cluster).
This is the **most impactful architectural change** — org isolation code in storage and API must be stripped.

---

## 2. Package-by-Package Audit Table

| Package | Path | LOC (approx) | Action | v6 Destination | Changes Required | WP Section |
|---------|------|-------------|--------|---------------|-----------------|------------|
| **predictor** | `internal/predictor/` | 2,299 | **RÉUTILISER** | `internal/pipeline/metrics/` | Rename pkg, remove OHE-specific wrappers, expose CA-ILR as `pkg/rupture/` public formula | §6, §7, §8, AppB |
| **analyzer** | `internal/analyzer/` | ~500 | **RÉÉCRIRE** | `internal/pipeline/traces/` | Keep topology graph logic; replace ILR-cluster analysis with trace bottleneck detection per §8.3 | §8 |
| **alerter** | `internal/alerter/` | ~400 | **RÉÉCRIRE** | `internal/rupture/` | Rupture detection, surge profiles, event emission; grouping engine → cascade suppression per §21.2 | §5, §21 |
| **api** | `internal/api/` | ~3,500 | **RÉUTILISER** | `internal/api/` | Strip /api/v1/* (keep behind --compat flag); implement /api/v2/* per §16.1; remove multi-tenant org handlers; add rupture/forecast/actions/explain/context endpoints | §16 |
| **storage** | `internal/storage/` | ~800 | **RÉUTILISER** | `internal/storage/` | Remove org isolation (no more `o:{orgID}:` prefix); simplify to single-tenant; keep BadgerDB, retention, audit trail | §14.1 |
| **receiver** | `internal/receiver/` | ~300 | **RÉUTILISER** | `internal/ingest/` | Merge with grpcserver + collector; rename to ingest; add Prometheus remote_write (Snappy protobuf) as primary path | §15.1 |
| **grpcserver** | `internal/grpcserver/` | ~250 | **RÉUTILISER** | `internal/ingest/` | Merge into ingest; keep proto definitions; update service name OHE → Kairo | §15.1 |
| **collector** | `internal/collector/` | ~350 | **JETER** | — | Kairo Core does not self-collect system metrics; receives from Prometheus/OTel. Disk/cgroup/container collectors are out-of-scope per §1.3 | — |
| **correlator** | `internal/correlator/` | ~300 | **RÉÉCRIRE** | `internal/fusion/` | Burst correlator → Bayesian fusion engine per §9; add multi-signal time alignment and conflict detection | §9 |
| **notifier** | `internal/notifier/` | ~200 | **RÉÉCRIRE** | `internal/actions/providers/` | Extend to 4 providers: Kubernetes, Webhook, Alertmanager, PagerDuty; add safety gates per §12.5 | §12 |
| **orchestrator** | `internal/orchestrator/` | 612 | **RÉÉCRIRE** | `cmd/kairo-core/` | Main binary wiring; new config schema (kairo.yaml per §17); mode: connected/stateless/shadow; billing → telemetry | §14, §17 |
| **billing** | `internal/billing/` | ~150 | **JETER** | `internal/telemetry/` | Billing metering (per-org webhooks) is removed. Self-monitoring metrics go to telemetry package per §22 | §22 |
| **eventbus** | `internal/eventbus/` | ~200 | **RÉUTILISER** | `internal/eventbus/` | Keep as-is; used for internal event propagation; extend for rupture events | §14 |
| **plugin** | `internal/plugin/` | ~150 | **RÉUTILISER** | `internal/actions/providers/` | Plugin sandbox usable as extension point for custom action providers | §12.4 |
| **vault** | `internal/vault/` | ~100 | **RÉUTILISER** | `internal/vault/` | Keep as-is; Vault integration for secret management retained | §17 |
| **cost** | `internal/cost/` | 0 | **JETER** | — | Empty package; no content | — |
| **processor** | `internal/processor/` | ~150 | **RÉUTILISER** | `internal/pipeline/metrics/` | Signal preprocessing; adapt for multi-signal (metrics/logs/traces) | §8 |
| **pkg/logger** | `pkg/logger/` | ~200 | **RÉUTILISER** | `pkg/logger/` | Keep as-is; zero-dep structured logger, Go 1.18 compat | §14 |
| **pkg/models** | `pkg/models/` | ~300 | **RÉÉCRIRE** | `pkg/models/` | Remove Org/APIKey/Dashboard/SLO models; add RuptureEvent, ActionRecommendation, CompositeSignal, ForecastResult | §16 |
| **pkg/utils** | `pkg/utils/` | ~100 | **RÉUTILISER** | `pkg/utils/` | Keep utilities; minor cleanup | — |
| **sdk/go** | `workdir/sdk/go` | ~200 | **RÉÉCRIRE** | `sdk/go/` `pkg/client/` | Rename ohe-sdk-go → kairo-client-go; update endpoints to v2 API; publish to pkg.go.dev | §14.2 |
| **sdk/python** | `sdk/python/` | ~150 | **RÉÉCRIRE** | `sdk/python/` | Rename ohe-sdk → kairo-client; update to v2 API | §14.2 |

---

## 3. New Packages (No OHE Equivalent)

| v6 Package | Path | Source | WP Section |
|-----------|------|--------|------------|
| **ingest** | `internal/ingest/` | Merge: receiver + grpcserver | §15.1 |
| **pipeline/metrics** | `internal/pipeline/metrics/` | From: predictor + processor | §8.1 |
| **pipeline/logs** | `internal/pipeline/logs/` | Partial: collector/logs → full log pipeline | §8.2 |
| **pipeline/traces** | `internal/pipeline/traces/` | From: analyzer/topology → extended | §8.3 |
| **fusion** | `internal/fusion/` | From: correlator → rewrite Bayesian | §9 |
| **rupture** | `internal/rupture/` | From: alerter → rewrite + surge profiles | §5, §6 |
| **composites** | `internal/composites/` | From: analyzer → extend with all 8 KPIs | §11 |
| **context** | `internal/context/` | NEW — no OHE equivalent | §10 |
| **actions/engine** | `internal/actions/engine/` | NEW — tier taxonomy, rule engine | §12.2 |
| **actions/providers** | `internal/actions/providers/` | From: notifier → extend | §12.4 |
| **actions/arbitration** | `internal/actions/arbitration/` | NEW | §12.3 |
| **actions/safety** | `internal/actions/safety/` | NEW — rate limiting, cooldown, rollback | §12.5 |
| **explain** | `internal/explain/` | From: handlers_explain.go → extract to package | §13 |
| **telemetry** | `internal/telemetry/` | NEW (from billing + self-metrics) | §22 |
| **pkg/rupture** | `pkg/rupture/` | NEW — public formula package | §6 |
| **pkg/composites** | `pkg/composites/` | NEW — public composite formulas | §11 |

---

## 4. Technical Debt Inventory

### 4.1 Missing Tests / Coverage Gaps

| Package | Current Coverage | v6 Target | Gap |
|---------|-----------------|-----------|-----|
| `internal/api` | 58.6% | ≥70% | Need v2 endpoint tests |
| `internal/orchestrator` | 64% | ≥75% | Main wiring coverage low |
| `internal/predictor` | ~65% | ≥80% | Ensemble + ARIMA untested paths |
| `internal/notifier` | ~45% | ≥80% (as providers) | Major rewrite needed |
| `internal/correlator` | ~55% | ≥80% (as fusion) | Bayesian logic new |
| `internal/alerter` | 89% | ≥80% (as rupture) | Good baseline to maintain |
| `pkg/logger` | 86.5% | ≥80% | Already green |
| `internal/billing` | 81.4% | N/A (JETER) | — |
| **Total (v5.1)** | 61.1% | **≥70% (v6 goal)** | +9 points needed |

### 4.2 Dependency Audit

| Dependency | Status | Action |
|-----------|--------|--------|
| `golang.org/x/exp` | Experimental; slices usage causes Go 1.21+ compat issue | Audit usages; remove or replace with stdlib equivalents |
| `google.golang.org/grpc v1.55.0-dev` | Dev release | Pin to stable `v1.64.0` |
| `gorilla/mux v1.8.1` | Replaced by stdlib mux in Go 1.22, but project is Go 1.18 | Keep; OK for now |
| `gorilla/websocket v1.5.1` | Used for WebSocket timeline | Keep; timeline feature retained |
| `dgraph-io/badger/v3` | Core storage; stable | Keep |

### 4.3 Code Debt Hotspots

| File | Issue | Action |
|------|-------|--------|
| `internal/api/handlers.go` (1924 LOC) | Exceeds 800-line limit; multi-tenant org logic mixed with prediction | Split into: handlers_ingest, handlers_rupture, handlers_kpi, handlers_actions, handlers_explain |
| `internal/orchestrator/orchestrator.go` (612 LOC) | Wires everything; too many responsibilities | Extract to: config, server, lifecycle submodules |
| `internal/storage/org_store.go` | Org isolation prefix `o:{orgID}:` throughout | Remove org dimension; simplify key schema |
| `internal/api/compat.go` | OHE v1 compat shim | Keep behind `--compat-ohe-v5` flag |

### 4.4 Multi-Tenant Removal Scope

OHE v5.1 was a multi-tenant SaaS platform (per-org isolation). Kairo Core v6 is a **single-tenant sidecar**.
This is the most pervasive breaking change:

Affected files:
- `internal/api/handlers.go` — all org-scoped handlers (`/api/v1/orgs/...`)
- `internal/api/middleware.go` — AuthMiddleware sets `org_id` in context
- `internal/storage/org_store.go` — entire OrgStore abstraction
- `internal/storage/audit.go` — org-scoped audit log
- `pkg/models/models.go` — Org, APIKey, QuotaConfig models
- `internal/billing/billing.go` — per-org usage events

Action: Strip all org-scoped logic. Authentication becomes simple JWT/API-key for single operator.

---

## 5. Heritage Assets Worth Preserving

These are the most valuable pieces from OHE v5.1:

1. **CA-ILR predictor** (`internal/predictor/cailr.go`, `ilr.go`) — 600+ LOC of validated math; core differentiator
2. **Ensemble engine** (`internal/predictor/ensemble.go`) — 254 LOC; combines 4 models with confidence scoring
3. **ARIMA + HoltWinters** (`internal/predictor/arima.go`, `holtwinters.go`) — mature, tested implementations
4. **BadgerDB wrapper** (`internal/storage/storage.go`) — battle-tested; schema change only (remove org prefix)
5. **gRPC server** (`internal/grpcserver/`) — keep proto; rename service OHE → Kairo
6. **OTLP receiver** (`internal/receiver/otlp.go`) — keep; already handles OTLP/HTTP
7. **DogStatsD receiver** (`internal/receiver/dogstatsd.go`) — keep as-is
8. **Logger** (`pkg/logger/logger.go`) — zero-dep, Go 1.18, keep
9. **EventBus** (`internal/eventbus/eventbus.go`) — pub/sub; extend for rupture events
10. **Vault** (`internal/vault/vault.go`) — keep

---

## 6. Reuse Decision Summary

| Category | Count | Packages |
|----------|-------|---------|
| RÉUTILISER (keep/adapt) | 11 | predictor, api, storage, receiver, grpcserver, eventbus, plugin, vault, processor, pkg/logger, pkg/utils |
| RÉÉCRIRE (rewrite same logic) | 6 | alerter→rupture, analyzer→pipeline/traces, correlator→fusion, notifier→actions/providers, orchestrator→cmd, sdk/go+sdk/python |
| JETER (delete) | 3 | collector, billing, cost |
| NEW (no equivalent) | 10 | ingest, pipeline/logs, context, actions/engine, actions/arbitration, actions/safety, explain, telemetry, pkg/rupture, pkg/composites |

---

Produced: 2026-04-24  
Next: MIGRATION.md (v5.1 → v6.0 mapping), then SPECS.md, then Phase 0 cleanup.

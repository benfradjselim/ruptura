# AGENTS.md — Kairo Core v6.0.0 Agent Roles

Document ID: KC-AGENTS-001
Date: April 2026
Status: Canonical — Phase 1 Output
Produced by: Orchestrator (Claude Code)

Each agent owns specific packages, works on a dedicated branch, merges via PR.
Specs reference: SPECS.md | Formulas: SPECS.md §3-4 | API: SPECS.md §8 | Config: SPECS.md §15

---

## ALPHA — Core Engine

**Owner:** Orchestrator (Claude Code)
**Branch:** `v6_alpha`
**Phase:** 2a — first to run; all others depend on it

### Packages Owned
| Package | Path | Source |
|---------|------|--------|
| Metric pipeline | `internal/pipeline/metrics/` | FROM: `internal/predictor/` + `internal/processor/` |
| Public rupture formula | `pkg/rupture/` | NEW |

### What to Build

**`internal/pipeline/metrics/`**
- Move all files from `internal/predictor/`: cailr.go, ilr.go, ensemble.go, arima.go, holtwinters.go, anomaly_*.go, threshold.go
- Move `internal/processor/processor.go`
- Update package name and all import paths to `github.com/benfradjselim/kairo-core`
- Remove OHE multi-tenant wrappers
- Expose clean interface:
  ```go
  type MetricPipeline interface {
      Ingest(host, metric string, value float64, ts time.Time)
      RuptureIndex(host, metric string) (float64, error)
      TTF(host, metric string) (time.Duration, error)
      Confidence(host, metric string) (float64, error)
      SurgeProfile(host, metric string) (string, error)
  }
  ```

**`pkg/rupture/`** — public, importable, fully documented
- `Index(alphaBurst, alphaStable float64) float64` — SPECS.md §3.2
- `TTF(current, threshold, alphaBurst float64) time.Duration` — SPECS.md §3.3
- `Classify(r float64) string` — returns "Stable"|"Elevated"|"Warning"|"Critical"|"Emergency"

### Tests Required
- All moved predictor tests pass with updated imports
- `pkg/rupture/`: TestIndex_stable, TestIndex_epsilon, TestIndex_emergency, TestTTF_basic, TestTTF_clamped, TestClassify_allTiers

### Interfaces Exported to Other Agents
```go
// internal/pipeline/metrics — consumed by fusion, composites, api
type MetricPipeline interface { ... }

// pkg/rupture — consumed by rupture, composites, explain
func Index(alphaBurst, alphaStable float64) float64
func TTF(current, threshold, alphaBurst float64) time.Duration
func Classify(r float64) string
```

### Commit Convention: `[ALPHA] description`
### Coverage Targets: pipeline/metrics >= 80% | pkg/rupture >= 85%

---

## BRAVO — Signal Pipelines

**Owner:** OpenCode
**Branch:** `v6_bravo`
**Phase:** 2b — parallel with CHARLIE; starts after ALPHA CI-green
**Mission file:** `/tmp/kairo_mission_BRAVO_1.md`

### Packages Owned
| Package | Path | Source |
|---------|------|--------|
| Ingest layer | `internal/ingest/` | FROM: receiver + grpcserver (merged) |
| Log pipeline | `internal/pipeline/logs/` | FROM: collector/logs skeleton → full rewrite |
| Trace pipeline | `internal/pipeline/traces/` | FROM: analyzer/topology → extended |

### What to Build

**`internal/ingest/`**
- Merge `internal/receiver/` (OTLP HTTP, DogStatsD UDP) and `internal/grpcserver/` (gRPC push)
- Add Prometheus remote_write receiver (PRIMARY per SPECS.md §8.9):
  - `POST /api/v2/write` — Snappy protobuf, Prometheus WriteRequest format
  - Decode WriteRequest → iterate TimeSeries → forward to MetricPipeline.Ingest()
- Interface:
  ```go
  type Ingestor interface {
      StartHTTP(addr string) error
      StartGRPC(addr string) error
      StartDogStatsD(addr string) error
      Stop(ctx context.Context) error
  }
  ```

**`internal/pipeline/logs/`** — SPECS.md §10
- Extractors (each produces numeric time series fed to MetricPipeline):
  - `ErrorRateExtractor`: counts ERROR/FATAL/CRITICAL per 15s bucket
  - `KeywordCounter`: configurable regex patterns
  - `BurstDetector`: volume per bucket vs short-term baseline
  - `NoveltyScorer`: disabled by default (experimental)
- Interface:
  ```go
  type LogPipeline interface {
      IngestLine(service string, line []byte, ts time.Time)
      IngestOTLP(ctx context.Context, req *otlplogs.ExportRequest) error
  }
  ```

**`internal/pipeline/traces/`** — SPECS.md §11
- `TopologyBuilder`: builds service dependency graph from spans
- Four analyzers (each produces numeric time series):
  - `LatencyPropagationAnalyzer`: propagation_factor(t)
  - `BottleneckScoreAnalyzer`: bottleneck_index(t); critical_path_pct threshold=0.3
  - `ErrorCascadeAnalyzer`: cascade_index(t) — exact formula SPECS.md §11
  - `FanoutPressureAnalyzer`: fanout_stress(t); threshold=50 calls/span
- Interface:
  ```go
  type TracePipeline interface {
      IngestSpan(span Span) error
      IngestOTLP(ctx context.Context, req *otlptrace.ExportRequest) error
      CascadeIndex(host string) (float64, error)
      DependencyGraph(host string) ([]Edge, error)
  }
  ```

### Tests Required
- ingest: HTTP WriteRequest handler test, DogStatsD parse test, gRPC ingest test
- logs: table-driven tests per extractor (known input → expected output)
- traces: topology builder test, cascade_index formula verification (SPECS.md §11)

### Interfaces Exported to Other Agents
```go
// consumed by api (ECHO), fusion (CHARLIE), composites (CHARLIE)
type Ingestor interface { ... }
type LogPipeline interface { ... }
type TracePipeline interface { ... }
type Span struct { TraceID, SpanID, ParentID string; Service string; StartTime, EndTime time.Time; Error bool }
type Edge struct { From, To string; Weight float64 }
```

### Commit Convention: `[BRAVO] description`
### Coverage Targets: ingest >= 80% | pipeline/logs >= 80% | pipeline/traces >= 80%

---

## CHARLIE — Fusion + Composites

**Owner:** OpenCode
**Branch:** `v6_charlie`
**Phase:** 2c — parallel with BRAVO; starts after ALPHA CI-green
**Mission file:** `/tmp/kairo_mission_CHARLIE_1.md`

### Packages Owned
| Package | Path | Source |
|---------|------|--------|
| Signal fusion | `internal/fusion/` | FROM: correlator → rewrite |
| Composite signals | `internal/composites/` | FROM: analyzer → extend to all 8 |
| Public composites | `pkg/composites/` | NEW |

### What to Build

**`internal/fusion/`** — WP §9 + SPECS.md §3.4
- WP gap: Bayesian algorithm not fully specified → implement weighted average as v6.0 default:
  ```
  R_fused(t) = 0.6*R_metric(t) + 0.2*R_log(t) + 0.2*R_trace(t)
  ```
- Time alignment: reject signals with timestamp lag > 30s
- Conflict detection: flag divergence > 2.0 between any two pipeline R values
- Interface:
  ```go
  type FusionEngine interface {
      SetMetricR(host string, r float64, ts time.Time)
      SetLogR(host string, r float64, ts time.Time)
      SetTraceR(host string, r float64, ts time.Time)
      FusedR(host string) (float64, time.Time, error)
  }
  ```

**`internal/composites/`** — SPECS.md §4 (all 8 formulas, exact)
- Stress, Fatigue (stateful accumulator), Pressure, Contagion (needs dependency graph),
  Resilience, Entropy, Sentiment, HealthScore
- Interface:
  ```go
  type CompositeEngine interface {
      Stress(host string) (float64, error)
      Fatigue(host string) (float64, error)
      Pressure(host string) (float64, error)
      Contagion(host string) (float64, error)
      Resilience(host string) (float64, error)
      Entropy(host string) (float64, error)
      Sentiment(host string) (float64, error)
      HealthScore(host string) (float64, error)
  }
  ```

**`pkg/composites/`** — pure exported functions, no state
- `Stress(factors []WeightedFactor) float64`
- `Fatigue(prev, deltaStress, lambda float64) float64`
- `Pressure(latencyZ, errorZ, wLat, wErr float64) float64`
- `HealthScore(stress, fatigue, pressure, contagion float64) float64`
- All formulas per SPECS.md §4

### Tests Required
- fusion: weighted average correctness, time-alignment rejection (lag > 30s), conflict detection
- composites: each formula verified against SPECS.md values; Fatigue half-life test (t_half = ln(2)/lambda); HealthScore always in [0,100]
- pkg/composites: pure function tests, deterministic with same inputs

### Interfaces Exported to Other Agents
```go
// consumed by api (ECHO), actions/engine (DELTA), explain (DELTA), telemetry (ECHO)
type FusionEngine interface { ... }
type CompositeEngine interface { ... }
```

### Commit Convention: `[CHARLIE] description`
### Coverage Targets: fusion >= 80% | composites >= 80% | pkg/composites >= 85%

---

## DELTA — Actions + Explainability

**Owner:** OpenCode
**Branch:** `v6_delta`
**Phase:** 3 — starts after ALPHA + BRAVO + CHARLIE all CI-green
**Mission file:** `/tmp/kairo_mission_DELTA_1.md`

### Packages Owned
| Package | Path | Source |
|---------|------|--------|
| Action engine | `internal/actions/engine/` | NEW |
| Action providers | `internal/actions/providers/` | FROM: notifier → extend |
| Arbitration | `internal/actions/arbitration/` | NEW |
| Safety gates | `internal/actions/safety/` | NEW |
| Explainability | `internal/explain/` | FROM: handlers_explain.go → extract + extend |

### What to Build

**`internal/actions/engine/`** — SPECS.md §12
- Rule evaluator: reads `rules.yaml`, evaluates (R, profile, confidence) → ActionRecommendation
- Tier determinator: maps per SPECS.md §12.1 table

**`internal/actions/providers/`** — SPECS.md §12.3
- `WebhookProvider`: HTTP POST rupture context; most portable
- `AlertmanagerProvider`: push alert via Alertmanager webhook API
- `KubernetesProvider`: scale/restart/cordon (requires k8s client-go; stub OK if dependency adds complexity)
- `PagerDutyProvider`: incident creation via Events API v2
- Interface: `type Provider interface { Execute(ctx context.Context, a Action) error; Name() string }`

**`internal/actions/arbitration/`**
- Deduplication: suppress duplicate (host, action_type) within cooldown window
- Priority queue: Tier 1 > Tier 2 > Tier 3; within tier by confidence desc

**`internal/actions/safety/`** — SPECS.md §12.1 safety gates
- Rate limiter: token bucket; default 6 Tier-1 actions/target/hour
- Cooldown tracker: per (host, action_type)
- Rollback trigger: monitor R post-action; R_new > R_old → rollback
- Emergency stop: atomic bool; `POST /api/v2/actions/emergency-stop` sets it
- Shadow mode: config flag; log without executing

**`internal/explain/`** — SPECS.md §8.7 + WP §13
- `Explain(ruptureID)`: which metrics contributed, normalized weights, pipeline that fired first
- `FormulaAudit(ruptureID)`: exact computation trace with intermediate values
- `PipelineDebug(ruptureID)`: per-pipeline R values at event time
- Interface:
  ```go
  type Explainer interface {
      Explain(ruptureID string) (*ExplainResponse, error)
      FormulaAudit(ruptureID string) (*FormulaAuditResponse, error)
      PipelineDebug(ruptureID string) (*PipelineDebugResponse, error)
  }
  ```

### Tests Required
- engine: rule evaluation from YAML, tier mapping for all R/confidence combos
- providers: WebhookProvider with httptest server; others with mocks
- arbitration: dedup within cooldown, priority ordering
- safety: rate limit enforcement (7th action blocked), emergency stop prevents execution
- explain: Explain response contains contributing metrics; FormulaAudit has all intermediate values

### Interfaces Exported to Other Agents
```go
// consumed by api (ECHO)
type ActionEngine interface { Recommend(event RuptureEvent) ([]ActionRecommendation, error); EmergencyStop() }
type Explainer interface { ... }
```

### Commit Convention: `[DELTA] description`
### Coverage Targets: actions/* >= 75% each | explain >= 75%

---

## ECHO — API + Integrations

**Owner:** OpenCode
**Branch:** `v6_echo`
**Phase:** 4 — starts after DELTA CI-green
**Mission file:** `/tmp/kairo_mission_ECHO_1.md`

### Packages Owned
| Package | Path | Source |
|---------|------|--------|
| REST API | `internal/api/` | FROM: existing → major rewrite |
| Context awareness | `internal/context/` | NEW |
| Self-telemetry | `internal/telemetry/` | NEW |
| Storage | `internal/storage/` | FROM: existing → strip org layer |

### What to Build

**`internal/api/`** — SPECS.md §8 (all 44 endpoints)
- Split into focused handler files (max 400 LOC each):
  - `handlers_ingest.go`, `handlers_rupture.go`, `handlers_kpi.go`
  - `handlers_forecast.go`, `handlers_actions.go`, `handlers_context.go`
  - `handlers_explain.go`, `handlers_health.go`, `handlers_compat.go`
- Strip all org-scoped handlers
- Auth middleware: JWT or API-key (single-operator, no multi-tenant)
- `--compat-ohe-v5` flag wires `/api/v1/*` routes

**`internal/context/`** — SPECS.md §13
- `TimeOfDayManager`: 24 hourly buckets; lambda_context per bucket
- `DayOfWeekManager`: weekday / weekend profiles
- `DeploymentDetector`: k8s events or Prometheus kube_* metrics; 60s pre + 300s post suppression
- `ManualContextStore`: CRUD with TTL; types: load_test, maintenance_window, incident_active, abnormal_traffic
- Interface:
  ```go
  type ContextManager interface {
      CurrentLambda(host string, t time.Time) float64
      IsSuppressionActive(host string) bool
      SetManualContext(c ManualContext) error
      ActiveContexts() []ManualContext
  }
  ```

**`internal/telemetry/`** — SPECS.md §9
- Register all 14 Prometheus metrics from SPECS.md §9
- Prometheus text-format handler at `/api/v2/metrics`
- Health handler at `/api/v2/health` with schema from SPECS.md §13
- Readiness handler at `/api/v2/ready`

**`internal/storage/`**
- Remove OrgStore, all `o:{orgID}:` key prefixes
- Implement single-tenant key schema (SPECS.md §14)
- Keep BadgerDB backend, retention, compaction
- Interface:
  ```go
  type Store interface {
      WriteMetric(host, metric string, value float64, ts time.Time) error
      WriteRupture(e RuptureEvent) error
      ReadRuptureHistory(host string, since time.Time) ([]RuptureEvent, error)
      WriteKPI(name, host string, value float64, ts time.Time) error
      ReadKPIHistory(name, host string, since time.Time) ([]KPIPoint, error)
      WriteAction(a ActionRecord) error
      WriteContext(c ContextEntry) error
      WriteSupression(s Suppression) error
  }
  ```

### Tests Required
- api: integration tests for all v2 endpoint groups (httptest); at least one test per handler file
- context: TimeOfDayManager lambda selection, DeploymentDetector suppression window, ManualContextStore TTL expiry
- telemetry: Prometheus metrics registered and scraped; health schema matches SPECS.md §13
- storage: write/read round-trip for each key type; key schema correctness

### Commit Convention: `[ECHO] description`
### Coverage Targets: api >= 70% | context >= 75% | telemetry >= 75% | storage >= 70%

---

## FOXTROT — Operability & Final Integration

**Owner:** Orchestrator (Claude Code)
**Branch:** `v6_foxtrot`
**Phase:** 5 — starts after ECHO CI-green

### Packages Owned
| Package | Path | Action |
|---------|------|--------|
| Main binary | `cmd/kairo-core/` | RÉÉCRIRE |
| Event bus | `internal/eventbus/` | RÉUTILISER + extend for rupture events |
| Vault | `internal/vault/` | RÉUTILISER as-is |
| Go SDK | `sdk/go/` | RÉÉCRIRE (rename + v2 API) |
| Python SDK | `sdk/python/` | RÉÉCRIRE (rename + v2 API) |

### What to Build

**`cmd/kairo-core/main.go`**
- Parse `kairo.yaml` (all sections per SPECS.md §15)
- Wire all packages: Ingestor → pipelines → FusionEngine → CompositeEngine → RuptureDetector → ActionEngine → API server
- Three modes: `connected` | `stateless` | `shadow`
- Graceful shutdown on SIGTERM/SIGINT
- `--compat-ohe-v5` flag to enable v1 compat routes

**SDK updates**
- `sdk/go/`: rename module `ohe-sdk-go` → `kairo-client-go`; typed clients for all v2 endpoints
- `sdk/python/`: rename package `ohe` → `kairo`; update all v2 endpoint paths

**Smoke test (must pass before merge)**
```bash
./kairo-core --config configs/kairo.yaml &
sleep 2
curl -sf http://localhost:8080/api/v2/health | grep -q '"status"'
curl -sf http://localhost:8080/api/v2/ready
curl -sf http://localhost:8080/api/v2/metrics | grep -q 'kairo_uptime_seconds'
```

### Commit Convention: `[FOXTROT] description`
### Coverage Targets: total >= 70% | binary <= 25 MB

---

## Agent Communication Protocol

| Mechanism | Purpose |
|-----------|---------|
| Git branch + PR | Code delivery and review |
| Go interfaces (defined above) | Cross-package contracts |
| `/tmp/kairo_mission_{ROLE}_{N}.md` | Orchestrator → OpenCode task assignment |
| `/tmp/kairo_mission_{ROLE}_{N}_done.md` | OpenCode → Orchestrator completion signal |
| GitHub Actions CI | Objective green/red gate per phase |

The Orchestrator reads done files, verifies CI status on the agent's branch, then creates the next mission file or proceeds to the next phase.

---

Produced: 2026-04-24

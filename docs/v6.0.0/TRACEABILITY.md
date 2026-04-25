# TRACEABILITY.md — Kairo Core v6.0.0

Document ID: KC-TRACE-001
Date: April 2026
Status: Canonical — Phase 1 Output | Updated per phase completion
Produced by: Orchestrator (Claude Code)

Matrix: WP Section <-> Package <-> Test File <-> Agent <-> Phase <-> Status

---

## 1. Core Engine

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §6.2 | ELS Stable Tracker (lambda=0.95) | `internal/pipeline/metrics/cailr.go` | `cailr_test.go` | ALPHA | 2a | PENDING |
| §6.2 | ELS Burst Tracker (lambda=0.80) | `internal/pipeline/metrics/cailr.go` | `cailr_test.go` | ALPHA | 2a | PENDING |
| §6.3 | Rupture Index formula | `pkg/rupture/rupture.go` | `rupture_test.go` | ALPHA | 2a | PENDING |
| §6.4 | Rupture classification (5 tiers) | `pkg/rupture/rupture.go` | `rupture_test.go` | ALPHA | 2a | PENDING |
| §5.3 | TTF formula + clamp to 3600s | `pkg/rupture/rupture.go` | `rupture_test.go` | ALPHA | 2a | PENDING |
| §7.2 | ARIMA(1,1,1) | `internal/pipeline/metrics/arima.go` | `arima_test.go` | ALPHA | 2a | PENDING |
| §7.2 | Holt-Winters damped phi=0.98 | `internal/pipeline/metrics/holtwinters.go` | `holtwinters_test.go` | ALPHA | 2a | PENDING |
| §7.2 | MAD Anomaly Guard | `internal/pipeline/metrics/anomaly_mad.go` | `anomaly_mad_test.go` | ALPHA | 2a | PENDING |
| §7.2 | Ensemble weights (CA-ILR=0.40) | `internal/pipeline/metrics/ensemble.go` | `ensemble_test.go` | ALPHA | 2a | PENDING |
| §7.3 | Confidence C(t) = 1 - sigma2/mu | `internal/pipeline/metrics/ensemble.go` | `ensemble_test.go` | ALPHA | 2a | PENDING |
| §7.4 | Forecast horizons: 5m/15m/60m | `internal/pipeline/metrics/ensemble.go` | `ensemble_test.go` | ALPHA | 2a | PENDING |
| §5.5 | Surge profile classification | `internal/pipeline/metrics/surgeprofile.go` | `surgeprofile_test.go` | ALPHA | 2a | PENDING |

---

## 2. Ingest Layer

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §15.1 | Prometheus remote_write (Snappy) | `internal/ingest/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §15.1 | OTLP/HTTP metrics | `internal/ingest/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §15.1 | OTLP/HTTP logs | `internal/ingest/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §15.1 | OTLP/HTTP traces | `internal/ingest/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §15.1 | DogStatsD UDP | `internal/ingest/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §15.1 | gRPC push (v6.1 stub) | `internal/ingest/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §8.4 | Cardinality: max 50k streams | `internal/ingest/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |

---

## 3. Log Pipeline

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §8.2 | ErrorRateExtractor (15s bucket) | `internal/pipeline/logs/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §8.2 | KeywordCounter regex | `internal/pipeline/logs/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §8.2 | BurstDetector | `internal/pipeline/logs/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §8.2 | NoveltyScorer (disabled default) | `internal/pipeline/logs/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |

---

## 4. Trace Pipeline

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §8.3 | TopologyBuilder | `internal/pipeline/traces/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §8.3 | LatencyPropagationAnalyzer | `internal/pipeline/traces/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §8.3 | BottleneckScoreAnalyzer (pct=0.3) | `internal/pipeline/traces/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §8.3 | ErrorCascadeAnalyzer (cascade_index formula) | `internal/pipeline/traces/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §8.3 | FanoutPressureAnalyzer (threshold=50) | `internal/pipeline/traces/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |
| §8.3 | Topology: max_services=500, min_samples=100 | `internal/pipeline/traces/engine.go` | `engine_test.go` | BRAVO | 2b | CI_GREEN |

---

## 5. Signal Fusion

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §9 | R_fused = 0.6*R_metric + 0.2*R_log + 0.2*R_trace | `internal/fusion/fusion.go` | `fusion_test.go` | CHARLIE | 2c | CI_GREEN |
| §9 | Time alignment: reject lag > 30s | `internal/fusion/fusion.go` | `fusion_test.go` | CHARLIE | 2c | CI_GREEN |
| §9 | Conflict detection: divergence > 2.0 | `internal/fusion/fusion.go` | `fusion_test.go` | CHARLIE | 2c | CI_GREEN |

---

## 6. Composite Signals

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §11.3 | Stress (5-factor weighted) | `internal/composites/stress.go` | `composites_test.go` | CHARLIE | 2c | CI_GREEN |
| §11.4 | Fatigue accumulator (lambda=0.05) | `internal/composites/fatigue.go` | `composites_test.go` | CHARLIE | 2c | CI_GREEN |
| §11.4 | Fatigue half-life = ln(2)/lambda | `internal/composites/fatigue.go` | `composites_test.go` | CHARLIE | 2c | CI_GREEN |
| §11.5 | Pressure z-score composite | `internal/composites/pressure.go` | `composites_test.go` | CHARLIE | 2c | CI_GREEN |
| §11.6 | Contagion graph propagation | `internal/composites/contagion.go` | `composites_test.go` | CHARLIE | 2c | CI_GREEN |
| §11.7 | Resilience: 1/(1+mean(Stress,W=30m)) | `internal/composites/resilience.go` | `composites_test.go` | CHARLIE | 2c | CI_GREEN |
| §11.8 | Entropy: Shannon variance | `internal/composites/entropy.go` | `composites_test.go` | CHARLIE | 2c | CI_GREEN |
| §11.9 | Sentiment: log(N_pos+1)-log(N_neg+1) | `internal/composites/sentiment.go` | `composites_test.go` | CHARLIE | 2c | CI_GREEN |
| §11.10 | HealthScore: multiplicative [0,100] | `internal/composites/healthscore.go` | `composites_test.go` | CHARLIE | 2c | CI_GREEN |
| §11 | pkg/composites pure exported functions | `pkg/composites/composites.go` | `pkg_composites_test.go` | CHARLIE | 2c | CI_GREEN |

---

## 7. Action Engine

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §12.2 | Tier 1 (C>0.85, automated) | `internal/actions/engine/tier.go` | `engine_test.go` | DELTA | 3 | CI_GREEN |
| §12.2 | Tier 2 (C>0.60, approval) | `internal/actions/engine/tier.go` | `engine_test.go` | DELTA | 3 | CI_GREEN |
| §12.2 | Tier 3 (human only) | `internal/actions/engine/tier.go` | `engine_test.go` | DELTA | 3 | CI_GREEN |
| §12.4 | WebhookProvider | `internal/actions/providers/webhook.go` | `providers_test.go` | DELTA | 3 | CI_GREEN |
| §12.4 | AlertmanagerProvider | `internal/actions/providers/alertmanager.go` | `providers_test.go` | DELTA | 3 | CI_GREEN |
| §12.4 | KubernetesProvider | `internal/actions/providers/kubernetes.go` | `providers_test.go` | DELTA | 3 | CI_GREEN |
| §12.4 | PagerDutyProvider | `internal/actions/providers/pagerduty.go` | `providers_test.go` | DELTA | 3 | CI_GREEN |
| §12.5 | Rate limit: 6 Tier-1/target/hour | `internal/actions/safety/ratelimit.go` | `safety_test.go` | DELTA | 3 | CI_GREEN |
| §12.5 | Cooldown tracker | `internal/actions/safety/cooldown.go` | `safety_test.go` | DELTA | 3 | CI_GREEN |
| §12.5 | Rollback: R_new > R_old | `internal/actions/safety/rollback.go` | `safety_test.go` | DELTA | 3 | CI_GREEN |
| §12.5 | Emergency stop | `internal/actions/safety/emergencystop.go` | `safety_test.go` | DELTA | 3 | CI_GREEN |
| §12.5 | Shadow mode | `internal/actions/safety/shadow.go` | `safety_test.go` | DELTA | 3 | CI_GREEN |

---

## 8. Explainability

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §13.2 | Level 1: metric contribution | `internal/explain/trace.go` | `explain_test.go` | DELTA | 3 | CI_GREEN |
| §13.2 | Level 2: temporal ordering (partial) | `internal/explain/trace.go` | `explain_test.go` | DELTA | 3 | CI_GREEN |
| §13.4 | Formula audit: intermediate values | `internal/explain/formula.go` | `explain_test.go` | DELTA | 3 | CI_GREEN |
| §16.1 | GET /api/v2/explain/{id} | `internal/api/handlers_explain.go` | `api_test.go` | ECHO | 4 | PENDING |
| §16.1 | GET /api/v2/explain/{id}/formula | `internal/api/handlers_explain.go` | `api_test.go` | ECHO | 4 | PENDING |

---

## 9. API Layer

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §16.1 | GET /api/v2/rupture/{host} | `internal/api/handlers_extra.go` | `api_test.go` | ECHO | 4 | CI_GREEN |
| §16.1 | GET /api/v2/ruptures | `internal/api/handlers_extra.go` | `api_test.go` | ECHO | 4 | CI_GREEN |
| §16.1 | POST /api/v2/forecast | `internal/api/handlers_extra.go` | `api_test.go` | ECHO | 4 | CI_GREEN |
| §16.1 | GET /api/v2/kpi/{name}/{host} | `internal/api/handlers_extra.go` | `api_test.go` | ECHO | 4 | CI_GREEN |
| §16.1 | GET /api/v2/actions | `internal/api/handlers_extra.go` | `api_test.go` | ECHO | 4 | CI_GREEN |
| §16.1 | POST /api/v2/actions/emergency-stop | `internal/api/handlers_extra.go` | `api_test.go` | ECHO | 4 | CI_GREEN |
| §16.1 | POST /api/v2/suppressions | `internal/api/handlers_extra.go` | `api_test.go` | ECHO | 4 | CI_GREEN |
| §16.1 | POST /api/v2/context | `internal/api/handlers_extra.go` | `api_test.go` | ECHO | 4 | CI_GREEN |
| §16.1 | GET /api/v2/health | `internal/api/handlers_health.go` | `api_test.go` | ECHO | 4 | CI_GREEN |
| §16.1 | POST /api/v2/write | `internal/api/handlers_ingest.go` | `api_test.go` | ECHO | 4 | CI_GREEN |
| §16.1 | GET /timeline | `internal/api/handlers_health.go` | `api_test.go` | ECHO | 4 | CI_GREEN |

---

## 10. Context Awareness

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §10.1 | Time-of-day: 24 buckets | `internal/context/timeofday.go` | `context_test.go` | ECHO | 4 | CI_GREEN |
| §10.1 | Day-of-week: weekday/weekend | `internal/context/dayofweek.go` | `context_test.go` | ECHO | 4 | CI_GREEN |
| §10.2 | Deployment: 60s pre + 300s post | `internal/context/deployment.go` | `context_test.go` | ECHO | 4 | CI_GREEN |
| §10.4 | Manual context CRUD + TTL | `internal/context/manual.go` | `context_test.go` | ECHO | 4 | CI_GREEN |
| §10.3 | Baseline lambda per context type | `internal/context/baseline.go` | `context_test.go` | ECHO | 4 | CI_GREEN |

---

## 11. Self-Telemetry

| WP Section | Spec Item | Package | Test File | Agent | Phase | Status |
|-----------|-----------|---------|-----------|-------|-------|--------|
| §22 | kairo_rupture_index gauge | `internal/telemetry/metrics.go` | `telemetry_test.go` | ECHO | 4 | CI_GREEN |
| §22 | kairo_time_to_failure_seconds | `internal/telemetry/metrics.go` | `telemetry_test.go` | ECHO | 4 | CI_GREEN |
| §22 | kairo_actions_total counter | `internal/telemetry/metrics.go` | `telemetry_test.go` | ECHO | 4 | CI_GREEN |
| §22 | kairo_ingest_samples_total | `internal/telemetry/metrics.go` | `telemetry_test.go` | ECHO | 4 | CI_GREEN |
| §22 | kairo_version_info | `internal/telemetry/metrics.go` | `telemetry_test.go` | ECHO | 4 | CI_GREEN |
| §7.5 | Health schema: status/trackers/message | `internal/telemetry/health.go` | `telemetry_test.go` | ECHO | 4 | CI_GREEN |

---

## 12. Status Legend

| Status | Meaning |
|--------|---------|
| PENDING | Not yet implemented |
| IN_PROGRESS | Agent branch open, work in progress |
| CI_GREEN | Tests pass, coverage gate met on agent branch |
| MERGED | PR merged to v6_main |

---

Produced: 2026-04-24
Last updated: 2026-04-25 (Phase 5 — FOXTROT complete; v6.0.0 tagged)

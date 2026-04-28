# SPECS.md — Kairo Core v6.1.0 Technical Specifications

Document ID: KC-SPECS-002
Date: 2026-04-27
Status: RELEASED — All §23–§26 implemented and shipped in v6.1.0
Produced by: Orchestrator (Claude Code)

> Base spec: see docs/v6.0.0/SPECS.md — all v6.0.0 behaviour is preserved unless
> explicitly overridden in this document.

Rule: If a spec is not in this document or docs/v6.0.0/SPECS.md, it is UNDEFINED.
Rule: This document covers ONLY v6.1 additions and changes.

---

## v6.1.0 Delta Specifications

### §23 — gRPC Agent Protocol (v6.1 full implementation)

The BRAVO phase shipped a stub gRPC push endpoint. v6.1 completes the implementation:

- `internal/ingest/grpc.go` — replace stub with real gRPC server using `google.golang.org/grpc`
- Proto: `api/proto/kairo/v1/ingest.proto` — `PushMetrics(stream MetricPoint) returns (PushResult)`
- Max message size: 4 MB
- TLS: optional, same cert as HTTP listener when `--tls-cert` is set
- Back-pressure: reject with RESOURCE_EXHAUSTED when ingest queue > 80% full

### §24 — Event Streaming (NATS/Kafka)

- Configurable via `kairo.yaml`: `eventbus.driver: nats | kafka | none` (default: `none`)
- Publish on every rupture state change: topic `kairo.rupture.{host}`
- Publish on every Tier-1 action: topic `kairo.actions.tier1`
- NATS: `nats.go` driver — JetStream, at-least-once delivery
- Kafka: `kafka.go` driver — `franz-go` library, exactly-once via idempotent producer
- `internal/eventbus/` — extend existing package; add `Driver` interface + two implementations

### §25 — Adaptive Ensemble Weighting

Current ensemble uses fixed weights: ARIMA=0.25, HW=0.35, CA-ILR=0.40.

v6.1 adds online weight adaptation:
- Track per-model MAE over a sliding 1-hour window (360 × 10s ticks)
- Weights updated every 60s: `w_i = (1/MAE_i) / sum(1/MAE_j)`
- Floor: each weight ≥ 0.05 (prevent collapse)
- Config: `ensemble.adaptive: true` (default `false` for backwards compat)
- Package: `internal/pipeline/metrics/ensemble.go` — extend `Ensemble` struct

### §26 — Operator / Multi-Cluster

Wire the existing `ohe/operator/` skeleton:
- CRD: `KairoInstance` — spec: `{image, port, storageSize, apiKey (secretRef)}`
- Controller: reconcile loop — create Deployment + Service + PVC per KairoInstance
- Multi-cluster: each cluster runs its own kairo-core; operator manages lifecycle only
- Package: `ohe/operator/` — implement controller using `controller-runtime`

---

## 1. Module & Binary

| Item | Value |
|------|-------|
| Go module | `github.com/benfradjselim/kairo-core` |
| Binary name | `kairo-core` |
| Go version | 1.18 (minimum; no 1.21+ features) |
| License | Apache 2.0 |
| Binary size target | <= 25 MB |
| Resident memory (idle, 10k streams) | ~30 MB |
| Resident memory (full, 50k streams) | ~45 MB |

---

## 2. Package Structure (WP §14.2)

```
cmd/
  kairo-core/              # Main binary entry point

internal/
  ingest/                  # Prom remote_write, OTLP, DogStatsD, gRPC receivers
  pipeline/
    metrics/               # Metric stream processing, CA-ILR, ensemble
    logs/                  # Log parsing, pattern extraction, quantitative streams
    traces/                # Trace topology, bottleneck detection, cascade scoring
  fusion/                  # Bayesian fusion, conflict detection, time alignment
  rupture/                 # Rupture detection, surge profiles, event emission
  composites/              # Stress, Fatigue, Pressure, Contagion, Resilience,
                           #   Entropy, Sentiment, HealthScore
  context/                 # Time-of-day, day-of-week, deployment awareness, manual
  actions/
    engine/                # Rule evaluation, tier determination
    providers/             # Kubernetes, webhook, alertmanager, pagerduty
    arbitration/           # Conflict detection, deduplication
    safety/                # Rate limiting, cooldown, rollback, emergency stop
  explain/                 # XAI trace generation, formula audit, pipeline debug
  api/                     # REST/gRPC handlers, middleware, auth
  storage/                 # BadgerDB wrapper, stateless mode
  telemetry/               # Self-monitoring, /metrics, health, profiling

pkg/
  rupture/                 # Public Rupture Index formula, importable
  composites/              # Public composite signal formulas, importable
  client/                  # Go SDK
  logger/                  # Zero-dep structured logger
  models/                  # Domain types: RuptureEvent, ActionRecommendation, etc.
  utils/                   # Shared utilities

sdk/
  go/                      # kairo-client-go
  python/                  # kairo-client
```

---

## 3. Core Formulas

### 3.1 Dual-Scale ELS Trackers (WP §6.2)

**Stable Tracker** — long memory, effective window ~60 min:
```
lambda_stable = 0.95
State: s_stable(t) = [alpha_stable(t), beta_stable(t)]^T
```

**Burst Tracker** — short memory, effective window ~5 min:
```
lambda_burst = 0.80
State: s_burst(t) = [alpha_burst(t), beta_burst(t)]^T
```

**Update Rule** (identical for both; lambda is tracker-specific):
```
innovation:        v_t  = m_t - (alpha(t-1) * Dt + beta(t-1))
Kalman gain:       K_t  = P(t-1)*[1,Dt]^T / (lambda + [1,Dt]*P(t-1)*[1,Dt]^T)
state update:      [alpha(t), beta(t)]^T = [alpha(t-1), beta(t-1)]^T + K_t * v_t
covariance update: P(t) = (I - K_t*[1,Dt]) * P(t-1) / lambda
```

### 3.2 Rupture Index (WP §6.3)

```
R(t) = |alpha_burst(t)| / max(|alpha_stable(t)|, epsilon)

  alpha_burst(t)  = slope from burst tracker
  alpha_stable(t) = slope from stable tracker
  epsilon         = 1e-6  (numerical stability)
```

**Classification** (WP §6.4):

| R Range | Class | Tier |
|---------|-------|------|
| < 1.0 | Stable | None |
| 1.0 – 1.5 | Elevated | None |
| 1.5 – 3.0 | Warning | Tier 3 |
| 3.0 – 5.0 | Critical | Tier 2 |
| >= 5.0 | Emergency | Tier 1 |

### 3.3 Time-to-Failure (WP §5.3)

```
TTF(t) = (theta_critical - m(t)) / alpha_burst(t)

  theta_critical = configurable critical threshold for signal class
  m(t)           = current observed value
  alpha_burst(t) = burst tracker slope
  Clamped to [0, 3600] seconds
```

### 3.4 Ensemble Confidence Score (WP §7.3)

```
C(t) = 1 - (sigma2(t) / mu(t))

  sigma2(t) = variance of forecasts across ensemble models
  mu(t)     = mean forecast across ensemble models
```

Thresholds: C > 0.85 -> Tier 1 eligible | 0.60-0.85 -> Tier 2 | < 0.60 -> Tier 3

### 3.5 Baseline Adaptation (WP §10.3)

```
beta_ctx(t) = lambda_ctx * beta_ctx(t-1) + (1 - lambda_ctx) * m(t)

  Normal operation:  lambda_ctx = 0.99
  Post-deployment:   lambda_ctx = 0.90
  Abnormal traffic:  lambda_ctx = 0.80
```

---

## 4. Composite Signal Formulas (WP §11)

### Stress (WP §11.3)
```
Stress(t) = sum_i w_i * g_i(m_i(t))

Default weights: CPU=0.25, Memory=0.25, IO=0.20, Network=0.15, ErrorRate=0.15
Transformations:
  CPU:        g(m) = m / theta_cpu
  Memory:     g(m) = max(0, (m - 0.5*theta_mem) / (0.5*theta_mem))
  IO:         g(m) = 1 - exp(-m / theta_io)
  Network:    g(m) = m / theta_net
  ErrorRate:  g(m) = 1 - exp(-3*m / theta_err)

Domain: [0, 1]    API: GET /api/v2/kpi/stress/{host}
```

### Fatigue (WP §11.4)
```
Fatigue(t) = max(0, Fatigue(t-1) + DeltaStress(t) - lambda * Fatigue(t-1))

  DeltaStress(t) = max(0, Stress(t) - Stress(t-1))
  lambda         = 0.05 (configurable)
  t_half         = ln(2) / lambda

Domain: [0, inf)    API: GET /api/v2/kpi/fatigue/{host}
```

### Pressure (WP §11.5)
```
Pressure(t) = w_lat * h(latency(t)) + w_err * h(error_rate(t))
  h(x) = (x - mu_x) / sigma_x   (z-score vs baseline)
  w_lat = w_err = 0.5

Domain: (-inf, inf)    API: GET /api/v2/kpi/pressure/{host}
```

### Contagion (WP §11.6)
```
Contagion(t) = (1/|E|) * sum_{(i,j) in E} I(R_i(t)>theta) * I(R_j(t)>theta) * w_ij

  E      = directed edges from service dependency graph (built from traces)
  theta  = 1.5 (default rupture threshold)
  w_ij   = normalized call frequency edge weight

Domain: [0, 1]    API: GET /api/v2/kpi/contagion/{host}
```

### Resilience (WP §11.7)
```
Resilience(t) = 1 / (1 + mean(Stress(tau) for tau in [t-W, t]))
  W = 30 min (default)

Domain: (0, 1]    API: GET /api/v2/kpi/resilience/{host}
```

### Entropy (WP §11.8)
```
Entropy(t) = -sum_i p_i(t) * log(p_i(t))
  p_i(t) = sigma2_i(t) / sum_j sigma2_j(t)   (normalized variance share)

Domain: [0, log(n)]    API: GET /api/v2/kpi/entropy/{host}
```

### Sentiment (WP §11.9)
```
Sentiment(t) = log(N_pos(t) + 1) - log(N_neg(t) + 1)
  N_pos = positive signals (HTTP 2xx, health pass, throughput OK)
  N_neg = negative signals (HTTP 5xx, timeout, health fail, log error)

Note: renamed from "Mood" in OHE v5.1 — same formula
Domain: (-inf, inf)    API: GET /api/v2/kpi/sentiment/{host}
```

### HealthScore (WP §11.10)
```
HealthScore(t) = 100 * prod_{k in K} min(1, max(0, 1 - w_k * s_k(t)))
  K = {Stress, Fatigue, Pressure, Contagion}

Scale: 90-100 Healthy | 70-89 Degraded | 50-69 At risk | 0-49 Critical
Domain: [0, 100]    API: GET /api/v2/kpi/healthscore/{host}
```

---

## 5. Ensemble Models (WP §7.2)

| Model | Weight | Role |
|-------|--------|------|
| CA-ILR | 0.40 | Primary rupture detection; lambda_burst=0.80, lambda_stable=0.95 |
| ARIMA(1,1,1) | 0.20 | Baseline comparison, medium-horizon |
| Holt-Winters (damped phi=0.98) | 0.20 | Seasonal pattern detection |
| MAD Anomaly Guard | 0.10 | Outlier suppression |
| Adaptive EWMA | 0.10 | Trend confirmation |

Forecast horizons: immediate=5m | short=15m | medium=60m

---

## 6. Signal Classes (WP §5.4)

| Class | Direction | Default Rupture Threshold |
|-------|-----------|--------------------------|
| saturation | Ascending | 3.0 |
| latency | Ascending | 2.5 |
| throughput | Both | 4.0 |
| error_rate | Ascending | 1.5 |
| inverse_health | Descending | 2.0 |
| event_rate | Both | N/A (converted to rate) |

---

## 7. Surge Profiles (WP §5.5)

| Profile | Signature | Typical TTF | Action |
|---------|-----------|-------------|--------|
| Spike | R rises to >5 instantly, rapid decay | < 2 min | Add replicas |
| Ramp | R monotonically increases 30+ min | 15–60 min | Restart before critical |
| Cycle | Periodic spikes, high autocorrelation | Predictable | Reschedule/isolate |
| Staircase | Step-function increases with deploys | Per-step | Validate deploy; pause |
| Drift | alpha_stable increases slowly over hours-days | Hours–days | Schedule defrag |
| Unknown | No profile match | Unknown | Escalate to human |

---

## 8. API Endpoint Map (WP §16.1)

### Rupture
```
GET  /api/v2/rupture/{host}
GET  /api/v2/rupture/{host}/history
GET  /api/v2/rupture/{host}/profile
GET  /api/v2/ruptures
```

### Forecasting
```
POST /api/v2/forecast
GET  /api/v2/forecast/{metric}/{host}
```

### Composite Signals
```
GET  /api/v2/kpi/{name}/{host}           # name: stress|fatigue|pressure|contagion|resilience|entropy|sentiment|healthscore
GET  /api/v2/kpi/{name}/{host}/history
```

### Actions
```
GET  /api/v2/actions
GET  /api/v2/actions/{id}
POST /api/v2/actions/{id}/approve
POST /api/v2/actions/{id}/reject
POST /api/v2/actions/{id}/rollback
POST /api/v2/actions/emergency-stop
```

### Suppressions
```
POST   /api/v2/suppressions
DELETE /api/v2/suppressions/{id}
GET    /api/v2/suppressions
```

### Context
```
POST   /api/v2/context
DELETE /api/v2/context/{id}
GET    /api/v2/context
```
Context types: load_test | maintenance_window | incident_active | abnormal_traffic

### Explainability
```
GET /api/v2/explain/{rupture_id}           # Full trace (v6.0: Full)
GET /api/v2/explain/{rupture_id}/formula   # Formula audit (v6.0: Full)
GET /api/v2/explain/{rupture_id}/pipeline  # Per-pipeline debug (v6.0: Partial)
```

### Health & Telemetry
```
GET /api/v2/health
GET /api/v2/ready
GET /api/v2/metrics
```

### Ingest
```
POST /api/v2/write         # Prometheus remote_write (Snappy protobuf) — PRIMARY
POST /api/v2/v1/metrics    # OTLP/HTTP metrics
POST /api/v2/v1/logs       # OTLP/HTTP logs
POST /api/v2/v1/traces     # OTLP/HTTP traces
```

### Native UI
```
GET /timeline              # HTML prediction timeline
```

---

## 9. Prometheus Metrics at /metrics (WP §22)

| Metric | Labels | Type |
|--------|--------|------|
| kairo_rupture_index | host, metric, severity | Gauge |
| kairo_time_to_failure_seconds | host, metric | Gauge |
| kairo_predicted_value | host, metric, horizon | Gauge |
| kairo_confidence | host | Gauge |
| kairo_fused_rupture_probability | host | Gauge |
| kairo_kpi_stress | host | Gauge |
| kairo_kpi_fatigue | host | Gauge |
| kairo_kpi_healthscore | host | Gauge |
| kairo_actions_total | type, tier, outcome | Counter |
| kairo_tracker_count | type, state | Gauge |
| kairo_ingest_samples_total | source | Counter |
| kairo_memory_bytes | — | Gauge |
| kairo_uptime_seconds | — | Counter |
| kairo_version_info | version | Gauge (value=1) |

---

## 10. Log Pipeline Extractors (WP §8.2)

| Extractor | Output | v6.0 Default |
|----------|--------|--------------|
| error_rate | log_error_rate(t) | enabled |
| keyword_counter | named streams per pattern | enabled |
| burst_detector | log_burst_index(t) | enabled |
| novelty_score | log_novelty(t) | **disabled** (experimental) |

Formula (error_rate):
```
log_error_rate(t) = count(entries in bucket(t) where severity in {ERROR,FATAL,CRITICAL}) / bucket_duration
bucket_size default: 15s
```

---

## 11. Trace Pipeline Analyzers (WP §8.3)

| Analyzer | Output | Default Threshold |
|---------|--------|------------------|
| latency_propagation | propagation_factor(t) | min_call_rate: 0.1/s |
| bottleneck_score | bottleneck_index(t) | critical_path_pct: 0.3 |
| error_cascade | cascade_index(t) | min_services_affected: 2 |
| fanout_pressure | fanout_stress(t) | fanout_threshold: 50 calls/span |

Formula (cascade_index):
```
cascade_index(t) = sum_{(i,j) in E} I(error_rate_i > theta_i) * I(error_rate_j > theta_j) * w_ij
```

Topology limits: max_services=500 | edge_min_samples=100

---

## 12. Action Engine (WP §12)

### Tier Taxonomy
| Tier | Automation | Min Confidence | Approval |
|------|-----------|---------------|---------|
| Tier 1 | Fully automated | C > 0.85 | Automatic (rate-limited, cooldown-gated, rollback-ready) |
| Tier 2 | Suggested | C > 0.60 | One-click webhook/API |
| Tier 3 | Human only | any | Manual |

### Safety Gates (WP §12.5)
| Gate | Default |
|------|---------|
| Rate limit | 6 Tier-1 actions/target/hour |
| Cooldown | configurable per action type |
| Rollback trigger | R_new > R_old in observation window |
| Namespace allowlist | configurable |
| Emergency stop | POST /api/v2/actions/emergency-stop |
| Shadow mode | mode: shadow in config |

### Action Providers (WP §12.4)
| Provider | Actions |
|---------|---------|
| Kubernetes | scale, restart, cordon, drain, isolate |
| Webhook | notify, trigger_pipeline, custom |
| Alertmanager | alert, silence |
| PagerDuty | page, incident_create, incident_update |

---

## 13. Health API Schema (WP §7.5)

```json
{
  "status": "warming | ready | degraded",
  "trackers": {
    "burst":  { "ready": true,  "metrics_tracked": 0 },
    "stable": { "ready": false, "metrics_ready": 0, "estimated_full_readiness": "RFC3339" }
  },
  "rupture_detection": "suppressed | degraded | active",
  "message": "string"
}
```

Cold-start timeline:
- 0–5 min: both trackers initializing; detection suppressed
- 5–60 min: burst ready, stable initializing; confidence ceiling 0.50
- 60+ min: full operation; confidence ceiling 1.00

---

## 14. Storage Key Schema (v6.0 single-tenant)

```
m:{host}:{metric}:{ts}       metric samples   (ts = RFC3339)
r:{id}                       rupture events
r:{host}:history:{ts}        rupture history
kpi:{name}:{host}:{ts}      composite signal values
fc:{metric}:{host}:{ts}     forecast results
ac:{id}                      action records
sp:{traceID}:{spanID}       spans
l:{service}:{ts}             log output
ctx:{id}                     context entries
sup:{id}                     suppression windows
```

---

## 15. Configuration Reference (WP §17)

```yaml
mode: connected | stateless | shadow

ingest:
  http_port: 8080
  grpc_port: 9090
  cardinality:
    max_total_streams: 50000
    max_per_host: 5000

pipelines:
  log:
    enabled: true
    extractors:
      error_rate: { enabled: true, levels: [ERROR, FATAL, CRITICAL], bucket_size: 15s }
      keyword_counter: { enabled: true, patterns: [] }
      burst_detector: { enabled: true, sensitivity: medium }
      novelty_score: { enabled: false }
  trace:
    enabled: true
    analyzers:
      latency_propagation: { enabled: true, min_call_rate: 0.1 }
      bottleneck_score: { enabled: true, critical_path_pct: 0.3 }
      error_cascade: { enabled: true, min_services_affected: 2 }
      fanout_pressure: { enabled: true, fanout_threshold: 50 }
    topology:
      discovery: automatic
      max_services: 500
      edge_min_samples: 100

predictor:
  stable_window: 60m
  burst_window: 5m
  rupture_threshold: 3.0
  horizons: { immediate: 5m, short: 15m, medium: 60m }
  confidence_thresholds: { auto_action: 0.85, alert: 0.60 }

context:
  time_of_day: { enabled: true, buckets: 24, min_samples_per_bucket: 60 }
  day_of_week: { enabled: true, profiles: [weekday, weekend] }
  deployment:
    enabled: true
    detection: auto
    suppression_window_before: 60s
    suppression_window_after: 300s
  manual: { enabled: true, api: /api/v2/context, default_ttl: 3600s }

fusion:
  method: bayesian    # UNDEFINED — WP gap
  priors: {}          # UNDEFINED — WP gap
  max_lag: 30s        # UNDEFINED — inferred

composites:
  fatigue: { r_threshold: 0.3, lambda: 0.05, persistent: true }
  resilience: { window: 30m }

actions:
  execution_mode: shadow | suggest | auto
  rules_path: /etc/kairo/rules.yaml
  safety:
    rate_limit_per_hour: 6
    namespace_allowlist: []
  providers:
    kubernetes: {}
    webhook: {}
    alertmanager: {}
    pagerduty: {}

outputs:
  grafana: { annotations_url: "", dashboard_id: 0, panel_id: 0 }
  kubernetes_events: { enabled: false }
  webhook: { url: "" }

storage:
  path: /var/lib/kairo
  ttls: {}   # UNDEFINED — WP gap

auth:
  jwt_secret: ""
  api_keys: []

telemetry:
  metrics: { enabled: true, path: /api/v2/metrics }
  profiling: { enabled: false }
```

---

## 16. Performance Targets (WP §23)

| Operation | Throughput | Latency p99 |
|-----------|-----------|-------------|
| Metric ingest HTTP | 10,000 req/s | < 2 ms |
| CA-ILR (50 metrics) | 3,500 KPI/s | < 1 ms |
| Ensemble prediction (4 models) | — | < 500 µs |
| Storage writes (BadgerDB NVMe) | 100,000 ops/s | < 1 ms |
| Cold start burst-ready | — | < 5 min |
| Cold start full-ready | — | < 60 min |

---

## 17. Test Coverage Targets

| Package | Target |
|---------|--------|
| internal/pipeline/metrics | >= 80% |
| internal/pipeline/logs | >= 80% |
| internal/pipeline/traces | >= 80% |
| internal/fusion | >= 80% |
| internal/rupture | >= 80% |
| internal/composites | >= 80% |
| internal/context | >= 75% |
| internal/actions/* | >= 75% |
| internal/explain | >= 75% |
| internal/api | >= 70% |
| internal/storage | >= 70% |
| pkg/* | >= 85% |
| **Total** | **>= 70%** |

---

## 18. UNDEFINED — WP Gaps

| Item | WP Ref | Gap |
|------|--------|-----|
| Bayesian fusion algorithm | §9 | Formulation, priors not specified |
| Fusion time alignment | §9 | Max lag not specified (inferred 30s) |
| TTF confidence interval formula | §5.3 | "derived from ensemble variance" — formula not given |
| Surge profile shape-matching algorithm | §5.5 | Method not specified |
| Context API response schema | §10.4 | JSON schema not given |
| Suppression API response schema | §16.1 | Not detailed |
| Action response schema | §16.1 | Not detailed |
| gRPC proto definition | §15.1 | Service interface not specified |
| kairoctl migrate-config | §26 | Tool mentioned but not specified |
| Adaptive ensemble weighting | §7.3 | v6.1 target — not in v6.0 scope |
| Storage TTLs | §14.1 | Default retention not specified |

---

Produced: 2026-04-24
Next: Phase 1 — ROADMAP.md, AGENTS.md, TRACEABILITY.md, DEV-GUIDE.md, CI/CD pipeline

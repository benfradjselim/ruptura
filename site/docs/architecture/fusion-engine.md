# Fusion Engine

The Fusion Engine is the analytical core of Ruptura. It combines normalised telemetry from all three signal sources (metrics, logs, traces) into 10 composite KPI signals, runs the adaptive ensemble, and produces the Fused Rupture Index™ for every tracked workload.

## Responsibilities

1. **Composite signal computation** — recalculate all 10 signals on every new data point
2. **Adaptive per-workload baselines** — after 24h of observation, thresholds become relative to each workload's Welford baseline
3. **Rupture detection** — compute `metricR`, `logR`, `traceR` and fuse into `FusedR`
4. **Topology-based contagion** — build real service dependency graph from OTLP trace spans
5. **XAI trace** — record which signals and pipelines contributed to each prediction

## Signal computation order

```
Raw metrics → stress → fatigue → pressure → humidity → velocity
Raw errors  → mood
Trace spans → contagion (topology-based; falls back to errors×cpu)
Log lines   → sentiment → (informs contagion)
stress + variance → entropy
all signals → health_score (additive-penalty model)
```

Signals are computed in dependency order: `stress` must be ready before `fatigue` and `pressure`.

## Three-source fusion

```
metricR  = CA-ILR ratio (5-min burst / 60-min stable slope)
logR     = burst_rate / log_baseline  (fires when error/warn > 3σ)
traceR   = span_error_rate × (p99_latency / latency_baseline)

FusedR   = weighted combination of available signals
           (each source contributes independently — FusedR requires ≥2 sources)
```

The Fused Rupture Index is more resistant to false positives than any single-source index. A metric spike alone won't push FusedR to critical if logs and traces are healthy.

## Adaptive ensemble inside the fusion engine

```go
type EnsembleState struct {
    Weights map[string]float64  // ca_ilr, arima, holt_winters, mad, ewma
    Errors  map[string]float64  // sliding 1-hour MAE per model
    Updated time.Time
}
```

Every 60 seconds, the fusion engine:

1. Collects actual vs predicted values for each model over the past 1 hour
2. Computes MAE per model: `MAE_i = mean(|actual − predicted_i|)`
3. Normalises inverse MAE to produce new weights
4. Stores updated weights in `EnsembleState`

The final prediction passed to the rupture detector is the weighted sum:

```
prediction(t) = Σ_i weight_i(t) × prediction_i(t)
```

## Rupture detector (CA-ILR)

```go
type RuptureDetector struct {
    stableILR *ILR   // 60-min window — long-term baseline
    burstILR  *ILR   // 5-min window  — micro-acceleration
}

func (d *RuptureDetector) R() float64 {
    denom := math.Abs(d.stableILR.Alpha)
    if denom < 1e-6 { denom = 1e-6 }
    return math.Abs(d.burstILR.Alpha) / denom
}
```

Each ILR runs Welford-style O(1) incremental updates — no history stored, constant memory per workload.

## WorkloadRef — treatment unit

All signals are keyed by `WorkloadRef` (namespace/kind/name), not by host. When multiple pods from the same Deployment send metrics, they are merged using these aggregation rules:

| Signal | Aggregation | Rationale |
|--------|-------------|-----------|
| Stress | max | worst pod defines workload stress |
| Fatigue | max | accumulated burden follows the most fatigued pod |
| Mood | min | workload mood is as low as the saddest pod |
| Pressure | max | highest pressure pod sets the alarm |
| Humidity | mean | spread errors across all pods |
| Contagion | max | if any pod is contagious, the workload is |
| Resilience | min | weakest pod limits overall resilience |
| Entropy | mean | disorder is a workload-wide property |
| Velocity | mean | aggregate rate of change |
| HealthScore | min | weakest pod governs workload health |

## Storage

BadgerDB embedded — no external database:

| Granularity | Retention | Use |
|-------------|---------|-----|
| 15 s | 7 days | Raw metrics |
| 1 min | 30 days | Aggregated metrics + logs |
| 5 min | 1 year | KPI snapshot history |
| 1 h | 400 days | Long-term compliance |

`FlushSnapshots()` is called on SIGTERM — no data loss on graceful shutdown.

## XAI — explainability trace

For every rupture, the fusion engine records a full explain trace:

```json
{
  "rupture_id": "r_abc123",
  "workload": { "namespace": "default", "kind": "Deployment", "name": "payment-api" },
  "fused_rupture_index": 4.2,
  "model_contributions": {
    "ca_ilr":       { "weight": 0.35, "prediction": 4.4 },
    "arima":        { "weight": 0.22, "prediction": 4.0 },
    "holt_winters": { "weight": 0.18, "prediction": 3.9 }
  },
  "top_factor": "fatigue",
  "signal_values": {
    "stress": 0.72, "fatigue": 0.81, "mood": 0.31,
    "pressure": 0.65, "contagion": 0.58, "health_score": 43.0
  },
  "primary_pipeline": "metric"
}
```

Retrieve the human-readable narrative at `GET /api/v2/explain/{rupture_id}/narrative`.

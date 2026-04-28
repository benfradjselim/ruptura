# Fusion Engine

The Fusion Engine is the analytical core of Kairo. It combines normalised telemetry from all three pipelines into 8 composite signals, runs the adaptive ensemble, and produces the Rupture Index™ for every tracked host.

## Responsibilities

1. **Composite signal computation** — recalculate all 8 signals on every new data point
2. **Adaptive ensemble** — maintain per-host model weights and update them every 60 s
3. **Rupture detection** — compute R = |α_burst| / |α_stable| and compare against thresholds
4. **XAI trace** — record which signals and models contributed to each prediction

## Signal computation order

```
Raw metrics → stress → fatigue → pressure
Raw errors  → contagion
stress + uptime + throughput → sentiment
stress + variance → entropy
historical slope → resilience
all 4 primary signals → healthscore
```

Signals are computed in dependency order: `stress` must be ready before `fatigue` and `pressure`.

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

## Rupture detector

```go
type RuptureDetector struct {
    stableILR *ILR   // 60-min window
    burstILR  *ILR   // 5-min window
}

func (d *RuptureDetector) R() float64 {
    denom := math.Abs(d.stableILR.Alpha)
    if denom < 1e-6 {
        denom = 1e-6
    }
    return math.Abs(d.burstILR.Alpha) / denom
}
```

## Storage

The fusion engine writes composite signal values to BadgerDB with tiered compaction:

| Granularity | Retention | Use |
|-------------|---------|-----|
| 15 s | 7 days | Raw metrics |
| 1 min | 30 days | Aggregated metrics + logs |
| 5 min | 1 year | KPI history |
| 1 h | 400 days | Long-term compliance |

## XAI — explainability trace

For every rupture, the fusion engine records:

```json
{
  "rupture_id": "r_abc123",
  "host": "web-01",
  "rupture_index": 4.2,
  "model_contributions": {
    "ca_ilr":       { "weight": 0.35, "prediction": 4.4 },
    "arima":        { "weight": 0.22, "prediction": 4.0 },
    "holt_winters": { "weight": 0.18, "prediction": 3.9 },
    "mad":          { "weight": 0.14, "prediction": 4.5 },
    "ewma":         { "weight": 0.11, "prediction": 4.1 }
  },
  "dominant_signal": "stress",
  "signal_values": {
    "stress": 0.72, "fatigue": 0.41, "pressure": 0.35,
    "contagion": 0.12, "healthscore": 43.2
  }
}
```

Retrieve it at `GET /api/v2/explain/{rupture_id}`.

# Adaptive Ensemble (Surge Profiles)

Kairo v6.1 introduces **adaptive ensemble weighting** — the engine continuously learns which prediction model fits your infrastructure's traffic patterns best, and shifts weight toward it automatically.

## The five models

| Model | Strengths | Weaknesses |
|-------|-----------|-----------|
| **CA-ILR** (Dual-scale) | O(1) update, detects acceleration, edge-native | Linear assumption within window |
| **ARIMA** | Strong on stationary series with trends | Computationally heavier, assumes linearity |
| **Holt-Winters** | Excellent on seasonal / periodic patterns | Requires clear seasonality |
| **MAD** (Median Absolute Deviation) | Robust to outliers | No temporal memory |
| **EWMA** (Exponentially Weighted Moving Average) | Smooth, reacts to recent data | Slow to detect sudden shifts |

## Weight adaptation algorithm

Every 60 seconds, Kairo evaluates prediction accuracy over the past 1-hour sliding window:

```
error_model_i = MAE(predicted_i, actual) over last 1 hour

weight_i = (1 / error_i) / Σ_j (1 / error_j)
```

Models with lower recent prediction error receive higher weight. Weights are normalised to sum to 1.0.

The final ensemble prediction is:

```
prediction(t) = Σ_i weight_i(t) × prediction_i(t)
```

## Enable adaptive weighting

```yaml
# kairo.yaml
ensemble:
  adaptive: true
```

When `false` (default), all five models are weighted equally at 0.20 each.

## Monitoring weights via API

```bash
GET /api/v2/ensemble/{host}
```

```json
{
  "host": "web-01",
  "updated_at": "2026-04-28T10:00:00Z",
  "weights": {
    "ca_ilr":       0.35,
    "arima":        0.22,
    "holt_winters": 0.18,
    "mad":          0.14,
    "ewma":         0.11
  }
}
```

## Surge profiles — how Kairo handles load spikes

Kairo does not require you to pre-define "surge profiles" or maintenance windows. The dissipative fatigue formula (`λ` healing) and the adaptive ensemble work together to avoid false alarms during planned load events:

1. **At spike onset** — `ILR_burst` slope increases, R rises.
2. **If the spike is brief** — fatigue dissipates (λ recovery), R falls back.
3. **If the spike sustains** — R stays elevated, ensemble weights shift toward models that track the new pattern.
4. **Weights rebalance** — once the spike ends, MAE improves for baseline-tracking models and they regain weight.

This means a nightly backup job gradually "teaches" the ensemble that high CPU at 02:00 UTC is expected — no manual intervention required.

## Prometheus self-metrics for ensemble

```
kairo_ensemble_weight{host="web-01",model="ca_ilr"}       0.35
kairo_ensemble_weight{host="web-01",model="arima"}         0.22
kairo_ensemble_weight{host="web-01",model="holt_winters"}  0.18
kairo_ensemble_weight{host="web-01",model="mad"}           0.14
kairo_ensemble_weight{host="web-01",model="ewma"}          0.11
```

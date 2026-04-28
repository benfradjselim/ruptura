# Rupture Index™

The Rupture Index™ (R) is Kairo's core predictive metric. It measures the ratio of short-term slope acceleration to the long-term baseline slope across every tracked metric.

## Formula

```
R(t) = |α_burst(t)| / max(|α_stable(t)|, ε)

  α_burst  = slope from the 5-min CA-ILR window  (captures sudden change)
  α_stable = slope from the 60-min CA-ILR window  (captures the baseline)
  ε        = 1e-6  (numerical stability guard)
```

When a metric is accelerating far faster than its baseline — a hallmark of memory leaks, cascade failures, and saturation events — R spikes above 1. Kairo raises actions when R crosses the configured thresholds.

## Threshold table

| R Range | State | Default Action |
|---------|-------|---------------|
| < 1.0 | Stable | None |
| 1.0 – 1.5 | Elevated | None |
| 1.5 – 3.0 | Warning | Tier-3 (human alert) |
| 3.0 – 5.0 | Critical | Tier-2 (suggested action) |
| ≥ 5.0 | Emergency | Tier-1 (automated action) |

Thresholds are configurable via `predictor.rupture_threshold` in `kairo.yaml`.

## Dual-scale CA-ILR engine

Kairo maintains **two** Incremental Linear Regression (ILR) instances per metric:

| Instance | Window | Purpose |
|----------|--------|---------|
| `ILR_stable` | 60 min (240 samples @ 15 s) | Long-term baseline trend |
| `ILR_burst` | 5 min (20 samples @ 15 s) | Micro-acceleration, crisis onset |

Each ILR runs Welford-style O(1) incremental updates — no history stored, constant memory:

```
μx(n+1) = μx(n) + (x_{n+1} − μx(n)) / (n+1)
C_xy    = C_xy + (x_{n+1} − μx(n)) · (y_{n+1} − μy(n+1))
α       = C_xy / V_x    (slope)
β       = μy − α · μx   (intercept)
```

## Why not LSTM or ARIMA?

| Model | MAE | RAM | Inference | Efficiency |
|-------|-----|-----|-----------|------------|
| LSTM | 2.0% | 200+ MB | 500 ms | < 0.0001 |
| ARIMA | 4.1% | 85 MB | 210 ms | 0.0001 |
| **ILR (Kairo)** | **6.2%** | **0.5 MB** | **0.8 ms** | **1,550×** |

ILR trades +2.1% MAE vs ARIMA for 170× less RAM and 262× faster inference. Validated over 40,320 samples on a Raspberry Pi 4 — Kairo runs on edge hardware.

## XAI — explaining a rupture

Every rupture has a traceable explanation endpoint:

```bash
curl http://localhost:8080/api/v2/explain/<rupture_id>
```

```json
{
  "rupture_id": "r_abc123",
  "host": "web-01",
  "rupture_index": 4.2,
  "state": "critical",
  "dominant_signal": "stress",
  "alpha_burst": 0.042,
  "alpha_stable": 0.010,
  "formula": "R = |α_burst| / |α_stable| = 0.042 / 0.010 = 4.2",
  "recommendation": "CPU stress is the primary driver — consider horizontal scaling"
}
```

## API

```bash
# Current rupture index for a host
GET /api/v2/rupture/{host}

# All active ruptures
GET /api/v2/ruptures

# Formula breakdown
GET /api/v2/explain/{rupture_id}/formula
```

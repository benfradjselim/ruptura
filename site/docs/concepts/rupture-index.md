# Rupture Index™

The Rupture Index™ measures how fast a metric is diverging from its baseline. It is the foundation of Ruptura's predictive capability — not a threshold alarm, but a rate-of-change signal that fires hours before a threshold would.

## The Fused Rupture Index

Ruptura combines three independent signal sources into a single **Fused Rupture Index (FusedR)** per Kubernetes workload:

```
FusedR = f(metricR, logR, traceR)

  metricR  = |α_burst| / max(|α_stable|, ε)   ← CA-ILR dual-scale slope ratio
  logR     = burst_rate / log_baseline          ← fires when error/warn > 3σ
  traceR   = span_error_rate × P99_deviation    ← from OTLP trace spans
  ε        = 1e-6  (numerical stability guard)
```

FusedR requires ≥2 sources to produce a value — a single noisy signal cannot push a workload to "critical". This dramatically reduces false positives vs. single-source indexes.

## Threshold table

| FusedR | State | Default Action |
|--------|-------|---------------|
| < 1.0 | Stable | None |
| 1.0 – 1.5 | Elevated | None |
| 1.5 – 3.0 | Warning | Tier-3 (human alert) |
| 3.0 – 5.0 | Critical | Tier-2 (suggested action) |
| ≥ 5.0 | Emergency | Tier-1 (automated action) |

Thresholds are configurable via `predictor.rupture_threshold` in `ruptura.yaml`.

## Metric component: dual-scale CA-ILR engine

Ruptura maintains **two** Incremental Linear Regression (ILR) instances per metric:

| Instance | Window | Purpose |
|----------|--------|---------|
| `ILR_stable` | 60 min (240 samples @ 15 s) | Long-term baseline trend |
| `ILR_burst` | 5 min (20 samples @ 15 s) | Micro-acceleration, crisis onset |

```
metricR(t) = |α_burst(t)| / max(|α_stable(t)|, ε)
```

When a metric is accelerating far faster than its baseline — a hallmark of memory leaks, cascade failures, and saturation events — `metricR` spikes above 1.

Each ILR runs Welford-style O(1) incremental updates — no history stored, constant memory per metric:

```
μx(n+1) = μx(n) + (x_{n+1} − μx(n)) / (n+1)
C_xy    = C_xy + (x_{n+1} − μx(n)) · (y_{n+1} − μy(n+1))
α       = C_xy / V_x    (slope)
```

## Log component: burst detector

The BurstDetector counts error/warn log lines per workload in a sliding window. When the rate exceeds 3σ above the workload's rolling baseline:

```
logR = burst_rate / log_baseline
```

`logR` is fed into the fusion engine as a second independent signal. A log burst alone at `logR=2.0` in a healthy metric environment will raise FusedR from 0 to ~1.0 — enough to flag the workload for monitoring, not enough to trigger automated action.

## Trace component: span error rate

OTLP trace spans carry error flags and latency values. For each workload:

```
traceR = span_error_rate × (p99_latency / latency_baseline)
```

This fires when a service is both failing calls AND getting slower — the typical signature of a cascade from an upstream dependency.

## Adaptive per-workload baselines

After 24h of observation, each workload's normal behavior is captured in a Welford online baseline. All three signal components are then computed relative to that baseline:

- A batch job that always produces `metricR=1.8` during its run → FusedR stays stable
- An API server that normally sees `logR=0.1` and suddenly spikes to `logR=3.5` → FusedR rises immediately

## Why not LSTM?

| Model | MAE | RAM | Inference | Efficiency |
|-------|-----|-----|-----------|------------|
| LSTM | 2.0% | 200+ MB | 500 ms | < 0.0001 |
| ARIMA | 4.1% | 85 MB | 210 ms | 0.0001 |
| **ILR (Ruptura)** | **6.2%** | **0.5 MB** | **0.8 ms** | **1,550×** |

ILR trades +2.1% MAE vs ARIMA for 170× less RAM and 262× faster inference. Ruptura runs on edge hardware and in resource-constrained clusters. The 5-model ensemble (CA-ILR + ARIMA + Holt-Winters + MAD + EWMA) closes most of the accuracy gap while retaining the efficiency advantage of the ILR backbone.

## API

```bash
# Fused Rupture Index for a Kubernetes workload (primary)
GET /api/v2/rupture/{namespace}/{workload}

# All workloads
GET /api/v2/ruptures

# Legacy host-based (non-K8s or backward compat)
GET /api/v2/rupture/{host}

# Human-readable narrative explanation
GET /api/v2/explain/{rupture_id}/narrative
```

Example:

```bash
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/rupture/default/payment-api
```

```json
{
  "workload": { "namespace": "default", "kind": "Deployment", "name": "payment-api" },
  "fused_rupture_index": 4.2,
  "health_score": 43,
  "state": "critical",
  "fatigue": { "value": 0.81, "state": "burnout_imminent" },
  "stress":  { "value": 0.72, "state": "stressed" },
  "timestamp": "2026-05-01T09:00:00Z"
}
```

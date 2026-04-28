# OHE v5.1.0 — Metrics & KPI Model Reference

## 1. Base Metrics

OHE ingests raw telemetry and normalises each signal to **[0, 1]** before combining.

| Metric | Key | Normalisation |
|--------|-----|---------------|
| CPU utilisation | `cpu_percent` | `/100` |
| RAM utilisation | `mem_percent` | `/100` |
| HTTP latency (p99 ms) | `latency_p99_ms` | `min(val/2000, 1)` |
| HTTP error rate | `error_rate` | `min(val/100, 1)` |
| Timeout rate | `timeout_rate` | `min(val/100, 1)` |
| Disk I/O utilisation | `disk_io_percent` | `/100` |
| Network saturation | `net_saturation` | `min(val/10Gbps, 1)` |

Additional metrics accepted via OTLP, Prometheus remote_write, DogStatsD, Loki, or Elasticsearch Bulk.

---

## 2. Stress Score (S)

The **Stress** KPI aggregates normalised signals into a single host-level pressure indicator.

```
S = 0.30 · CPU_n
  + 0.20 · RAM_n
  + 0.20 · Latency_n
  + 0.20 · Error_n
  + 0.10 · Timeout_n
```

Range: `[0, 1]`. Values above `0.7` trigger the `HIGH_STRESS` alert.

---

## 3. Fatigue Model (F) — Dissipative Accumulator

Fatigue captures chronic, accumulated Stress that does not recover between measurement windows.

```
F_t = max(0,  F_{t-1}  +  (S_t − R_threshold)  −  λ)
```

| Parameter | Default | Meaning |
|-----------|---------|---------|
| `R_threshold` | `0.3` | Stress must exceed this to accumulate |
| `λ` (lambda) | `0.05` | Natural recovery rate per tick |

- `F_t = 0` → fully recovered
- `F_t > 0.7` → `FATIGUE_HIGH` alert
- `F_t > 0.9` → `FATIGUE_CRITICAL` alert

---

## 4. CA-ILR Predictor — Dual-Scale Regression

The **Context-Aware Incremental Linear Regression (CA-ILR)** maintains two independent exponential moving windows.

### 4.1 Stable Tracker (long-horizon)

```
α_stable_{t} = α_stable_{t-1} · λ_s  +  S_t · (1 − λ_s)
```
`λ_s = 0.95` (60-minute effective window)

### 4.2 Burst Tracker (short-horizon)

```
α_burst_{t} = α_burst_{t-1} · λ_b  +  S_t · (1 − λ_b)
```
`λ_b = 0.80` (5-minute effective window)

### 4.3 Rupture Index

```
RuptureIndex = α_burst / α_stable
```

When `RuptureIndex > rupture_threshold` (default `3.0`), OHE fires a `RUPTURE` event which:
- Triggers level-3 alert
- Notifies all configured channels
- Records a `RuptureEvent` in audit log

### 4.4 Prediction Horizon

The predictor emits forecasts for `t+5`, `t+15`, `t+30`, `t+60` minutes using the stable slope. Confidence bands are `±1σ` of the residual distribution.

---

## 5. Derived KPIs

| KPI | Formula | Thresholds |
|-----|---------|------------|
| **Mood** | `1 − S` | `<0.3` = critical |
| **Pressure** | `clamp(F / 0.9, 0, 1)` | `>0.8` = alert |
| **Humidity** | `(Latency_n + Error_n) / 2` | `>0.6` = alert |
| **Contagion** | Rate of Stress spread across correlated hosts | `>0.5` = alert |
| **Resilience** | `1 − F_t` | `<0.3` = alert |
| **Entropy** | Shannon entropy over metric distribution per host | `>0.85` = chaos |
| **Velocity** | `dS/dt` — rate of stress change per tick | `>0.05/tick` = alert |

---

## 6. HealthScore Composite

```
HealthScore = 100
            − 30 · Stress
            − 25 · Fatigue
            − 20 · Humidity
            − 15 · (1 − Resilience)
            − 10 · Entropy
```

Range: `[0, 100]`. OHE SLO engine uses HealthScore as the default signal for availability SLOs.

---

## 7. SLO Computation

```
availability = (good_events / total_events) × 100
error_budget  = (1 − target/100) × window_hours × 3600   (seconds)
burn_rate     = (1 − availability/100) / (1 − target/100)
```

SLO status is `breached` when `availability < target` at window end.

---

## 8. Alert Severity Levels

| Level | Condition | Escalation |
|-------|-----------|------------|
| `INFO` | KPI crossed info threshold | Log only |
| `WARNING` | S > 0.5 or F > 0.5 | Notification channel |
| `HIGH_STRESS` | S > 0.7 | PagerDuty / webhook |
| `FATIGUE_HIGH` | F > 0.7 | Notification channel + webhook |
| `FATIGUE_CRITICAL` | F > 0.9 | All channels |
| `RUPTURE` | RuptureIndex > 3.0 | All channels, escalation delay 0 |

---

## 9. XAI — Explainability Output

Every alert includes a machine-readable explanation:

```json
{
  "kpi": "stress",
  "value": 0.82,
  "threshold": 0.70,
  "contributors": [
    { "metric": "cpu_percent",  "weight": 0.30, "value": 0.94, "contribution": 0.282 },
    { "metric": "error_rate",   "weight": 0.20, "value": 0.61, "contribution": 0.122 },
    { "metric": "latency_p99",  "weight": 0.20, "value": 0.55, "contribution": 0.110 }
  ],
  "rupture_index": 3.4,
  "fatigue": 0.61,
  "recommended_action": "scale_out"
}
```

The `recommended_action` field is set by the ensemble reasoner based on the dominant contributor.

---

## 10. Metric Retention

| Tier | Resolution | Retention |
|------|-----------|-----------|
| Raw | 15-second ticks | 7 days |
| 5-min aggregate | mean/max/p99 | 30 days |
| 1-hour aggregate | mean/max/p99 | 365 days |
| 1-day aggregate | mean/max | unlimited |

Downsampling runs automatically via the background compaction job (configurable, default: every 6 hours).

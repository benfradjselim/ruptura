# Composite Signals

Ruptura computes 10 composite KPI signals from raw telemetry. Each maps multiple input metrics to a single 0–1 interpretable index with a published formula. No black boxes.

The dashboard and alert rules speak SRE-native vocabulary (Reliability, CPU Pressure, Memory Pressure, Error Budget Burn Rate, Blast Radius, Incident Probability). This page uses that vocabulary first, with the underlying Ruptura signal name — the one you'll see in API responses, Prometheus metric labels, and curl examples below — in parentheses.

## Signal overview

| SRE-native name (Ruptura signal) | Range | What it measures |
|-----------------------------------|-------|-----------------|
| CPU Pressure (`stress`) | 0–1 | Instantaneous load pressure across CPU, RAM, latency, errors, timeouts |
| Memory Pressure (`fatigue`) | 0–1 | Accumulated stress over time — dissipative, recovers during low-stress periods |
| Trend (`mood`) | 0–1 | System well-being: uptime × throughput vs errors × timeouts × restarts |
| Load Index (`pressure`) | 0–1 | Rate of change in stress + integrated error load (storm approaching) |
| Saturation (`humidity`) | 0–1 | Error × timeout density relative to throughput |
| Blast Radius (`contagion`) | 0–1 | Error propagation across service dependencies (topology-based when traces available) |
| Resilience (`resilience`) | 0–1 | How quickly the workload recovers from stress |
| Entropy (`entropy`) | 0–1 | Behavioral unpredictability — rolling variance of HealthScore |
| Velocity (`velocity`) | 0–1 | Rate of change of HealthScore — how fast the workload is degrading |
| Reliability / SLO Probability (`health_score`) | 0–100 | Composite: additive-penalty sum of the primary signals |
| Error Budget Burn Rate (`fused_rupture_index`) | 0–10 | How fast the workload is burning through its error budget — banded into Low / Elevated / High / Critical incident probability |

---

## Formulas

### CPU Pressure (Stress)

```
stress(t) = 0.3·CPU(t) + 0.2·RAM(t) + 0.2·Latency(t) + 0.2·Errors(t) + 0.1·Timeouts(t)
```

All inputs normalised to [0, 1].

| stress | State |
|--------|-------|
| < 0.3 | Calm |
| 0.3 – 0.6 | Nervous |
| 0.6 – 0.8 | Stressed |
| ≥ 0.8 | Panic |

### Memory Pressure (Fatigue, dissipative)

```
F(t) = max(0, F(t−1) + (stress(t) − R_threshold) − λ)

  R_threshold = 0.3  (rest threshold — stress below this heals fatigue)
  λ           = 0.05 (healing rate per 15-second interval)
```

The dissipative term `λ` prevents false fatigue alarms from legitimate scheduled spikes (nightly backups, batch jobs). After 24h of observation, each workload's baseline is learned and thresholds become relative — a batch job at 90% CPU is never "fatigued" if that is its normal state.

| fatigue | State | Recommended action |
|---------|-------|--------------------|
| < 0.3 | Rested | Normal monitoring |
| 0.3 – 0.6 | Tired | Increase observation frequency |
| 0.6 – 0.8 | Exhausted | Plan maintenance window |
| ≥ 0.8 | Burnout imminent | Preventive restart |

### Trend (Mood)

```
mood(t) = log(uptime × throughput + 1) / log(errors × timeouts × restarts + 2)
```

High mood = service is happy and performant. Low mood = degraded user experience regardless of raw CPU/memory numbers.

| mood | State |
|------|-------|
| > 0.7 | Happy |
| 0.5–0.7 | Content |
| 0.3–0.5 | Neutral |
| 0.1–0.3 | Sad |
| < 0.1 | Depressed |

### Load Index (Pressure)

```
pressure(t) = d(stress̄)/dt + ∫₀ᵗ errors̄(τ) dτ
```

| pressure | Interpretation |
|----------|---------------|
| > 0.1 sustained for 10 min | Storm likely in ~2 hours |
| Stable | Steady-state conditions |
| Declining | System recovering |
| ≥ 0.8 | Storm approaching |

### Saturation (Humidity)

```
humidity(t) = (errors(t) × timeouts(t)) / max(throughput(t), ε)
```

A high-throughput service absorbs more errors before becoming "humid." A low-throughput service with even a few errors/timeouts gets high humidity — which is usually a sign of a problem.

### Blast Radius (Contagion)

**When trace spans are available** (OTLP traces ingested):

```
contagion(t) = Σ_{i,j} E_{ij}(t) × D_{ij}
```

Where `E_ij` = error rate from service i to j (from real trace spans), `D_ij` = dependency weight (0–1) from call volume.

**Fallback** (no trace topology):

```
contagion(t) ≈ errors(t) × cpu(t)   # proxy signal
```

| contagion | State | Action |
|-----------|-------|--------|
| < 0.3 | Isolated | Normal |
| 0.3 – 0.6 | Spreading | Monitor closely |
| 0.6 – 0.8 | Epidemic | Isolate affected services |
| ≥ 0.8 | Pandemic | Global incident response |

### Resilience

```
resilience(t) = mood(t) × (1 − fatigue(t)) × (1 − contagion(t))
```

A workload that is in good mood, not fatigued, and not spreading errors to its peers has high resilience.

### Entropy

```
entropy(t) = MAD(HealthScore history, window=20)
```

The median absolute deviation of the last 20 HealthScore samples. High entropy means the workload is behaving unpredictably — oscillating between healthy and degraded.

### Velocity

```
velocity(t) = |ΔHealthScore| / Δt
```

Rate of change of HealthScore. High velocity means the workload is degrading (or recovering) rapidly.

### Reliability / SLO Probability (Health Score)

```
health_score = 100 × (1 − (
    w_stress    × stress +
    w_fatigue   × fatigue +
    w_mood      × (1 − mood) +
    w_pressure  × pressure +
    w_humidity  × humidity +
    w_contagion × contagion
))
```

Default weights: `stress=0.25, fatigue=0.20, mood=0.20, pressure=0.15, humidity=0.10, contagion=0.10`.

Additive-penalty model. A single high signal degrades the score proportionally — it does not collapse the score the way a multiplicative model would. Below 60 indicates a workload needing attention.

Weights are configurable per workload or namespace (v6.6.0+). See [Signal Weight Configuration](../api/reference.md#signal-weight-configuration) for the API reference and Helm `workloadWeights` for static bootstrap config.

| health_score | State |
|-------------|-------|
| 80–100 | Excellent |
| 60–80 | Good |
| 40–60 | Fair |
| 20–40 | Poor |
| < 20 | Critical |

---

## Calibration Warm-Up

For the first 96 observations (~24 hours at the default 15-second interval), Ruptura is in **calibrating** state. During this period:

- KPI signals are computed and stored normally
- Rupture predictions and Tier-1/Tier-2 action recommendations are **suppressed** — the baseline is not yet reliable enough to act on
- The API response includes a clear calibration status so you are never confused by the silence

Every rupture snapshot carries:

```json
{
  "status": "calibrating",
  "calibration_progress": 43,
  "calibration_eta_minutes": 820
}
```

Once calibration completes, `status` switches to `"active"` and the full prediction + action pipeline comes online.

```json
{
  "status": "active",
  "calibration_progress": 100,
  "calibration_eta_minutes": 0
}
```

You can fast-track calibration in demos using [ruptura-sim](../operations/simulation.md).

---

## HealthScore Trend Forecast

When a workload is `active` (calibration complete) and at least 10 health history points are available, Ruptura runs an OLS linear regression over the rolling 60-point health history and projects the critical-threshold crossing time.

```json
{
  "health_forecast": {
    "trend": "degrading",
    "in_15min": 51.2,
    "in_30min": 38.7,
    "critical_eta_minutes": 28
  }
}
```

| field | Meaning |
|-------|---------|
| `trend` | `"improving"` \| `"stable"` \| `"degrading"` |
| `in_15min` | Projected HealthScore (0–100) in 15 minutes |
| `in_30min` | Projected HealthScore (0–100) in 30 minutes |
| `critical_eta_minutes` | Minutes until HealthScore is projected to fall below 40 (Fair → Poor). `0` if not degrading toward critical. |

This turns "your score is 54" into "you have 28 minutes." The forecast is `null` during calibration and when the trend is flat (insufficient variance to project).

---

## Adaptive Per-Workload Baselines

After 96 observations (~24 hours at the default 15s interval), Ruptura switches from global thresholds to **workload-specific baselines** using Welford online statistics.

- A batch job at 90% CPU → `stress = 0.9` globally, but z-score = 0.1 (normal for this workload) → no alarm
- An API server normally at 10% CPU, now at 40% → z-score = 4.2 → stress alarm fires

Fatigue thresholds remain absolute because sustained effort IS fatigue regardless of whether it is normal.

---

## API

```bash
# By Kubernetes workload (primary)
GET /api/v2/kpi/{signal}/{namespace}/{workload}

# By legacy host name (fallback)
GET /api/v2/kpi/{signal}/{host}

# Full workload snapshot (all signals at once)
GET /api/v2/rupture/{namespace}/{workload}
```

Example request:

```bash
curl -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v2/kpi/fatigue/default/payment-api"
```

Example response:

```json
{
  "signal": "fatigue",
  "workload": {
    "namespace": "default",
    "kind": "Deployment",
    "name": "payment-api"
  },
  "value": 0.81,
  "state": "burnout_imminent",
  "timestamp": "2026-05-01T09:00:00Z"
}
```

---

## Prometheus metrics

Scrape at `GET /api/v2/metrics`.

All 10 signals (plus `fused_rupture_index` and `throughput`) are exported as:

```
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="fatigue"} 0.81
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="stress"} 0.52
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="health_score"} 74.0
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="fused_rupture_index"} 1.8
# ... one series per signal per workload
```

The Grafana dashboard at `deploy/grafana/dashboards/ruptura_overview.json` is pre-configured to use these label selectors.

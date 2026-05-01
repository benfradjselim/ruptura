# Composite Signals

Ruptura computes 10 composite KPI signals from raw telemetry. Each maps multiple input metrics to a single 0–1 interpretable index with a published formula. No black boxes.

## Signal overview

| Signal | Range | What it measures |
|--------|-------|-----------------|
| `stress` | 0–1 | Instantaneous load pressure across CPU, RAM, latency, errors, timeouts |
| `fatigue` | 0–1 | Accumulated stress over time — dissipative, recovers during low-stress periods |
| `mood` | 0–1 | System well-being: uptime × throughput vs errors × timeouts × restarts |
| `pressure` | 0–1 | Rate of change in stress + integrated error load (storm approaching) |
| `humidity` | 0–1 | Error × timeout density relative to throughput |
| `contagion` | 0–1 | Error propagation across service dependencies (topology-based when traces available) |
| `resilience` | 0–1 | How quickly the workload recovers from stress |
| `entropy` | 0–1 | Behavioral unpredictability — rolling variance of HealthScore |
| `velocity` | 0–1 | Rate of change of HealthScore — how fast the workload is degrading |
| `health_score` | 0–100 | Composite: additive-penalty sum of the primary signals |

---

## Formulas

### Stress

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

### Fatigue (dissipative)

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

### Mood

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

### Pressure

```
pressure(t) = d(stress̄)/dt + ∫₀ᵗ errors̄(τ) dτ
```

| pressure | Interpretation |
|----------|---------------|
| > 0.1 sustained for 10 min | Storm likely in ~2 hours |
| Stable | Steady-state conditions |
| Declining | System recovering |
| ≥ 0.8 | Storm approaching |

### Humidity

```
humidity(t) = (errors(t) × timeouts(t)) / max(throughput(t), ε)
```

A high-throughput service absorbs more errors before becoming "humid." A low-throughput service with even a few errors/timeouts gets high humidity — which is usually a sign of a problem.

### Contagion

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

### Health Score

```
health_score = 100 × (1 − (
    0.25 × stress +
    0.20 × fatigue +
    0.20 × (1 − mood) +
    0.15 × pressure +
    0.10 × humidity +
    0.10 × contagion
))
```

Additive-penalty model. A single high signal degrades the score proportionally — it does not collapse the score the way a multiplicative model would. Below 60 indicates a workload needing attention.

| health_score | State |
|-------------|-------|
| 80–100 | Excellent |
| 60–80 | Good |
| 40–60 | Fair |
| 20–40 | Poor |
| < 20 | Critical |

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

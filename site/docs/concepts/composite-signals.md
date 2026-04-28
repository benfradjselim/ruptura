# Composite Signals

Ruptura computes 8 composite signals from raw telemetry. Each maps multiple input metrics to a single 0–1 interpretable index with a published formula. No black boxes.

## Signal overview

| Signal | Range | What it measures |
|--------|-------|-----------------|
| `stress` | 0–1 | Instantaneous load pressure across CPU, RAM, latency, errors |
| `fatigue` | 0–1 | Accumulated stress over time (dissipative — recovers during idle) |
| `pressure` | 0–1 | Rate of change in stress + integrated error load |
| `contagion` | 0–1 | Error propagation speed across service dependencies |
| `resilience` | 0–1 | Inverse of failure probability — how quickly does the system recover? |
| `entropy` | 0–1 | Configuration drift and unpredictability in system behaviour |
| `sentiment` | 0–1 | Mood proxy: uptime × throughput vs errors × restarts |
| `healthscore` | 0–100 | Product signal: `(1-stress) × (1-fatigue) × (1-pressure) × (1-contagion) × 100` |

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
F(t) = max(0,  F(t−1) + (stress(t) − R_threshold) − λ)

  R_threshold = 0.3  (rest threshold)
  λ           = 0.05 (healing rate per interval)
```

The dissipative term `λ` prevents false fatigue alarms from legitimate scheduled spikes (nightly backups, batch jobs). The system "heals" during low-stress periods.

| fatigue | State | Recommended action |
|---------|-------|--------------------|
| < 0.3 | Rested | Normal monitoring |
| 0.3 – 0.6 | Tired | Increase observation frequency |
| 0.6 – 0.8 | Exhausted | Plan maintenance window |
| ≥ 0.8 | Burnout imminent | Preventive restart |

### Pressure

```
pressure(t) = d(stress̄)/dt + ∫₀ᵗ errors̄(τ) dτ
```

| pressure | Interpretation |
|----------|---------------|
| > 0.1 sustained for 10 min | Storm likely in ~2 hours |
| Stable | Steady-state conditions |
| < 0 | System recovering |

### Contagion

```
contagion(t) = Σ_{i,j} E_{ij}(t) × D_{ij}
```

Where `E_ij` = error rate from service i to j, `D_ij` = dependency weight (0–1).

| contagion | State | Action |
|-----------|-------|--------|
| < 0.3 | Isolated | Normal |
| 0.3 – 0.6 | Spreading | Monitor closely |
| 0.6 – 0.8 | Epidemic | Isolate affected services |
| ≥ 0.8 | Pandemic | Global incident response |

### Sentiment

```
sentiment(t) = (Uptime(t) × Throughput(t)) / (Errors(t) × Timeouts(t) × Restarts(t) + ε)
```

A high sentiment means the service is happy and performant. Near zero indicates degraded user experience.

### Health Score

```
healthscore = (1 − stress) × (1 − fatigue) × (1 − pressure) × (1 − contagion) × 100
```

A single 0–100 operational dashboard number. Below 60 indicates a service needing attention.

---

## API

```bash
# Query any signal for a host
GET /api/v2/kpi/{signal}/{host}

# All signals for a host
GET /api/v2/kpi/healthscore/web-01
GET /api/v2/kpi/stress/web-01
GET /api/v2/kpi/fatigue/web-01
```

Example response:

```json
{
  "signal": "stress",
  "host": "web-01",
  "value": 0.72,
  "state": "Stressed",
  "trend": "up",
  "timestamp": "2026-04-28T10:00:00Z"
}
```

## Prometheus self-metrics

Ruptura exports all 8 signals as Prometheus metrics:

```
rpt_kpi_healthscore{host="web-01"} 61.4
rpt_kpi_stress{host="web-01"} 0.72
rpt_kpi_fatigue{host="web-01"} 0.41
# ... (one series per signal per host)
```

Scrape at `GET /api/v2/metrics`.

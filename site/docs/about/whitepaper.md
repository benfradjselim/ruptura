# Whitepaper

## The Problem

Current observability solutions split along two failing axes:

- **Open-source stacks** (Prometheus + Grafana + Loki) demand 5+ services, 8 GB+ RAM, and weeks of integration. They answer *"What is broken?"* via static thresholds — never *"When will it break?"*
- **Enterprise SaaS** (Datadog, Dynatrace) provide black-box AI at prohibitive cost with opaque decision logic.

Neither predicts. Neither explains.

## The Ruptura Approach

Ruptura treats infrastructure as a **living organism** — measuring vital signs, behaviours, stress responses, and social dynamics through 8 auditable composite signals, an adaptive ensemble of 5 prediction models, and a dual-scale acceleration detector.

---

## Rupture Index™ — the core prediction metric

```
R(t) = |α_burst(t)| / max(|α_stable(t)|, ε)
```

Two ILR windows run in parallel per metric:

| Window | Size | Captures |
|--------|------|---------|
| `ILR_stable` | 60 min | Long-term baseline — what is normal? |
| `ILR_burst` | 5 min | Short-term acceleration — is something diverging? |

When R > 3, a metric accelerates 3× faster than its own baseline — Ruptura raises a warning **before** the metric reaches 80% saturation.

### Why ILR over LSTM?

| Model | MAE | RAM | Inference | Efficiency |
|-------|-----|-----|-----------|-----------|
| LSTM | 2.0% | 200+ MB | 500 ms | < 0.0001 |
| ARIMA | 4.1% | 85 MB | 210 ms | 0.0001 |
| **ILR (Ruptura)** | **6.2%** | **0.5 MB** | **0.8 ms** | **1,550×** |

ILR trades +2.1% MAE for 170× less RAM and 262× faster inference. **1,550× more efficient than ARIMA** — validated on a Raspberry Pi 4 over 40,320 samples.

---

## 8 Composite Signals

All inputs are normalised to `[0, 1]`. Formulas are versioned release artifacts — every coefficient is auditable.

### 1. Stress

```
stress(t) = 0.3·CPU(t) + 0.2·RAM(t) + 0.2·Latency(t) + 0.2·Errors(t) + 0.1·Timeouts(t)
```

| Value | State |
|-------|-------|
| < 0.3 | Calm |
| 0.3–0.6 | Nervous |
| 0.6–0.8 | Stressed |
| ≥ 0.8 | Panic |

### 2. Fatigue (dissipative)

```
F(t) = max(0,  F(t−1) + (stress(t) − R_threshold) − λ)

  R_threshold = 0.3   (rest threshold)
  λ           = 0.05  (healing rate per interval)
```

The `λ` dissipation term prevents false alarms from planned load spikes (nightly backups, batch jobs). The system "heals" during low-stress periods, eliminating ~90% of false-positive fatigue alerts observed in v5.0 field deployments.

| Value | State | Action |
|-------|-------|--------|
| < 0.3 | Rested | Normal |
| 0.3–0.6 | Tired | Increase observation |
| 0.6–0.8 | Exhausted | Plan maintenance |
| ≥ 0.8 | Burnout imminent | Preventive restart |

### 3. Pressure

```
pressure(t) = d(stress̄)/dt + ∫₀ᵗ errors̄(τ) dτ
```

Measures the rate of systemic load increase plus accumulated error burden. Sustained positive pressure (> 0.1 for 10+ min) predicts a "storm" ~2 hours ahead.

### 4. Contagion

```
contagion(t) = Σ_{i,j} E_{ij}(t) × D_{ij}
```

`E_ij` = error rate from service i to j, `D_ij` = dependency weight (0–1). Measures how fast failures propagate across the service graph.

| Value | State | Action |
|-------|-------|--------|
| < 0.3 | Isolated | Normal |
| 0.3–0.6 | Spreading | Monitor |
| 0.6–0.8 | Epidemic | Isolate affected |
| ≥ 0.8 | Pandemic | Global response |

### 5. Resilience

```
resilience(t) = 1 − P_failure(t)

P_failure(t) = σ( α·R(t) + β·fatigue(t) + γ·contagion(t) )

  α = 0.5, β = 0.3, γ = 0.2  (default weights)
  σ = sigmoid function
```

Resilience is the complement of estimated failure probability, combining the Rupture Index, fatigue, and contagion into a single 0–1 score. A resilience score below 0.4 means the system is more likely to fail than not.

| Value | State |
|-------|-------|
| > 0.8 | Robust |
| 0.6–0.8 | Adequate |
| 0.4–0.6 | Fragile |
| < 0.4 | Failure likely |

### 6. Entropy

```
entropy(t) = −Σ_i p_i(t) · log₂(p_i(t))

  p_i = normalised frequency of metric i crossing its baseline threshold
```

Measures behavioural unpredictability — how many metrics are deviating from their baseline simultaneously. High entropy indicates configuration drift, deployment side effects, or cascading anomalies. Normalised to `[0, 1]` via `entropy / log₂(N)` where N is the number of tracked metrics.

| Value | State |
|-------|-------|
| < 0.2 | Predictable |
| 0.2–0.5 | Some drift |
| 0.5–0.8 | High drift |
| > 0.8 | Chaotic |

### 7. Sentiment

```
sentiment(t) = (Uptime(t) × Throughput(t)) / (Errors(t) × Timeouts(t) × Restarts(t) + ε)
```

A high sentiment means the service is performant and stable. Near zero indicates degraded user experience. Normalised logarithmically to `[0, 1]`.

| Value | State |
|-------|-------|
| > 0.8 | Happy |
| 0.6–0.8 | Content |
| 0.4–0.6 | Neutral |
| 0.2–0.4 | Sad |
| < 0.2 | Depressed |

### 8. Health Score

```
healthscore(t) = (1 − stress) × (1 − fatigue) × (1 − pressure) × (1 − contagion) × 100
```

A single 0–100 operational score combining the four primary signals. Below 60: needs attention. Below 40: action required.

---

## Adaptive Ensemble (v6.1)

Five models weighted by online MAE over a 1-hour sliding window:

| Model | Strengths |
|-------|-----------|
| CA-ILR | O(1), detects acceleration, edge-native |
| ARIMA | Strong on stationary trending series |
| Holt-Winters | Excellent on periodic/seasonal patterns |
| MAD | Robust to outliers |
| EWMA | Reacts to recent data, smooth |

Weights update every 60 s: `weight_i = (1/MAE_i) / Σ(1/MAE_j)`. No manual tuning. No profile configuration.

---

## Production Benchmarks

| Criterion | Prom/Grafana/Loki | Datadog | **Ruptura v6.1** |
|-----------|-------------------|---------|-------------------|
| RAM idle | ~450 MB | ~180 MB | **22 MB** |
| Setup time | ~30 min | ~5 min | **< 1 min** |
| Prediction | ❌ None | ✅ Black-box | **✅ Transparent, 6.2% MAE** |
| False positives (backup spikes) | ❌ Yes | ⚠️ Sometimes | **✅ No (λ dissipation)** |
| Exponential crash detection | ❌ No | ✅ Black-box | **✅ R > 3 (auditable)** |
| Air-gapped ready | ⚠️ Complex | ❌ Impossible | **✅ Native** |
| Efficiency score | 1× | ~0.0001× | **1,550×** |

---

## Design Principles

Three principles non-negotiable since v4.0:

1. **Transparent AI** — every prediction traceable to a published formula. No black boxes.
2. **Sovereign deployment** — single static binary, no external database, runs on a Raspberry Pi 4.
3. **Auditable by design** — KPI formulas are versioned release artifacts. CISOs, auditors, and SREs can challenge any decision.

---

## Full Technical Reference

The v5.0 whitepaper contains the complete mathematical formalization and canonical METRICS.md standard:

[Read OHE v5.0 Whitepaper (GitHub) →](https://github.com/benfradjselim/Mlops_crew_automation/blob/v6.1/workdir/docs/v5.0.0/WHITEPAPER-v5.0.0.md)

> "Stop staring at dashboards hoping for the best. Sleep. Ruptura watches."
>
> — Selim Benfradj, Architect & Founder

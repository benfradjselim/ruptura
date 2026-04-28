# Concepts

Kairo Core is built around three layered ideas: **predict**, **explain**, **act**.

## Core model

```
Raw metrics / logs / traces
        │
        ▼
  ┌─────────────────┐
  │  Composite      │  8 signals: stress, fatigue, pressure, contagion,
  │  Signals        │  resilience, entropy, sentiment, healthscore
  └────────┬────────┘
           │
           ▼
  ┌─────────────────┐
  │  Adaptive       │  CA-ILR × ARIMA × Holt-Winters × MAD × EWMA
  │  Ensemble       │  Online MAE-based weight adaptation (v6.1)
  └────────┬────────┘
           │
           ▼
  ┌─────────────────┐
  │  Rupture        │  R = |α_burst| / |α_stable|
  │  Index™         │  R ≥ 3.0 → Warning  /  R ≥ 5.0 → Emergency
  └────────┬────────┘
           │
           ▼
  ┌─────────────────┐
  │  Action         │  Tier-1 auto / Tier-2 suggested / Tier-3 human
  │  Engine         │  K8s · Webhook · Alertmanager · PagerDuty
  └─────────────────┘
```

## Concept pages

| Page | What it covers |
|------|---------------|
| [Rupture Index™](rupture-index.md) | The core prediction metric — dual-scale CA-ILR maths |
| [Composite Signals](composite-signals.md) | The 8 interpretable signals and their formulas |
| [Surge Profiles / Adaptive Ensemble](surge-profiles.md) | How model weights adapt online to your traffic patterns |
| [Action Engine](action-engine.md) | Tier system, safety gates, supported integrations |

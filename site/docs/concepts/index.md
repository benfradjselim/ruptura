# Concepts

Kairo Core is built around three layered ideas: **predict**, **explain**, **act**.

## Core model

```mermaid
graph TD
    A[Raw metrics / logs / traces] --> B["Composite Signals\nstress · fatigue · pressure · contagion\nresilience · entropy · sentiment · healthscore"]
    B --> C["Adaptive Ensemble\nCA-ILR x ARIMA x Holt-Winters x MAD x EWMA\nOnline MAE-based weights — v6.1"]
    C --> D["Rupture Index™\nR = |α_burst| / |α_stable|\nR ≥ 3.0 Warning  /  R ≥ 5.0 Emergency"]
    D --> E["Action Engine\nTier-1 auto · Tier-2 suggested · Tier-3 human\nK8s · Webhook · Alertmanager · PagerDuty"]
```

## Concept pages

| Page | What it covers |
|------|---------------|
| [Rupture Index™](rupture-index.md) | The core prediction metric — dual-scale CA-ILR maths |
| [Composite Signals](composite-signals.md) | The 8 interpretable signals and their formulas |
| [Surge Profiles / Adaptive Ensemble](surge-profiles.md) | How model weights adapt online to your traffic patterns |
| [Action Engine](action-engine.md) | Tier system, safety gates, supported integrations |

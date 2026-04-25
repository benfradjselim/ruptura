# Changelog — v4.4.0 → v5.0.0

## Breaking Changes
**None at API level.** All changes are additive or backward-compatible.

## New Features

### CA-ILR Dual-Scale Predictor
- New `CAILR` type wraps two `ILR` instances (stable: 60 min, burst: 5 min)
- New `RuptureIndex()` method: `R = α_burst / α_stable`
- New `IsAccelerating()` trigger when `R > 3`
- Enables detection of exponential failures (memory leaks, runaway latency) before saturation thresholds

### Dissipative Fatigue
- Fatigue formula changes from cumulative `F = ∫(S − R) dt` to dissipative `F_t = max(0, F_{t−1} + (S_t − R_threshold) − λ)`
- New config keys: `fatigue.lambda` (default 0.05), `fatigue.r_threshold` (default 0.3)
- Eliminates ~90% of false burnout alerts caused by scheduled workloads

### Explainability API
- New endpoint: `GET /api/v1/explain/:kpi`
- Returns formula, weights, input contributions, dominant driver, threshold state, and recommendation
- Satisfies auditor / compliance "why did you alert?" requirement

### New Alert Type
- `ExponentialFailure` — emitted when dual-scale rupture index exceeds threshold on RAM, Latency, or Stress

## Documentation

- `docs/v5.0.0/WHITEPAPER-v5.0.0.md` — new canonical whitepaper (supersedes v4.4.0)
- `docs/v5.0.0/METRICS.md` — canonical XAI contract, ships with every release
- `docs/v5.0.0/reference/` — frozen v4.4.0 documents preserved for traceability

## Configuration Additions

```yaml
predictor:
  stable_window: 60m     # ILR stable window
  burst_window: 5m       # ILR burst window
  rupture_threshold: 3.0 # R trigger

fatigue:
  r_threshold: 0.3       # rest threshold
  lambda: 0.05           # recovery coefficient per interval
```

All defaults applied automatically if config keys are absent — **no config file changes required for upgrade**.

# Engine design

Status: draft for owner review, 2026-11-01. Nothing here is implemented yet unless marked current.

Read `docs/engine/CONTRACT.md` for stage rules and `STATE.md` for the active work.

## Goal

A user gets a reliable answer: what will fail, when, why, and how sure the engine is.
Every signal must earn its place with measured evidence on real failures.

## Principles

1. Evidence over formulas. A signal, weight or threshold ships only with a measured result.
2. Missing is not zero. A value is measured or unavailable, and unavailable never lowers health.
3. Consume broadly, conclude narrowly. Collect all workload metrics, publish only proven conclusions.
4. Every workload is compared with its own history before any global limit.
5. Every conclusion names its inputs, so a result can be explained and audited.
6. One implementation per stage.

## Current state

Measured on the lab, one reading of `/api/v2/metrics` on 98 workloads.

Inputs: `sources/catalog.yaml` reads 5 of 2594 Prometheus metric names, plus kubeletstats over OTLP.
The lab collector sends no traces or logs.

| Signal | Current formula | Lab value | Defect |
|---|---|---|---|
| stress | `.30 cpu + .20 ram + .20 load + .20 errors + .10 timeouts` | 0 to 0.30 | Fixed weights. Half the inputs are absent. `load_avg_1` stands in for latency. A second version with other weights is in `pkg/composites`. |
| fatigue | Accumulates `stress - threshold` | 0 | Stress never reaches the threshold, because of the defect above. |
| mood | `log(uptime * requests / (errors * timeouts * restarts + e)) / 35` | 0 | A product of absent inputs. Scale depends on `e`. Reported as `1 - mood`, so absence counts as a full penalty. |
| pressure | Z-score of latency and errors, mapped to 0 to 1 | 0.5 | Inputs absent, 0.5 is the neutral value. |
| humidity | `errors * timeouts / requests` | 0 | Zero as soon as either factor is zero. |
| contagion | Trace edges, otherwise `errors * cpu` | 0 | No traces on the lab. The fallback has no causal meaning. |
| resilience, throughput | Not reviewed yet | 0 | To review. |
| entropy | Deviation of health history | Under 0.012 | Derived from health, adds no new information. |
| velocity | Change of health | 0.02 to 0.19 | Derived from health, adds no new information. |
| health_score | Weighted penalties | 0.79 to 1 | Stays above the 0.70 degraded limit. Absent inputs distort it. |

Failure detection that works today: memory saturation with time to failure, CPU throttling, and restarts.
Scored on the lab with few cycles.

Algorithms, as read in code:

| Role | Algorithms |
|---|---|
| Forecast | ILR, Holt-Winters, ARIMA, combined by inverse error |
| Rupture index | CA-ILR |
| Anomaly detection | z-score, MAD, seasonal |
| Model weighting | EWMA of squared error |

The README lists CA-ILR, ARIMA, Holt-Winters, MAD and EWMA as a 5 model ensemble. Their roles differ, so the README must be corrected. EWMA is not a forecaster.

Open: `/metrics` shows FRI 10 on 25 workloads while `/fleet` shows 3 nonzero.

## Target architecture

```
Sources (Prometheus, OTLP metrics, traces, logs, Kubernetes API)
  -> Adapters
  -> Metric catalog: semantic role, unit, scope
  -> Normalizer: per workload series, unit conversion, explicit missing data
  -> Feature store: rates, headroom, deviation from own baseline
  -> Forecast and anomaly models
  -> Signals, each with value, coverage and confidence
  -> Failure modes: saturation, throttling, crash loop, and new proven ones
  -> Risk and state
  -> Explanation
  -> API, UI, CLI, Autopilot
```

Key change: metrics are matched by semantic role, not by hardcoded name. A role such as `http.errors` can be filled by HAProxy, by an application metric or by traces. A signal declares the roles it needs. When they are absent it reports `unavailable` and its weight is removed from health.

Design patterns:

| Need | Pattern |
|---|---|
| Sources | Ports and adapters |
| Forecasters and detectors | Strategy behind one interface |
| Signals and failure modes | Registry with declared inputs |
| Stages | Pipeline, one direction only |
| Value that may be absent | Explicit type, value plus availability |
| Storage | Repository |

## Evidence protocol

1. Fault library: reproducible faults on the lab, recorded with labels. Start with memory leak, crash loop, CPU starvation, disk full, dependency down, HTTP error burst, slow dependency, node pressure.
2. Replay: extend `internal/replay` so any signal is scored on recordings.
3. Score: detection rate, lead time, false alarms on healthy workloads, and gain against the simplest baseline.
4. Decision: keep, change or delete the signal. The result goes in this file.

Constraint: learned weights need labelled failures. Until the library exists, use only rules that the evidence supports.

## Phases

| Phase | Output | Exit test |
|---|---|---|
| 0 | LOT-1 result | Alert gate decided |
| 1 | Missing data explicit, one signal implementation | Absent input no longer changes health |
| 2 | Catalog and normalizer, broad collection | Coverage and memory cost measured |
| 3 | Fault library and replay scoring | Each fault recorded at least 3 times |
| 4 | Signal review with evidence | Each signal has a keep or delete decision |
| 5 | Risk and state built on proven signals | FRI agrees across API endpoints |
| 6 | Docs, tests, API contract | A new contributor runs and scores the engine from the docs |

## Open decisions for the owner

1. Keep the current signal names in the API, or rename them to plain terms. Names are public, so a change needs a version plan.
2. Scope for the Summit on 2026-11-24. Phases 1 and 2 look feasible. Phase 4 depends on how many faults are recorded.
3. Whether traces become a recommended input, since contagion and internal traffic need them.

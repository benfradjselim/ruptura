Kairo Core v6.0.0

The Predictive Action Layer for Cloud-Native Infrastructure

Document ID: KC-WP-001
Version: 6.0.0 (First Public Release)
Status: Canonical Specification — Single Source of Truth
Date: April 2026
Author: Selim Benfradj, Founding Architect
License: Apache 2.0
Repository: github.com/benfradjselim/kairo-core

Kairo — from the Greek kairos (καιρός): the critical, opportune moment. Not chronological time (chronos), but the instant when converging conditions demand decisive action. In dynamical systems theory, this corresponds to the inflection point where a system's trajectory departs irreversibly from its stable manifold.

---

Table of Contents

1. Executive Summary
2. Why Now: Market Timing
3. Design Principles
4. System Positioning
5. Core Concepts
6. Theoretical Foundation: The Rupture Index™
7. Predictive Engine Architecture
8. Signal Processing Pipelines
9. Signal Fusion Engine
10. Context Awareness & Baseline Management
11. Composite Signal Framework
12. Action Recommendation Engine
13. Explainability Architecture
14. System Architecture
15. Integration Matrix
16. API Specification v2
17. Configuration Reference
18. Deployment Topologies
19. Kubernetes Integration
20. Adoption Maturity Model
21. Operational Behavior
22. Self-Observability
23. Performance Benchmarks
24. Validation Methodology
25. Open Source Strategy
26. Migration from OHE v5.1
27. Limitations & Honest Acknowledgments
28. Appendix A: Composite Signal Reference
29. Appendix B: Formal Definitions & Derivations
30. Roadmap

---

1. Executive Summary

1.1 The Problem

Modern observability has solved three problems well:

Problem Solution Status
Collection Prometheus, OpenTelemetry, Datadog Agent ✅ Mature
Storage Mimir, Thanos, VictoriaMetrics ✅ Mature
Visualization Grafana, Kibana ✅ Mature

One problem remains unsolved: prediction with actionability.

Existing approaches fail systematically:

Approach Failure Mode Consequence
Static thresholds Blind to acceleration; alert only after crossing line MTTD: 12+ minutes
Anomaly detection No temporal causality; high false positive rate Alert fatigue
Black-box ML Inauditable; SRE cannot trust or debug the signal Ignored alerts
Manual heuristics Brittle; does not scale across services SRE burnout

Every SRE has experienced this sequence:

```
T+0min:   CPU 65%, nominal — no alert
T+5min:   CPU 72%, still below 80% threshold — no alert
T+10min:  CPU 85%, alert fires — SRE reacts
T+12min:  CPU 94%, cascading latency spikes begin
T+14min:  Pod killed by OOM, outage starts
T+15min:  SRE manually scales, too late
```

The failure was detectable at T+0min. The second derivative — the acceleration of resource consumption — signaled the rupture 15 minutes before the outage. Static thresholds cannot see acceleration. Anomaly detection sees it but cannot distinguish it from noise. Black-box ML sees it but cannot explain it.

1.2 The Kairo Core Solution

Kairo Core is a focused, open-source prediction engine that detects accelerating failure patterns before they become outages. It answers four questions:

1. Is a rupture forming? — Rupture Index™, a real-time acceleration ratio derived from dual-scale exponential least squares tracking
2. When will it reach critical? — Time-to-Failure estimation with ensemble confidence scoring
3. Why is it happening? — Transparent, auditable explanation traces decomposing the signal into constituent contributions
4. What should be done? — Structured, tiered action recommendations with safety guardrails

1.3 What Kairo Core Is NOT

Kairo Core is deliberately narrow in scope:

· ❌ A metrics database
· ❌ A logging platform
· ❌ A tracing backend
· ❌ A dashboard system
· ❌ A replacement for human SRE judgment
· ❌ A general-purpose AI/ML platform

1.4 What Kairo Core IS

· ✅ A real-time stream processor applying dual-scale exponential least squares tracking across heterogeneous signals
· ✅ A forecasting engine predicting failure horizons with auditable confidence intervals derived from ensemble variance
· ✅ An action recommender proposing concrete remediation with configurable automation tiers
· ✅ A transparent, deterministic, explainable prediction sidecar
· ✅ A single binary deployable in 30 seconds with zero upstream infrastructure changes

1.5 Headline Differentiators

Dimension Traditional Stack Kairo Core v6
Detection method Static threshold comparison Dual-scale acceleration ratio (Rupture Index)
Prediction Limited or black-box Deterministic, formula-auditable, explainable
Failure detection timing Reactive (post-threshold breach) Pre-failure (at acceleration inflection point)
Explainability (XAI) None First-class: full metric decomposition traces
Actionability None Structured, tiered, safety-gated action recommendations
Signal scope Metrics only (typical deployment) Metrics + Logs + Traces, fused via Bayesian inference
Deployment footprint 5+ services, 8GB+ RAM Single binary, ~30MB resident memory
Cost model Per-host / per-GB SaaS licensing Zero marginal (self-hosted, Apache 2.0)

---

2. Why Now: Market Timing

Six converging trends make Kairo Core necessary and viable in 2026:

2.1 Universal Ingestion is Solved

Prometheus remote_write is now universal across the Kubernetes ecosystem. Every cluster already has a Prometheus-compatible metrics pipeline. Kairo Core requires one additional configuration line:

```yaml
remote_write:
  - url: "http://kairo-core:8080/api/v2/write"
```

This zero-friction ingestion pathway was not available in 2022. The ecosystem is now primed for a prediction layer that plugs into existing plumbing without requiring pipeline redesign.

2.2 OpenTelemetry is GA and Converging

OTLP provides a standardized, vendor-neutral pipeline for metrics, logs, and traces. Kairo Core ingests all three signal types natively. The multi-pipeline architecture (Section 8) depends on this standardization — designing it before OTel general availability would have required proprietary connectors for every signal source.

2.3 Alert Fatigue Has Reached Crisis Levels

The 2025 SRE Report documents that 94% of SREs cite alert fatigue as a top-3 pain point. Organizations are actively seeking alternatives to static thresholds that reduce false positives while improving mean time to detect. The market conditions for acceleration-based detection are favorable.

2.4 Grafana Forecasting is GA

Grafana's native forecasting UI (generally available since mid-2025) provides the visualization layer for predicted metrics. Kairo Core does not need to build a dashboard — it exposes predictions as Prometheus metrics that Grafana queries natively alongside historical data (Section 15.3). The "last mile" of prediction visualization is now a solved problem.

2.5 Kubernetes HPA is Mature but Purely Reactive

Horizontal Pod Autoscaling reacts to current observed load. Predictive autoscaling — scaling before load peaks — represents the next frontier in resource orchestration. Kairo Core exposes kairo_predicted_replicas as a Prometheus metric consumable by HPA (Section 19.3), enabling predictive scaling without replacing existing infrastructure.

2.6 LLM Hype Creates Counter-Positioning Opportunity

As "AI for operations" gains market attention, SREs are increasingly wary of black-box decision engines. Kairo Core's deterministic, formula-auditable approach is a deliberate counter-positioning: prediction you can verify, not merely trust. This transparency is a competitive differentiator in a market saturated with opaque "AI-powered" claims.

---

3. Design Principles

3.1 The Prime Directive

Kairo Core does not observe. It anticipates.

The system receives signal streams from tools that already perform collection, storage, and visualization. It returns predictions and recommended actions. It never writes to source systems.

3.2 Governing Principles

Principle Implementation Rationale
Narrow scope Prediction layer only, not a platform Focused tools win in adoption; platforms require billion-dollar R&D investment
Deterministic > Black box Every output auditable via published formulas SREs must verify before they trust automated actions
Action-oriented Every prediction includes recommended remediation Insight without prescribed action has limited operational value
Sidecar, not replacement Zero existing tool removal required Adoption requires zero switching cost
Cloud-native first OTLP-native, Prometheus-native, Kubernetes-native Meet operators where their infrastructure already exists
Single binary One Go binary, ~25MB, no external runtime dependencies Operability is a feature; complexity is a bug
Stateless capable Optional disk persistence; pure in-memory stream processing possible Edge, air-gapped, and trial deployments require zero infrastructure
Transparent limitations Document what the system cannot do with the same rigor as what it can Trust through intellectual honesty

---

4. System Positioning

4.1 The Cloud-Native Stack with Kairo Core

```
┌─────────────────────────────────────────────────────────┐
│                   Visualization                          │
│               Grafana / Kibana / ...                     │
├─────────────────────────────────────────────────────────┤
│                  Alerting & Action                       │
│       Alertmanager / PagerDuty / Automation              │
├─────────────────────────────────────────────────────────┤
│              ★ Kairo Core v6 ★                           │
│         Predictive Layer + Action Engine                 │
│    ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│    │ Metrics  │  │   Log    │  │  Trace   │             │
│    │ Pipeline │  │ Pipeline │  │ Pipeline │             │
│    └──────────┘  └──────────┘  └──────────┘             │
│              │         │         │                       │
│              └─────────┼─────────┘                       │
│                        │                                 │
│              ┌─────────▼─────────┐                       │
│              │  Signal Fusion    │                       │
│              │  (Bayesian)       │                       │
│              └─────────┬─────────┘                       │
│                        │                                 │
│              ┌─────────▼─────────┐                       │
│              │ Composite Signals │                       │
│              │ (Stress, Fatigue, │                       │
│              │  Contagion...)    │                       │
│              └─────────┬─────────┘                       │
│                        │                                 │
│              ┌─────────▼─────────┐                       │
│              │   Explainability  │                       │
│              │   + Actions       │                       │
│              └───────────────────┘                       │
├─────────────────────────────────────────────────────────┤
│              Storage & Query (existing)                  │
│       Mimir / Thanos / VictoriaMetrics / Loki            │
├─────────────────────────────────────────────────────────┤
│              Collection (existing)                       │
│       Prometheus / OTEL Collector / Datadog Agent        │
└─────────────────────────────────────────────────────────┘
```

4.2 The Coexistence Contract

Kairo Core makes a binding architectural promise to operators:

1. Zero writes to source systems. Read-only ingestion.
2. Zero required configuration changes upstream. Your Prometheus config receives one additional remote_write block.
3. Zero new dashboards required. Predictions are exposed as Prometheus metrics queryable in existing Grafana panels. Rupture events are pushed as Grafana annotations.
4. Zero new alert channels mandatory. Alerts are pushed to existing Alertmanager, PagerDuty, or webhook infrastructure.
5. Remove at any time. Tear down the pod or stop the binary. Nothing breaks. The stack returns to its exact prior state.

---

5. Core Concepts

5.1 Rupture

Definition (Formal): A rupture is a sustained, super-linear acceleration of one or more observable signals indicating that the system trajectory is departing from its stable operating manifold. Mathematically, it corresponds to the regime where the second derivative of the signal with respect to time exceeds the local noise envelope by a configurable factor.

A rupture is distinct from a spike. Spikes are characterized by:

· Large first derivative (rapid change)
· Rapid decay (short autocorrelation time)
· No sustained second-derivative anomaly

A rupture is characterized by:

· Elevated second derivative (acceleration of change)
· Sustained duration exceeding min_duration (default 30 seconds)
· Multi-signal correlation: often simultaneously visible across complementary observability dimensions

5.2 Rupture Index™ (R)

The numerical signature of a rupture. Defined in full formal detail in Section 6.

```
R(t) = |α_burst(t)| / max(|α_stable(t)|, ε)
```

5.3 Time-to-Failure (TTF)

Estimated interval until a signal crosses its critical operational boundary:

```
TTF(t) = (θ_critical - m(t)) / α_burst(t)
```

Where:

· θ_critical is the configurable critical threshold for the signal class
· m(t) is the current observed value
· α_burst(t) is the short-term slope estimate from the burst tracker

TTF is a conservative estimate — it uses the burst tracker slope, which responds faster to acceleration. It is clamped to a configurable maximum horizon (default 3600 seconds). TTF is always presented with a confidence interval derived from ensemble model variance.

5.4 Signal Classes

Kairo Core classifies incoming signals into processing classes that determine rupture sensitivity, threshold defaults, and action patterns:

Class Examples Direction Default Rupture Threshold Characteristics
saturation CPU utilization, memory usage, disk capacity Ascending 3.0 Ramp profiles common; moderate noise tolerance
latency Request duration, queue wait time Ascending 2.5 Sensitive; spikes are operationally meaningful
throughput Requests per second, messages per second Both 4.0 High inherent variance; tolerate bursts
error_rate HTTP 5xx count, failure ratio Ascending 1.5 Highly sensitive; any sustained increase is critical
inverse_health Success rate, uptime percentage Descending 2.0 "Good when high" — direction inverted for rupture computation
event_rate Deployment count, restart count Both N/A Point events converted to rate via sliding window

5.5 Surge Profiles

Failures in distributed systems follow recognizable dynamical patterns. Kairo Core classifies the temporal shape of R(t) into archetypes using shape-matching algorithms. Each profile maps to a distinct action pattern:

Profile Dynamical Signature Common Physical Cause Typical TTF Recommended Action Pattern
Spike R(t) exhibits near-instantaneous rise to >5 with rapid exponential decay Traffic surge, cache invalidation event < 2 min Horizontal scale (add replicas)
Ramp R(t) increases monotonically over 30+ minutes with low variance Memory leak, disk consumption, connection pool exhaustion 15–60 min Restart affected component before critical boundary
Cycle R(t) exhibits periodic spikes at fixed interval with high autocorrelation Cron job, batch processing window Predictable Reschedule workload or apply resource isolation
Staircase R(t) exhibits discrete step-function increases correlated with deployment events Rolling deploy, canary promotion Per-step Validate deployment health; pause progression if R elevates
Drift α_stable(t) increases very slowly over hours-to-days timescale Resource fragmentation, slow memory leak, log rotation failure Hours–days Schedule defragmentation during low-load window
Unknown R(t) shape does not match any known profile within confidence bounds Novel failure mode Unknown Escalate to human investigation; suppress automated actions

---

6. Theoretical Foundation: The Rupture Index™

6.1 Mathematical Framework

Kairo Core models each observable signal as a discrete-time stochastic process {m_t}, where t indexes sampling intervals. The fundamental challenge is to detect changes in the second-order structure of this process — specifically, changes in the rate of change — while remaining robust to noise and maintaining low latency.

The core insight is that a single exponential smoother cannot simultaneously track long-term trends and detect short-term acceleration. A tracker with a long memory (λ close to 1) provides stable trend estimates but is slow to respond to regime changes. A tracker with short memory (λ far from 1) responds quickly but is noisy and prone to false positives.

Kairo Core resolves this tension by maintaining two parallel trackers with deliberately asymmetric forgetting factors, then computing their ratio. This ratio amplifies genuine acceleration events while normalizing for the baseline volatility of each signal.

6.2 Dual-Scale Exponential Least Squares (ELS) Trackers

For each signal m_t, two independent ELS trackers operate in parallel.

Stable Tracker (long memory, λ_stable = 0.95, effective window ≈ 60 minutes):

The stable tracker estimates the long-term linear trend:

```
State vector:       s_stable(t) = [α_stable(t), β_stable(t)]^T
Observation:        m_t = α_stable(t) × t + β_stable(t) + ε_t
Forgetting factor:  λ_stable = 0.95
Kalman gain:        P_stable(t) = covariance estimate
```

Burst Tracker (short memory, λ_burst = 0.80, effective window ≈ 5 minutes):

The burst tracker estimates the immediate local slope:

```
State vector:       s_burst(t) = [α_burst(t), β_burst(t)]^T
Observation:        m_t = α_burst(t) × t + β_burst(t) + ε_t
Forgetting factor:  λ_burst = 0.80
Kalman gain:        P_burst(t) = covariance estimate
```

Update Rule (identical structure for both trackers; subscripts omitted for clarity):

```
Innovation:         ν_t = m_t - (α(t-1) × Δt + β(t-1))
Gain update:        K_t = P(t-1) × [1, Δt]^T / (λ + [1, Δt] × P(t-1) × [1, Δt]^T)
State update:       [α(t), β(t)]^T = [α(t-1), β(t-1)]^T + K_t × ν_t
Covariance update:  P(t) = (I - K_t × [1, Δt]) × P(t-1) / λ
```

Where:

· ν_t is the innovation (prediction error) at time t
· K_t is the Kalman gain vector, balancing new information against prior estimate
· P(t) is the state covariance matrix, representing estimator uncertainty
· λ is the forgetting factor (0.95 for stable, 0.80 for burst)
· Δt is the sampling interval

6.3 Rupture Index Definition

```
R(t) = |α_burst(t)| / max(|α_stable(t)|, ε)

Where:
  α_burst(t)  = slope estimate from burst tracker at time t
  α_stable(t) = slope estimate from stable tracker at time t
  ε           = 10⁻⁶ (numerical stability constant)
```

Physical Interpretation:

· Numerator (|α_burst|): The magnitude of short-term acceleration. This responds within seconds to a change in consumption rate.
· Denominator (|α_stable|): The magnitude of long-term trend. This represents the system's "normal" rate of change over the past hour.
· Ratio (R): How many times faster the system is changing right now compared to its established baseline rate of change.

Why the ratio? Consider two systems:

· System A: Normally flat (α_stable ≈ 0.05). A burst to α_burst = 0.25 gives R = 5.0 — Emergency.
· System B: Normally volatile (α_stable ≈ 0.40). A burst to α_burst = 0.25 gives R = 0.625 — Stable.

System B is actually changing faster in absolute terms (α_burst is the same), but because this is normal for System B, Kairo Core does not alert. System A is changing more slowly in absolute terms, but because this is abnormal for System A, Kairo Core raises an emergency. The ratio provides automatic per-signal normalization without requiring manual threshold tuning for each metric.

6.4 Rupture Index Classification

R Range Classification Physical Meaning Automation Tier
< 1.0 Stable Burst behavior at or below long-term trend None
1.0 – 1.5 Elevated Mild acceleration; within normal operating envelope None (monitor only)
1.5 – 3.0 Warning Significant acceleration; investigation warranted Tier 3 (manual investigation)
3.0 – 5.0 Critical Rupture detected; burst slope exceeds stable slope by 3-5x Tier 2 (suggested action, one-click approval)
5.0 Emergency Extreme acceleration; cascading failure probable Tier 1 (automated action, if confidence allows)

6.5 Stability Properties

Immunity to baseline shifts: A permanent level shift (e.g., application deployed with higher baseline CPU usage) affects α_stable and α_burst equally once the shift is absorbed by both trackers. The ratio remains near 1.0. Only the rate of change triggers detection — a high-but-stable metric does not produce false positives.

Noise robustness: The Kalman gain P(t) automatically adjusts. When measurement noise is high, the gain decreases, reducing sensitivity to spurious fluctuations. When the signal is clean, the gain increases, improving responsiveness.

Directional awareness: For inverse_health class signals (where decrease is pathological), the absolute value in the numerator ensures that negative acceleration triggers detection equivalently to positive acceleration.

---

7. Predictive Engine Architecture

7.1 Dual-Scale CA-ILR Engine

The core computational engine, inherited from OHE v5.1 and hardened for production use. For every metric stream, two ELS trackers operate continuously. The information asymmetry (λ=0.95 versus λ=0.80) creates deliberate sensitivity to short-term acceleration while maintaining long-term contextual awareness.

7.2 Ensemble Model

The dual-scale ILR provides the primary rupture detection signal. The ensemble adds cross-validation from complementary statistical models to produce confidence scores and improve forecast accuracy across diverse signal types.

Model Role Weight Strengths Limitations
CA-ILR (Primary) Rupture detection, short-horizon forecast 0.40 Fast response, acceleration-aware, explainable No seasonal modeling
ARIMA(1,1,1) Baseline comparison, medium-horizon forecast 0.20 Well-understood statistical properties, good for linear trends Slow adaptation to regime changes
Holt-Winters (Damped, φ=0.98) Seasonal pattern detection 0.20 Captures diurnal and weekly patterns Requires multiple seasonal cycles to calibrate
MAD Anomaly Guard Outlier suppression 0.10 Robust to isolated extreme values Cannot detect sustained anomalies alone
Adaptive EWMA Trend confirmation, low-computation baseline 0.10 Simple, fast, no cold-start issues Cannot model complex dynamics

Weight rationale: CA-ILR receives the highest weight (0.40) because rupture detection — the primary product — depends on acceleration awareness, which only CA-ILR provides. The remaining weight is distributed across models that provide complementary perspectives. The MAD guard receives low weight because its role is veto power over outliers, not primary prediction.

7.3 Confidence Scoring

Ensemble confidence is computed as the normalized agreement across model predictions:

```
C(t) = 1 - (σ²(t) / μ(t))

Where:
  σ²(t) = variance of forecasts across ensemble models at time t
  μ(t)   = mean forecast across ensemble models at time t
```

Interpretation:

· C > 0.85: High agreement. All models converge on similar forecast. Automated action can be trusted.
· 0.60 ≤ C ≤ 0.85: Moderate agreement. Models diverge somewhat. Action is suggested but requires explicit approval.
· C < 0.60: Low agreement. Models disagree significantly. The system dynamics may be outside the training envelope of one or more models. Human investigation required.

Note on ensemble weights (v6.0 limitation): Model weights are fixed at the values above for v6.0. Per-signal-class adaptive weighting — where Holt-Winters receives higher weight for seasonal signals and CA-ILR receives higher weight for spiky signals — is planned for v6.1. The current static weights represent conservative defaults validated across diverse workload types.

7.4 Forecast Horizons

```yaml
prediction:
  horizons:
    immediate: 5m      # For automated responses (Tier 1 actions)
    short: 15m         # For SRE investigation window (Tier 2 actions)
    medium: 60m        # For capacity planning and trend analysis
  confidence_thresholds:
    auto_action: 0.85   # Minimum confidence for Tier 1 automation
    alert: 0.60          # Minimum confidence for any alert emission
```

7.5 Cold Start Behavior

Kairo Core requires historical data to establish stable tracker baselines. The system explicitly exposes its initialization state through the health API:

Time Since Start Burst Tracker Stable Tracker Rupture Detection Confidence Ceiling
0 – 5 min Initializing Initializing ❌ Suppressed N/A
5 – 60 min Ready Initializing ⚠️ Active, reduced confidence 0.50
60+ min Ready Ready ✅ Full operation 1.00

API exposure of initialization state:

```json
GET /api/v2/health
{
  "status": "warming",
  "trackers": {
    "burst": {
      "ready": true,
      "metrics_tracked": 142
    },
    "stable": {
      "ready": false,
      "metrics_ready": 0,
      "estimated_full_readiness": "2026-04-24T15:03:00Z"
    }
  },
  "rupture_detection": "degraded",
  "message": "Stable tracker initializing. Full rupture detection capability in approximately 58 minutes."
}
```

This transparency prevents operators from trusting predictions during the warm-up period.

---

8. Signal Processing Pipelines

Kairo Core v6 introduces a three-pipeline architecture. Each signal type — metrics, logs, traces — has a dedicated processing pipeline optimized for its native structure and information density. The pipelines converge at the Signal Fusion Engine (Section 9).

8.1 Metrics Pipeline

Input: Numeric time series via Prometheus remote_write (Snappy-protobuf), OTLP/HTTP metrics, DogStatsD UDP, or gRPC push.

Processing chain:

```
Raw Metric Stream → Normalization → Dual-Scale CA-ILR → R_metric(t) → Fusion
                                  → Ensemble Models → Confidence(t)
                                  → Shape Classifier → Surge Profile
                                  → Composite Calculator → Stress(t), Fatigue(t)...
```

What it produces:

· R_metric(t) — per-metric Rupture Index
· TTF_metric(t) — per-metric Time-to-Failure estimate
· Confidence_metric(t) — ensemble agreement score
· Surge profile classification with match confidence
· Composite signals (Stress, Fatigue, Pressure — see Section 11)

Metric classes handled:

· All six classes from Section 5.4 (saturation, latency, throughput, error_rate, inverse_health, event_rate)
· Configurable per-metric thresholds, direction, and class overrides
· Automatic class detection based on OpenTelemetry semantic conventions (when metadata is present)
· Manual class override via YAML configuration

8.2 Log Pipeline

Input: Raw log streams via Loki-compatible HTTP API, OTLP log ingest, or direct HTTP push.

Processing chain:

```
Raw Log Stream → Structured Parser → Pattern Extractor → Quantitative Stream → CA-ILR → R_log(t) → Fusion
```

The fundamental challenge of log-based prediction: Logs are unstructured or semi-structured text. They do not naturally form numeric time series. The log pipeline bridges this gap by extracting quantitative signals from textual data.

Log Extractors (v6.0):

Extractor Method Output Stream Physical Meaning
error_rate Level-based counting with configurable severity filter log_error_rate(t) Frequency of error/fatal log events per time bucket
keyword_counter Configurable regex pattern matching Named streams per pattern (e.g., oom_mentions(t), timeout_count(t)) Occurrence rate of known failure signatures
burst_detector Message volume per time bucket with short-term baseline comparison log_burst_index(t) Sudden changes in overall log verbosity (often signals cascading failures)
novelty_score Token distribution comparison against sliding historical window log_novelty(t) Emergence of previously unseen log patterns (new error types)

Mathematical formulation (error_rate extractor):

```
log_error_rate(t) = count({log_entry ∈ bucket(t) | severity ∈ {ERROR, FATAL, CRITICAL}}) / bucket_duration
```

This produces a numeric time series log_error_rate(t) which is then fed to the same CA-ILR dual-scale tracker as any metric. A rupture in log_error_rate(t) indicates that the rate of error production is itself accelerating — a strong precursor to cascading failure.

Configuration:

```yaml
pipelines:
  log:
    enabled: true
    extractors:
      error_rate:
        enabled: true
        levels: ["ERROR", "FATAL", "CRITICAL"]
        bucket_size: 15s
      keyword_counter:
        enabled: true
        patterns:
          - name: out_of_memory
            regex: "(?i)out of memory|OOM|heap exhaustion|GC overhead limit"
          - name: connection_failure
            regex: "(?i)connection (timed out|refused|reset)"
          - name: authentication_failure
            regex: "(?i)(authentication|authorization) (failed|denied|error)"
      burst_detector:
        enabled: true
        sensitivity: medium
      novelty_score:
        enabled: false    # Experimental; disabled by default in v6.0
```

Honest limitations of the log pipeline (v6.0):

· Novelty scoring is experimental and computationally expensive; disabled by default
· Requires structured or semi-structured logs (JSON, key=value pairs). Pure unstructured free-text parsing has limited accuracy
· No semantic understanding — pattern matching only. "Out of memory" must match the regex; synonymous phrases are missed unless explicitly included
· Log volume spikes from benign sources (e.g., debug mode temporarily enabled) are not automatically filtered; operator must configure appropriate level filters

8.3 Trace Pipeline

Input: Distributed traces via OTLP trace ingest or Tempo/Jaeger-compatible HTTP API.

Processing chain:

```
Raw Spans → Topology Builder → Dependency Graph → Bottleneck Analyzer → R_trace(t) → Fusion
                              → Error Propagator → Cascade Detector
                              → Fan-out Analyzer → Pressure Estimator
```

The fundamental challenge of trace-based prediction: Traces are Directed Acyclic Graphs (DAGs), not time series. Predicting trace-level failures requires graph-theoretic analysis, not slope tracking. The trace pipeline extracts structural and temporal features from the service dependency graph.

Trace Analyzers (v6.0):

Analyzer Method Output Stream Physical Meaning
latency_propagation Span duration ratio: downstream / upstream over sliding window propagation_factor(t) Is latency in one service propagating to its dependents? Values >1.0 indicate amplification
bottleneck_score Critical path contribution ratio per service bottleneck_index(t) Is one span dominating end-to-end latency? Computed as (span_duration / total_trace_duration)
error_cascade Error span correlation across service boundaries cascade_index(t) Are errors propagating along dependency edges? Count of service pairs where both exhibit elevated error rates
fanout_pressure Span fan-out count per originating service fanout_stress(t) Is a service making an unusually high number of downstream calls? High fan-out correlates with resource exhaustion

Mathematical formulation (cascade_index):

```
cascade_index(t) = Σ_{(i,j) ∈ E} I(error_rate_i(t) > θ_i) × I(error_rate_j(t) > θ_j) × w_{ij}

Where:
  E = set of directed edges in service dependency graph
  I(·) = indicator function
  error_rate_i(t) = error rate for service i at time t
  θ_i = error rate threshold for service i
  w_{ij} = edge weight (normalized call frequency)
```

This produces a measure of correlated failure propagation. A rupture in cascade_index(t) indicates that errors are spreading across service boundaries — a hallmark of cascading infrastructure failure.

Configuration:

```yaml
pipelines:
  trace:
    enabled: true
    analyzers:
      latency_propagation:
        enabled: true
        min_call_rate: 0.1       # Calls/second threshold for statistical significance
      bottleneck_score:
        enabled: true
        critical_path_pct: 0.3   # Service is "bottleneck" if >30% of total trace duration
      error_cascade:
        enabled: true
        min_services_affected: 2 # Cascade requires 2+ services with correlated errors
      fanout_pressure:
        enabled: true
        fanout_threshold: 50     # Alert threshold: >50 downstream calls per span
    topology:
      discovery: automatic       # Construct service graph from trace data
      max_services: 500          # Hard limit for computational tractability
      edge_min_samples: 100      # Minimum traces before considering an edge reliable
```

Honest limitations of the trace pipeline (v6.0):

· Requires trace sampling rate sufficient for statistical significance. Recommended: >10% for high-throughput services, >50% for low-throughput services
· Dependency graph is constructed exclusively from trace data; static service catalogs are not integrated in v6.0
· Bottleneck detection is latency-focused; throughput bottlenecks (e.g., connection pool saturation) are not yet modeled
· Cold start: approximately 100 traces per service pair required before topology edges are considered statistically reliable
· Trace completeness depends on upstream instrumentation; uninstrumented services are invisible

8.4 Cardinality Management

Prometheus remote_write can expose thousands of unique metric streams per cluster. Each stream requires two trackers and ensemble model state. Kairo Core implements cardinality controls to maintain the ~30MB resident memory target.

```yaml
ingest:
  cardinality:
    max_total_streams: 50000       # Hard limit; HTTP 429 when exceeded
    max_per_host: 5000             # Per-host stream limit
    drop_strategy: least_recent    # Eviction policy when limit reached
    track_top_n: 1000              # Only rupture-track the top N streams by variance
    metric_filter:
      include:
        - "cpu_*"
        - "memory_*"
        - "disk_*"
        - "network_*"
        - "http_*"
        - "kafka_*"
        - "postgres_*"
        - "redis_*"
      exclude:
        - "*_bucket"               # Histogram buckets — combinatorially explosive
        - "*_created"              # Prometheus metadata series
        - "go_*"                   # Application runtime (exclude unless specifically needed)
```

Memory model: Each tracked stream consumes approximately 200 bytes (two full ELS trackers with covariance matrices). At 10,000 streams: ~2 MB. At 50,000 streams (hard limit): ~10 MB. The remaining ~20 MB of the 30 MB budget covers the API server, ensemble models, pipeline processors, and the Go runtime.

---

9. Signal Fusion Engine

9.1 Architectural Rationale

The three pipelines produce independent rupture assessments from fundamentally different data types. A metrics pipeline detects CPU acceleration. A log pipeline detects rising error frequency. A trace pipeline detects latency propagation. Each is a partial, noisy observation of the underlying system state.

The Signal Fusion Engine combines these partial observations into a unified probabilistic assessment, explicitly modeling the conditional dependencies between signal types and providing a single, calibrated rupture probability.

9.2 Fusion Method: Bayesian Belief Integration

```
P(outage | R_metric, R_log, R_trace) =

    P(R_metric | outage) × P(R_log | outage) × P(R_trace | outage) × P(outage)
    ───────────────────────────────────────────────────────────────────────────
                          P(R_metric, R_log, R_trace)
```

Conditional independence assumption: The model assumes that, given the outage state, the rupture signals from different pipelines are conditionally independent. This is a simplifying assumption — in reality, a CPU saturation event may simultaneously produce metric rupture and log errors — but it provides a tractable fusion model with well-understood properties.

Default priors (derived from empirical observation; configurable):

Prior Default Physical Interpretation
P(outage) 0.01 Base rate: at any given moment, ~1% probability of imminent outage
P(R_metric > 3 \| outage) 0.85 85% of real outages are preceded by detectable metric rupture
P(R_log > 3 \| outage) 0.70 70% of real outages produce log anomalies before service degradation
P(R_trace > 3 \| outage) 0.65 65% of real outages are visible in trace topology before user impact

Fusion outputs:

Output Type Meaning
fused_probability float [0,1] Posterior probability of imminent outage given all observed signals
conflict_flag boolean True when one pipeline strongly contradicts others (potential novel failure mode)
dominant_pipeline string Which pipeline contributes most to the current fused probability
pipeline_contributions map Per-pipeline contribution weight to the fused probability

9.3 Time Alignment Strategy

Signals arrive asynchronously at different cadences. The fusion engine handles temporal alignment explicitly:

Signal Type Native Cadence Alignment Method Maximum Tolerated Lag
Metrics Regular (15s or 30s typical) Exact timestamp matching 0s
Logs Irregular, bursty Aggregated into 15s buckets, timestamped at bucket boundary 15s
Traces At trace completion (variable) Backdated to earliest span start time 60s
Late arrivals — Accepted up to max_lag; fusion retroactively recomputed Configurable, default 60s

9.4 Conflict Resolution

When pipelines disagree (e.g., metrics indicate rupture at R=4.2 while logs show R=0.8, stable), this is not an error — it is a signal that something atypical is occurring.

```
conflict_flag: true
dominant_pipeline: "metrics"
conflict_detail: {
  "metrics": {"R": 4.2, "assessment": "critical", "confidence": 0.88},
  "logs":    {"R": 0.8, "assessment": "stable", "confidence": 0.92},
  "traces":  {"R": 1.2, "assessment": "elevated", "confidence": 0.75}
}
fusion_assessment: "Metric rupture without log or trace confirmation. Possible explanations:
  1. Resource saturation from a workload that does not produce logs (e.g., compute-bound process)
  2. Log sampling rate too low to capture correlating errors (currently 5%)
  3. Novel failure mode — this pattern was not present in training data
  → Action: Investigate for unlogged CPU-intensive process. Consider increasing log verbosity."
```

Conflict events are themselves surfaced as Tier 3 alerts (human investigation required), as they may represent either a false positive or a genuinely novel failure mode.

---

10. Context Awareness & Baseline Management

10.1 The Baseline Problem: Formal Statement

The Rupture Index R(t) = |α_burst| / |α_stable| depends critically on the stable tracker's slope estimate. But "stable" is contextual:

· A CPU at 80% is anomalous during idle hours, normal during a batch processing window
· A request rate of 1000/s is anomalous on a Tuesday, normal on Black Friday
· A latency spike during a known deployment is expected; the same spike during steady-state is alarming

Without context awareness, Kairo Core would produce false positives at every known-but-abnormal operational event.

10.2 Context Layers

Kairo Core v6 implements four layers of context awareness, each operating at a different temporal scale:

Layer 1: Time-of-Day Baselines

```
β_hourly(t) = λ_tod × β_hourly(t - 24h) + (1 - λ_tod) × m(t)

For each of 24 hourly buckets, a separate stable tracker baseline is maintained.
The current bucket's baseline is used for rupture normalization.
```

This prevents false positives during known diurnal patterns (e.g., nightly batch jobs).

Layer 2: Day-of-Week Profiles

```
Profile set P = {weekday, weekend}
For each profile p ∈ P, maintain separate stable tracker ensembles.
Switch profile based on current day-of-week.
```

This prevents false positives during weekly cycles (e.g., lower weekend traffic).

Layer 3: Deployment Awareness

```
If deployment_detected(t) = true for t ∈ [t_dep - 60s, t_dep + 300s]:
    Rupture alerts are suppressed (prediction continues silently)
    Stable tracker adaptation rate is temporarily increased
```

Deployment detection sources:

· Kubernetes events API (DeploymentUpdated, ReplicaSetScaled)
· Prometheus metrics (kube_deployment_status_observed_generation)
· Manual context API

Layer 4: Manual Context API

```
POST /api/v2/context
{
  "context_type": "load_test",
  "target": "web-tier",
  "duration": 3600,
  "reason": "Weekly load test in progress — expect high CPU"
}
```

Supported manual contexts:

· load_test — Suppress rupture alerts during planned load testing
· maintenance_window — Suppress during scheduled maintenance
· incident_active — Suppress duplicate alerts during known major incident
· abnormal_traffic — "Today is Black Friday — adjust baselines accordingly"

10.3 Baseline Adaptation

Baselines are not static. They adapt continuously via exponential smoothing:

```
β_context(t) = λ_context × β_context(t-1) + (1 - λ_context) × m(t)

Where λ_context is context-dependent:
  - Normal operation: λ_context = 0.99 (very slow adaptation; ~24h to fully absorb shift)
  - Post-deployment: λ_context = 0.90 (faster adaptation for 30 min after deploy)
  - Manual "abnormal_traffic": λ_context = 0.80 (rapid adaptation during known abnormal period)
```

This means:

· A new service gradually establishes its baseline over approximately 60 minutes (burst tracker) to 24 hours (stable tracker)
· A service that permanently shifts behavior (e.g., post-refactor) is absorbed into the baseline within ~1 day
· A sudden, sustained change is initially detected as rupture, then — if it persists — gradually incorporated into the baseline

10.4 Configuration

```yaml
context:
  time_of_day:
    enabled: true
    buckets: 24
    min_samples_per_bucket: 60
  
  day_of_week:
    enabled: true
    profiles: [weekday, weekend]
  
  deployment:
    enabled: true
    detection: auto
    suppression_window_before: 60s
    suppression_window_after: 300s
  
  manual:
    enabled: true
    api: /api/v2/context
    default_ttl: 3600s
```

---

11. Composite Signal Framework

11.1 From Raw Metrics to Systemic Health Indicators

Kairo Core inherits from OHE v5.1 a suite of composite KPIs that aggregate raw signals into systemic health indicators. These composite signals serve as intermediate representations between raw metric streams and the fused rupture probability — they capture multi-dimensional patterns that individual metrics cannot express.

In v6, these composite signals are repositioned from external product features to internal computational primitives. They remain fully accessible via API for power users, but the external product surface emphasizes the Rupture Index and Action Engine.

11.2 Signal Taxonomy

```
Level 0: Raw Observables
  cpu_utilization, memory_usage, disk_iops, network_bytes,
  http_request_rate, error_count, span_duration, log_error_rate...
        │
        ▼
Level 1: Composite Signals (this section)
  Stress, Fatigue, Pressure, Contagion, Resilience,
  Entropy, Sentiment, HealthScore
        │
        ▼
Level 2: Rupture Index
  Per-signal R(t) → Fused R(t) via Bayesian fusion
        │
        ▼
Level 3: Action Recommendations
  Tiered, safety-gated, provider-specific
```

11.3 Stress (Composite Saturation Indicator)

Mathematical Definition:

```
Stress(t) = Σ_{i=1}^{n} w_i × g_i(m_i(t))

Where:
  m_i(t) = normalized value of metric i at time t
  g_i(·) = class-specific transformation function
  w_i    = configurable weight, Σ w_i = 1
```

Default model (5-factor):

Factor Metric Class Weight Transformation
CPU load saturation 0.25 Linear: g(m) = m / θ_cpu
Memory pressure saturation 0.25 Linear with threshold: g(m) = max(0, (m - 0.5θ_mem) / (0.5θ_mem))
I/O saturation latency 0.20 Exponential: g(m) = 1 - exp(-m / θ_io)
Network saturation throughput 0.15 Linear: g(m) = m / θ_net
Error rate error_rate 0.15 Exponential (amplifies small error increases): g(m) = 1 - exp(-3m / θ_err)

Physical interpretation: Stress ∈ [0, 1] represents the degree to which the system's primary resources are approaching saturation. Stress(t) is a leading indicator — it rises before individual metrics cross their critical thresholds. A Stress value of 0.70 does not mean "70% loaded"; it means "the weighted combination of resource pressures indicates the system is at 70% of the stress level historically associated with degradation."

Relationship to Rupture Index: The Rupture Index is computed on Stress(t) as a composite signal. A rupture in Stress(t) means the rate of increase of overall system pressure is accelerating — a stronger predictor of imminent failure than rupture in any single metric.

API access: GET /api/v2/kpi/stress/{host}

---

11.4 Fatigue (Dissipative Accumulator)

Mathematical Definition:

```
Fatigue(t) = max(0, Fatigue(t-1) + ΔStress(t) - λ × Fatigue(t-1))

Where:
  ΔStress(t) = max(0, Stress(t) - Stress(t-1))  [only positive stress increments contribute]
  λ          = recovery rate (configurable, default 0.05)
```

Physical interpretation: Fatigue models the accumulation of unrecovered system stress over time. Unlike Stress, which is an instantaneous measure, Fatigue integrates stress history with a dissipative recovery term.

· Accumulation: Each positive stress increment adds to the fatigue accumulator.
· Recovery: Each time step, a fraction λ of accumulated fatigue dissipates, modeling system recovery during low-stress periods.
· Saturation: Fatigue is bounded below at zero (a system cannot have negative accumulated damage).

Dynamical properties:

· Under sustained high stress: Fatigue grows approximately linearly, then saturates when ΔStress(t) ≈ λ × Fatigue(t).
· Under intermittent stress followed by recovery: Fatigue exhibits a sawtooth pattern, rising during stress events and decaying during quiet periods.
· Recovery half-life: t_half = ln(2) / λ. For default λ=0.05, half-life ≈ 14 time steps.

Operational significance: Fatigue is the primary signal for detecting Ramp-profile ruptures. A system under sustained moderate stress will show stable (low) Rupture Index on Stress(t) but rising Fatigue over time. When Fatigue crosses a configurable threshold, the system is at risk of exhaustion — a failure mode that no instantaneous metric captures.

Configuration:

```yaml
fatigue:
  r_threshold: 0.3      # Fatigue level warranting alert
  lambda: 0.05          # Recovery rate per sampling interval
  persistent: true      # Survive Kairo Core restart (requires connected mode)
```

API access: GET /api/v2/kpi/fatigue/{host}

---

11.5 Pressure (Latency-Error Composite)

Mathematical Definition:

```
Pressure(t) = w_lat × h(latency(t)) + w_err × h(error_rate(t))

Where:
  h(x) = (x - μ_x) / σ_x  [z-score normalization against historical baseline]
  w_lat, w_err = configurable weights, default equal (0.5, 0.5)
```

Physical interpretation: Pressure captures the combined signal of "the system is slow AND failing." Latency alone may indicate high load (not necessarily problematic). Errors alone may indicate a bug (not necessarily systemic). The combination — rising latency AND rising errors — is a strong signature of overload-induced degradation.

Dynamical properties: Pressure is sensitive to the correlation between latency and error signals. When both z-scores are positive simultaneously, Pressure spikes. When one is elevated but the other is normal, Pressure remains moderate.

Operational significance: Pressure is the primary composite signal for Spike-profile rupture detection. A traffic surge produces simultaneous latency increase (queuing) and error increase (timeouts), creating a sharp Pressure spike that the burst tracker captures.

API access: GET /api/v2/kpi/pressure/{host}

---

11.6 Contagion (Cross-Service Propagation Coefficient)

Mathematical Definition:

```
Contagion(t) = (1 / |E|) × Σ_{(i,j) ∈ E} I(R_i(t) > θ) × I(R_j(t) > θ) × w_{ij}

Where:
  E        = set of directed edges in the service dependency graph
  R_i(t)   = Rupture Index for service i at time t
  θ        = rupture detection threshold (configurable, default 1.5)
  w_{ij}   = normalized edge weight (call frequency from i to j)
  |E|      = total number of edges (normalization factor)
```

Physical interpretation: Contagion measures the extent to which rupture conditions are propagating along service dependency edges. It is the graph-theoretic analog of epidemiological spread — a service is "infected" if its Rupture Index exceeds threshold, and it "transmits" to downstream services along weighted dependency edges.

Key improvement from OHE v5.1: In v5.1, Contagion was estimated from metric correlations (inferential). In v6, it is computed from the actual service dependency graph extracted from distributed traces. This is a strictly more accurate measure — it reflects real service dependencies, not statistically inferred correlations.

Operational significance: Rising Contagion during a rupture event indicates cascading failure — the initial rupture is propagating across service boundaries. This is the signal that triggers the cascade suppression behavior (Section 21.2), where individual rupture alerts are consolidated into a single cascade event and automated actions are suppressed pending human coordination.

API access: GET /api/v2/kpi/contagion/{host}

---

11.7 Resilience (Recovery Capacity Indicator)

Mathematical Definition:

```
Resilience(t) = 1 / (1 + mean({Stress(τ) | τ ∈ [t - W, t]}))

Where:
  W = observation window (configurable, default 30 minutes)
```

Physical interpretation: Resilience is the inverse of sustained stress — a system that experiences stress but recovers quickly is resilient; a system that remains under stress for extended periods has low resilience. Values near 1.0 indicate high recovery capacity; values approaching 0 indicate chronic stress.

Operational significance: Resilience is a slow-moving indicator used for capacity planning and action cooldown calibration. Systems with low Resilience scores should have shorter cooldown windows (they need more frequent intervention) and lower automation thresholds (they are more fragile).

API access: GET /api/v2/kpi/resilience/{host}

---

11.8 Entropy (Metric Variance Disorder)

Mathematical Definition:

```
Entropy(t) = -Σ_{i=1}^{n} p_i(t) × log(p_i(t))

Where:
  p_i(t) = σ²_i(t) / Σ_{j=1}^{n} σ²_j(t)  [normalized variance share of metric i]
  σ²_i(t) = variance of metric i over window [t - W, t]
```

Physical interpretation: Entropy measures the disorder of metric variance across the system. When all metrics exhibit similar variance, Entropy is high (disorder). When one metric dominates the variance, Entropy is low (concentration).

Operational significance: Slowly rising Entropy over days-to-weeks is a signature of the Drift surge profile — resource fragmentation causes increasing variance across metrics. It is a long-horizon indicator used for capacity planning, not real-time alerting.

API access: GET /api/v2/kpi/entropy/{host}

---

11.9 Sentiment (Signal Quality Ratio)

Mathematical Definition:

```
Sentiment(t) = log(N_pos(t) + ε) - log(N_neg(t) + ε)

Where:
  N_pos(t) = count of "positive" signals in window [t - W, t]
  N_neg(t) = count of "negative" signals in window [t - W, t]
  ε        = 1 (smoothing constant)
```

Positive signals include: Successful requests (HTTP 2xx), healthy check passes, throughput within expected range.

Negative signals include: Errors (HTTP 5xx), timeouts, health check failures, log error patterns.

Physical interpretation: Sentiment is the log-ratio of positive to negative system signals, analogous to a signal-to-noise ratio in communications theory. Positive values indicate predominantly healthy signals; negative values indicate predominantly degraded signals.

Note on terminology: This signal was named "Mood" in OHE v5.1. The term has been changed to "Sentiment" in v6 to remove anthropomorphic language. The mathematical definition is unchanged.

API access: GET /api/v2/kpi/sentiment/{host}

---

11.10 HealthScore (Executive Summary Indicator)

Mathematical Definition:

```
HealthScore(t) = 100 × Π_{k ∈ K} min(1, max(0, 1 - w_k × s_k(t)))

Where:
  K = set of composite signals {Stress, Fatigue, Pressure, Contagion}
  s_k(t) = normalized value of signal k at time t
  w_k = configurable weight for signal k
```

Physical interpretation: HealthScore ∈ [0, 100] is a multiplicative composite providing a single-number summary of system health. It is analogous to an ETF (Exchange-Traded Fund) index: it aggregates diverse indicators into one number suitable for executive dashboards and high-level operational overviews.

Interpretation scale:

· 90–100: Healthy. All composite signals within normal range.
· 70–89: Degraded. One or more composite signals elevated but below critical.
· 50–69: At risk. Multiple composite signals significantly elevated.
· 0–49: Critical. Immediate attention required.

API access: GET /api/v2/kpi/healthscore/{host}

---

11.11 Composite Signal Visibility

All composite signals are fully accessible via the v2 API:

```
GET /api/v2/kpi/stress/{host}
GET /api/v2/kpi/fatigue/{host}
GET /api/v2/kpi/pressure/{host}
GET /api/v2/kpi/contagion/{host}
GET /api/v2/kpi/resilience/{host}
GET /api/v2/kpi/entropy/{host}
GET /api/v2/kpi/sentiment/{host}
GET /api/v2/kpi/healthscore/{host}
```

Each endpoint returns the current value, historical window, and contributing factors. Composite signals are also exposed as Prometheus metrics at /metrics:

```
kairo_kpi_stress{host="web-01"} 0.72
kairo_kpi_fatigue{host="web-01"} 0.45
kairo_kpi_healthscore{host="web-01"} 68
```

Design rationale for reduced external prominence: OHE v5.1 positioned these nine KPIs as the primary product surface. Market feedback indicated this created confusion — too many signals, unclear hierarchy, anthropomorphic naming. Kairo Core v6 positions the Rupture Index as the single primary signal, with composite KPIs available as advanced diagnostics for power users. This is a product positioning change, not a removal of capability.

---

12. Action Recommendation Engine

12.1 Philosophy

Prediction without prescribed action has limited operational value.

Kairo Core distinguishes between detection (identifying that a rupture is occurring) and prescription (recommending what to do about it). Every rupture event includes at least one concrete, executable action recommendation. The system provides safety guardrails at every automation tier.

12.2 Action Taxonomy

Tier Automation Level Confidence Required Example Actions Approval Mechanism
Tier 1 Fully automated C > 0.85 HPA maxReplicas adjustment, pod restart, traffic shift Automatic (rate-limited, cooldown-gated, rollback-ready)
Tier 2 Suggested, approval required C > 0.60 Scale deployment, drain node, switch upstream provider One-click via webhook, API, or UI
Tier 3 Human investigation required Any C Unusual pattern, unknown profile, high-impact isolation Manual decision; no automation path

12.3 Rule Engine

v6.0 uses deterministic, configurable action rules. Each rule specifies a condition (based on rupture index, signal class, surge profile, and confidence) and an ordered list of actions with associated parameters.

```yaml
actions:
  rules_path: /etc/kairo/rules.yaml   # External rules file for hot-reload
```

Full rule examples and specification are provided in the configuration reference (Section 17).

12.4 Action Providers

Actions are executed through pluggable providers, each implementing a standard interface:

Provider Actions Supported Requirements
Kubernetes scale, restart, cordon, drain, isolate (network policy) ServiceAccount with scoped RBAC
Webhook notify, trigger_pipeline, custom automation HTTP endpoint
Alertmanager alert, silence Alertmanager API URL
PagerDuty page, incident_create, incident_update API key with incident scope

12.5 Safety Architecture

All Tier-1 automated actions are gated by a multi-layer safety system:

1. Rate limiting: Maximum N Tier-1 actions per target per hour (configurable, default 6)
2. Cooldown windows: Mandatory observation period after each action before next action is permitted
3. Rollback triggers: Automatic rollback if R_new > R_old within observation window
4. Namespace allowlist: Only permitted Kubernetes namespaces are eligible for automated actions
5. Emergency stop: Runtime kill switch disables all Tier-1 automation immediately
6. Shadow mode: Alternative execution mode that logs what would have been done without executing

Full action engine specification including arbitration, deduplication, rollback, and human override design is provided in the configuration reference (Section 17) and operational behavior documentation (Section 21).

---

13. Explainability Architecture

13.1 Design Principle

Every prediction must be auditable to its mathematical roots.

Kairo Core implements transparency at every computational layer. The /explain API provides a full trace from raw input signals through composite KPIs, rupture index computation, ensemble model contributions, and final fused probability. No black-box model is employed anywhere in the prediction chain.

13.2 Explainability Levels

Level Content API Endpoint v6.0 Status
Level 1: Metric Contribution Which signals contributed to the rupture, with normalized weights and individual rupture indices /explain/{id} ✅ Full
Level 2: Temporal Ordering Which signal deviated first, propagation sequence, lead/lag analysis /explain/{id}/trace ✅ Partial (lead/lag available; full temporal decomposition in v6.1)
Level 3: Topological Causality Service dependency chain of failure propagation, bottleneck identification Future ❌ v6.2 target
Level 4: Counterfactual Reasoning "Would the outage have occurred if service X had been scaled 5 minutes earlier?" Future ❌ Research objective

13.3 Explanation Response Schema

Full response schema with all fields documented is provided in the API specification (Section 16).

13.4 Auditable Formulas

Every numeric output from Kairo Core is produced by a published, deterministic formula. The formula library is maintained in METRICS.md in the repository root and is versioned alongside the codebase. The /explain/{id}/formula endpoint returns the exact computation trace for a specific event, including intermediate values at each step.

---

14. System Architecture

14.1 Component Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                         Kairo Core v6                             │
│                                                                   │
│  ┌──────────────────┐  ┌──────────────────┐  ┌───────────────┐   │
│  │   Ingest Layer    │  │ Signal Pipelines │  │ Action Engine │   │
│  │                   │  │                  │  │               │   │
│  │ • Prom remote_wr  │  │ • Metrics Pipe   │  │ • Rule Engine │   │
│  │ • OTLP HTTP       │─▶│ • Log Pipe       │─▶│ • Providers   │   │
│  │ • DogStatsD UDP   │  │ • Trace Pipe     │  │ • Arbitration │   │
│  │ • gRPC Push       │  │ • Fusion Engine  │  │ • Safety Gate │   │
│  └──────────────────┘  └────────┬─────────┘  └───────────────┘   │
│                                 │                                  │
│  ┌──────────────────┐  ┌────────▼─────────┐  ┌───────────────┐   │
│  │  Context Manager  │  │Composite Signals │  │  Self-Monitor │   │
│  │                   │  │                  │  │               │   │
│  │ • Time-of-day     │  │ • Stress         │  │ • /metrics    │   │
│  │ • Day-of-week     │  │ • Fatigue        │  │ • /health     │   │
│  │ • Deployment      │  │ • Pressure       │  │ • Self-alerts │   │
│  │ • Manual context  │  │ • Contagion ...  │  │ • Profiling   │   │
│  └──────────────────┘  └────────┬─────────┘  └───────────────┘   │
│                                 │                                  │
│  ┌──────────────────┐  ┌────────▼─────────┐                      │
│  │   Explain API    │  │  Storage (Opt.)  │                      │
│  │                  │  │  BadgerDB/Mem    │                      │
│  └──────────────────┘  └──────────────────┘                      │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                     API Layer                                │  │
│  │  REST :8080 │ gRPC :9090 │ /metrics scrape │ Timeline :8080 │  │
│  └─────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

14.2 Package Structure

```
cmd/
  kairo-core/              # Main binary entry point

internal/
  ingest/                  # Prom remote_write, OTLP, DogStatsD, gRPC receivers
  pipeline/
    metrics/               # Metric stream processing, CA-ILR, ensemble
    logs/                  # Log parsing, pattern extraction, quantitative streams
    traces/                # Trace topology, bottleneck detection, cascade scoring
  fusion/                  # Bayesian fusion, conflict detection, time alignment
  rupture/                 # Rupture detection, surge profiles, event emission
  composites/              # Stress, Fatigue, Pressure, Contagion, Resilience, Entropy, Sentiment, HealthScore
  context/                 # Time-of-day, day-of-week, deployment awareness, manual context
  actions/
    engine/                # Rule evaluation, tier determination
    providers/             # Kubernetes, webhook, alertmanager, pagerduty
    arbitration/           # Conflict detection, deduplication
    safety/                # Rate limiting, cooldown, rollback, emergency stop
  explain/                 # XAI trace generation, formula audit, pipeline debug
  api/                     # REST/gRPC handlers, middleware, auth
  storage/                 # BadgerDB wrapper, stateless mode
  telemetry/               # Self-monitoring, /metrics, health, profiling

pkg/
  rupture/                 # Public Rupture Index formula, importable
  composites/              # Public composite signal formulas, importable
  client/                  # Go SDK

sdk/
  go/                      # kairo-client-go
  python/                  # kairo-client
```

---

15. Integration Matrix

15.1 Inputs: What Kairo Core Consumes

Source Protocol Signal Types Maturity
Prometheus remote_write (Snappy protobuf) Metrics ✅ Primary
OpenTelemetry OTLP/HTTP Metrics, Logs, Traces ✅ Primary
Datadog Agent DogStatsD UDP Metrics ✅ Compatible
Loki Loki-compatible HTTP API Logs ✅ Compatible
Tempo / Jaeger OTLP trace ingest Traces ✅ Compatible
Custom gRPC protobuf (JSON codec) Any ✅ Available

15.2 Outputs: Where Predictions Land

Output Format Content
Grafana Annotations HTTP annotation API Rupture events as vertical markers with rich Markdown context
Grafana Metrics Prometheus /metrics kairo_predicted_* time series queryable as panel data alongside historical metrics
Alertmanager Standard webhook Alerts with action recommendations embedded in alert body
Kubernetes Events core/v1.Event Cluster-visible rupture events
PagerDuty / OpsGenie Webhook adapter Incident creation with full rupture context
Kairo Timeline Built-in HTML (:8080/timeline) Predicted future events rendered on Gantt-style chart
Custom Webhook JSON POST Full rupture event payload

15.3 Grafana Integration Detail

Kairo Core integrates with Grafana through two complementary channels, neither requiring Grafana modifications:

Channel 1: Predicted Metrics as Prometheus Time Series

Kairo Core exposes predictions at /metrics as standard Prometheus gauges:

```
kairo_rupture_index{host, metric, severity}
kairo_time_to_failure_seconds{host, metric}
kairo_predicted_value{host, metric, horizon}
kairo_rupture_confidence{host}
kairo_fused_rupture_probability{host}
kairo_kpi_stress{host}
kairo_kpi_healthscore{host}
```

In Grafana, an operator creates a panel with two Prometheus queries: the actual metric from the existing Prometheus datasource, and kairo_predicted_value from the Kairo Core /metrics endpoint. Grafana renders both as time series on the same axes. Because predictions carry future timestamps, the predicted line extends visually beyond the current time marker. This works with zero Grafana modifications — it leverages Grafana's native ability to render any Prometheus metric with a timestamp.

Channel 2: Rupture Events as Annotations

Kairo Core pushes rupture events to the Grafana HTTP annotation API:

```json
POST https://grafana.internal/api/annotations
{
  "time": 1713957780000,
  "text": "# ⚠️ Rupture Detected: web-01\n\n**Rupture Index:** 4.2 (Critical)\n**Profile:** Spike\n**Predicted TTF:** 420s\n\n**Recommended Action:** Scale web-tier +2 replicas\n\n[View Full Explanation →](https://kairo.internal/api/v2/explain/rupt_001)",
  "tags": ["rupture", "severity:critical", "host:web-01", "profile:spike", "source:kairo-core"],
  "dashboardId": 5,
  "panelId": 12
}
```

Channel 3: Kairo Native Timeline

For visualization patterns that Grafana's time-series architecture cannot represent — specifically, predicted future event timelines across multiple signals — Kairo Core serves a minimal prediction timeline at :8080/timeline. This is a focused, single-purpose view, not a general-purpose dashboard. It complements, rather than competes with, Grafana.

---

16. API Specification v2

16.1 Endpoint Map

```
# Rupture Analysis
GET    /api/v2/rupture/{host}              # Current rupture state
GET    /api/v2/rupture/{host}/history      # Rupture Index timeline
GET    /api/v2/rupture/{host}/profile      # Surge profile classification
GET    /api/v2/ruptures                    # All active ruptures

# Forecasting
POST   /api/v2/forecast                    # Batch forecast
GET    /api/v2/forecast/{metric}/{host}    # Single metric forecast

# Composite Signals (KPIs)
GET    /api/v2/kpi/{name}/{host}           # Current composite value
GET    /api/v2/kpi/{name}/{host}/history   # Historical composite values
# Valid names: stress, fatigue, pressure, contagion, resilience, entropy, sentiment, healthscore

# Actions
GET    /api/v2/actions                     # List recent actions
GET    /api/v2/actions/{id}                # Action detail
POST   /api/v2/actions/{id}/approve        # Approve pending Tier-2
POST   /api/v2/actions/{id}/reject         # Reject pending action
POST   /api/v2/actions/{id}/rollback       # Rollback executed action
POST   /api/v2/actions/emergency-stop      # Kill switch

# Suppressions
POST   /api/v2/suppressions                # Create suppression
DELETE /api/v2/suppressions/{id}           # Remove suppression
GET    /api/v2/suppressions                # List active suppressions

# Context
POST   /api/v2/context                     # Set manual context
DELETE /api/v2/context/{id}                # Clear context
GET    /api/v2/context                     # List active contexts

# Explainability
GET    /api/v2/explain/{rupture_id}        # Full explanation trace
GET    /api/v2/explain/{rupture_id}/formula # Formula audit
GET    /api/v2/explain/{rupture_id}/pipeline # Per-pipeline debug

# Health & Telemetry
GET    /api/v2/health                      # Liveness + tracker state
GET    /api/v2/ready                       # Readiness probe
GET    /api/v2/metrics                     # Prometheus scrape endpoint

# Ingest
POST   /api/v2/write                       # Prometheus remote_write
POST   /api/v2/v1/metrics                  # OTLP/HTTP metrics
POST   /api/v2/v1/logs                     # OTLP/HTTP logs
POST   /api/v2/v1/traces                   # OTLP/HTTP traces

# Timeline (Native HTML)
GET    /timeline                           # Prediction timeline
```

---

17. Configuration Reference

Complete configuration reference with all options, defaults, and environment variable mappings. See the configuration file kairo.yaml specification for full details.

Key configuration sections:

```yaml
mode: connected | stateless | shadow
ingest: prometheus_remote_write, otlp, dogstatsd, cardinality
pipelines: metrics, log (extractors), trace (analyzers)
predictor: stable_window, burst_window, rupture_threshold, horizons, thresholds
context: time_of_day, day_of_week, deployment, manual
fusion: method, priors, max_lag
composites: stress, fatigue (r_threshold, lambda), pressure, contagion, resilience, entropy, sentiment, healthscore
actions: execution_mode, providers, arbitration, deduplication, safety, rules_path
outputs: grafana, kubernetes_events, webhook
storage: path, ttls, compaction
auth: jwt_secret, api_keys
telemetry: metrics, profiling
```

---

18. Deployment Topologies

Three supported deployment topologies, from simple sidecar to centralized service:

· Sidecar per cluster (recommended): One Kairo Core instance per Kubernetes cluster, ingesting from the local Prometheus
· Centralized service: Single Kairo Core instance receiving remote_write from multiple clusters, with per-cluster logical isolation via X-Cluster-ID header
· Edge / air-gapped: Stateless mode, single binary, 30MB memory, no container runtime required

---

19. Kubernetes Integration

Native Kubernetes integration through:

· Events API: Rupture events emitted as core/v1.Event objects visible to kubectl get events
· Prometheus metrics: Full self-telemetry and prediction metrics exposed at /metrics for Prometheus scraping
· HPA integration (experimental): kairo_predicted_replicas metric consumable by HorizontalPodAutoscaler for predictive scaling

---

20. Adoption Maturity Model

Phase Weeks Automation Level Operator Action
Phase 1: Shadow 1–2 None Review shadow-action.log weekly. Validate predictions.
Phase 2: Alert Only 3–4 Tier 3 (manual) Rupture alerts → SRE pager. SRE manually executes actions.
Phase 3: Tier-2 Auto 5–8 Tier 2 (approval) Actions suggested with one-click approval.
Phase 4: Tier-1 Auto 9+ Tier 1 (auto) High-confidence actions auto-executed with rollback guard.

---

21. Operational Behavior

21.1 Failure Mode Handling

Failure Condition Kairo Core Behavior
Ingest stream stalls Alert after 60s. Continue prediction on last known data. Confidence decays linearly with staleness.
Malformed ingest data HTTP 400 with error detail. Log. Do not crash.
Disk full (connected mode) Stop persistence. Continue in-memory prediction. Emit critical self-alert.
Ingest flood HTTP 429 with Retry-After header. Per-source rate limiting.
All metrics flatline Detect "dead signal" state. Alert: possible collection failure.
Clock skew > 10s Warn in health endpoint. Fusion time-alignment may degrade.
Memory pressure Reduce window sizes (60m→30m→15m). Drop least-recent streams first.
CPU throttled Drop ensemble; continue CA-ILR primary only. Accuracy degraded but detection continues.

21.2 Cascade Handling

During major cascading failures (>10 simultaneous ruptures across >5 hosts):

1. Suppress individual rupture alerts
2. Emit single "Cascade Event" with full topology and timeline
3. Suppress all Tier-1 automated actions (human coordination required)
4. Continue Tier-3 notifications with full context

---

22. Self-Observability

Kairo Core exposes comprehensive self-telemetry at /metrics:

```
kairo_rupture_index{host, metric, severity}
kairo_time_to_failure_seconds{host, metric}
kairo_predicted_value{host, metric, horizon}
kairo_confidence{host}
kairo_fused_rupture_probability{host}
kairo_kpi_stress{host}
kairo_kpi_fatigue{host}
kairo_kpi_healthscore{host}
kairo_actions_total{type, tier, outcome}
kairo_tracker_count{type, state}
kairo_ingest_samples_total{source}
kairo_memory_bytes
kairo_uptime_seconds
kairo_version_info{version}
```

Shipped self-alerting rules for Alertmanager are included in the distribution.

---

23. Performance Benchmarks

Operation Throughput Latency p99 Notes
Metric ingest (HTTP) 10,000 req/s <2 ms Single binary
CA-ILR computation (50 metrics) 3,500 KPI/s <1 ms Per 15s cycle
Ensemble prediction (4 models) — <500 µs 1,550x faster than ARIMA alone
Storage writes (BadgerDB, NVMe) 100,000 ops/s <1 ms Burst-buffer optimized
Memory (idle, 10,000 streams) — — ~30 MB
Memory (full, 50,000 streams) — — ~45 MB
Binary size — — ~25 MB
Cold start to burst-ready — — <5 min
Cold start to full-ready — — <60 min

---

24. Validation Methodology

The Rupture Index is validated through three complementary approaches:

1. Retrospective analysis on public failure datasets (Google Borg traces, AWS post-mortem timelines)
2. Chaos engineering via LitmusChaos and Chaos Mesh with known fault injection
3. Synthetic testing on generated metric streams with ground-truth failure labels

Preliminary results on 50-node Kubernetes failure dataset (public benchmark):

· Precision: 0.82
· Recall: 0.76
· MTTD: 2.1 min (vs 12.4 min for static thresholds)
· False Positives: 8/week (vs 45/week typical for threshold-based alerting)

Full validation report, dataset references, and reproduction instructions: docs/VALIDATION.md

---

25. Open Source Strategy

Open Core (Apache 2.0 — Always Free)

Complete prediction engine, all three pipelines, signal fusion, static action rules, composite signals, shadow mode, single-cluster operation, up to 50,000 metric streams, community support.

Kairo Cloud (Future SaaS)

Multi-cluster dashboard, cross-cluster correlation, learned actions, advanced explainability, long-term storage, team workflows, SLA-backed uptime, managed upgrades.

Kairo Enterprise (Future On-Premises)

Kairo Cloud feature set, self-hosted, SSO, audit logging, advanced RBAC, air-gapped deployment.

---

26. Migration from OHE v5.1

v5.1 Component v6.0 Destination
OHE binary kairo-core
Config YAML Compatible; kairoctl migrate-config for translation
/api/v1/* Deprecated aliases under --compat-ohe-v5
Go SDK ohe-sdk-go Superseded by kairo-client-go (6-month overlap)
Python SDK ohe-sdk Superseded by kairo-client (6-month overlap)
BadgerDB storage Schema-compatible; seamless upgrade
KPI names Preserved at /api/v2/kpi/*
CA-ILR predictor Preserved, config-compatible, extended with ensemble
60-endpoint API Consolidated into v2 API

---

27. Limitations & Honest Acknowledgments

Kairo Core v6.0 makes explicit what it cannot yet do:

· No causal inference. Detects WHAT is rupturing and WHAT CONTRIBUTES. Root cause analysis is a research goal.
· No semantic log understanding. Log pipeline uses pattern matching, not natural language processing.
· Static ensemble weights. Per-signal-class adaptive weighting planned for v6.1.
· Cold start requires 60 minutes for full stable tracker readiness.
· Preliminary validation on limited public datasets. Shadow mode is the recommended path to production.
· No cross-cluster correlation in open-source core.

Full limitations documentation: Section 27 of this whitepaper.

---

28. Appendix A: Composite Signal Reference

Signal Formula Family Domain Primary Use API Path
Stress Weighted linear combination [0, 1] Composite saturation indicator; input to rupture detection /api/v2/kpi/stress/{host}
Fatigue Dissipative accumulator [0, ∞) Sustained stress detection; Ramp profile identification /api/v2/kpi/fatigue/{host}
Pressure Z-score composite (-∞, ∞) Latency-error correlation; Spike profile identification /api/v2/kpi/pressure/{host}
Contagion Graph propagation coefficient [0, 1] Cross-service failure spread; cascade detection /api/v2/kpi/contagion/{host}
Resilience Inverse sustained stress (0, 1] Recovery capacity; cooldown calibration /api/v2/kpi/resilience/{host}
Entropy Shannon entropy [0, log(n)] Metric variance disorder; Drift profile detection /api/v2/kpi/entropy/{host}
Sentiment Log-ratio (-∞, ∞) Signal quality; positive/negative event balance /api/v2/kpi/sentiment/{host}
HealthScore Multiplicative composite [0, 100] Executive summary; high-level health overview /api/v2/kpi/healthscore/{host}

---

29. Appendix B: Formal Definitions & Derivations

B.1 Exponential Least Squares Derivation

The ELS tracker solves the weighted least squares problem at each time step:

```
minimize Σ_{i=1}^{t} λ^{t-i} × (m_i - (α × t_i + β))²
```

The recursive solution produces the update rules given in Section 6.2. The forgetting factor λ controls the effective memory of the estimator: the weight of an observation decays exponentially with age, with characteristic time scale τ = -1 / ln(λ). For λ=0.95, τ ≈ 19.5 samples (~20 minutes at 60s sampling). For λ=0.80, τ ≈ 4.5 samples (~4.5 minutes at 60s sampling).

B.2 Rupture Index Sensitivity Analysis

The Rupture Index R = α_burst / α_stable exhibits the following sensitivity properties:

· Gain: dR/dα_burst = 1/α_stable. Sensitivity to burst acceleration is inversely proportional to the stable trend magnitude. Systems with flat baselines (small α_stable) are highly sensitive to small bursts.
· Noise suppression: dR/dα_stable = -α_burst/α_stable². The negative sign means that if both trackers increase proportionally (a global trend shift), R decreases slightly — providing automatic normalization against baseline shifts.

B.3 Fatigue Dissipation Time Constant

The fatigue recovery half-life is:

```
t_half = ln(2) / λ

For λ = 0.05: t_half ≈ 13.9 sampling intervals
For 15s sampling: t_half ≈ 3.5 minutes
For 60s sampling: t_half ≈ 13.9 minutes
```

---

30. Roadmap

v6.0.0 (Current — April 2026)

· ✅ Three-pipeline architecture (Metrics + Logs + Traces)
· ✅ Signal Fusion Engine (Bayesian)
· ✅ Composite signal framework (8 KPIs)
· ✅ Action Engine with Tier 1/2/3 taxonomy
· ✅ Context awareness (4 layers)
· ✅ Shadow mode
· ✅ Comprehensive self-observability
· ✅ Migration path from OHE v5.1

v6.1.0 (Q3 2026)

· Adaptive ensemble weighting
· Explainability Level 2 (full temporal decomposition)
· NATS/Kafka event streaming
· Grafana Canvas panel integration
· OTel semantic convention auto-classification
· SDK publication (pkg.go.dev, PyPI)

v6.2.0 (Q4 2026)

· Dynamic/learned action recommendations
· Explainability Level 3 (topological causality)
· Kubernetes CRD: RupturePolicy, RuptureAlert
· Trace throughput bottleneck analysis

v7.0.0 (2027 Horizon)

· S3-native storage backend
· Distributed multi-region topology
· Explainability Level 4 (counterfactual reasoning)
· ML-assisted rupture threshold auto-tuning
· Kairo Operator (full Kubernetes controller)

---

Document Metadata

Field Value
Document ID KC-WP-001
Version 6.0.0
Status Canonical Specification — Single Source of Truth
Date April 2026
Author Selim Benfradj, Founding Architect
License Apache 2.0
Repository github.com/benfradjselim/kairo-core
Supersedes OHE-WP-006 (OHE v5.1.0)

---

End of Document

Kairo Core v6.0.0 — Detecting the critical moment before it becomes the critical failure. 
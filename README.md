# Ruptura

<p align="center">
  <img src="assets/logo/ruptura-icon-256.png" alt="Ruptura" width="120" />
</p>

**The Predictive Action Layer for Cloud-Native Infrastructure.**

Ruptura detects workload ruptures before they cause outages — using the Fused Rupture Index™, 10 composite KPI signals with adaptive per-workload baselines, and an action engine that responds automatically with safety gates.

→ **[Technical documentation & quickstart](workdir/README.md)**
→ **[Website & full docs](https://benfradjselim.github.io/ruptura/)**
→ **[API Specification](docs/openapi.yaml)**
→ **[Live dashboard (GitHub Codespace)](https://improved-yodel-v69jjx9w754jcj5x-8080.app.github.dev/ui/)**

---

## Project Status

| Version | Date | Status |
|---------|------|--------|
| v6.8.2 | 2026-05-10 | ✅ Released — OOMKill prevention: BadgerDB memory tuning, GC loop, GOMEMLIMIT |
| v6.8.1 | 2026-05-09 | ✅ Released — fleet heatmap visibility and color fix |
| v6.8.0 | 2026-05-09 | ✅ Released — stable dashboard, correct workload identity |
| v6.7.0 | 2026-05-06 | ✅ Released — embedded web dashboard, air-gap safe, vendor-local assets |
| v6.6.3 | 2026-05-06 | ✅ Released — pre-v7 security & correctness hardening |
| v6.6.0 | 2026-05-05 | ✅ Released — per-workload signal weight tuning |
| v6.5.0 | 2026-05-05 | ✅ Released — edition gate (community / autopilot) |
| v6.4.0 | 2026-05-05 | ✅ Released — rupture fingerprinting + business signal layer |
| v6.3.0 | 2026-05-04 | ✅ Released — calibration warm-up, HealthScore forecast, ruptura-sim |
| v6.2.2 | 2026-04-30 | ✅ Released — anomaly REST endpoints, all v6.x gaps resolved |
| v6.1.0 | 2026-04-27 | ✅ Released — gRPC, eventbus, adaptive ensemble, K8s operator |

**Operator:**

| Version | Date | Status |
|---------|------|--------|
| ruptura-operator v0.6.9 | 2026-05-07 | 🔄 Submitted to Red Hat OperatorHub — certification pipeline running |
| ruptura-operator v0.6.8 | 2026-05-07 | ✅ Merged into OperatorHub community-operators — ServiceAccount fix, RBAC fix |
| ruptura-operator v0.6.7 | 2026-05-07 | ✅ Merged into OperatorHub community-operators |

**Active branch:** `main` · **Module:** `github.com/benfradjselim/ruptura`

---

## How Ruptura Works

Traditional monitoring fires an alert after something breaks. Ruptura works differently: it continuously measures *how fast* each workload is diverging from its own normal behavior, and acts before that divergence becomes an outage.

### 1 — Telemetry ingestion

Ruptura ingests three independent signal sources:

```
Prometheus remote_write  → metric pipeline  (CPU, RAM, latency, error rates...)
OTLP logs (port 4317)   → log pipeline     (error/warn burst detection)
OTLP traces (port 4317) → trace pipeline   (span error rate, P99 latency, service graph)
```

Multiple pods from the same Kubernetes Deployment are automatically merged into a single **WorkloadRef** treatment unit (`namespace/kind/name`). You never look at pod-level noise — only at workload-level health.

### 2 — 10 Composite KPI signals

Every workload gets 10 auditable signals recomputed on every data point:

| Signal | What it measures |
|--------|-----------------|
| `stress` | Instantaneous load: `0.3·CPU + 0.2·RAM + 0.2·latency + 0.2·errors + 0.1·timeouts` |
| `fatigue` | Stress accumulated over time — dissipates during low-stress periods |
| `mood` | System well-being: `log(uptime × throughput + 1) / log(errors × timeouts × restarts + 2)` |
| `pressure` | Rate-of-change of stress + integrated error load (early storm signal) |
| `humidity` | Error × timeout density relative to throughput |
| `contagion` | Error propagation across service dependencies (real trace edges when available) |
| `resilience` | How fast the workload recovers: `mood × (1−fatigue) × (1−contagion)` |
| `entropy` | Behavioral unpredictability: rolling variance of HealthScore |
| `velocity` | Rate of HealthScore change — how fast degradation is progressing |
| `health_score` | Additive-penalty composite (0–100): `100 × (1 − (0.25·stress + 0.20·fatigue + ...))` |

All formulas are versioned release artifacts — no black boxes.

### 3 — Calibration warm-up & adaptive baselines

For the first 24h, Ruptura is in `calibrating` state — signals are recorded but predictions and actions are suppressed until the baseline is ready. Every API response carries `calibration_progress` (0–100) and `calibration_eta_minutes`.

After calibration, every threshold becomes relative to that workload's own Welford baseline:

- A batch job at 90% CPU every night → z-score ≈ 0.1 → no alarm
- An API server normally at 10% suddenly at 40% → z-score = 4.2 → stress alarm fires

A **HealthScore trend forecast** is computed once active: OLS regression over the rolling health history projects `in_15min`, `in_30min`, and `critical_eta_minutes`. "Your score is 54" becomes "you have 28 minutes."

### 4 — Fused Rupture Index™

Three independent signals are fused into a single rupture index per workload:

```
metricR  = |α_burst| / max(|α_stable|, ε)   ← 5-min vs 60-min ILR slope ratio
logR     = burst_rate / log_baseline          ← fires when error/warn > 3σ
traceR   = span_error_rate × P99_deviation    ← from OTLP trace spans

FusedR   = weighted combination (requires ≥ 2 sources to reach "critical")
```

FusedR requires at least two independent sources to agree — a single noisy metric cannot push a workload to critical. This eliminates false positives from transient spikes.

| FusedR | State | Default action |
|--------|-------|---------------|
| < 1.5 | Stable / Elevated | None |
| 1.5 – 3.0 | Warning | Tier-3 — human alert |
| 3.0 – 5.0 | Critical | Tier-2 — suggested action (approve via API) |
| ≥ 5.0 | Emergency | Tier-1 — automated action |

### 5 — Adaptive ensemble (5 models)

The rupture detector uses a 5-model ensemble with online MAE-based weighting:

| Model | Strengths |
|-------|-----------|
| CA-ILR (dual-scale) | O(1) update, detects acceleration, sub-millisecond |
| ARIMA | Strong on stationary series with trends |
| Holt-Winters | Excellent on seasonal / periodic patterns |
| MAD | Robust to outliers |
| EWMA | Reacts quickly to recent shifts |

Every 60 seconds, models are re-weighted based on their actual prediction error over the past hour. No configuration needed — the ensemble adapts automatically to your traffic patterns.

### 6 — Rupture fingerprinting & business signals

At every confirmed rupture (FusedR ≥ 3.0), an 11-dimensional KPI vector is stored as a fingerprint. Future queries run cosine similarity against all past fingerprints — a match ≥ 0.85 surfaces as `pattern_match` with the prior resolution note.

Three business signals are embedded in every snapshot: `slo_burn_velocity` (are you burning your error budget too fast?), `blast_radius` (how many downstream services depend on this workload?), and `recovery_debt` (how many near-misses in the last 7 days?).

### 7 — Action engine with safety gates

When FusedR crosses a threshold, the action engine fires:

```
FusedR ≥ threshold
      ↓
Safety gates (rate limit · cooldown · namespace allowlist · confidence threshold)
      ↓
execution_mode?
  shadow  → log only
  suggest → queue at /api/v2/actions for human approval
  auto    → execute immediately (Tier-1) or queue (Tier-2)
      ↓
K8s (scale · restart · cordon · drain) / Webhook / Alertmanager / PagerDuty
```

### 8 — Narrative explain

Every rupture event gets a structured English explanation:

```
GET /api/v2/explain/{rupture_id}/narrative

→ "payment-api has been accumulating fatigue for 72h (fatigue 0.81).
   A contagion wave from payment-db propagated via the payment-api→payment-db
   call edge and pushed FusedR from 1.8 to 4.2 in 18 minutes.
   This is a cascade rupture, not an isolated spike.
   Recommended action: scale payment-api by 2 replicas."
```

---

## Install in 60 seconds

**Kubernetes (Helm):**

```bash
helm install ruptura helm \
  --namespace ruptura-system \
  --create-namespace \
  --set apiKey=$(openssl rand -hex 32)

kubectl port-forward svc/ruptura 8080:80 -n ruptura-system
curl http://localhost:8080/api/v2/health
```

**Docker:**

```bash
docker run -d \
  --name ruptura \
  -p 8080:8080 -p 4317:4317 \
  -v ruptura-data:/var/lib/ruptura/data \
  -e RUPTURA_API_KEY=$(openssl rand -hex 32) \
  ghcr.io/benfradjselim/ruptura:6.8.2
```

---

## What's Inside

```
workdir/                  Ruptura Go source (v6.8.2)
  cmd/ruptura/            Main binary
  internal/               Engine, pipelines, API, storage, actions, fusion
  internal/ui/static/     Embedded web dashboard (served at :8080, air-gap safe)
  pkg/                    Public Go packages (rupture, composites, client)
  deploy/
    *.yaml                Kustomize manifests
    grafana/              Grafana dashboard JSON + provisioning
  operator/               Kubernetes operator (ruptura-operator v0.6.9)
                          RupturaInstance CRD · Deployment + Service + PVC + SA + Route
                          UBI9-based image — certified for Red Hat OperatorHub

helm/                     Helm chart (v0.6.9, appVersion 6.8.2)
bundle/                   OLM bundle (OperatorHub submission format)
catalog/                  File-Based Catalog for OLM
operators/                community-operators + Red Hat certified-operators submission tree

docs/
  v6.0.0/                 SPECS, AGENTS, ROADMAP, whitepaper, DEV-GUIDE
  v6.1.0/                 v6.1 delta specs
  judgment.md             Design conscience — gaps, milestones, version log
```

---

## Roadmap

```
ruptura (application)
v6.8.2 ✅  OOMKill prevention — BadgerDB memory tuning, periodic GC, GOMEMLIMIT soft cap
v6.8.1 ✅  Fleet heatmap visibility and color fix
v6.8.0 ✅  Stable dashboard · correct workload identity · continuous seed loop
v6.7.0 ✅  Embedded web dashboard — air-gap safe, vendor-local Chart.js + Alpine.js
v6.6.3 ✅  Pre-v7 security & correctness hardening (timing-safe auth, emergency stop, forecast fix)
v6.6.0 ✅  Per-workload signal weight tuning (runtime + env bootstrap)
v6.5.0 ✅  Edition gate — community (read-only) / autopilot (full execution)
v6.4.0 ✅  Rupture fingerprinting · business signal layer (SLO burn, blast radius)
v6.3.0 ✅  Calibration warm-up · HealthScore ETA forecast · ruptura-sim
v6.2.x ✅  Fused Rupture Index · workload-level signals · adaptive baselines
            narrative explain · topology contagion · maintenance windows
v6.1.0 ✅  gRPC ingest · NATS/Kafka eventbus · adaptive ensemble · K8s operator
v7.0.0 ⏳  multi-tenant opt-in (X-Org-ID) · Python SDK v2

ruptura-operator (Kubernetes operator — OperatorHub + Red Hat OperatorHub)
v0.6.9 🔄  Submitted to Red Hat OperatorHub — UBI9 base image, required certification labels
v0.6.8 ✅  Merged into OperatorHub — ServiceAccount fix · RBAC fix · Prometheus metrics
v0.6.7 ✅  First OperatorHub release — merged into community-operators
```

---

## CNCF

Ruptura targets alignment with CNCF sandbox criteria: Apache 2.0 license, open governance ([GOVERNANCE.md](GOVERNANCE.md)), documented security policy ([SECURITY.md](SECURITY.md)), public roadmap. A sandbox application requires demonstrable production adoption — contributions and production feedback are the path there.

---

## License

Apache 2.0

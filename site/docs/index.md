# Ruptura

<p align="center">
  <img src="ruptura-icon.png" alt="Ruptura" width="100" />
</p>

**The Predictive Action Layer for Cloud-Native Infrastructure.**

Ruptura detects workload ruptures before they cause outages — using the Fused Rupture Index™, 10 composite KPI signals with adaptive per-workload baselines, and an action engine that responds automatically with safety gates.

→ **[Getting Started →](getting-started/installation.md)** · **[GitHub](https://github.com/benfradjselim/ruptura)**

---

## Why Ruptura?

| Traditional Observability | Ruptura |
|--------------------------|---------|
| Threshold alerts fire *after* the fact | Fused Rupture Index™ detects divergence **hours early** |
| Global thresholds — batch jobs always "stressed" | **Adaptive per-workload baselines** after 24 h observation |
| "host-123 CPU 78%" — what does it mean? | "payment-api is exhausted — 72 h fatigue accumulation, cascade from payment-db" |
| Manual incident response | Tier-1 actions (scale, restart, rollback) with safety gates |
| 5+ tools: Prom + Grafana + AM + Loki + PD | **One `helm install`**, two pods, no external database |
| Numbers, no reasoning | **Narrative explain** — structured English causal chain |

---

## v7 Architecture

v7 ships as **two separate Kubernetes pods** behind a shared Helm chart:

```
┌────────────────────────────────────────────────────────────┐
│                      ruptura-system                         │
│                                                             │
│  ┌───────────────────────┐    ┌────────────────────────┐   │
│  │    ruptura-engine     │    │      ruptura-ui         │   │
│  │    (Go binary)        │    │  (Svelte 4 + nginx)     │   │
│  │                       │    │                          │   │
│  │  :8080  REST API      │◄───│  nginx proxies /api/    │   │
│  │  :4317  OTLP ingest   │    │  injects Bearer token   │   │
│  │                       │    │  :80   dashboard UI      │   │
│  └───────────────────────┘    └────────────────────────┘   │
│          NodePort 31468               NodePort 31469         │
│          NodePort 31470 (OTLP)                               │
└────────────────────────────────────────────────────────────┘
```

| Port | Purpose |
|------|---------|
| 31468 | Engine REST API (`/api/v2/*`) |
| 31469 | Svelte dashboard (light/dark mode, SSE live events) |
| 31470 | OTLP ingest (`/api/v2/write`, `/otlp/v1/metrics`, `/otlp/v1/logs`, `/otlp/v1/traces`) |

---

## Core Concepts

### Fused Rupture Index™

Ruptura fuses three independent signal sources — raw metrics, OTLP logs, and OTLP trace spans — into a single rupture index per Kubernetes workload:

```
FusedR = weighted_average(metricR, logR, traceR)

  metricR = |α_burst| / max(|α_stable|, ε)   CA-ILR dual-scale slope ratio
  logR    = burst_rate / log_baseline          fires when error/warn > 3σ
  traceR  = span_error_rate × P99_deviation    from OTLP trace spans
```

FusedR requires at least **two** sources — a single noisy signal cannot push a workload to "critical."

| FusedR | State | Default action |
|--------|-------|---------------|
| < 1.5 | Stable / Elevated | None |
| 1.5 – 3.0 | Warning | Tier-3 (human alert) |
| 3.0 – 5.0 | Critical | Tier-2 (suggested action) |
| ≥ 5.0 | Emergency | Tier-1 (automated action) |

### 10 Composite KPI Signals

Every workload gets 10 auditable signals computed every 15 seconds:

| Signal | Measures |
|--------|----------|
| **Stress** | CPU + latency burst |
| **Fatigue** | Cumulative baseline deviation (long-term wear) |
| **Mood** | Log error/warn sentiment ratio |
| **Pressure** | Memory + disk saturation |
| **Humidity** | Forecast variance — how predictable behavior is |
| **Contagion** | Error propagation from upstream services |
| **Resilience** | Recovery speed after spikes |
| **Entropy** | Internal signal disorder |
| **Velocity** | Request rate acceleration |
| **Throughput** | Data volume processed per cycle |

Each signal carries `value`, `state` (ok / warning / critical), and `trend` (rising / falling / stable).

### WorkloadRef — Kubernetes-Native Treatment Unit

Ruptura groups all signals by **Kubernetes workload** (`namespace/kind/name`), not by host. Multiple pods from the same Deployment are merged into a single health view. OTLP resource attributes (`k8s.deployment.name`, `k8s.namespace.name`, etc.) are extracted automatically.

### Adaptive Ensemble — 5 models, no configuration

Ruptura runs five prediction models in parallel and weights them by recent prediction accuracy:

| Model | Strengths |
|-------|-----------|
| **CA-ILR** (dual-scale) | O(1) update · detects acceleration · edge-native |
| **ARIMA** | Stationary series with trends |
| **Holt-Winters** | Seasonal / periodic patterns |
| **MAD** | Robust to outliers |
| **EWMA** | Reacts quickly to recent shifts |

Every 60 seconds, each model's weight is recomputed from its MAE over the past hour. No manual configuration needed.

### HealthScore Forecast

Projects HealthScore +15 and +30 minutes forward. `critical_eta_minutes` appears on the workload card when the projected score is heading toward critical — shows "⚠ Critical in ~12m" in the UI.

### Rupture Fingerprinting

At every confirmed rupture (FusedR ≥ 3.0), an 11-dimensional KPI vector is stored. Future queries run cosine similarity — a match ≥ 0.85 surfaces as `pattern_match` with the prior resolution note. Operators can immediately apply a known fix.

### SSE Live Event Stream

`GET /api/v2/events` — Server-Sent Events. Every rupture and recovery fires in real time. The Fleet dashboard shows a live rupture counter that updates without polling.

### Action Engine — three tiers, multiple safety gates

| Tier | Trigger | Mode |
|------|---------|------|
| Tier-1 | FusedR ≥ 5.0 + confidence ≥ 0.85 | Automatic (K8s scale/restart/cordon) |
| Tier-2 | FusedR ≥ 3.0 + confidence ≥ 0.60 | Suggested — approve via `POST /api/v2/actions/{id}/approve` |
| Tier-3 | FusedR ≥ 1.5 | Alert only (Alertmanager / PagerDuty / webhook) |

Safety gates: per-target rate limit (6/hour), cooldown (300s), namespace allowlist, confidence threshold, emergency stop.

---

## How it works end to end

```
Prometheus remote_write ──┐
OTLP metrics/logs/traces ─┼─► Ingest (port 31470)
                           │
              WorkloadRef grouping
         (namespace / kind / name — pods merged)
                           │
              Adaptive per-workload baselines
           (Welford online stats · active after 24h)
                           │
              10 Composite KPI signals computed every 15s
      stress · fatigue · mood · pressure · humidity
      contagion · resilience · entropy · velocity · throughput
                           │
              5-model adaptive ensemble
         CA-ILR · ARIMA · Holt-Winters · MAD · EWMA
               online MAE-based weights · 60s update
                           │
           Fused Rupture Index™  (FusedR)
       metricR + logR + traceR  ─ requires ≥ 2 sources
                           │
           ┌───────────────┼───────────────┐
           │               │               │
       Tier-3 alert    Tier-2 suggest  Tier-1 auto
     (FusedR ≥ 1.5)  (FusedR ≥ 3.0)  (FusedR ≥ 5.0)
           │               │               │
     AM / PagerDuty    approve via API   K8s / webhook
                           │
              Narrative explain
         GET /api/v2/explain/{id}/narrative
```

---

## Quick Start

=== "Kubernetes (Helm — OCI)"

    ```bash
    helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
      --namespace ruptura-system \
      --create-namespace \
      --set apiKey=$(openssl rand -hex 32) \
      --set ui.enabled=true \
      --set ui.service.type=NodePort \
      --set ui.nodePort=31469 \
      --set service.type=NodePort \
      --set otlpNodePort=31470

    # Dashboard:   http://<node-ip>:31469/
    # Engine API:  http://<node-ip>:31468/api/v2/health
    # OTLP ingest: http://<node-ip>:31470/api/v2/write
    ```

=== "Red Hat OperatorHub (OpenShift)"

    Install from the embedded OperatorHub in the OpenShift web console, or via CLI:

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: ruptura-operator
      namespace: openshift-operators
    spec:
      channel: stable
      name: ruptura-operator
      source: redhat-marketplace
      sourceNamespace: openshift-marketplace
    EOF

    kubectl apply -f - <<EOF
    apiVersion: ruptura.io/v1alpha1
    kind: RupturaInstance
    metadata:
      name: ruptura
      namespace: ruptura-system
    spec:
      edition: community
      storageSize: 10Gi
    EOF
    ```

=== "Inject synthetic workloads"

    ```bash
    python3 scripts/simulate.py
    # Sends 5 workloads every 5s:
    #   gateway        — stable/healthy
    #   order-service  — slow-burn CPU stress (45→90% over 10 min)
    #   payment-api    — error bursts every 2 min (8→43%)
    #   cache-worker   — traffic spikes every 3 min (1200 req/s)
    #   ml-inference   — noisy/calibrating new workload
    ```

=== "Send OTLP metrics directly"

    ```bash
    curl -X POST http://<node-ip>:31470/api/v2/write \
      -H "Content-Type: application/json" \
      -d '{
        "timeseries": [{
          "Labels": [
            {"Name": "__name__",   "Value": "cpu_percent"},
            {"Name": "host",       "Value": "default/Deployment/my-app"},
            {"Name": "namespace",  "Value": "default"},
            {"Name": "deployment", "Value": "my-app"}
          ],
          "Samples": [{"Value": 72.5, "Timestamp": 1234567890000}]
        }]
      }'
    ```

---

## Dashboard (v7)

Svelte 4 SPA with nginx reverse proxy — light/dark mode toggle, full SSE integration.

| View | What you see |
|------|-------------|
| **Fleet** | Workload grid. Per-card: health ring (actual KPI value), 10 signal mini-bars, calibration progress, rupture warning |
| **Fleet → Signals** | 10 KPI cells, PatternMatch warning, BusinessSignals, explain panel |
| **Fleet → History** | Time-series — toggle any of 12 signals; Chart.js |
| **Fleet → Forecast** | HealthScore projection chart |
| **Fleet → Predictions** | Per-metric ensemble predictions |
| **Fleet → Events** | SSE live rupture/recovery log |
| **Fleet → Logs** | Last 200 log lines for the workload |
| **Fleet → Actions** | Approve / reject Tier-2 actions |
| **Fleet → Kubernetes** | Pod list, replicas, resources, labels |
| **Topology** | Service dependency graph from OTLP traces. Click node → health bar + FusedR. Click edge → call rate, error rate, P99 latency |
| **Engine** | Runtime stats, analyzer state, ingest rates, cumulative data flow, BadgerDB storage |
| **Alerts** | Active / resolved alert feed |
| **Nodes** | K8s node health — CPU, memory, disk pressure |
| **Settings** | Data sources, Ingest Stats (live totals), preferences |

---

## Current Release

**v7.0.23** — OTLP datasource activation, FusedR credibility fix, cluster hardening.

| Version | Date | Status |
|---------|------|--------|
| v7.0.23 | 2026-05-22 | ✅ Released — UI endpoint crash fixes, simulator host label, CI stability |
| v7.0.22 | 2026-05-22 | ✅ Released — OTLP datasource TCP-dial test, SSRF bypass for push endpoints, FusedR cap on load |
| v7.0.21 | 2026-05-20 | ✅ Released — workload lifecycle phases (calibrating/active/rupture) in UI |
| v7.0.20 | 2026-05-19 | ✅ Released — CA-ILR cap fix, pre-deploy pod cleanup, 20-minute deploy timeout |
| v7.0.10 | 2026-05-17 | ✅ Released — Database tab (per-type retention, purge controls), Settings overhaul |
| v7.0.5 | 2026-05-16 | ✅ Released — Actions tab Tier-2 approve/reject, emergency stop button |
| v7.0.4 | 2026-05-15 | ✅ Released — OTLP NodePort 31470 exposed, workload simulator |
| v7.0.3 | 2026-05-15 | ✅ Released — JSON crash fix, topology overhaul, health scores per workload |
| v7.0.0 | 2026-05-15 | ✅ Released — v7 architecture: separate UI pod, SSE, k8s metadata, node health |
| v6.8.13 | 2026-05-13 | ✅ Released — log/trace ingest counters, Live Data Flow, ruptura-ctl v1.0.0 |

**Operator:**

| Version | Date | Status |
|---------|------|--------|
| ruptura-operator v0.7.0 | 2026-05-22 | ✅ Released — eviction-loop protection, 3-pod threshold, 3-minute cooldown |
| ruptura-operator v0.6.9 | 2026-05-07 | ✅ Released — submitted to Red Hat OperatorHub |
| ruptura-operator v0.6.8 | 2026-05-07 | ✅ Merged into OperatorHub community-operators |

[Full changelog →](community/roadmap.md) · [Getting Started →](getting-started/installation.md) · [Dashboard Tour →](getting-started/dashboard-tour.md) · [Architecture →](architecture/index.md)

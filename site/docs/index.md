# Ruptura

**The Predictive Action Layer for Cloud-Native Infrastructure.**

Ruptura detects workload ruptures before they cause outages — and acts on them automatically via Kubernetes, webhooks, and alerting integrations. A single Go binary, no external database required.

---

## Why Ruptura?

| Traditional Observability | Ruptura |
|--------------------------|-----------|
| Threshold alerts fire *after* the fact | Fused Rupture Index™ detects divergence **hours early** |
| Global thresholds — batch jobs always "stressed" | **Adaptive per-workload baselines** after 24h observation |
| "host-123 CPU 78%" — what does it mean? | "payment-api is exhausted — 72h fatigue accumulation, cascade from payment-db" |
| Manual incident response | Tier-1 actions (scale, restart, rollback) with safety gates |
| 5+ tools: Prom + Grafana + AM + Loki + PD | **One binary**, one `kubectl apply` |
| Numbers, no reasoning | **Narrative explain** — structured English causal chain |

---

## Core Concepts

### Fused Rupture Index™

```
FusedR = f(metricR, logR, traceR)
```

Three signal sources — metrics, logs, traces — fused into a single rupture index per Kubernetes workload.

| FusedR | State | Ruptura Action |
|--------|-------|-------------|
| < 1.5 | Stable / Elevated | None |
| 1.5 – 3.0 | Warning | Tier-3 (human alert) |
| 3.0 – 5.0 | Critical | Tier-2 (suggested action) |
| ≥ 5.0 | Emergency | Tier-1 (automated action) |

### 10 Composite KPI Signals

`stress` · `fatigue` · `mood` · `pressure` · `humidity` · `contagion` · `resilience` · `entropy` · `velocity` · `health_score`

Each maps raw metrics to a single interpretable 0–1 index with published formulas. `health_score` is a 0–100 additive-penalty composite. No black boxes.

### WorkloadRef — Kubernetes-Native Treatment Unit

Ruptura groups signals by **Kubernetes workload** (`namespace/kind/name`), not by host. Multiple pods from the same Deployment are merged into a single health view. OTLP resource attributes (`k8s.deployment.name`, `k8s.namespace.name`, etc.) are extracted automatically.

---

## Quick Start

=== "Helm (recommended)"

    ```bash
    helm install ruptura workdir/deploy/helm/ruptura \
      --namespace ruptura-system \
      --create-namespace \
      --set auth.apiKey=$(openssl rand -hex 32)

    kubectl port-forward svc/ruptura 8080:80 -n ruptura-system
    curl http://localhost:8080/api/v2/health
    ```

=== "kubectl"

    ```bash
    git clone https://github.com/benfradjselim/ruptura.git
    cd ruptura
    kubectl apply -f workdir/deploy/
    kubectl port-forward svc/ruptura 8080:80 -n ruptura-system
    curl http://localhost:8080/api/v2/health
    ```

=== "Docker"

    ```bash
    docker run -d \
      -p 8080:8080 \
      -p 4317:4317 \
      -v ruptura-data:/var/lib/ruptura/data \
      -e RUPTURA_API_KEY=$(openssl rand -hex 32) \
      ghcr.io/benfradjselim/ruptura:6.2.2

    curl http://localhost:8080/api/v2/health
    ```

---

## Current Release

**v6.2.2** — all v6.x engineering gaps resolved. Stable engine, ready for production evaluation.

- WorkloadRef-native pipeline (namespace/kind/workload, not host)
- Adaptive per-workload baselines (no false alarms from batch jobs)
- Narrative explain at `/api/v2/explain/{id}/narrative`
- Topology-based contagion from real trace service edges
- Maintenance windows via `/api/v2/suppressions`
- Anomaly REST endpoints at `/api/v2/anomalies`
- Fused Rupture Index (metricR + logR + traceR) in every rupture response
- 37 packages pass `go test -race ./...`

[Full changelog →](community/roadmap.md) · [Getting Started →](getting-started/installation.md) · [API Reference →](api/reference.md)

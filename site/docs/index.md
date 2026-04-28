# Kairo Core

**The Predictive Action Layer for Cloud-Native Infrastructure.**

Kairo Core detects infrastructure ruptures before they cause outages — and acts on them automatically via Kubernetes, webhooks, and alerting integrations. A single Go binary, no external database required.

---

## Why Kairo?

| Traditional Observability | Kairo Core |
|--------------------------|-----------|
| Threshold alerts fire *after* the fact | Rupture Index™ detects divergence **hours early** |
| Rules defined per metric | Adaptive ensemble learns your baseline automatically |
| Manual incident response | Tier-1 actions (scale, restart, rollback) with safety gates |
| 5+ tools: Prom + Grafana + AM + Loki + PD | **One binary**, one `kubectl apply` |
| No reasoning about *why* | Full XAI trace for every prediction |

---

## Core Concepts

### Rupture Index™

```
R(t) = |α_burst(t)| / max(|α_stable(t)|, ε)
```

| R Range | State | Kairo Action |
|---------|-------|-------------|
| < 1.5 | Stable / Elevated | None |
| 1.5 – 3.0 | Warning | Tier-3 (human) |
| 3.0 – 5.0 | Critical | Tier-2 (suggested) |
| ≥ 5.0 | Emergency | Tier-1 (automated) |

### 8 Composite Signals

`stress` · `fatigue` · `pressure` · `contagion` · `resilience` · `entropy` · `sentiment` · `healthscore`

Each maps raw metrics to a single interpretable 0–1 index with published formulas.

---

## Quick Start

=== "Kubernetes"

    ```bash
    git clone https://github.com/benfradjselim/kairo-core.git
    cd kairo-core
    docker build -t kairo-core:6.1.0 .
    kubectl apply -f deploy/
    kubectl port-forward svc/kairo-core 8080:8080 -n kairo-system
    curl http://localhost:8080/api/v2/health
    ```

=== "Docker"

    ```bash
    docker run -d \
      -p 8080:8080 \
      -v kairo-data:/var/lib/kairo \
      -e KAIRO_JWT_SECRET=$(openssl rand -hex 32) \
      kairo-core:6.1.0

    curl http://localhost:8080/api/v2/health
    ```

=== "Helm"

    ```bash
    helm install kairo-core ./helm \
      --namespace kairo-system \
      --create-namespace \
      --set auth.jwtSecret=$(openssl rand -hex 32)
    ```

---

## Current Release

**v6.1.0** — gRPC ingest · NATS/Kafka eventbus · Adaptive ensemble weighting · K8s Operator · Go SDK

[Full changelog →](community/roadmap.md) · [Getting Started →](getting-started/installation.md) · [API Reference →](api/reference.md)

 


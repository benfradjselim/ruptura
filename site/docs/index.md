# Ruptura

<p align="center">
  <img src="ruptura-icon.png" alt="Ruptura" width="100" />
</p>

**The Predictive Action Layer for Cloud-Native Infrastructure.**

Ruptura detects workload ruptures before they cause outages — and acts on them automatically via Kubernetes, webhooks, and alerting integrations. A single Go binary, no external database required.

---

## Why Ruptura?

| Traditional Observability | Ruptura |
|--------------------------|---------|
| Threshold alerts fire *after* the fact | Fused Rupture Index™ detects divergence **hours early** |
| Global thresholds — batch jobs always "stressed" | **Adaptive per-workload baselines** after 24 h observation |
| "host-123 CPU 78%" — what does it mean? | "payment-api is exhausted — 72 h fatigue accumulation, cascade from payment-db" |
| Manual incident response | Tier-1 actions (scale, restart, rollback) with safety gates |
| 5+ tools: Prom + Grafana + AM + Loki + PD | **One binary**, one `helm install` |
| Numbers, no reasoning | **Narrative explain** — structured English causal chain |

---

## Core Concepts

### Fused Rupture Index™

Ruptura fuses three independent signal sources — raw metrics, OTLP logs, and OTLP trace spans — into a single rupture index per Kubernetes workload:

```
FusedR = f(metricR, logR, traceR)

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

Every workload gets 10 auditable signals computed from raw telemetry with published formulas:

`stress` · `fatigue` · `mood` · `pressure` · `humidity` · `contagion` · `resilience` · `entropy` · `velocity` · `health_score`

`health_score` (0–100) is an additive-penalty composite. No black boxes — every coefficient is a versioned release artifact.

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

Every 60 seconds, each model's weight is recomputed from its MAE over the past hour. A batch job that runs every night at 02:00 gradually shifts weight toward Holt-Winters — no manual configuration needed.

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
OTLP metrics/logs/traces ─┼─► metric · log · trace pipelines
gRPC ingest ──────────────┘
                           │
              WorkloadRef grouping
         (namespace / kind / name — pods merged)
                           │
              Adaptive per-workload baselines
           (Welford online stats · active after 24h)
                           │
              10 Composite KPI signals computed
      stress · fatigue · mood · pressure · humidity
      contagion · resilience · entropy · velocity · health_score
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

=== "Red Hat OperatorHub (OpenShift)"

    Install on OpenShift 4.12+ from the embedded OperatorHub in the OpenShift web console, or via CLI:

    ```bash
    # Create a Subscription in the openshift-operators namespace
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

    # Create an instance
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

    The operator automatically creates an OpenShift `Route` with edge TLS — no additional ingress config required.

=== "OperatorHub (OLM)"

    Install from [OperatorHub](https://operatorhub.io/operator/ruptura-operator) on any OLM-enabled cluster:

    ```bash
    # Subscribe to the operator
    kubectl apply -f - <<EOF
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: ruptura-operator
      namespace: operators
    spec:
      channel: stable
      name: ruptura-operator
      source: operatorhubio-catalog
      sourceNamespace: olm
    EOF

    # Create an instance
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

=== "Helm (recommended)"

    ```bash
    git clone https://github.com/benfradjselim/ruptura.git
    cd ruptura

    helm install ruptura helm \
      --namespace ruptura-system \
      --create-namespace \
      --set apiKey=$(openssl rand -hex 32)

    kubectl port-forward svc/ruptura 8080:80 -n ruptura-system
    curl http://localhost:8080/api/v2/health
    ```

=== "Docker"

    ```bash
    docker run -d \
      --name ruptura \
      -p 8080:8080 \
      -p 4317:4317 \
      -v ruptura-data:/var/lib/ruptura/data \
      -e RUPTURA_API_KEY=$(openssl rand -hex 32) \
      ghcr.io/benfradjselim/ruptura:6.7.0

    curl http://localhost:8080/api/v2/health
    ```

=== "kubectl (inline)"

    ```bash
    # 1 — Namespace + RBAC
    kubectl apply -f - <<'EOF'
    apiVersion: v1
    kind: Namespace
    metadata:
      name: ruptura-system
    ---
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: ruptura
      namespace: ruptura-system
    ---
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: ruptura
    rules:
      - apiGroups: ["apps"]
        resources: ["deployments","statefulsets","daemonsets","replicasets"]
        verbs: ["get","list","watch"]
      - apiGroups: [""]
        resources: ["pods","nodes","namespaces","services"]
        verbs: ["get","list","watch"]
    ---
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: ruptura
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: ruptura
    subjects:
      - kind: ServiceAccount
        name: ruptura
        namespace: ruptura-system
    EOF

    # 2 — API key secret
    kubectl create secret generic ruptura-secrets \
      -n ruptura-system \
      --from-literal=api-key=$(openssl rand -hex 32)

    # 3 — Storage + Deployment + Services (see Installation for full YAML)
    kubectl port-forward svc/ruptura 8080:80 -n ruptura-system
    curl http://localhost:8080/api/v2/health
    ```

---

## Current Release

**v6.7.0** — embedded web dashboard, air-gap safe. Production-ready for Kubernetes evaluation.

- Self-contained Svelte UI served by the binary — open `http://localhost:8080`, no Grafana required
- Chart.js and Alpine.js vendored locally — no CDN calls, works in air-gapped environments
- Brand logo embedded in the dashboard topbar
- All security hardening from v6.6.3: timing-safe auth, emergency stop wired, Slowloris protection
- All 37 packages pass `go test -race ./...`

**ruptura-operator v0.6.9** — submitted to [Red Hat OperatorHub](https://catalog.redhat.com/software/operators) for OpenShift certification.

- UBI9 micro base image — satisfies Red Hat preflight `BasedOnUBI` certification check
- Required Red Hat container labels (`name`, `vendor`, `version`, `release`, `summary`, `description`) added to both images
- Default app image bumped to `ruptura:v6.7.0`
- Available on [OperatorHub](https://operatorhub.io/operator/ruptura-operator) (community) · Red Hat OperatorHub certification in progress

**ruptura-operator v0.6.8** — live on [OperatorHub](https://operatorhub.io/operator/ruptura-operator).

- Installs via OLM `Subscription` on any Kubernetes or OpenShift cluster
- Manages `RupturaInstance` CRD → Deployment + Service + PVC + ServiceAccount
- ServiceAccount bug fixed, RBAC corrected, Prometheus metrics on `:9090`

[Full changelog →](community/roadmap.md) · [Getting Started →](getting-started/installation.md) · [Dashboard Tour →](getting-started/dashboard-tour.md) · [Operator →](architecture/operator.md)

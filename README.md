# Ruptura — Customizable Predictive Observability for Kubernetes

<p align="center">
  <img src="assets/logo/ruptura-icon-256.png" alt="Ruptura" width="120" />
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.18-00ADD8?logo=go">
  <img alt="License" src="https://img.shields.io/badge/License-Apache%202.0-blue.svg">
  <img alt="Version" src="https://img.shields.io/github/v/release/benfradjselim/ruptura">
  <img alt="CNCF Sandbox" src="https://img.shields.io/badge/CNCF-Sandbox%20Applicant-0086FF">
  <a href="https://codecov.io/gh/benfradjselim/ruptura"><img alt="Coverage" src="https://codecov.io/gh/benfradjselim/ruptura/branch/main/graph/badge.svg"></a>
</p>

<p align="center">
  <b>Declare what you want to protect. Ruptura tells you what will fail, when, and why, and shows the evidence.</b>
</p>

Ruptura is an open source, cloud native predictive observability tool for Kubernetes and OpenShift. It sits on top of the stack you already run, reads your telemetry, and adds the conclusion that metrics and dashboards leave to you.

> **Project status.** Ruptura is being rebuilt around the vision below. Each feature is marked as available, in development or planned, and nothing is claimed without a measurement. See [Status](#status).

---

## The vision

The model is the one you know from an OpenTelemetry Collector or a ClusterLogForwarder: you install an operator, describe what you want in one custom resource, and the operator does the rest. The engine and its algorithms stay the same for everyone. What changes is what you ask it to watch.

```yaml
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: shop
  namespace: ruptura
spec:
  deployment:
    mode: Deployment
  level:
    scope: namespace          # pod, workload, namespace, service, node, cluster...
    selector:
      matchLabels: {team: shop}
  input:
    profile: standard         # minimal, standard, full, deep
    sources:
      - prometheus: {url: https://thanos-querier.openshift-monitoring.svc:9091}
      - kubernetesEvents: {enabled: true}
  analysis:
    sensitivity: balanced     # conservative, balanced, aggressive
    horizon: 30m
  alerting:
    channels:
      - name: ops
        alertmanager: {url: http://alertmanager:9093}
  logging:
    level: info
```

*Target design, see [Status](#status). `RupturaInstance` currently deploys the engine only.*

The operator validates the resource. If it is invalid, nothing is deployed and `status` explains why. If it is valid, the operator generates standard Kubernetes objects and keeps them in the declared state.

### One meaning, every level

A signal keeps the same meaning at every level. Only what it observes changes. Contagion, for example, is a problem spreading between neighbours: between containers in a pod, between replicas of a workload, between pods on a node.

### Three honest outcomes

Ruptura does not claim to explain every failure. Each result is one of three things:

| Case | What you get |
|------|--------------|
| Known failure | Failure name, estimated time, evidence, and whether it was predicted or only detected |
| Abnormal behaviour | Signal state and the metrics that moved, with no invented cause |
| Out of scope | `unavailable`: Ruptura says it cannot see, and missing data never counts as a zero |

Each version closes a list of failures. A version ends when those failures are proven, not when everything is covered.

### Proof, not promises

Every version must publish a reproducible benchmark against standard alternatives: the alerts shipped with OpenShift and tuned `predict_linear` rules. Losses are published too.

First result: on a steady, linear memory leak, a tuned `predict_linear` rule beat Ruptura, with 26 to 28 minutes of warning against 20 to 21. If that is your only risk, use the rule. Ruptura must earn its place on failures that a single rule cannot handle, and this is still being measured.

---

## Status

| Capability | State |
|------------|-------|
| Memory saturation, predicted against the limit | In development, measured on one lab |
| Crash loop, CPU throttling | In development, detected |
| Alert noise reduction | In development, 2.3 alerts per minute on the lab, at least 92 percent lower |
| Workload discovery, Helm chart, operator that deploys the engine | Available |
| Autopilot actions | Commercial edition |
| Explicit missing data, structured failure results | In development |
| Metric catalog by role, input profiles, Kubernetes Events | In development |
| Startup stuck, replicas unavailable | In development, not yet proven |
| `RupturaInstance` with level, input, analysis, alerting | Planned for the first version |
| Public benchmark | In progress |
| Node, namespace and cluster levels | Roadmap |
| Logs through OTLP, traces, eBPF deep profile | Roadmap |
| OpenShift Console plugin | Roadmap |

"In development" means on the [`fix/engine-prediction`](https://github.com/benfradjselim/ruptura/tree/fix/engine-prediction) branch, not yet in a release. Measurements come from a single OpenShift lab with demonstration faults. Results from real clusters are not available yet.

---

## Quick Start

One command, no Helm required — a single manifest with namespace, RBAC,
Deployment, Services, and a Job that auto-generates your API key on first
apply:

```bash
kubectl apply -f https://raw.githubusercontent.com/benfradjselim/ruptura/main/install/ruptura.yaml
kubectl -n ruptura-system get pods
kubectl -n ruptura-system logs job/ruptura-init-apikey   # prints the generated API key
```

Prefer Helm (production installs, upgrades, values overrides)? Same result,
more control:

```bash
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system \
  --create-namespace \
  --set apiKey=$(openssl rand -hex 32) \
  --set ui.enabled=true
```

```bash
kubectl -n ruptura-system port-forward svc/ruptura 8080:80 &      # engine API
kubectl -n ruptura-system port-forward svc/ruptura-ui 8081:80 &   # dashboard
```

| Endpoint | URL |
|----------|-----|
| Dashboard | `http://localhost:8081/` |
| Engine API | `http://localhost:8080/api/v2/health` |
| OTLP ingest | `<node-ip>:31470` (fixed NodePort) |

Send synthetic workloads to try it:

```bash
python3 scripts/simulate.py --host <node-ip> --port 31470
```

### No Kubernetes? Try it locally

```bash
docker compose up --build
```

Starts the engine in demo mode (7 days of synthetic baseline data across 12
workloads, no calibration wait) plus the dashboard — nothing else required.
Dashboard: `http://localhost:8081/` (log in with any username, password
`ruptura-local-dev`). Engine API: `http://localhost:8080/api/v2/health`.

For Go development without Docker at all:

```bash
cd workdir && go run ./cmd/ruptura --demo
```

---

## How it works

```
Sources (Prometheus, OTLP, Kubernetes API)
  -> Metric catalog: meaning, unit, level
  -> Normalizer: per target data, explicit missing values
  -> Forecast and anomaly models
  -> Signals
  -> Failure recipes
  -> Result, explanation, alerts
```

Forecasting combines ILR, Holt-Winters and ARIMA, weighted by recent error, with CA-ILR for rupture detection. Anomaly detection uses z-score, MAD and seasonal methods. The signals are being reviewed, and a signal stays only if it proves its value on real failures. See [docs/engine/DESIGN.md](docs/engine/DESIGN.md).

---

## Lab Setup (Civo / k3s)

A ready-to-deploy lab environment with Prometheus, OTel Collector, and 6 synthetic test apps:

```bash
export KUBECONFIG=~/your-kubeconfig
bash lab-setup/setup.sh
```

Deploys: Prometheus + kube-state-metrics + OTel Collector + Ruptura + 6 apps covering all failure scenarios.

---

## CLI — ruptura-ctl

```bash
ruptura-ctl status              # fleet overview
ruptura-ctl status -w 5         # watch mode, refresh every 5s
ruptura-ctl get ruptures        # active rupture events (server-side filtered)
ruptura-ctl describe workload <ns/kind/name>
ruptura-ctl actions             # pending Tier-2 actions
ruptura-ctl context add "production/*" --type maintenance --duration 2h
ruptura-ctl suppress create "staging/*" 30m
ruptura-ctl emergency-stop      # halt all Tier-1 actions (requires confirmation)
```

---

## Security

| Property | Status |
|----------|--------|
| Auth fail-closed | `RUPTURA_API_KEY` required — no silent open access |
| `/api/v2/metrics` | Public (Prometheus scraping without token) |
| `/api/v2/health` `/ready` | Public (k8s probes) |
| All other routes | Bearer token required |
| Constant-time comparison | `crypto/subtle` on all auth checks |
| Atomic storage | Crash-safe compaction — no double-averaging on restart |

---

## Documentation

| Resource | Link |
|----------|------|
| Engine design and vision | [docs/engine/DESIGN.md](docs/engine/DESIGN.md) |
| Technical docs & API reference | [docs/REFERENCE.md](docs/REFERENCE.md) |
| Engine & concepts | [workdir/README.md](workdir/README.md) |
| CLI reference | [site/docs/cli/rupturactl.md](site/docs/cli/rupturactl.md) |
| Website | <https://benfradjselim.github.io/ruptura/> |
| CNCF Sandbox proposal | [docs/cncf-sandbox-proposal.md](docs/cncf-sandbox-proposal.md) |

---

## Community

- **Discussions** — [github.com/benfradjselim/ruptura/discussions](https://github.com/benfradjselim/ruptura/discussions)
- **Issues** — [github.com/benfradjselim/ruptura/issues](https://github.com/benfradjselim/ruptura/issues)
- **CNCF Sandbox** — proposal in preparation

---

## License

Copyright 2024–2026 Selim Benfradj and the Ruptura Authors.  
Licensed under the [Apache License, Version 2.0](LICENSE).

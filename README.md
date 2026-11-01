<p align="center">
  <img src="assets/logo/ruptura-icon-256.png" alt="Ruptura" width="140" />
</p>

<h1 align="center">Ruptura</h1>

<p align="center">
  <b>Predictive observability for Kubernetes and OpenShift.</b><br/>
  Know what will fail, when, and why, before your users do.
</p>

<p align="center">
  <a href="https://github.com/benfradjselim/ruptura/releases"><img alt="Release" src="https://img.shields.io/github/v/release/benfradjselim/ruptura"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/License-Apache%202.0-blue.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.18-00ADD8?logo=go">
  <a href="https://codecov.io/gh/benfradjselim/ruptura"><img alt="Coverage" src="https://codecov.io/gh/benfradjselim/ruptura/branch/main/graph/badge.svg"></a>
  <a href="https://github.com/benfradjselim/ruptura/discussions"><img alt="Discussions" src="https://img.shields.io/github/discussions/benfradjselim/ruptura"></a>
</p>

<p align="center">
  <a href="#getting-started">Getting started</a> ·
  <a href="https://benfradjselim.github.io/ruptura/">Documentation</a> ·
  <a href="#roadmap">Roadmap</a> ·
  <a href="CONTRIBUTING.md">Contributing</a>
</p>

---

Ruptura runs in your cluster, on top of the monitoring stack you already have. It turns your telemetry into clear conclusions: the failure, the time left, the evidence, and the next step.

## Why Ruptura

- **Conclusions, not charts.** One structured result per failure, ready for your alerts and your tools.
- **Your scope, your rules.** Choose the level to watch, how deep to collect, and where results go.
- **Learns each workload.** Compares every workload with its own history, not with a global threshold.
- **Honest by design.** Separates predicted from detected failures, and reports the data it cannot see.
- **Kubernetes native.** One custom resource, one operator, no external database.
- **Open source.** Apache 2.0, runs entirely in your cluster.

## One resource to describe it all

```yaml
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: shop
spec:
  level:
    scope: namespace
    selector:
      matchLabels: {team: shop}
  input:
    profile: standard
  analysis:
    sensitivity: balanced
    horizon: 30m
  alerting:
    channels:
      - name: ops
        alertmanager: {url: http://alertmanager:9093}
```

Ruptura validates it, deploys what it needs, and reports status on the resource.

```yaml
level: workload
target: shop/orders
failureMode: memory_saturation
kind: predicted
state: critical
etaSeconds: 720
recommendedAction: Review the memory limit
```

> This configuration is under active development. See the [roadmap](#roadmap).

## Getting started

```bash
kubectl apply -f https://raw.githubusercontent.com/benfradjselim/ruptura/main/install/ruptura.yaml
kubectl -n ruptura-system get pods
```

Or with Helm:

```bash
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system --create-namespace \
  --set apiKey=$(openssl rand -hex 32) \
  --set ui.enabled=true
```

Try it locally, no cluster needed:

```bash
docker compose up --build
```

Then open <http://localhost:8081> and sign in with password `ruptura-local-dev`.

## How it works

```
 Prometheus · OTLP · Kubernetes API
               │
         Metric catalog
               │
     Forecasts and anomalies
               │
       Signals and failures
               │
  Alerts · Dashboards · Your tools
```

## Roadmap

| Status | Scope |
|---|---|
| **Now** | Pod and workload failures, memory saturation, crash loops, CPU throttling, Kubernetes Events, `RupturaInstance` configuration |
| **Next** | Node level, noisy neighbours, failure propagation, logs |
| **Later** | Namespace and cluster levels, traces, eBPF, OpenShift Console plugin |

Ruptura is in active development and not yet recommended for production. Early adopters are welcome.

## Community

- [Discussions](https://github.com/benfradjselim/ruptura/discussions) for questions and ideas
- [Issues](https://github.com/benfradjselim/ruptura/issues) for bugs and requests
- [Contributing guide](CONTRIBUTING.md) · [Code of conduct](CODE_OF_CONDUCT.md) · [Security policy](SECURITY.md) · [Governance](GOVERNANCE.md)

## License

Licensed under the [Apache License 2.0](LICENSE).

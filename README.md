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

- **Conclusions first.** One structured result per failure, ready for your alerts and your tools, with a forecast chart and the time range when you want to see why.
- **Knows what it takes down.** A call graph read from what your cluster already declares names the callers each failure can hit.
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
  namespace: ruptura-demo
spec:
  level:
    scope: workload
    selector:
      matchLabels: {team: shop}
  alerting:
    channels:
      - name: cluster
        alertmanager:
          url: https://alertmanager-main.openshift-monitoring.svc:9094
          serviceAccountToken: true
```

The operator validates it, deploys the engine, and reports status on the resource. On OpenShift it also grants the engine access to cluster metrics and to the cluster Alertmanager, so failures follow the same routes, silences and receivers as platform alerts.

Each failure is one structured result, from `GET /api/v2/failures`:

```json
{
  "level": "workload",
  "target": "demo-shop/Deployment/shop-db",
  "failureMode": "saturation",
  "kind": "predicted",
  "state": "degraded",
  "etaSeconds": 626,
  "etaEarlySeconds": 540,
  "etaLateSeconds": 790,
  "recommendedAction": "Review the memory limit or restart the pod before it is killed",
  "evidence": ["Memory at 75% of its limit", "Can hit its callers: shop-api, shop-load"],
  "affects": ["demo-shop/Deployment/shop-api", "demo-shop/Deployment/shop-load"]
}
```

Failures covered today: memory saturation, a replica that diverges from its siblings, a release that leaks, OOM kill, crash loop, pod stuck at startup, unavailable replicas, stuck rollout, CPU throttling, and failures recurring on a regular rhythm. The [failure catalog](failures/) lists 66 Kubernetes and OpenShift failures and where Ruptura stands on each.

## Getting started

### OpenShift and Kubernetes, with the operator

```bash
oc apply -f https://raw.githubusercontent.com/benfradjselim/ruptura/main/install/operator.yaml
oc -n ruptura-system rollout status deploy/ruptura-operator
```

On Kubernetes, also set `spec.input.prometheus.url` to your Prometheus query endpoint. Then create the `RupturaInstance` and watch it reach `Running`:

```bash
oc get rupturainstance -A
```

### Engine only, with Helm

```bash
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system --create-namespace \
  --set apiKey=$(openssl rand -hex 32) \
  --set ui.enabled=true \
  --set thanos.url=http://prometheus.monitoring.svc:9090
```

Every failure mode except `crash_loop` reads Prometheus, so set `thanos.url` to your Prometheus or Thanos query endpoint.

### Locally, no cluster needed

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
| **Now** | Pod and workload failures, Kubernetes Events, `RupturaInstance` configuration, Alertmanager, including the OpenShift cluster Alertmanager |
| **Next** | Node level, pods without a memory limit, failure propagation along the call graph, logs |
| **Later** | Namespace and cluster levels, traces, eBPF, OpenShift Console plugin |

Ruptura is in active development and not yet recommended for production. Early adopters are welcome.

## Community

- [Discussions](https://github.com/benfradjselim/ruptura/discussions) for questions and ideas
- [Issues](https://github.com/benfradjselim/ruptura/issues) for bugs and requests
- [Contributing guide](CONTRIBUTING.md) · [Code of conduct](CODE_OF_CONDUCT.md) · [Security policy](SECURITY.md) · [Governance](GOVERNANCE.md)

## License

Licensed under the [Apache License 2.0](LICENSE).

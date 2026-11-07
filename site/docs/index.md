# Ruptura

Ruptura runs in your Kubernetes or OpenShift cluster, on top of the monitoring stack you already have. It turns telemetry into named failures: what fails, when, why, and what to do next.

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

- **Named failures, not scores.** Each result names a [failure mode](failures.md), its evidence and an action.
- **Predicted or detected, always said.** A forecast is never presented as a fact: it comes with its time range, and missing data is reported as missing.
- **What it takes down.** A call graph read from what the cluster declares names the callers each failure can hit, in the result and in the alert.
- **One resource.** A [`RupturaInstance`](rupturainstance.md) declares what to watch and where results go. The operator does the rest.
- **Your alert pipeline.** Results go to [Alertmanager](alerting.md), including the OpenShift cluster Alertmanager.

Start with the [installation](install.md). The [failure catalog](https://github.com/benfradjselim/ruptura/tree/main/failures) lists 66 Kubernetes and OpenShift failures and where Ruptura stands on each.

Ruptura is in active development and not yet recommended for production.

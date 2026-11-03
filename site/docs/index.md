# Ruptura

Ruptura runs in your Kubernetes or OpenShift cluster, on top of the monitoring stack you already have. It turns telemetry into named failures: what fails, when, why, and what to do next.

```json
{
  "level": "workload",
  "target": "demo-shop/Deployment/step-leak",
  "failureMode": "saturation",
  "kind": "predicted",
  "state": "degraded",
  "etaSeconds": 626,
  "recommendedAction": "Review the memory limit or restart the pod before it is killed",
  "evidence": ["Memory at 75% of its limit"]
}
```

- **Named failures, not scores.** Each result names a [failure mode](failures.md), its evidence and an action.
- **Predicted or detected, always said.** A forecast is never presented as a fact, and missing data is reported as missing.
- **One resource.** A [`RupturaInstance`](rupturainstance.md) declares what to watch and where results go. The operator does the rest.
- **Your alert pipeline.** Results go to [Alertmanager](alerting.md), including the OpenShift cluster Alertmanager.

Start with the [installation](install.md).

Ruptura is in active development and not yet recommended for production.

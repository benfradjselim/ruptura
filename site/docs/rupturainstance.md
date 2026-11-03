# RupturaInstance

A `RupturaInstance` declares what one engine watches and where its results go. Every field is optional.

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
  input:
    profile: standard
  analysis:
    sensitivity: balanced
    horizon: 30m
  alerting:
    channels:
      - name: cluster
        alertmanager:
          url: https://alertmanager-main.openshift-monitoring.svc:9094
          serviceAccountToken: true
  logging:
    level: info
```

| Field | Values | Default | Effect |
|---|---|---|---|
| `level.scope` | `pod`, `workload`, `namespace` | `workload` | How results are reported. Other values in the schema are planned and refused for now. |
| `level.selector.matchLabels` | labels | none | Only workloads with these labels are reported and alerted. |
| `input.profile` | `minimal`, `standard` | `standard` | How much telemetry is collected. `full` and `deep` are planned. |
| `analysis.sensitivity` | `conservative`, `balanced`, `aggressive` | `balanced` | How early forecasts warn. |
| `analysis.horizon` | duration | `30m` | How far ahead forecasts look. |
| `alerting.channels` | list | none | Where results are sent, see [Alerting](alerting.md). |
| `logging.level` | `debug`, `info`, `warn`, `error` | `info` | Engine log level. |
| `image` | image reference | engine version of the operator | Engine image. |
| `storageSize` | quantity | `10Gi` | Size of the engine volume. |
| `apiKeyRef` | Secret name | generated | Secret holding the API key under `api-key`. |
| `resources` | requests and limits | 100m and 128Mi, 1 CPU and 512Mi | Engine resources. |

## Status

| Field | Meaning |
|---|---|
| `phase` | `Running`, or `Failed` with the reason in `message` |
| `readyReplicas`, `availableReplicas` | Engine pods ready |
| `observedGeneration`, `lastReconcileTime` | Last spec handled |

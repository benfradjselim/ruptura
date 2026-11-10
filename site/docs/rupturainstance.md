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
    prometheus:
      url: http://prometheus.monitoring.svc:9090
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
| `level.namespaceSelector.matchLabels` | labels | none | Only namespaces with these labels are reported and alerted, for example `ruptura.io/enabled: "true"` with `oc label namespace shop ruptura.io/enabled=true`. Empty watches every namespace. |
| `input.profile` | `minimal`, `standard` | `standard` | How much telemetry is collected. `full` and `deep` are planned. |
| `input.prometheus.url` | `http` or `https` URL | OpenShift: the cluster Thanos querier. Kubernetes: none | Prometheus compatible query endpoint. Without it only `crash_loop` is reported. |
| `analysis.sensitivity` | `conservative`, `balanced`, `aggressive` | `balanced` | How early forecasts warn. |
| `analysis.horizon` | duration | `30m` | How far ahead forecasts look. |
| `alerting.channels` | list | none | Where results are sent, see [Alerting](alerting.md). |
| `logging.level` | `debug`, `info`, `warn`, `error` | `info` | Engine log level. |
| `image` | image reference | engine version of the operator | Engine image. |
| `storageSize` | quantity | `10Gi` | Size of the engine volume. |
| `storageClassName` | StorageClass name | cluster default | StorageClass of the engine volume claim. |
| `ephemeralStorage` | `true`, `false` | `false` | Keep engine data in an emptyDir instead of a volume claim, for clusters without working storage. History is lost when the pod restarts. |
| `console.enabled` | `true`, `false` | `true` | Deploy the web console as `<name>-console`, with its Service and, on OpenShift, its Route. Sign in with the API key. |
| `console.image` | image reference | console of the operator release | Console image. |
| `apiKeyRef` | Secret name | generated | Secret holding the API key under `api-key`. |
| `resources` | requests and limits | 100m and 128Mi, 1 CPU and 512Mi | Engine resources. |

## Prometheus with TLS and authentication

```yaml
spec:
  input:
    prometheus:
      url: https://prometheus.example.com:9090
      tls:
        ca: {configMapKeyRef: {name: prom-ca, key: ca.crt}}
        cert: {secretKeyRef: {name: prom-client, key: tls.crt}}
        key: {secretKeyRef: {name: prom-client, key: tls.key}}
        serverName: prometheus.example.com
      authorization:
        basicAuth:
          username: {secretKeyRef: {name: prom-basic, key: user}}
          password: {secretKeyRef: {name: prom-basic, key: pass}}
```

| Field | Effect |
|---|---|
| `tls.ca` | Extra CA trusted on top of the system and cluster CAs. |
| `tls.cert`, `tls.key` | Client certificate for mutual TLS. Both or neither. The key must come from a Secret. |
| `tls.serverName` | Name checked in the server certificate. |
| `authorization` | Exactly one of `bearerToken` (Secret), `basicAuth`, or `serviceAccountToken: true`. Without it no credential is sent. |

On OpenShift, without `input.prometheus`, the engine reads the cluster Thanos querier with its service account.

## OTLP receiver with TLS

```yaml
spec:
  input:
    otlp:
      tls:
        cert: {secretKeyRef: {name: otlp-server, key: tls.crt}}
        key: {secretKeyRef: {name: otlp-server, key: tls.key}}
        clientCA: {configMapKeyRef: {name: otel-ca, key: ca.crt}}
```

The engine receives data the standard way, so an OpenTelemetry Collector or a Prometheus needs no special setting:

| Protocol | Port | Collector or Prometheus setting |
|---|---|---|
| OTLP gRPC | 4317 | `otlp` exporter, `endpoint: <instance>.<namespace>.svc:4317` |
| OTLP HTTP, protobuf or JSON, gzip | 4318 | `otlphttp` exporter, `endpoint: https://<instance>.<namespace>.svc:4318` |
| Prometheus remote write 1.0 | 4318 | `remote_write: url: https://<instance>.<namespace>.svc:4318/api/v1/write` |

TLS applies to both ports. With `clientCA`, clients must present a certificate signed by it. Over the ingest rate (`RUPTURA_INGEST_RPS`), gRPC answers `RESOURCE_EXHAUSTED` and HTTP answers 429 with `Retry-After`, which exporters retry.

## Secrets and ConfigMaps

Each value comes from exactly one `secretKeyRef` or `configMapKeyRef` in the namespace of the instance. Private keys, tokens and passwords must come from a Secret.

- A malformed block is refused when you apply it.
- The operator then reads every reference and checks certificates and key pairs. A new instance with a missing or invalid reference deploys nothing and reports `Failed`. A running instance keeps its engine and reports `Degraded` until the reference is fixed.
- When a referenced Secret or ConfigMap changes, the engine restarts with the new values.

## Status

| Field | Meaning |
|---|---|
| `phase` | `Running`, `Failed` (nothing deployed) or `Degraded` (change not applied, engine kept), with the reason in `message` |
| `readyReplicas`, `availableReplicas` | Engine pods ready |
| `observedGeneration`, `lastReconcileTime` | Last spec handled |

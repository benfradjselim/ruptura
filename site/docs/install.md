# Installation

## With the operator

One manifest installs the `RupturaInstance` resource definition, the operator and its permissions in `ruptura-system`.

```bash
oc apply -f https://raw.githubusercontent.com/benfradjselim/ruptura/main/install/operator.yaml
oc -n ruptura-system rollout status deploy/ruptura-operator
```

Create an instance in any namespace:

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
```

```bash
oc get rupturainstance -n ruptura-demo
```

The instance reaches `Running` in about a minute. The operator creates, in the namespace of the instance: the engine Deployment, a Service, a PersistentVolumeClaim, an API key Secret and a ServiceAccount. On OpenShift it also creates a Route, a ConfigMap filled with the service CA, and bindings to `cluster-monitoring-view` so the engine reads cluster metrics through Thanos.

An invalid instance deploys nothing: its status reports phase `Failed` with every problem found.

Deleting the instance removes everything the operator created, cluster bindings included.

While the engine is not ready, `status.message` gives the cause Kubernetes reports: a failing container with its last log line, a pod the scheduler cannot place, an unbound volume claim, or a pod refused at creation. `oc get rupturainstance` shows it in the Message column.

One operator manages an instance at a time. It holds a Lease named `<instance>-operator` in the namespace of the instance, and `status.operator` names its namespace. A second operator on the same cluster leaves the instance alone, and takes it over only after the holder stops renewing for two minutes. Operators older than 1.4.0 do not use the Lease: remove them before installing 1.4.0.

### Upgrading to 1.4.2

- The engine pod meets the Pod Security `restricted` profile: seccomp `RuntimeDefault`, all capabilities dropped, read-only root filesystem. It restarts once after the upgrade.
- `status.conditions` reports `Available`, `Progressing` and `Degraded`, read by the OpenShift console, Argo CD and kstatus.
- The operator no longer lists or watches Secrets across the cluster.
- The web console image listens on 8080 with any user ID and proxies `/api/` to `RUPTURA_BACKEND_URL`.

### Upgrading to 1.4.0

- OTLP over HTTP moved from port 4317, paths `/otlp/v1/*`, to the standard port 4318, paths `/v1/metrics`, `/v1/logs`, `/v1/traces`. Port 4317 now serves OTLP gRPC. Point `otlphttp` exporters at `:4318`, and `otlp` exporters at `:4317`.
- Prometheus remote write is received at `:4318/api/v1/write`.
- The engine pod restarts once after the upgrade.

On Kubernetes the operator installs the same way. Every failure mode except `crash_loop` reads Prometheus, so declare its query endpoint:

```yaml
spec:
  input:
    prometheus:
      url: http://prometheus.monitoring.svc:9090
```

## Kubernetes, with Helm

```bash
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system --create-namespace \
  --set apiKey=$(openssl rand -hex 32) \
  --set ui.enabled=true \
  --set thanos.url=http://prometheus.monitoring.svc:9090
```

`thanos.url` is your Prometheus or Thanos query endpoint. With `--set openshift.enabled=true` the chart uses the OpenShift Thanos querier instead.

## Locally

```bash
git clone https://github.com/benfradjselim/ruptura && cd ruptura
docker compose up --build
```

Open <http://localhost:8081> and sign in with password `ruptura-local-dev`.

## Images

| Component | Image |
|---|---|
| Engine | `docker.io/selimbf/ruptura` |
| Operator | `ghcr.io/benfradjselim/ruptura-operator` |
| UI | `ghcr.io/benfradjselim/ruptura-ui` |
| Helm chart | `oci://ghcr.io/benfradjselim/charts/ruptura` |

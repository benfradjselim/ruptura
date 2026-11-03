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

# Installation

## ruptura-ctl (CLI)

`ruptura-ctl` is the command-line interface for Ruptura. It connects to the Ruptura API from **outside the pod** — from your workstation, a CI pipeline, or a Kubernetes Job.

```bash
# Quick install (Linux amd64)
curl -Lo ruptura-ctl \
  https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-linux-amd64
chmod +x ruptura-ctl && sudo mv ruptura-ctl /usr/local/bin/

# Point it at your instance
export RUPTURA_URL=http://localhost:8080
export RUPTURA_API_KEY=<your-api-key>
ruptura-ctl status
```

For full install options (macOS, arm64, Go install, kubectl plugin, in-cluster Job, OpenShift) see the [CLI Reference →](../cli/rupturactl.md).

---

## Kubernetes

The recommended production deployment uses the Helm chart. It deploys a single Ruptura binary with BadgerDB embedded storage — no external database.

### Using Helm (recommended)

```bash
git clone https://github.com/benfradjselim/ruptura.git
cd ruptura

helm install ruptura helm \
  --namespace ruptura-system \
  --create-namespace \
  --set apiKey=$(openssl rand -hex 32)
```

Verify:

```bash
kubectl get pods -n ruptura-system
kubectl port-forward svc/ruptura 8080:80 -n ruptura-system
curl http://localhost:8080/api/v2/health
# {"status":"ok","rupture_detection":"active","uptime_seconds":5}
```

Common Helm options:

```bash
--set apiKey=<your-key>          # API bearer token (required for prod)
--set persistence.size=20Gi               # PVC size (default: 10Gi)
--set image.tag=6.8.4                 # Pin to a specific version
--set serviceMonitor.enabled=true     # Prometheus Operator scrape
--set grafana.dashboards.enabled=true # Grafana dashboard ConfigMap
```

Upgrade:

```bash
helm upgrade ruptura helm --namespace ruptura-system
```

### Using `kubectl`

```bash
git clone https://github.com/benfradjselim/ruptura.git
cd ruptura

# Generate and apply API key secret before deploying
kubectl create namespace ruptura-system
kubectl create secret generic ruptura-secrets \
  -n ruptura-system \
  --from-literal=api-key=$(openssl rand -hex 32)

# Deploy all manifests
kubectl apply -f workdir/deploy/

# Verify pods are running
kubectl get pods -n ruptura-system

# Port-forward to test locally
kubectl port-forward svc/ruptura 8080:80 -n ruptura-system
curl http://localhost:8080/api/v2/health
```

### Using the RupturaInstance CRD (Operator)

If you have the Ruptura Operator installed (via OLM / OperatorHub, Red Hat OperatorHub, or manually), deploy a full instance declaratively:

```yaml
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: ruptura
  namespace: ruptura-system
spec:
  edition: community        # community (read-only actions) | autopilot (full execution)
  storageSize: 10Gi         # PVC size for BadgerDB (default: 10Gi)
  replicas: 1               # must be 1 — BadgerDB is single-writer
  apiKeyRef: ruptura-secret # name of a Secret with key 'api-key' (optional)
```

```bash
kubectl apply -f ruptura-instance.yaml

# Watch the operator reconcile it:
kubectl get rupturainstance -n ruptura-system -w
```

The operator creates: ServiceAccount `ruptura-instance`, PVC `{name}-data`, Deployment `{name}`, Service `{name}`. On OpenShift it also creates a Route with edge TLS.

See [Operator →](../architecture/operator.md) for full CRD reference, OLM install instructions, and Red Hat OperatorHub (OpenShift) setup.

---

## Docker

```bash
docker run -d \
  --name ruptura \
  -p 8080:8080 \
  -p 4317:4317 \
  -v ruptura-data:/var/lib/ruptura/data \
  -e RUPTURA_API_KEY=$(openssl rand -hex 32) \
  ghcr.io/benfradjselim/ruptura:6.8.4
```

| Port | Protocol | Purpose |
|------|----------|---------|
| 8080 | HTTP | REST API v2 · Prometheus metrics scrape |
| 4317 | HTTP | OTLP ingest (metrics, logs, traces) |

!!! note "OTLP vs REST ports"
    Send OTLP telemetry to port **4317**. The `/api/v2/v1/*` paths on port 8080 return `421 Misdirected Request` with a message directing you to port 4317. This is intentional — one endpoint per protocol.

Verify:

```bash
curl http://localhost:8080/api/v2/health
# {"status":"ok","rupture_detection":"active","uptime_seconds":3}
```

The built-in web dashboard is available at `http://localhost:8080` — no Grafana or external tools required. All assets are bundled in the binary and work in air-gapped environments.

---

## Build from Source

Requires Go 1.21+:

```bash
git clone https://github.com/benfradjselim/ruptura.git
cd ruptura/workdir
go build -o ruptura ./cmd/ruptura
./ruptura --port=8080 --otlp-port=4317 --storage=/tmp/ruptura-data
```

Run tests:

```bash
go test -race -timeout=120s ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | grep total
```

Lint Helm chart:

```bash
helm lint helm/
```

---

## Sending telemetry

### Prometheus remote_write

```yaml
# prometheus.yml
remote_write:
  - url: http://ruptura:8080/api/v2/write
    authorization:
      credentials: <your-api-key>
```

### OTLP (OpenTelemetry Collector)

```yaml
# otel-collector.yaml exporters section
exporters:
  otlphttp:
    endpoint: http://ruptura:4317
    headers:
      Authorization: "Bearer <your-api-key>"
```

Ruptura reads `k8s.namespace.name`, `k8s.deployment.name`, `k8s.statefulset.name`, `k8s.daemonset.name` from OTLP resource attributes. Multiple pods from the same Deployment are automatically merged into a single workload health view.

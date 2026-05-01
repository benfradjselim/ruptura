# Installation

## Kubernetes

The recommended production deployment uses the Helm chart. It deploys a single Ruptura binary with BadgerDB embedded storage — no external database.

### Using Helm (recommended)

```bash
git clone https://github.com/benfradjselim/ruptura.git
cd ruptura

helm install ruptura workdir/deploy/helm/ruptura \
  --namespace ruptura-system \
  --create-namespace \
  --set auth.apiKey=$(openssl rand -hex 32)
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
--set auth.apiKey=<your-key>          # API bearer token (required for prod)
--set storage.size=20Gi               # PVC size (default: 10Gi)
--set image.tag=6.2.2                 # Pin to a specific version
--set serviceMonitor.enabled=true     # Prometheus Operator scrape
--set grafana.dashboards.enabled=true # Grafana dashboard ConfigMap
```

Upgrade:

```bash
helm upgrade ruptura workdir/deploy/helm/ruptura --namespace ruptura-system
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

If you have the Ruptura Operator installed, deploy a full instance declaratively:

```yaml
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: production
  namespace: ruptura-system
spec:
  image: ghcr.io/benfradjselim/ruptura:6.2.2
  port: 8080
  storageSize: 20Gi
  apiKey:
    secretRef: ruptura-api-key
  replicas: 1
```

```bash
kubectl apply -f ruptura-instance.yaml
```

See [Operator →](../architecture/operator.md) for full CRD reference.

---

## Docker

```bash
docker run -d \
  --name ruptura \
  -p 8080:8080 \
  -p 4317:4317 \
  -v ruptura-data:/var/lib/ruptura/data \
  -e RUPTURA_API_KEY=$(openssl rand -hex 32) \
  ghcr.io/benfradjselim/ruptura:6.2.2
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
helm lint deploy/helm/ruptura/
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

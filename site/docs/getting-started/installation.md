# Installation

## Quick start (60 seconds)

=== "Kubernetes / Helm (OCI)"

    ```bash
    helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
      --namespace ruptura-system \
      --create-namespace \
      --set apiKey=$(openssl rand -hex 32) \
      --set ui.enabled=true \
      --set ui.service.type=NodePort \
      --set ui.nodePort=31469 \
      --set service.type=NodePort \
      --set otlpNodePort=31470
    ```

    Verify:

    ```bash
    kubectl get pods -n ruptura-system
    # NAME                             READY   STATUS    RESTARTS   AGE
    # ruptura-engine-xxx               1/1     Running   0          30s
    # ruptura-ui-xxx                   1/1     Running   0          30s

    curl http://<node-ip>:31468/api/v2/health
    # {"status":"ok","version":"7.0.4",...}
    ```

    | Endpoint | URL |
    |----------|-----|
    | Dashboard | `http://<node-ip>:31469/` |
    | Engine API | `http://<node-ip>:31468/api/v2/health` |
    | OTLP ingest | `http://<node-ip>:31470/api/v2/write` |

=== "Docker"

    ```bash
    docker run -d --name ruptura \
      -p 8080:8080 -p 4317:4317 \
      -v ruptura-data:/var/lib/ruptura/data \
      -e RUPTURA_API_KEY=$(openssl rand -hex 32) \
      ghcr.io/benfradjselim/ruptura:v7.0.4
    curl http://localhost:8080/api/v2/health
    ```

    !!! note "Dashboard in Docker"
        In Docker mode, deploy the UI container separately or use `kubectl port-forward` for the full dashboard experience. The engine API is available at port 8080.

=== "ruptura-ctl (CLI)"

    ```bash
    curl -Lo ruptura-ctl \
      https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-linux-amd64
    chmod +x ruptura-ctl && sudo mv ruptura-ctl /usr/local/bin/
    export RUPTURA_URL=http://<node-ip>:31468
    export RUPTURA_API_KEY=<your-api-key>
    ruptura-ctl status
    ```

---

## ruptura-ctl

`ruptura-ctl` **v1.0.0** is the command-line interface for Ruptura. It connects to the Ruptura REST API from your workstation, a CI pipeline, or from inside the cluster.

### Install

=== "Linux amd64"

    ```bash
    curl -Lo ruptura-ctl \
      https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-linux-amd64
    chmod +x ruptura-ctl && sudo mv ruptura-ctl /usr/local/bin/
    ruptura-ctl version
    # ruptura-ctl v1.0.0
    ```

=== "Linux arm64"

    ```bash
    curl -Lo ruptura-ctl \
      https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-linux-arm64
    chmod +x ruptura-ctl && sudo mv ruptura-ctl /usr/local/bin/
    ```

=== "macOS (Apple Silicon)"

    ```bash
    curl -Lo ruptura-ctl \
      https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-darwin-arm64
    chmod +x ruptura-ctl && sudo mv ruptura-ctl /usr/local/bin/
    ```

=== "macOS (Intel)"

    ```bash
    curl -Lo ruptura-ctl \
      https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-darwin-amd64
    chmod +x ruptura-ctl && sudo mv ruptura-ctl /usr/local/bin/
    ```

=== "Go install"

    ```bash
    go install github.com/benfradjselim/ruptura/cmd/ruptura-ctl@latest
    ```

=== "Build from source"

    ```bash
    git clone https://github.com/benfradjselim/ruptura.git
    cd ruptura/workdir
    go build -o ruptura-ctl ./cmd/ruptura-ctl
    sudo mv ruptura-ctl /usr/local/bin/
    ```

### Configure

```bash
export RUPTURA_URL=http://<node-ip>:31468    # engine NodePort
export RUPTURA_API_KEY=<your-api-key>

ruptura-ctl version          # ruptura-ctl v1.0.0
ruptura-ctl health           # server version, uptime, ingest stats
ruptura-ctl status           # live workload health table
ruptura-ctl get workloads    # all workloads with KPI breakdown
```

Install as a `kubectl` plugin:

```bash
sudo cp ruptura-ctl /usr/local/bin/kubectl-ruptura
kubectl ruptura status
```

---

## Kubernetes (Helm)

### Install from OCI registry (recommended)

```bash
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system \
  --create-namespace \
  --set apiKey=$(openssl rand -hex 32) \
  --set ui.enabled=true \
  --set ui.service.type=NodePort \
  --set ui.nodePort=31469 \
  --set service.type=NodePort \
  --set otlpNodePort=31470 \
  --set resources.limits.memory=512Mi \
  --set persistence.size=10Gi
```

### Common options

| Flag | Default | Description |
|------|---------|-------------|
| `--set apiKey=<key>` | required | API bearer token |
| `--set ui.enabled=true` | `false` | Deploy the Svelte dashboard pod |
| `--set ui.service.type=NodePort` | `ClusterIP` | Expose dashboard via NodePort |
| `--set ui.nodePort=31469` | auto | NodePort for the dashboard |
| `--set service.type=NodePort` | `ClusterIP` | Expose engine API via NodePort |
| `--set otlpNodePort=31470` | `31470` | NodePort for OTLP ingest |
| `--set resources.limits.memory=512Mi` | `512Mi` | Engine memory limit |
| `--set-string goMemLimit="400MiB"` | `"400MiB"` | Go GC soft limit — ~85% of memory limit |
| `--set persistence.size=10Gi` | `10Gi` | PVC size for BadgerDB storage |
| `--set image.tag=v7.0.4` | `latest` | Pin to specific version |
| `--set serviceMonitor.enabled=true` | `false` | Prometheus Operator scrape |
| `--set edition=autopilot` | `community` | Enable Tier-1 auto-execution |

### Upgrade

```bash
helm upgrade ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system \
  --reuse-values \
  --set image.tag=v7.0.4
```

### Verify

```bash
kubectl get pods -n ruptura-system
kubectl logs -n ruptura-system -l app.kubernetes.io/name=ruptura --tail=20

curl http://<node-ip>:31468/api/v2/health
# {"status":"ok","version":"7.0.4","edition":"community",...}
```

---

## Kubernetes (Operator / CRD)

If you have the Ruptura Operator installed (via OLM or manually):

```yaml
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: ruptura
  namespace: ruptura-system
spec:
  edition: community        # community | autopilot
  storageSize: 10Gi
  apiKeyRef: ruptura-secret # Secret with key 'api-key'
```

```bash
kubectl apply -f ruptura-instance.yaml
kubectl get rupturainstance -n ruptura-system -w
```

See [Operator →](../architecture/operator.md) for full CRD reference and OLM setup.

---

## Docker

```bash
docker run -d --name ruptura \
  -p 8080:8080 \
  -p 4317:4317 \
  -v ruptura-data:/var/lib/ruptura/data \
  -e RUPTURA_API_KEY=$(openssl rand -hex 32) \
  ghcr.io/benfradjselim/ruptura:v7.0.4
```

| Port | Purpose |
|------|---------|
| `8080` | REST API · Prometheus self-metrics |
| `4317` | OTLP ingest — metrics, logs, traces (JSON over HTTP) |

---

## Build from source

```bash
git clone https://github.com/benfradjselim/ruptura.git
cd ruptura/workdir
go build -o ruptura ./cmd/ruptura
./ruptura --port=8080 --otlp-port=4317 \
          --api-key=$(openssl rand -hex 32) \
          --storage=/tmp/ruptura-data
```

Run tests:

```bash
go test -race -timeout=120s ./...
```

Build the UI:

```bash
cd ruptura/ui
npm install
npm run build
# Output: ui/dist/  (served by the ruptura-ui container)
```

---

## Sending telemetry

### Prometheus remote_write

The `host` label must equal the workload key (`namespace/Kind/name`) so the pipeline can index correctly:

```yaml
# prometheus.yml
remote_write:
  - url: http://<node-ip>:31470/api/v2/write
    authorization:
      credentials: <your-api-key>
```

Or send JSON directly:

```bash
curl -X POST http://<node-ip>:31470/api/v2/write \
  -H "Content-Type: application/json" \
  -d '{
    "timeseries": [{
      "Labels": [
        {"Name": "__name__",   "Value": "cpu_percent"},
        {"Name": "host",       "Value": "default/Deployment/my-app"},
        {"Name": "namespace",  "Value": "default"},
        {"Name": "deployment", "Value": "my-app"}
      ],
      "Samples": [{"Value": 72.5, "Timestamp": 1234567890000}]
    }]
  }'
```

### OTLP / OpenTelemetry Collector

```yaml
# otel-collector.yaml — exporters section
exporters:
  otlphttp/ruptura:
    endpoint: http://<node-ip>:31470
    encoding: json          # Ruptura accepts JSON only — no protobuf
    compression: none       # no gzip
    headers:
      Authorization: "Bearer <your-api-key>"
```

Ruptura reads `k8s.namespace.name`, `k8s.deployment.name`, etc. from OTLP resource attributes and groups signals by Kubernetes workload automatically.

!!! important "OTLP format constraints"
    Ruptura's OTLP endpoint accepts **JSON only** (no protobuf) and **no compression** (no gzip).

### Workload simulator

Inject synthetic workloads with distinct failure profiles to test the system immediately:

```bash
python3 scripts/simulate.py [--host HOST] [--port PORT] [--interval SEC]
# Default target: http://185.229.225.115:31470

# Workloads injected every 5s:
#   gateway        — stable/healthy (CPU ~22%, err ~0.3%)
#   order-service  — slow-burn CPU stress (45→90% over 10 min)
#   payment-api    — error bursts every 2 min (8→43% error rate)
#   cache-worker   — traffic spikes every 3 min (up to 1350 rps)
#   ml-inference   — noisy/calibrating (high variance, new workload)
```

See [Operations → Simulation →](../operations/simulation.md) for full simulator documentation.

---

## Direct curl (raw OTLP JSON)

```bash
# Send a log
curl -X POST http://<node-ip>:31470/otlp/v1/logs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <api-key>" \
  -d '{
    "resourceLogs": [{
      "resource": {"attributes": [
        {"key": "service.name", "value": {"stringValue": "my-service"}},
        {"key": "k8s.namespace.name", "value": {"stringValue": "production"}},
        {"key": "k8s.deployment.name", "value": {"stringValue": "my-service"}}
      ]},
      "scopeLogs": [{"logRecords": [{
        "timeUnixNano": "1715780000000000000",
        "severityNumber": 17,
        "severityText": "ERROR",
        "body": {"stringValue": "payment timeout after 5000ms"}
      }]}]
    }]
  }'
```

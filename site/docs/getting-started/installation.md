# Installation

## Quick start (60 seconds)

=== "Kubernetes / k3s (Helm)"

    ```bash
    helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
      --namespace ruptura-system --create-namespace \
      --set apiKey=$(openssl rand -hex 32) \
      --set resources.limits.memory=2Gi \
      --set persistence.size=10Gi
    ```

    Check it:

    ```bash
    kubectl get pods -n ruptura-system
    curl http://<node-ip>:<nodeport>/api/v2/health
    ```

=== "Docker"

    ```bash
    docker run -d --name ruptura \
      -p 8080:8080 -p 4317:4317 \
      -v ruptura-data:/var/lib/ruptura/data \
      -e RUPTURA_API_KEY=$(openssl rand -hex 32) \
      ghcr.io/benfradjselim/ruptura:v6.8.13
    curl http://localhost:8080/api/v2/health
    ```

=== "ruptura-ctl (CLI)"

    ```bash
    curl -Lo ruptura-ctl \
      https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-linux-amd64
    chmod +x ruptura-ctl && sudo mv ruptura-ctl /usr/local/bin/
    export RUPTURA_URL=http://localhost:8080
    export RUPTURA_API_KEY=<your-api-key>
    ruptura-ctl status
    ```

---

## ruptura-ctl

`ruptura-ctl` **v1.0.0** is the command-line interface for Ruptura. It connects to the Ruptura REST API from your workstation, a CI pipeline, or from inside the cluster. It is versioned **independently** from the server — you can run `ruptura-ctl v1.0.0` against any `ruptura >= v6.8.x`.

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
export RUPTURA_URL=http://<host>:8080      # or NodePort URL
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
  --set resources.limits.memory=2Gi \
  --set-string goMemLimit="1700MiB" \
  --set persistence.size=10Gi
```

### Common options

| Flag | Default | Description |
|------|---------|-------------|
| `--set apiKey=<key>` | required | API bearer token |
| `--set resources.limits.memory=2Gi` | `512Mi` | Memory limit — BadgerDB needs headroom |
| `--set-string goMemLimit="1700MiB"` | `"400MiB"` | Go GC soft limit — set to ~85% of memory limit. **Use MiB/GiB suffix, not bytes.** |
| `--set persistence.size=10Gi` | `10Gi` | PVC size for BadgerDB storage |
| `--set image.tag=v6.8.13` | `latest` | Pin to specific version |
| `--set service.type=NodePort` | `ClusterIP` | Expose via NodePort |
| `--set service.nodePorts.api=31468` | auto | NodePort for REST API |
| `--set serviceMonitor.enabled=true` | `false` | Prometheus Operator scrape |

### Upgrade

```bash
helm upgrade ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system \
  --reuse-values \
  --set image.tag=v6.8.13
```

### Verify

```bash
kubectl get pods -n ruptura-system
kubectl logs -n ruptura-system -l app.kubernetes.io/name=ruptura --tail=20

# Port-forward if not using NodePort
kubectl port-forward svc/ruptura 8080:80 -n ruptura-system

curl http://localhost:8080/api/v2/health
# {"status":"starting","version":"6.8.13","edition":"community",...}
```

!!! warning "Memory limit — important"
    Set `resources.limits.memory` to at least **2Gi** on production clusters. BadgerDB and the analysis engine can spike past 1Gi under load. Set `goMemLimit` to ~85% of the memory limit using Go memory suffixes (`1700MiB` for a `2Gi` limit) — **never use raw byte counts**, Helm may render them in scientific notation which crashes the Go runtime.

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
  ghcr.io/benfradjselim/ruptura:v6.8.13
```

| Port | Purpose |
|------|---------|
| `8080` | REST API · Prometheus self-metrics · Web dashboard |
| `4317` | OTLP ingest — metrics, logs, traces (JSON over HTTP) |

The web dashboard is embedded in the binary and served at `http://localhost:8080/ui/`. It requires no external tools and works in air-gapped environments.

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

---

## Sending telemetry

Ruptura accepts three ingest methods. See the [Ingest Guide →](ingest.md) for full details and language-specific examples.

### Prometheus remote_write

```yaml
# prometheus.yml
remote_write:
  - url: http://ruptura:8080/api/v2/write
    authorization:
      credentials: <your-api-key>
```

### OTLP / OpenTelemetry Collector

```yaml
# otel-collector.yaml — exporters section
exporters:
  otlphttp/ruptura:
    endpoint: http://ruptura:4317
    encoding: json          # Ruptura accepts JSON only — no protobuf
    compression: none       # no gzip
    headers:
      Authorization: "Bearer <your-api-key>"
```

!!! important "OTLP format constraints"
    Ruptura's OTLP endpoint accepts **JSON only** (no protobuf) and **no compression** (no gzip). Configure your OTel Collector exporter with `encoding: json` and `compression: none`.

### Direct curl (raw OTLP JSON)

```bash
# Send a log
curl -X POST http://<host>:4317/otlp/v1/logs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <api-key>" \
  -d '{
    "resourceLogs": [{
      "resource": {"attributes": [
        {"key": "service.name", "value": {"stringValue": "my-service"}},
        {"key": "k8s.namespace.name", "value": {"stringValue": "production"}}
      ]},
      "scopeLogs": [{"logRecords": [{
        "timeUnixNano": "'$(date +%s%N)'",
        "severityNumber": 17,
        "severityText": "ERROR",
        "body": {"stringValue": "payment timeout after 5000ms"}
      }]}]
    }]
  }'
```

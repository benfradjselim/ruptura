# Production Deployment

## Kubernetes with Helm (recommended)

```bash
git clone https://github.com/benfradjselim/ruptura.git
cd ruptura

helm install ruptura helm \
  --namespace ruptura-system \
  --create-namespace \
  --set apiKey=$(openssl rand -hex 32) \
  --set persistence.size=20Gi \
  --set actions.executionMode=suggest
```

Key Helm values for production:

```yaml
# helm/values.yaml overrides
apiKey: ""                   # required: set via --set apiKey=... or existing secret
autodiscovery:
  enabled: true              # auto-discovers all Deployments/StatefulSets

persistence:
  size: 20Gi                 # BadgerDB storage

ingestRPS: 1000              # token-bucket rate limit on ingest

serviceMonitor:
  enabled: true              # for Prometheus Operator scraping
  interval: 15s

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 1000m
    memory: 512Mi
```

Upgrade:

```bash
helm upgrade ruptura helm --namespace ruptura-system --set apiKey=<your-key>
```

## RupturaInstance Operator

If you prefer a declarative CRD-based approach:

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
# Create the API key secret first
kubectl create secret generic ruptura-api-key \
  --from-literal=api-key=$(openssl rand -hex 32) \
  -n ruptura-system

kubectl apply -f ruptura-instance.yaml
```

## kubectl (kustomize manifests)

```bash
# Create namespace and API key secret first
kubectl create namespace ruptura-system
kubectl create secret generic ruptura-secrets \
  -n ruptura-system \
  --from-literal=api-key=$(openssl rand -hex 32)

# Apply all manifests
kubectl apply -f workdir/deploy/

# Verify
kubectl get pods -n ruptura-system
kubectl port-forward svc/ruptura 8080:80 -n ruptura-system
curl http://localhost:8080/api/v2/health
```

## Docker Compose (single-host)

```yaml
# docker-compose.yml
services:
  ruptura:
    image: ghcr.io/benfradjselim/ruptura:6.2.2
    ports:
      - "8080:8080"
      - "4317:4317"
    volumes:
      - ruptura-data:/var/lib/ruptura/data
    environment:
      RUPTURA_API_KEY: "${RUPTURA_API_KEY}"
      RUPTURA_INGEST_RPS: "1000"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v2/health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  ruptura-data:
```

```bash
RUPTURA_API_KEY=$(openssl rand -hex 32) docker compose up -d
```

## Prometheus integration

Add Ruptura as a scrape target and configure remote_write:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: ruptura
    static_configs:
      - targets: ["ruptura:8080"]
    metrics_path: /api/v2/metrics

remote_write:
  - url: http://ruptura:8080/api/v2/write
    authorization:
      credentials: "<your-api-key>"
```

## Alertmanager rules

```yaml
# ruptura-alerts.yml
groups:
  - name: ruptura
    rules:
      - alert: RupturaCritical
        expr: |
          ruptura_kpi{signal="fused_rupture_index"} > 3.0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Rupture Index critical on {{ $labels.workload }}"
          description: "FusedR={{ $value | printf \"%.1f\" }} — check /api/v2/explain"

      - alert: RupturaHealthScoreLow
        expr: |
          ruptura_kpi{signal="health_score"} < 40
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Health score below 40 on {{ $labels.workload }}"

      - alert: RupturaDown
        expr: up{job="ruptura"} == 0
        for: 1m
        labels:
          severity: critical
```

## Grafana

Enable the bundled dashboard:

```bash
# Via Helm
helm upgrade ruptura helm -n ruptura-system \
  --set grafana.dashboards.enabled=true

# Or import manually
# File: workdir/deploy/grafana/dashboards/ruptura_overview.json
```

Point a Prometheus datasource at `http://ruptura:8080/api/v2/metrics`.

## Resource sizing

| Deployment size | Workloads | RAM | CPU | Storage |
|-----------------|-----------|-----|-----|---------|
| Dev / edge | < 10 | 64 MB | 0.1 core | 1 GB |
| Small | < 50 | 128 MB | 0.25 core | 10 GB |
| Medium | 50–300 | 256 MB | 0.5 core | 30 GB |
| Large | 300+ | 512 MB | 1 core | 100 GB |

Ruptura uses BadgerDB embedded — storage scales with the number of workloads and retention settings (`kpis_days: 400` default).

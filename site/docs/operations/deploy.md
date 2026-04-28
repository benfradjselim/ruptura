# Production Deployment

## Kubernetes with Helm (recommended)

```bash
helm install ruptura ./helm \
  --namespace ruptura-system \
  --create-namespace \
  --set auth.jwtSecret=$(openssl rand -hex 32) \
  --set storage.size=50Gi \
  --set actions.executionMode=suggest \
  --set actions.safety.namespaceAllowlist="{production,staging}"
```

### Production `values.yaml`

```yaml
# helm/values.yaml (production overrides)
replicaCount: 1

image:
  repository: ruptura
  tag: "6.1.0"
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 8080
  grpcPort: 9090

storage:
  size: 50Gi
  storageClass: "standard"

auth:
  jwtSecret: ""  # set via --set or RUPTURA_JWT_SECRET env var

actions:
  executionMode: suggest
  safety:
    rateLimitPerHour: 6
    cooldownSeconds: 300
    namespaceAllowlist:
      - production
      - staging

ensemble:
  adaptive: true

resources:
  requests:
    memory: "128Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "1000m"
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
  image: ruptura:6.1.0
  port: 8080
  storageSize: 50Gi
  apiKey:
    secretRef: ruptura-api-key
  replicas: 1
```

```bash
# Create the API key secret first
kubectl create secret generic ruptura-api-key \
  --from-literal=key=$(openssl rand -hex 32) \
  -n ruptura-system

kubectl apply -f ruptura-instance.yaml
```

## Docker Compose (single-host production)

```yaml
# docker-compose.prod.yml
version: "3.8"
services:
  ruptura:
    image: ruptura:6.1.0
    ports:
      - "8080:8080"
      - "9090:9090"
    volumes:
      - ruptura-data:/var/lib/ruptura
      - ./ruptura.yaml:/etc/kairo/ruptura.yaml:ro
    environment:
      RUPTURA_JWT_SECRET: "${RUPTURA_JWT_SECRET}"
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
RUPTURA_JWT_SECRET=$(openssl rand -hex 32) docker compose -f docker-compose.prod.yml up -d
```

## Prometheus integration

Add Ruptura as a scrape target:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: ruptura
    static_configs:
      - targets: ["ruptura:8080"]
    metrics_path: /api/v2/metrics
    bearer_token: "<api-key>"

remote_write:
  - url: http://ruptura:8080/api/v2/write
    authorization:
      credentials: "<api-key>"
```

## Alertmanager rules

```yaml
# ruptura-alerts.yml
groups:
  - name: ruptura
    rules:
      - alert: RupturaRuptureCritical
        expr: rpt_rupture_index > 3.0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Rupture Index critical on {{ $labels.host }}"
          description: "R={{ $value | printf \"%.1f\" }} — check /api/v2/explain"

      - alert: RupturaHealthScoreLow
        expr: rpt_kpi_healthscore < 40
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Health score below 40 on {{ $labels.host }}"
```

## Resource sizing

| Deployment size | RAM | CPU | Storage |
|-----------------|-----|-----|---------|
| Edge / dev | 64 MB | 0.1 core | 1 GB |
| Small (< 50 hosts) | 128 MB | 0.25 core | 10 GB |
| Medium (50–500 hosts) | 256 MB | 0.5 core | 30 GB |
| Large (500+ hosts) | 512 MB | 1 core | 100 GB |

Ruptura uses BadgerDB embedded — storage scales with the number of hosts and retention settings.

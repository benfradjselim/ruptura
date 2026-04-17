# OHE — Observability Holistic Engine

<p align="center">
  <img src="https://img.shields.io/badge/version-4.2.0-blue?style=for-the-badge" alt="Version">
  <img src="https://img.shields.io/badge/promql-passthrough-purple?style=for-the-badge" alt="PromQL">
  <img src="https://img.shields.io/badge/multi--tenancy-organisations-orange?style=for-the-badge" alt="Multi-tenancy">
  <img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=for-the-badge&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/kubernetes-native-326CE5?style=for-the-badge&logo=kubernetes" alt="Kubernetes">
  <img src="https://img.shields.io/badge/license-MIT-green?style=for-the-badge" alt="License">
</p>

<p align="center">
  <strong>Stop reacting. Start predicting.</strong><br>
  OHE is a self-hosted, Kubernetes-native observability platform with ML-powered predictive alerting.<br>
  One binary replaces Grafana + Prometheus + Alertmanager + Loki — no external databases required.
</p>

---

## v4.2.0 — What's New

### PromQL Passthrough Proxy
Connect OHE to any existing Prometheus-compatible backend and query it directly from your dashboards.

- `POST /api/v1/datasources/{id}/proxy` — proxy PromQL to any registered datasource
- Supports `query` and `query_range` types
- Body: `{"query": "rate(http_requests_total[5m])", "start": 1700000000, "end": 1700003600, "step": 15, "type": "query_range"}`
- Works with Prometheus, Thanos, VictoriaMetrics, Grafana Mimir

### Query Widget
Live PromQL results inside any dashboard panel.

- New widget type `"query"` — renders a live results table from any registered datasource
- Configure via widget options: `datasource_id`, `query`, `range`, `step`
- Refreshes with the dashboard's auto-refresh interval

### Organisations API — Multi-tenancy Foundation
Isolate teams, projects, or customers in separate workspaces.

- `GET / POST /api/v1/orgs` — list and create organisations
- `GET / PUT / DELETE /api/v1/orgs/{id}` — manage individual orgs
- Slug auto-generated from name (URL-safe, unique)
- Orgs UI page under Configure in the sidebar
- Storage: `org:{id}` keys in BadgerDB — no schema migrations needed

---

## Full Feature Set (v4.0.0 → v4.2.0)

| Feature | Version |
|---------|---------|
| System metrics — CPU, memory, disk, network, load, uptime | v4.0.0 |
| 10 Holistic KPIs — stress, fatigue, mood, pressure, entropy, velocity… | v4.0.0 |
| ML forecasting — exponential smoothing, predicts exhaustion hours ahead | v4.0.0 |
| REST API + WebSocket live feed | v4.0.0 |
| Svelte UI embedded in binary (no CDN, no Node.js at runtime) | v4.0.0 |
| 14 built-in dashboard templates | v4.0.0 |
| Predictive dashboards — Next 1h / 6h / 24h mode | v4.0.0 |
| Alert rules engine — threshold and KPI-based | v4.0.0 |
| Alert delivery — Slack, PagerDuty, webhook | v4.1.0 |
| Kubernetes DaemonSet agent — real host metrics from every node | v4.1.0 |
| SSRF hardening for datasource URLs | v4.1.0 |
| **PromQL passthrough proxy** | **v4.2.0** |
| **Query widget — live PromQL results in dashboards** | **v4.2.0** |
| **Organisations API — multi-tenant workspaces** | **v4.2.0** |
| Prometheus `/metrics` exposition | v4.0.0 |
| OTLP / Loki / Elasticsearch / Datadog / DogStatsD ingestion | v4.0.0 |
| JWT auth + user management | v4.0.0 |
| BadgerDB embedded storage — no external database | v4.0.0 |

---

## Quick Start

### Kubernetes

```bash
git clone https://github.com/benfradjselim/Mlops_crew_automation.git
cd Mlops_crew_automation/workdir

cd ui && npm install && npm run build && cd ..
docker build -t ohe:latest .

kubectl apply -f deploy/pvc.yaml
kubectl apply -f deploy/secrets.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/central-deployment.yaml
kubectl apply -f deploy/agent-daemonset.yaml

kubectl logs -n ohe-system deploy/ohe-central | grep -A4 "FIRST BOOT"
kubectl port-forward svc/ohe-central 8080:80 -n ohe-system
```

### Docker Compose

```yaml
version: "3.9"
services:
  ohe:
    image: ohe:latest
    ports: ["8080:8080"]
    volumes: ["ohe-data:/var/lib/ohe/data"]
    environment:
      OHE_ADMIN_PASSWORD: changeme
volumes:
  ohe-data:
```

---

## Connect to Prometheus

```bash
# Register Prometheus as a datasource
curl -X POST http://localhost:8080/api/v1/datasources \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Prometheus","type":"prometheus","url":"http://prometheus.monitoring.svc:9090"}'

# Proxy a PromQL query
curl -X POST http://localhost:8080/api/v1/datasources/{id}/proxy \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"rate(node_cpu_seconds_total[5m])","type":"query_range","start":1700000000,"end":1700003600,"step":15}'
```

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OHE_ADMIN_PASSWORD` | auto-generated | Override admin password |
| `OHE_AUTH_ENABLED` | `false` | Enforce JWT authentication |
| `OHE_JWT_SECRET` | `change-me-in-production` | JWT signing secret |
| `OHE_TRUSTED_DATASOURCE_HOSTS` | — | Comma-separated IPs/hostnames bypassing SSRF check |
| `OHE_PORT` | `8080` | HTTP listen port |
| `OHE_STORAGE_PATH` | `/var/lib/ohe/data` | BadgerDB data directory |
| `OHE_COLLECT_INTERVAL` | `15s` | Metric collection interval |

---

## What Comes Next

| Version | Focus |
|---------|-------|
| v4.3.0 | 3-tier long-term retention (400 days) + full RBAC with org isolation |
| v4.4.0 | SLO engine, widget resize, 20 dashboard templates |

---

## License

MIT — free to use, modify, and deploy.

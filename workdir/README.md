# OHE — Observability Holistic Engine

<p align="center">
  <img src="https://img.shields.io/badge/version-4.0.0-blue?style=for-the-badge" alt="Version">
  <img src="https://img.shields.io/badge/release-initial-orange?style=for-the-badge" alt="Initial Release">
  <img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=for-the-badge&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/kubernetes-native-326CE5?style=for-the-badge&logo=kubernetes" alt="Kubernetes">
  <img src="https://img.shields.io/badge/single%20binary-no%20deps-success?style=for-the-badge" alt="Single Binary">
  <img src="https://img.shields.io/badge/license-MIT-green?style=for-the-badge" alt="License">
</p>

<p align="center">
  <strong>Stop reacting. Start predicting.</strong><br>
  OHE is a self-hosted, Kubernetes-native observability platform with ML-powered predictive alerting.<br>
  One binary replaces Grafana + Prometheus + Alertmanager + Loki — no external databases required.
</p>

---

## v4.0.0 — What's in This Release

This is the **initial production release** of OHE. It ships the full platform foundation:

| Area | Delivered |
|------|-----------|
| Metrics | System CPU, memory, disk, network, load average, process count, uptime |
| Intelligence | 10 Holistic KPIs (stress, fatigue, mood, pressure, humidity, contagion, resilience, entropy, velocity, health_score) |
| ML Forecasting | Exponential smoothing + trend extrapolation — predicts CPU/memory exhaustion hours ahead |
| REST API | Full JSON REST API with JWT auth |
| WebSocket | Live metric feed for real-time dashboards |
| UI | Embedded Svelte UI — no CDN, no Node.js at runtime |
| Templates | 14 built-in dashboard templates |
| Alerting | Rule engine with threshold and KPI-based rules, severity levels |
| Ingestion | Prometheus, OTLP, Loki, Elasticsearch, Datadog, DogStatsD |
| Storage | BadgerDB embedded key-value store — no external database |
| Kubernetes | Operator + DaemonSet agent manifests |

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

## Architecture

```
┌──────────────────────────────────────────────────┐
│                  ohe-central                     │
│  collector → processor → analyzer → alerter      │
│                  ↓             ↓                 │
│             BadgerDB       predictor             │
│                  ↓          REST API + WS        │
│            retention      Svelte UI (embedded)   │
└──────────────────────────────────────────────────┘
         ↑                        ↑
    ohe-agent              Prometheus / OTLP / Loki
  (DaemonSet)             (any compatible source)
```

**Design decisions:**
- **Single binary** — Go + Svelte UI via `embed.FS`, nothing to install at runtime
- **BadgerDB** — embedded, no Postgres/MySQL dependency
- **Agent = same binary** — one image to build, one image to maintain

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OHE_ADMIN_PASSWORD` | auto-generated | Override admin password |
| `OHE_AUTH_ENABLED` | `false` | Enforce JWT authentication |
| `OHE_JWT_SECRET` | `change-me-in-production` | JWT signing secret |
| `OHE_PORT` | `8080` | HTTP listen port |
| `OHE_STORAGE_PATH` | `/var/lib/ohe/data` | BadgerDB data directory |
| `OHE_COLLECT_INTERVAL` | `15s` | Metric collection interval |
| `OHE_DOGSTATSD_ADDR` | `:8125` | DogStatsD UDP listener |

---

## What Comes Next

| Version | Focus |
|---------|-------|
| v4.1.0 | Alert delivery (Slack, PagerDuty, webhook) + Kubernetes DaemonSet agent |
| v4.2.0 | PromQL passthrough + multi-tenancy organisations |
| v4.3.0 | 3-tier long-term retention (400 days) + full RBAC |
| v4.4.0 | SLO engine, widget resize, 20 dashboard templates |

---

## License

MIT — free to use, modify, and deploy.

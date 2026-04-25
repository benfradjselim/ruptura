# OHE — Observability Holistic Engine

<p align="center">
  <img src="https://img.shields.io/badge/version-4.4.0-0069ff?style=for-the-badge" alt="Version 4.4.0">
  <img src="https://img.shields.io/badge/single%20binary-zero%20deps-22c55e?style=for-the-badge" alt="Single Binary">
  <img src="https://img.shields.io/badge/kubernetes-native-326CE5?style=for-the-badge&logo=kubernetes&logoColor=white" alt="Kubernetes Native">
  <img src="https://img.shields.io/badge/ML--powered-predictive%20alerting-f97316?style=for-the-badge" alt="ML Powered">
  <img src="https://img.shields.io/badge/retention-400%20days-8b5cf6?style=for-the-badge" alt="400 Day Retention">
  <img src="https://img.shields.io/badge/license-MIT-gray?style=for-the-badge" alt="MIT License">
</p>

<br>

<p align="center">
  <b>Predict infrastructure failures hours before they happen.</b><br>
  One binary. One <code>kubectl apply</code>. No external databases. No configuration headaches.<br>
  Replace Grafana + Prometheus + Alertmanager + Loki with a platform that thinks ahead.
</p>

<br>

---

## The Problem OHE Solves

Every ops team has the same pain: you get paged *after* the outage. Your dashboards show what broke. Your alerts fire when it's already too late. You're running 5 tools that weren't designed to work together, and your on-call engineer is burning out on noise.

**OHE is built around a different idea:** infrastructure intelligence, not just infrastructure visibility.

| What you have today | What OHE gives you |
|--------------------|--------------------|
| Alerts fire when something **breaks** | OHE warns you when something **is about to break** — hours ahead |
| 5+ tools: Grafana, Prometheus, Alertmanager, Loki, PagerDuty connector | **1 binary**, `1 kubectl apply` |
| Raw metric dumps with manual threshold tuning | **10 Holistic KPIs** — composite signals that eliminate noise |
| Data expires after days | **400-day history** via automatic 3-tier downsampling |
| No SLO engine without expensive SaaS | Built-in **SLO / Error Budget** with burn rate and compliance tracking |
| One dashboard namespace for all teams | **Multi-tenant organisations** with RBAC per role |
| Hours to build a useful dashboard | **20 production-ready templates** — live in 30 seconds |

---

## How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                         ohe-central                             │
│                                                                 │
│  collector ──► processor ──► analyzer ──► alerter ──► notifier  │
│                    │               │           │                │
│               BadgerDB         predictor   SLO engine           │
│             (3-tier store)         │           │                │
│                    │           REST API ── WebSocket hub        │
│               retention              │                          │
│              compaction         Svelte UI (embedded, no CDN)    │
└─────────────────────────────────────────────────────────────────┘
          ▲                               ▲
     ohe-agent                    Prometheus / OTLP / Loki
   (DaemonSet,                   Datadog / Elasticsearch /
   1 per node)                   DogStatsD / any source
```

- **ohe-central** — the server: collects, stores, analyzes, alerts, and serves the UI
- **ohe-agent** — the same binary in agent mode, runs as a DaemonSet on every node
- **BadgerDB** — embedded key-value store. Zero external database requirement
- **Svelte UI** — embedded via Go's `embed.FS`. No CDN, works fully air-gapped

---

## Feature Overview

### Predictive Intelligence
- **ML forecasting** — exponential smoothing + trend extrapolation predicts CPU/memory exhaustion hours ahead
- **10 Holistic KPIs** — composite system-health signals: `stress`, `fatigue`, `mood`, `pressure`, `humidity`, `contagion`, `resilience`, `entropy`, `velocity`, `health_score`
- **Predictive dashboards** — switch any dashboard to "Next 1h / 6h / 24h" forecast mode
- **SLO / Error Budget engine** — define targets, track compliance %, burn rate, and remaining error budget in minutes

### Dashboards & Visualisation
- **20 built-in templates** — deploy any standard operations view in one click:

  | Template | Purpose |
  |---------|---------|
  | System Overview | CPU, memory, disk, network, load |
  | KPI Panorama | All 10 holistic KPIs with 300° arc gauges |
  | Predictions Panorama | ML forecast for every metric |
  | SRE Golden Signals | Latency, traffic, errors, saturation |
  | Executive Health Board | Single-pane executive summary |
  | Capacity Planning | Trend extrapolation for resource planning |
  | Full-Stack Application | App + infra + logs in one view |
  | Network Monitor | Bandwidth, packet loss, interface health |
  | Security Overview | Auth events, anomaly scores, access patterns |
  | Alert Center | Active alerts + rule inventory |
  | + 10 more | Logs, APM, containers, SLO tracker, custom |

- **Widget types** — timeseries, gauge (300° arc), stat card, KPI card, prediction chart, alert feed, top-N table, PromQL query, SLO status
- **Widget resize** — ◀ ▶ ▲ ▼ controls in edit mode on a responsive 4-column grid
- **Dashboard tabs** — group multiple dashboards in a single tabbed view
- **Live refresh** — auto-refresh from 5 seconds to 5 minutes

### Alerting & Notifications
- **Rule engine** — threshold and KPI-based rules, severity: `info` / `warning` / `critical` / `emergency`
- **Delivery** — Slack, PagerDuty, webhook with per-channel severity filtering
- **Lifecycle** — active → acknowledged → silenced → resolved
- **Test endpoint** — fire a live test payload to any channel instantly

### Long-Term Storage
| Tier | Retention | Resolution | Auto-selected when |
|------|-----------|------------|-------------------|
| Raw | 7 days | ~15 seconds | Query window < 6h |
| 5-min rollup | 35 days | 5-minute avg | Query window 6h – 7d |
| 1-hour rollup | **400 days** | 1-hour avg | Query window > 7d |

Compaction runs automatically every 30 minutes. Trigger on-demand: `POST /api/v1/retention/compact`.

### Multi-Tenancy & Access Control
- **Organisations** — isolated tenant workspaces with slug-based routing
- **RBAC** — `admin` / `operator` / `viewer` roles enforced at the API level
- **Org-scoped resources** — dashboards, datasources, and users carry `org_id`
- **JWT authentication** — stateless, org-aware, configurable secret

### Data Ingestion — Drop-in Compatibility

| Protocol | Endpoint | Works With |
|---------|---------|-----------|
| Native agent push | `POST /api/v1/ingest` | OHE DaemonSet agent |
| Prometheus scrape | `GET /metrics` | Prometheus, VictoriaMetrics |
| OTLP HTTP | `/otlp/v1/{traces,metrics,logs}` | OpenTelemetry Collector |
| Loki push | `POST /loki/api/v1/push` | Grafana Agent, Promtail |
| Elasticsearch bulk | `POST /_bulk` | Filebeat, Vector, Logstash |
| Datadog agent | `POST /api/v1/series` + `/api/v2/logs` | Datadog Agent |
| DogStatsD | UDP `:8125` | StatsD, DogStatsD clients |
| PromQL proxy | `POST /api/v1/datasources/{id}/proxy` | Prometheus, Thanos, Mimir |

---

## Quick Start — Kubernetes (5 minutes)

### Prerequisites
- Kubernetes cluster (k3s, EKS, GKE, AKS, or local)
- `kubectl` configured
- `docker` or equivalent to build the image

### Step 1 — Build

```bash
git clone https://github.com/benfradjselim/Mlops_crew_automation.git
cd Mlops_crew_automation/workdir

cd ui && npm install && npm run build && cd ..
docker build -t ohe:latest .
```

For a private registry:
```bash
docker tag ohe:latest your-registry/ohe:4.4.0
docker push your-registry/ohe:4.4.0
# Update the image field in deploy/central-deployment.yaml
```

### Step 2 — Deploy

```bash
kubectl apply -f deploy/pvc.yaml                # Persistent storage
kubectl apply -f deploy/secrets.yaml            # Secrets
kubectl apply -f deploy/configmap.yaml          # Runtime config
kubectl apply -f deploy/rbac.yaml               # RBAC for agent DaemonSet
kubectl apply -f deploy/central-deployment.yaml # Central server
kubectl apply -f deploy/agent-daemonset.yaml    # Agent on every node
```

### Step 3 — Get credentials

```bash
kubectl logs -n ohe-system deploy/ohe-central | grep -A4 "FIRST BOOT"
```
```
║  Username : admin
║  Password : <auto-generated>
```

Override the password at any time:
```bash
kubectl set env deployment/ohe-central -n ohe-system OHE_ADMIN_PASSWORD=YourSecurePassword
```

### Step 4 — Open the UI

**Local / development**
```bash
kubectl port-forward svc/ohe-central 8080:80 -n ohe-system
# Open http://localhost:8080
```

**Production — Ingress**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ohe
  namespace: ohe-system
spec:
  rules:
  - host: ohe.yourcompany.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: ohe-central
            port:
              number: 80
```

**Production — LoadBalancer**
```bash
kubectl patch svc ohe-central -n ohe-system -p '{"spec":{"type":"LoadBalancer"}}'
kubectl get svc ohe-central -n ohe-system
```

---

## Quick Start — Docker Compose (single machine)

```yaml
version: "3.9"
services:
  ohe:
    image: ohe:latest
    ports:
      - "8080:8080"
    volumes:
      - ohe-data:/var/lib/ohe/data
    environment:
      OHE_ADMIN_PASSWORD: changeme
volumes:
  ohe-data:
```
```bash
docker compose up -d
# Open http://localhost:8080
```

---

## Configuration

All configuration is via environment variables — no YAML config file required.

| Variable | Default | Description |
|----------|---------|-------------|
| `OHE_ADMIN_PASSWORD` | auto-generated (printed at boot) | Override the admin password |
| `OHE_AUTH_ENABLED` | `false` | Set `true` to enforce JWT authentication |
| `OHE_JWT_SECRET` | `change-me-in-production` | JWT signing secret — **must be changed in production** |
| `OHE_TRUSTED_DATASOURCE_HOSTS` | — | Comma-separated IPs/hostnames that bypass the SSRF check |
| `OHE_PORT` | `8080` | HTTP listen port |
| `OHE_STORAGE_PATH` | `/var/lib/ohe/data` | BadgerDB data directory |
| `OHE_COLLECT_INTERVAL` | `15s` | Metric collection interval |
| `OHE_DOGSTATSD_ADDR` | `:8125` | DogStatsD UDP listener |

**Enable authentication (production)**
```bash
kubectl set env deployment/ohe-central -n ohe-system \
  OHE_AUTH_ENABLED=true \
  OHE_JWT_SECRET=$(openssl rand -hex 32)
```

---

## API Reference

All endpoints are prefixed `/api/v1`. Authentication via `Authorization: Bearer <jwt>`.

### Core
```
GET  /health/live            Kubernetes liveness probe
GET  /health/ready           Kubernetes readiness probe
GET  /health                 Full health status + version
GET  /config                 Runtime configuration
```

### Metrics & Predictions
```
GET  /metrics                      Current snapshot (all hosts)
GET  /metrics/{name}               Time-series range (?from=&to=&host=)
GET  /metrics/{name}/range         Tiered range query (auto-selects tier by window)
GET  /metrics/{name}/aggregate     Aggregation (?agg=avg|min|max|p95)
GET  /kpis                         Holistic KPI snapshot
GET  /kpis/{name}/predict          KPI forecast
GET  /predict                      ML forecast (?host=&horizon=120&metric=)
POST /query                        QQL query {query, from, to, step_seconds}
```

### SLOs
```
GET  /slos                  List all SLOs
POST /slos                  Create SLO {name, metric, target, window, comparator, threshold}
PUT  /slos/{id}             Update SLO
DEL  /slos/{id}             Delete SLO
GET  /slos/status           Live status for all SLOs
GET  /slos/{id}/status      Compliance %, burn rate, error budget remaining
```

### Dashboards & Templates
```
GET  /dashboards               List dashboards
POST /dashboards               Create dashboard
PUT  /dashboards/{id}          Update dashboard  [operator]
DEL  /dashboards/{id}          Delete dashboard  [operator]
GET  /dashboards/{id}/export   Export as JSON
POST /dashboards/import        Import from JSON  [operator]
GET  /templates                List 20 built-in templates
POST /templates/{id}/apply     Instantiate template as dashboard  [operator]
```

### Alerting
```
GET  /alerts                         List alerts (?active=true&severity=critical)
POST /alerts/{id}/acknowledge
POST /alerts/{id}/silence
GET  /alert-rules                    List rules
POST /alert-rules                    Create rule  [operator]
PUT  /alert-rules/{name}             Update rule  [operator]
DEL  /alert-rules/{name}             Delete rule  [operator]
```

### Notifications
```
GET  /notifications             List channels
POST /notifications             Create {name, type, url, severities}  [operator]
PUT  /notifications/{id}        Update  [operator]
DEL  /notifications/{id}        Delete  [operator]
POST /notifications/{id}/test   Fire test payload  [operator]
```

### Datasources & PromQL Proxy
```
GET  /datasources               List datasources
POST /datasources               Register datasource  [operator]
PUT  /datasources/{id}          Update  [operator]
POST /datasources/{id}/test     Test connectivity  [operator]
POST /datasources/{id}/proxy    Proxy PromQL {query, start, end, step, type}
```

### Multi-tenancy
```
GET  /orgs                  List organisations
POST /orgs                  Create {name, slug, description}  [operator]
PUT  /orgs/{id}             Update  [operator]
GET  /orgs/{id}/members     List members
POST /orgs/{id}/members     Invite user to org  [operator]
```

### Auth & Users
```
POST /auth/login            Login → JWT {username, password}
POST /auth/refresh
GET  /auth/users            List users  [admin]
POST /auth/users            Create user  [admin]
DEL  /auth/users/{id}       Delete user  [admin]
PUT  /auth/users/{id}/org   Assign user to org  [admin]
```

### Retention & Fleet
```
GET  /retention/stats       Data point counts per storage tier
POST /retention/compact     Trigger on-demand downsampling  [operator]
GET  /fleet                 Aggregated health across all hosts
GET  /kpis/multi            KPI snapshot for multiple hosts
```

---

## Changelog

### v4.4.0 — SLO Engine · Widget Resize · 20 Templates
- **SLO / Error Budget engine** — define targets, track compliance %, burn rate, remaining budget
- **SLO widget** (`type: "slo"`) and full SLO management page in the UI
- **300° arc Gauge redesign** — color-inverted for stress/fatigue KPIs (lower = greener)
- **Widget resize** — ◀ ▶ ▲ ▼ controls in edit mode, `w × h` stored on a 4-column grid
- **6 new dashboard templates** — Executive Health Board, Capacity Planning, Full-Stack App, Network Monitor, SRE Golden Signals, Predictions Panorama — **20 templates total**

### v4.3.0 — Long-Term Retention · Full RBAC
- **3-tier retention** — raw (7d) → 5-min rollups (35d) → 1-hour rollups (400d)
- Automatic compaction every 30 min + on-demand `POST /retention/compact`
- `GET /metrics/{name}/range` auto-selects the correct tier by query window
- **Full RBAC** — `org_id` on users, dashboards, datasources; JWT carries org context
- Org member management (`GET / POST /orgs/{id}/members`)

### v4.2.0 — PromQL Passthrough · Multi-tenancy
- **PromQL passthrough** — `POST /datasources/{id}/proxy` proxies to Prometheus / Thanos / VictoriaMetrics
- **Query widget** (`type: "query"`) — live PromQL results table in any dashboard
- **Organisations API** — full CRUD with slug auto-generation and BadgerDB persistence

### v4.1.0 — Alert Delivery · Kubernetes Agent
- **Alert delivery** — webhook, Slack, PagerDuty wired end-to-end with per-severity filtering
- **Kubernetes DaemonSet agent** — real host metrics (CPU, memory, disk, network) from every node
- **SSRF hardening** — `OHE_TRUSTED_DATASOURCE_HOSTS` for cluster-internal Prometheus

### v4.0.0 — Initial Release
- System metrics, 10 holistic KPIs, ML forecasting, REST API, WebSocket live feed
- Embedded Svelte UI, 14 built-in dashboard templates, alert rules engine
- Prometheus / OTLP / Loki / Elasticsearch / Datadog / DogStatsD ingestion
- JWT authentication, BadgerDB storage, Kubernetes operator

---

## Production Security Checklist

- [ ] `OHE_AUTH_ENABLED=true`
- [ ] Strong `OHE_JWT_SECRET` — minimum 32 random bytes (`openssl rand -hex 32`)
- [ ] Change default admin password
- [ ] Set `OHE_TRUSTED_DATASOURCE_HOSTS` to your Prometheus ClusterIP
- [ ] Mount a PVC at `/var/lib/ohe/data` for persistent storage
- [ ] Apply `deploy/network-policy.yaml` to restrict `ohe-system` namespace access
- [ ] TLS termination at the Ingress — never expose port 8080 directly to the internet

---

## License

MIT — free to use, modify, and deploy. Contributions welcome.

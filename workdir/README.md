# Ruptura

<p align="center">
  <img src="https://img.shields.io/badge/version-6.1.1-0069ff?style=for-the-badge" alt="v6.1.1">
  <img src="https://img.shields.io/badge/go-1.18+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go 1.18+">
  <img src="https://img.shields.io/badge/license-Apache%202.0-green?style=for-the-badge" alt="Apache 2.0">
  <img src="https://img.shields.io/badge/kubernetes-native-326CE5?style=for-the-badge&logo=kubernetes&logoColor=white" alt="Kubernetes Native">
  <img src="https://img.shields.io/badge/coverage-70%25+-brightgreen?style=for-the-badge" alt="Coverage">
</p>

<p align="center">
  <b>The Predictive Action Layer for Cloud-Native Infrastructure.</b><br>
  Ruptura detects infrastructure ruptures before they cause outages — and acts on them automatically.
</p>

---

## What Ruptura Does

Traditional observability tells you what broke. Ruptura tells you **what is about to break** — and triggers the right action before users feel it.

| Traditional Observability | Ruptura |
|--------------------------|-----------|
| Threshold alerts fire after the fact | Rupture Index™ detects divergence **hours early** |
| You define rules per metric | Adaptive ensemble learns your baseline automatically |
| Manual incident response | Tier-1 actions (scale, restart, rollback) fire automatically with safety gates |
| 5+ tools: Prom + Grafana + AM + Loki + PD | **One binary**, one `kubectl apply` |
| No reasoning about why | Full XAI trace for every prediction |

---

## Core Concepts

### Rupture Index™

```
R(t) = |α_burst(t)| / max(|α_stable(t)|, ε)

  α_burst  = slope from 5-min CA-ILR tracker  (detects sudden change)
  α_stable = slope from 60-min CA-ILR tracker (tracks baseline)
  ε        = 1e-6 (numerical stability)
```

| R Range | State | Action |
|---------|-------|--------|
| < 1.0 | Stable | None |
| 1.0–1.5 | Elevated | None |
| 1.5–3.0 | Warning | Tier-3 (human) |
| 3.0–5.0 | Critical | Tier-2 (suggested) |
| ≥ 5.0 | Emergency | Tier-1 (automated) |

### Adaptive Ensemble (v6.1)

Five models (CA-ILR, ARIMA, Holt-Winters, MAD, EWMA) combined with online MAE-based weight adaptation — weights update every 60s based on per-model error over a sliding 1-hour window.

### 8 Composite Signals

`stress` · `fatigue` · `pressure` · `contagion` · `resilience` · `entropy` · `sentiment` · `healthscore`

Each maps multiple raw metrics to a single interpretable signal. `healthscore` is a 0–100 product of the first four.

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                      ruptura                          │
│                                                          │
│  Ingest ──► Metric/Log/Trace pipelines ──► Fusion        │
│     │              │                         │           │
│  gRPC           Composites              RuptureDetector  │
│  OTLP           (8 signals)                  │           │
│  Prom rw        Adaptive                  Actions        │
│  DogStatsD      Ensemble              (K8s/Webhook/PD)   │
│     │                                        │           │
│  NATS/Kafka eventbus ◄──────────────── XAI Explain       │
│                                              │           │
│              REST API v2 (44 endpoints) ─────┘           │
│              K8s Operator (RupturaInstance CRD)            │
└──────────────────────────────────────────────────────────┘
```

**Single binary** — BadgerDB embedded, no external database required.

---

## Quick Start

### Kubernetes (recommended)

```bash
git clone https://github.com/benfradjselim/ruptura.git
cd ruptura

# Build
docker build -t ruptura:6.1.1 .

# Deploy
kubectl apply -f deploy/

# Port-forward
kubectl port-forward svc/ruptura 8080:8080 -n ruptura-system

# Health check
curl http://localhost:8080/api/v2/health
```

### Docker

```bash
docker run -d \
  -p 8080:8080 \
  -v ruptura-data:/var/lib/ruptura \
  -e RUPTURA_JWT_SECRET=$(openssl rand -hex 32) \
  ruptura:6.1.1

curl http://localhost:8080/api/v2/health
```

### Helm

```bash
helm install ruptura ./helm \
  --namespace ruptura-system \
  --create-namespace \
  --set auth.jwtSecret=$(openssl rand -hex 32)
```

---

## Configuration (`ruptura.yaml`)

```yaml
mode: connected          # connected | stateless | shadow

ingest:
  http_port: 8080
  grpc_port: 9090

eventbus:
  driver: none           # none | nats | kafka
  # nats: { url: "nats://localhost:4222" }
  # kafka: { brokers: ["localhost:9092"] }

ensemble:
  adaptive: false        # true = online MAE-based weight adaptation (v6.1)

predictor:
  rupture_threshold: 3.0
  confidence_thresholds:
    auto_action: 0.85
    alert: 0.60

actions:
  execution_mode: shadow  # shadow | suggest | auto
  safety:
    rate_limit_per_hour: 6

auth:
  jwt_secret: ""
  api_keys: []

storage:
  path: /var/lib/ruptura
```

---

## API

All endpoints at `/api/v2/`. Auth via `Authorization: Bearer <jwt>` or `X-API-Key: ohe_*`.

### Key Endpoints

```
# Ingest
POST /api/v2/write              Prometheus remote_write (primary)
POST /api/v2/v1/metrics         OTLP/HTTP metrics
POST /api/v2/v1/logs            OTLP/HTTP logs
POST /api/v2/v1/traces          OTLP/HTTP traces

# Rupture
GET  /api/v2/rupture/{host}
GET  /api/v2/ruptures

# Composite KPIs
GET  /api/v2/kpi/{name}/{host}  # name: stress|fatigue|pressure|contagion|resilience|entropy|sentiment|healthscore

# Actions
GET  /api/v2/actions
POST /api/v2/actions/{id}/approve
POST /api/v2/actions/emergency-stop

# Explainability
GET  /api/v2/explain/{rupture_id}
GET  /api/v2/explain/{rupture_id}/formula

# Health
GET  /api/v2/health
GET  /api/v2/metrics            Prometheus self-metrics (14 series)
```

Full API reference: [docs/v6.0.0/SPECS.md §8](docs/v6.0.0/SPECS.md)

---

## SDKs

**Go**
```go
import "github.com/benfradjselim/ruptura/sdk/go"

client := ruptura.New("http://ruptura:8080", "ohe_your_api_key")
rupture, _ := client.RuptureIndex("web-01")
weights, _ := client.EnsembleWeights("web-01")  // v6.1
```

**Python**
```python
from ruptura import Client

client = Client("http://ruptura:8080", api_key="ohe_your_api_key")
rupture = client.rupture_index("web-01")
```

---

## What Gets Emitted

### Actions (when `execution_mode: auto`)

| Provider | Actions |
|----------|---------|
| Kubernetes | scale, restart, cordon, drain, isolate |
| Webhook | notify, trigger_pipeline, custom |
| Alertmanager | alert, silence |
| PagerDuty | incident create/update |

Safety gates: 6 Tier-1 actions/target/hour · cooldown · rollback trigger · emergency stop · namespace allowlist.

### Events (when eventbus configured)

```
ruptura.rupture.{host}      on every rupture state change
ruptura.actions.tier1       on every Tier-1 automated action
```

### Prometheus Self-Metrics

`rpt_rupture_index` · `rpt_time_to_failure_seconds` · `rpt_kpi_healthscore` · `rpt_actions_total` · `rpt_ingest_samples_total` · `rpt_uptime_seconds` + 8 more.

---

## Kubernetes Operator (v6.1)

```yaml
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: production
spec:
  image: ruptura:6.1.1
  port: 8080
  storageSize: 10Gi
  apiKey:
    secretRef: ruptura-api-key
```

The operator reconciles Deployment + Service + PVC per `RupturaInstance`.

---

## Coverage

| Package group | Coverage |
|--------------|---------|
| pkg/ (rupture, composites, client) | 85–100% |
| internal/pipeline/* | 84–89% |
| internal/actions/* | 83–100% |
| internal/fusion + composites | 93% |
| internal/api + storage | 70–78% |
| ohe/operator | 85% |
| **Total** | **≥70%** |

---

## Development

```bash
go build ./...
go test -race -timeout=120s ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | grep total
golangci-lint run --timeout=5m
```

See [docs/v6.0.0/DEV-GUIDE.md](docs/v6.0.0/DEV-GUIDE.md) for the full dev guide.

---

## Changelog

### v6.1.0 — 2026-04-27
- **§23** Real gRPC ingest server (google.golang.org/grpc, 4MB max, back-pressure)
- **§24** NATS/Kafka eventbus — JetStream at-least-once + franz-go exactly-once
- **§25** Adaptive ensemble weighting — online MAE-based, 1-hour sliding window, 60s update
- **§26** Kubernetes operator — RupturaInstance CRD, controller-runtime reconcile loop
- Go SDK `ruptura-go` — full v2 API coverage

### v6.0.0 — 2026-04-25
- Complete clean-room rewrite from OHE v5.1 as `github.com/benfradjselim/ruptura`
- CA-ILR dual-scale ELS engine, ensemble of 5 models, 8 composite signals
- 44-endpoint REST API v2, XAI explainability, single-tenant BadgerDB storage
- Action engine (K8s/Webhook/Alertmanager/PagerDuty) with safety gates
- OTLP + Prometheus remote_write + DogStatsD ingest
- ≥70% test coverage across all packages

### v5.1.0 (OHE) — 2026-04-19
- Go + Python SDK, Prometheus remote_write, gRPC agent, Vault integration, plugin system

---

## Roadmap

| Version | Target | Focus |
|---------|--------|-------|
| **v6.1.0** | ✅ Released | gRPC, eventbus, adaptive ensemble, K8s operator |
| v6.2.0 | Q2 2026 | ruptura-ctl CLI, web dashboard v2, multi-tenant opt-in |
| v6.3.0 | Q3 2026 | SaaS self-serve, billing, managed cloud |

---

## License

Apache 2.0 — see [LICENSE](LICENSE)

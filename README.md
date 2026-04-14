# Observability Holistic Engine (OHE) v4.0.0

> **"Prevention is better than cure"**

OHE treats your infrastructure as a **living organism** — it doesn't just tell you what is wrong, it tells you **when and how it will go wrong**.

---

## Why OHE?

| Solution | Question Answered |
|---|---|
| Classic monitoring | CPU is at 85% |
| APM solutions | Service A is slow |
| **OHE v4.0** | **Storm in 2 hours, high fatigue, contagion spreading** |

---

## Quick Start

```bash
# One-liner install (central mode on port 8080)
curl -sSL https://ohe.io/install | bash

# Or build from source
git clone https://github.com/benfradjselim/ohe
cd ohe/workdir
go build -o ohe ./cmd/ohe/

# Run central (API + UI + local collection)
./ohe central --port 8080

# Run agent on each node (pushes to central)
./ohe agent --central-url http://central:8080
```

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                 OHE — Single Binary                  │
├──────────────────────────────────────────────────────┤
│  Collector → Processor → Analyzer → Predictor        │
│                    ↓                                 │
│            Badger (embedded TSDB)                    │
│                    ↓                                 │
│     REST API :8080  +  WebSocket /api/v1/ws          │
│                    ↓                                 │
│              Embedded Svelte UI                      │
└──────────────────────────────────────────────────────┘
```

**Two modes:**

| Mode | Role | Port |
|---|---|---|
| `central` | API server, UI, local collection, storage | 8080 |
| `agent` | Collects metrics, pushes to central every 15s | 8081 |

**Communication:** HTTP JSON (agent → central via `/api/v1/ingest`). No external dependencies.

---

## Holistic KPIs

OHE computes 6 composite KPIs treating infrastructure as an organism:

| KPI | Formula | States |
|---|---|---|
| **Stress** | `0.30·CPU + 0.20·RAM + 0.20·Load + 0.20·Errors + 0.10·Timeouts` | calm / nervous / stressed / panic |
| **Fatigue** | `∫(Stress − Recovery) dt` | rested / tired / exhausted / burnout |
| **Mood** | `(Uptime × Throughput) / (Errors × Timeouts × Restarts + ε)` | happy / content / neutral / sad / depressed |
| **Pressure** | `dStress/dt + ∫Errors dt` | stable / rising / storm_approaching / improving |
| **Humidity** | `(Errors × Timeouts) / Throughput` | dry / humid / very_humid / storm |
| **Contagion** | `Errors × CPU_load` | low / moderate / epidemic / pandemic |

### Predictive Alerts

| Condition | Alert |
|---|---|
| Pressure > 0.7 for 10 min | Storm in ~2 hours — scale up |
| Fatigue > 0.8 | Burnout imminent — schedule restart |
| Contagion > 0.6 | Epidemic — isolate services |
| Humidity > 0.5 | Error storm — activate circuit breaker |

---

## API Reference

All responses follow:
```json
{ "success": true, "data": {}, "timestamp": "2026-04-14T10:00:00Z" }
```

### Core Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/health` | — | Health check |
| GET | `/api/v1/config` | — | Runtime config |
| GET | `/api/v1/metrics` | viewer | Current normalized metrics |
| GET | `/api/v1/metrics/{name}` | viewer | Metric time series |
| GET | `/api/v1/metrics/{name}/aggregate` | viewer | avg/min/max/p95/p99 |
| POST | `/api/v1/query` | viewer | Metric query by name + time range |
| GET | `/api/v1/kpis` | viewer | Current KPI snapshot |
| GET | `/api/v1/kpis/{name}` | viewer | KPI time series |
| GET | `/api/v1/kpis/{name}/predict` | viewer | ILR prediction |
| GET | `/api/v1/predict?horizon=120` | viewer | All predictions |
| GET | `/api/v1/alerts` | viewer | Active alerts |
| POST | `/api/v1/alerts/{id}/acknowledge` | viewer | Acknowledge alert |
| POST | `/api/v1/alerts/{id}/silence` | viewer | Silence alert |
| DELETE | `/api/v1/alerts/{id}` | operator | Delete alert |
| GET/POST | `/api/v1/dashboards` | viewer/operator | List / Create |
| GET | `/api/v1/dashboards/{id}` | viewer | Get dashboard |
| PUT/DELETE | `/api/v1/dashboards/{id}` | operator | Update / Delete |
| GET | `/api/v1/dashboards/{id}/export` | viewer | Export dashboard JSON |
| POST | `/api/v1/dashboards/import` | operator | Import dashboard |
| GET | `/api/v1/templates` | viewer | List built-in templates |
| GET | `/api/v1/templates/{id}` | viewer | Get template |
| POST | `/api/v1/templates/{id}/apply` | operator | Instantiate template as dashboard |
| GET/POST | `/api/v1/datasources` | viewer/operator | List / Create |
| GET | `/api/v1/datasources/{id}` | viewer | Get datasource |
| PUT/DELETE | `/api/v1/datasources/{id}` | operator | Update / Delete |
| POST | `/api/v1/datasources/{id}/test` | operator | Test connectivity |
| POST | `/api/v1/auth/setup` | — | First-boot admin creation (once only) |
| POST | `/api/v1/auth/login` | — | JWT login |
| POST | `/api/v1/auth/refresh` | viewer | Refresh token |
| POST | `/api/v1/auth/logout` | viewer | Logout (stateless) |
| GET/POST/DELETE | `/api/v1/auth/users` | admin | User management |
| POST | `/api/v1/ingest` | operator | Agent metric push |
| WS | `/api/v1/ws` | viewer | Live KPI stream |

---

## Prediction Engine: ILR

OHE uses **Incremental Linear Regression (ILR)** — a zero-dependency, pure Go predictor:

| Metric | ILR | ARIMA | LSTM |
|---|---|---|---|
| Accuracy (MAE) | 6.2% | 4.1% | 2.0% |
| Memory | **0.5 MB** | 85 MB | 200+ MB |
| Inference | **0.8 ms** | 210 ms | 500 ms |
| Dependencies | **None** | Python | GPU |

ILR is 193,750× more resource-efficient than ARIMA while achieving comparable trend accuracy.

---

## Configuration

```yaml
# configs/central.yaml
mode: central
host: ""                        # auto-detected via os.Hostname()
port: 8080
storage_path: /var/lib/ohe/central
collect_interval: 15s
buffer_size: 10000
auth_enabled: false
jwt_secret: "change-me-in-production"
allowed_origins: []             # empty = wildcard * (dev); list origins for prod
                                # e.g. ["https://dashboard.example.com"]
```

```yaml
# configs/agent.yaml
mode: agent
port: 8081
central_url: http://central:8080
collect_interval: 15s
```

### First-Boot Admin Setup

On first start with an empty database, OHE auto-generates a random admin password and prints it to stdout. Alternatively, call the setup endpoint once manually:

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/setup \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"ChangeMe123!"}'
```

The `/auth/setup` endpoint returns `409 Conflict` if any user already exists, making it safe to call idempotently during provisioning.

---

## Resource Constraints

| Component | Memory | CPU | Storage |
|---|---|---|---|
| Agent | < 100 MB | < 1 core | — |
| Central | < 500 MB | < 2 cores | < 10 GB / 30 days |

**Storage TTLs:** Metrics 7d · Logs 30d · KPIs 7d · Alerts 90d

---

## Project Structure

```
workdir/
├── cmd/ohe/           Main entry point (agent + central)
├── internal/
│   ├── collector/     System metrics via /proc
│   ├── processor/     Normalize · Aggregate · Downsample
│   ├── analyzer/      Stress · Fatigue · Mood · Pressure · Humidity · Contagion
│   ├── predictor/     ILR + Dynamic thresholds + Anomaly detection
│   ├── storage/       Badger TSDB wrapper
│   ├── api/           REST handlers · Middleware · WebSocket
│   ├── alerter/       Rule engine + alert lifecycle
│   └── orchestrator/  Engine wiring + goroutine lifecycle
├── pkg/
│   ├── models/        Shared data types
│   └── utils/         Math · Circular buffer · Helpers
└── configs/           agent.yaml · central.yaml
```

---

## Development

```bash
# Run tests
go test ./... -v

# Build
go build -ldflags="-s -w" -o ohe ./cmd/ohe/

# Run locally
./ohe central --storage /tmp/ohe-data --port 8080
```

---

## Roadmap

| Phase | Goal | Status |
|---|---|---|
| 1 | Collection + Core KPIs + Storage | ✅ Done |
| 2 | ILR Predictions + Alerting | ✅ Done |
| 3 | Full REST API + WebSocket + Security | ✅ Done |
| 3.1 | RBAC · CORS allowlist · Rate limiting · SSRF protection | ✅ Done |
| 3.2 | First-boot admin seeding · Templates · Setup endpoint | ✅ Done |
| 4 | Svelte UI + Dashboards | 🔄 In Progress |
| 5 | HA + K8s Operator | Planned |
| 6 | Distributed Tracing + Multi-cluster | Planned |

---

**Author:** Selim Benfradj · **License:** MIT · **Version:** 4.0.0

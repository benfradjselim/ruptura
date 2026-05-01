# Configuration

Ruptura is primarily configured via environment variables. A `ruptura.yaml` config file is optional — pass it with `--config /path/to/ruptura.yaml`.

## Environment variables

These are the most important variables for getting started:

| Env var | Default | Description |
|---------|---------|-------------|
| `RUPTURA_API_KEY` | _(empty)_ | Bearer token for all API requests. If empty, authentication is disabled (dev only). |
| `RUPTURA_INGEST_RPS` | `1000` | Token-bucket rate limit (requests/second) on the ingest server |
| `RUPTURA_LOG_LEVEL` | `info` | Log verbosity: `debug` \| `info` \| `warn` \| `error` |

## CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | REST API + Prometheus remote_write server port |
| `--otlp-port` | `4317` | OTLP ingest server port (metrics, logs, traces) |
| `--storage` | `/var/lib/ruptura/data` | BadgerDB data directory |
| `--config` | _(none)_ | Path to optional `ruptura.yaml` config file |

## Full config file reference

```yaml
# ruptura.yaml

mode: connected          # connected | stateless | shadow

ingest:
  http_port: 8080        # REST API + Prometheus remote_write
  otlp_port: 4317        # OTLP HTTP ingest (metrics, logs, traces)
  grpc_port: 9090        # gRPC ingest

eventbus:
  driver: none           # none | nats | kafka
  # nats:
  #   url: "nats://localhost:4222"
  #   stream: ruptura-events
  # kafka:
  #   brokers: ["localhost:9092"]
  #   topic: ruptura.events

ensemble:
  adaptive: true         # Online MAE-based weight adaptation (recommended)
  # weights updated every 60s over a 1-hour sliding window when adaptive: true

predictor:
  rupture_threshold: 3.0          # FusedR ≥ threshold → Warning
  confidence_thresholds:
    auto_action: 0.85             # confidence required for Tier-1 auto-action
    alert: 0.60                   # confidence required to raise an alert

baselines:
  observation_window: 96          # observations before adaptive baselines activate
                                  # default 96 × 15s interval ≈ 24h

actions:
  execution_mode: suggest         # shadow | suggest | auto
  safety:
    rate_limit_per_hour: 6        # max Tier-1 actions per target per hour
    cooldown_seconds: 300         # min gap between actions on the same target
    namespace_allowlist:          # only act on pods in these namespaces
      - production
      - staging

auth:
  api_key: ""                     # set via RUPTURA_API_KEY env var

storage:
  path: /var/lib/ruptura/data     # BadgerDB data directory
  retention:
    kpis_days: 400                # long-term KPI retention
```

## Key options

### `mode`

| Value | Behaviour |
|-------|-----------|
| `connected` | Full rupture detection, actions, and XAI |
| `stateless` | No persistence — useful for testing ingest |
| `shadow` | Actions computed but never executed (dry-run) |

### `actions.execution_mode`

| Value | Behaviour |
|-------|-----------|
| `shadow` | Actions logged only — nothing executed |
| `suggest` | Actions queued, waiting for `/api/v2/actions/{id}/approve` |
| `auto` | Tier-1 actions executed automatically with safety gates |

For production, start with `suggest` to validate Ruptura's recommendations before enabling `auto`.

### `baselines.observation_window`

The number of 15-second observations before Ruptura switches from global thresholds to per-workload adaptive baselines. At the default of 96 (≈24h), you will see more initial alerts during the learning period — this is expected. After the window, false positives from batch jobs and normally-loaded services drop significantly.

### `ensemble.adaptive`

When `true`, Ruptura computes per-model MAE over a 1-hour sliding window and normalises prediction weights every 60 seconds. Models with lower error receive more weight. The five models are: CA-ILR, ARIMA, Holt-Winters, MAD, EWMA.

## Minimal production setup (Helm)

```bash
helm install ruptura workdir/deploy/helm/ruptura \
  --namespace ruptura-system \
  --create-namespace \
  --set auth.apiKey=$(openssl rand -hex 32) \
  --set storage.size=20Gi \
  --set serviceMonitor.enabled=true
```

## Minimal production setup (Docker)

```bash
docker run -d \
  --name ruptura \
  -p 8080:8080 \
  -p 4317:4317 \
  -v ruptura-data:/var/lib/ruptura/data \
  -e RUPTURA_API_KEY=$(openssl rand -hex 32) \
  -e RUPTURA_INGEST_RPS=5000 \
  -e RUPTURA_LOG_LEVEL=info \
  ghcr.io/benfradjselim/ruptura:6.2.2
```

# Configuration

Ruptura is configured via `ruptura.yaml`. Pass the path with `--config`.

## Full reference

```yaml
# ruptura.yaml

mode: connected          # connected | stateless | shadow

ingest:
  http_port: 8080        # REST API + Prometheus remote_write
  grpc_port: 9090        # gRPC ingest (v6.1)

eventbus:
  driver: none           # none | nats | kafka
  # nats:
  #   url: "nats://localhost:4222"
  #   stream: ruptura-events
  # kafka:
  #   brokers: ["localhost:9092"]
  #   topic: ruptura.events

ensemble:
  adaptive: false        # true = online MAE-based weight adaptation (v6.1)
  # weights are updated every 60s over a 1-hour sliding window when adaptive: true

predictor:
  rupture_threshold: 3.0          # R ≥ threshold → Warning
  confidence_thresholds:
    auto_action: 0.85             # confidence required for Tier-1 auto-action
    alert: 0.60                   # confidence required to raise an alert

actions:
  execution_mode: shadow          # shadow | suggest | auto
  safety:
    rate_limit_per_hour: 6        # max Tier-1 actions per target per hour
    cooldown_seconds: 300         # min gap between actions on the same target
    namespace_allowlist:          # only act on pods in these namespaces
      - production
      - staging

auth:
  jwt_secret: ""                  # required — set via env RUPTURA_JWT_SECRET
  api_keys: []                    # pre-provisioned keys (ohe_* format)

storage:
  path: /var/lib/ruptura            # BadgerDB data directory
  retention:
    raw_metrics_days: 7
    logs_days: 30
    kpis_days: 400                # compliance-ready long-term KPI retention
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
| `suggest` | Actions POSTed as suggestions, wait for `/approve` |
| `auto` | Tier-1 actions executed automatically with safety gates |

### `ensemble.adaptive`

When `true`, Ruptura computes per-model MAE over a 1-hour sliding window and normalises weights every 60 seconds. Models with lower error receive more weight. The five models are: CA-ILR, ARIMA, Holt-Winters, MAD, EWMA.

When `false`, equal weights are used (default: 0.20 each).

## Environment variables

Every YAML key can be overridden with an environment variable using `RUPTURA_` prefix and `_`-separated key path:

| Env var | Equivalent YAML |
|---------|----------------|
| `RUPTURA_JWT_SECRET` | `auth.jwt_secret` |
| `RUPTURA_INGEST_HTTP_PORT` | `ingest.http_port` |
| `RUPTURA_INGEST_GRPC_PORT` | `ingest.grpc_port` |
| `RUPTURA_ACTIONS_EXECUTION_MODE` | `actions.execution_mode` |
| `RUPTURA_ENSEMBLE_ADAPTIVE` | `ensemble.adaptive` |
| `RUPTURA_STORAGE_PATH` | `storage.path` |

## Minimal production config

```yaml
mode: connected
auth:
  jwt_secret: "${RUPTURA_JWT_SECRET}"
actions:
  execution_mode: suggest
  safety:
    rate_limit_per_hour: 6
    namespace_allowlist:
      - production
storage:
  path: /var/lib/ruptura
```

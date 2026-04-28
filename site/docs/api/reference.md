# API Reference

Base URL: `http://<host>:8080/api/v2`

All requests require `Authorization: Bearer <jwt_or_api_key>` unless noted.

---

## Health

### `GET /health`

Returns server health. No auth required.

```bash
curl http://localhost:8080/api/v2/health
```

```json
{"status":"ok","rupture_detection":"active","uptime_seconds":3842}
```

### `GET /ready`

Kubernetes readiness probe. Returns `204` when ready, `503` during startup.

---

## Ingest

### `POST /write`

Prometheus remote_write (protobuf). Used by Prometheus `remote_write` config.

### `POST /v1/metrics`

OTLP/HTTP metrics (protobuf or JSON).

### `POST /v1/logs`

OTLP/HTTP logs.

### `POST /v1/traces`

OTLP/HTTP traces.

---

## Rupture

### `GET /rupture/{host}`

Get the current Rupture Index for a host.

```bash
curl -H "Authorization: Bearer $KEY" \
  http://localhost:8080/api/v2/rupture/web-01
```

```json
{
  "host": "web-01",
  "rupture_index": 4.2,
  "state": "critical",
  "time_to_failure_seconds": 1800,
  "dominant_signal": "stress",
  "alpha_burst": 0.042,
  "alpha_stable": 0.010,
  "timestamp": "2026-04-28T10:00:00Z"
}
```

States: `stable` · `elevated` · `warning` · `critical` · `emergency`

### `GET /ruptures`

List all active ruptures across all hosts.

```json
{
  "ruptures": [
    { "host": "web-01", "rupture_index": 4.2, "state": "critical" },
    { "host": "db-01",  "rupture_index": 1.8, "state": "warning" }
  ]
}
```

---

## Composite KPIs

### `GET /kpi/{signal}/{host}`

Query a composite signal for a host.

**Signals:** `stress` · `fatigue` · `pressure` · `contagion` · `resilience` · `entropy` · `sentiment` · `healthscore`

```bash
curl -H "Authorization: Bearer $KEY" \
  http://localhost:8080/api/v2/kpi/stress/web-01
```

```json
{
  "signal": "stress",
  "host": "web-01",
  "value": 0.72,
  "state": "Stressed",
  "trend": "up",
  "timestamp": "2026-04-28T10:00:00Z"
}
```

---

## Adaptive Ensemble

### `GET /ensemble/{host}`

Get current model weights for a host.

```bash
curl -H "Authorization: Bearer $KEY" \
  http://localhost:8080/api/v2/ensemble/web-01
```

```json
{
  "host": "web-01",
  "updated_at": "2026-04-28T10:00:00Z",
  "adaptive": true,
  "weights": {
    "ca_ilr":       0.35,
    "arima":        0.22,
    "holt_winters": 0.18,
    "mad":          0.14,
    "ewma":         0.11
  }
}
```

---

## Actions

### `GET /actions`

List all pending / recent actions.

```json
{
  "actions": [
    {
      "id": "act_abc",
      "host": "web-01",
      "type": "scale",
      "params": {"replicas": 5},
      "tier": 2,
      "status": "pending",
      "rupture_id": "r_abc123",
      "created_at": "2026-04-28T10:00:00Z"
    }
  ]
}
```

### `POST /actions/{id}/approve`

Approve a Tier-2 pending action.

```bash
curl -X POST -H "Authorization: Bearer $KEY" \
  http://localhost:8080/api/v2/actions/act_abc/approve
```

### `POST /actions/emergency-stop`

Immediately halt all Tier-1 automated actions globally.

```bash
curl -X POST -H "Authorization: Bearer $KEY" \
  http://localhost:8080/api/v2/actions/emergency-stop
```

---

## Explainability

### `GET /explain/{rupture_id}`

Full XAI trace for a rupture.

```json
{
  "rupture_id": "r_abc123",
  "host": "web-01",
  "rupture_index": 4.2,
  "formula": "R = |α_burst| / |α_stable| = 0.042 / 0.010 = 4.2",
  "model_contributions": {
    "ca_ilr": {"weight": 0.35, "prediction": 4.4}
  },
  "dominant_signal": "stress",
  "recommendation": "CPU stress is the primary driver — consider horizontal scaling"
}
```

### `GET /explain/{rupture_id}/formula`

Returns just the formula and coefficients (lighter response for dashboards).

---

## Auth

### `POST /auth/login`

```bash
curl -X POST http://localhost:8080/api/v2/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<jwt_secret>"}'
```

Returns `{"token":"eyJ...","expires_at":"..."}`.

### `POST /auth/refresh`

Exchange a valid JWT for a fresh one. Pass existing token in `Authorization` header.

---

## API Keys

### `GET /apikeys`

List all API keys for the current user.

### `POST /apikeys`

Create a new API key.

```bash
curl -X POST -H "Authorization: Bearer $JWT" \
  http://localhost:8080/api/v2/apikeys \
  -H "Content-Type: application/json" \
  -d '{"name":"ci-pipeline","scopes":["read","write"]}'
```

```json
{"id":"key_abc","name":"ci-pipeline","key":"ohe_abc123...","created_at":"..."}
```

The raw key value is only returned once.

### `DELETE /apikeys/{id}`

Revoke an API key.

---

## Self-Metrics (Prometheus)

### `GET /metrics`

Returns Ruptura's own Prometheus metrics (no auth required).

Key series:

```
rpt_rupture_index{host="web-01"}                  4.2
rpt_time_to_failure_seconds{host="web-01"}        1800
rpt_kpi_healthscore{host="web-01"}                43.2
rpt_kpi_stress{host="web-01"}                     0.72
rpt_actions_total{tier="1",result="ok"}           12
rpt_ingest_samples_total{source="prometheus"}     840200
rpt_ensemble_weight{host="web-01",model="ca_ilr"} 0.35
rpt_uptime_seconds                                3842
```

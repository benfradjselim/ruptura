# OHE v5.1.0 — API Reference

Base URL: `http://<host>:8080`

All `/api/v1/*` endpoints require `Authorization: Bearer <jwt>` or `X-API-Key: <key>` unless noted.
Multi-tenant requests add `X-Org-ID: <orgID>` header.

---

## Authentication

### POST /api/v1/auth/setup
First-run user creation (only available when zero users exist).

**Body**
```json
{ "username": "admin", "password": "changeme", "role": "admin" }
```
**Response 200**
```json
{ "token": "<jwt>", "expires_at": "2026-04-20T10:00:00Z" }
```

### POST /api/v1/auth/login
```json
{ "username": "admin", "password": "changeme" }
```
**Response 200**
```json
{ "token": "<jwt>", "expires_at": "2026-04-20T10:00:00Z" }
```

### POST /api/v1/auth/logout
Revokes the current JWT. No body.

### POST /api/v1/auth/refresh
Issues a new JWT without re-authenticating. No body; uses current Bearer token.

---

## Users (admin only)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/auth/users` | List all users |
| POST | `/api/v1/auth/users` | Create user |
| GET | `/api/v1/auth/users/{id}` | Get user |
| DELETE | `/api/v1/auth/users/{id}` | Delete user |
| PUT | `/api/v1/auth/users/{id}/org` | Assign user to org |

**Create user body**
```json
{ "username": "alice", "password": "secret", "role": "operator" }
```

---

## System

### GET /api/v1/health
Returns service version and uptime. No auth required.

**Response 200**
```json
{ "status": "ok", "version": "5.1.0", "uptime_seconds": 3600 }
```

### GET /api/v1/health/live
Liveness probe — 200 if process is alive.

### GET /api/v1/health/ready
Readiness probe — 200 if storage is open and ready.

### GET /api/v1/config
Returns effective runtime config (secrets redacted).

### GET /api/v1/openapi.yaml
Serves the OpenAPI 3.0 spec for this instance.

### GET /metrics
Prometheus text exposition — all KPIs emitted as gauges.

---

## Metrics

### GET /api/v1/metrics
List all known metric names.

**Response 200**
```json
["cpu_percent", "mem_percent", "latency_p99_ms"]
```

### GET /api/v1/metrics/{name}
Get the latest value for a metric across all hosts.

**Query params:** `host` (optional filter)

### GET /api/v1/metrics/{name}/range
Timeseries range query.

**Query params**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `host` | string | — | Filter by host |
| `start` | RFC3339 | now-1h | Range start |
| `end` | RFC3339 | now | Range end |
| `step` | string | `1m` | Aggregation step |

### GET /api/v1/metrics/{name}/aggregate
Returns `min`, `max`, `avg`, `p50`, `p95`, `p99` over a window.

**Query params:** `host`, `start`, `end`

---

## KPIs

### GET /api/v1/kpis
List all computed KPI names for the caller's org.

### GET /api/v1/kpis/multi
Batch fetch latest values for multiple KPIs.

**Query params:** `names` (comma-separated), `host`

**Response 200**
```json
{
  "stress": 0.42,
  "fatigue": 0.18,
  "mood": 0.58
}
```

### GET /api/v1/kpis/{name}
Latest value + history for a single KPI.

### GET /api/v1/kpis/{name}/predict
Forecast values at `t+5`, `t+15`, `t+30`, `t+60` minutes.

**Response 200**
```json
{
  "kpi": "stress",
  "predictions": [
    { "horizon_min": 5,  "value": 0.45, "lower": 0.38, "upper": 0.52 },
    { "horizon_min": 15, "value": 0.49, "lower": 0.40, "upper": 0.58 }
  ]
}
```

---

## Explainability

### GET /api/v1/explain/{kpi}
Returns XAI breakdown for the latest value of a KPI.

**Response 200**
```json
{
  "kpi": "stress",
  "value": 0.82,
  "contributors": [
    { "metric": "cpu_percent", "weight": 0.30, "value": 0.94, "contribution": 0.282 }
  ],
  "rupture_index": 3.4,
  "recommended_action": "scale_out"
}
```

---

## Alerts

### GET /api/v1/alerts
List alerts. Query params: `status` (`firing`|`resolved`|`silenced`), `host`, `limit`.

### GET /api/v1/alerts/{id}
Get single alert.

### DELETE /api/v1/alerts/{id}
Delete alert (operator+).

### POST /api/v1/alerts/{id}/acknowledge
Acknowledge a firing alert. No body.

### POST /api/v1/alerts/{id}/silence
Silence an alert for a duration.

**Body**
```json
{ "duration": "2h", "reason": "scheduled maintenance" }
```

---

## Alert Rules

### GET /api/v1/alert-rules
List all configured rules.

**Response 200**
```json
[{ "name": "high-stress", "kpi": "stress", "threshold": 0.7, "comparator": "gte", "severity": "HIGH_STRESS" }]
```

### POST /api/v1/alert-rules (operator+)
Create a rule.

**Body**
```json
{ "name": "high-stress", "kpi": "stress", "threshold": 0.7, "comparator": "gte", "severity": "HIGH_STRESS" }
```

### PUT /api/v1/alert-rules/{name} (operator+)
Update a rule.

### DELETE /api/v1/alert-rules/{name} (operator+)
Delete a rule.

---

## Dashboards

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/dashboards` | List all dashboards |
| POST | `/api/v1/dashboards` | Create dashboard |
| POST | `/api/v1/dashboards/import` (operator+) | Import JSON dashboard |
| GET | `/api/v1/dashboards/{id}` | Get dashboard |
| PUT | `/api/v1/dashboards/{id}` (operator+) | Update dashboard |
| DELETE | `/api/v1/dashboards/{id}` (operator+) | Delete dashboard |
| GET | `/api/v1/dashboards/{id}/export` | Export dashboard JSON |

---

## SLOs

### GET /api/v1/slos
List SLOs.

### POST /api/v1/slos (operator+)
Create SLO.

**Body**
```json
{
  "name": "api-availability",
  "kpi": "stress",
  "target": 99.9,
  "window": "30d",
  "comparator": "lte",
  "threshold": 0.7
}
```

### GET /api/v1/slos/status
All SLO statuses with burn-rate and error budget remaining.

### GET /api/v1/slos/{id}/status
Single SLO status.

### PUT /api/v1/slos/{id} (operator+)
Update SLO.

### DELETE /api/v1/slos/{id} (operator+)
Delete SLO.

---

## Notification Channels

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/notifications` | List channels |
| POST | `/api/v1/notifications` (operator+) | Create channel |
| GET | `/api/v1/notifications/{id}` | Get channel |
| PUT | `/api/v1/notifications/{id}` (operator+) | Update channel |
| DELETE | `/api/v1/notifications/{id}` (operator+) | Delete channel |
| POST | `/api/v1/notifications/{id}/test` (operator+) | Fire test notification |

**Create body (webhook)**
```json
{ "name": "ops-webhook", "type": "webhook", "url": "https://hooks.example.com/ohe", "enabled": true }
```

**Supported types:** `webhook`, `slack`, `pagerduty`, `email`

---

## DataSources

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/datasources` | List sources |
| POST | `/api/v1/datasources` (operator+) | Create source |
| GET | `/api/v1/datasources/{id}` | Get source |
| PUT | `/api/v1/datasources/{id}` (operator+) | Update source |
| DELETE | `/api/v1/datasources/{id}` (operator+) | Delete source |
| POST | `/api/v1/datasources/{id}/test` (operator+) | Test connectivity |
| POST | `/api/v1/datasources/{id}/proxy` | Proxy query to source |

---

## Orgs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/orgs` | List orgs |
| POST | `/api/v1/orgs` (operator+) | Create org |
| GET | `/api/v1/orgs/{id}` | Get org |
| PUT | `/api/v1/orgs/{id}` (operator+) | Update org |
| DELETE | `/api/v1/orgs/{id}` (operator+) | Delete org |
| GET | `/api/v1/orgs/{id}/members` | List members |
| POST | `/api/v1/orgs/{id}/members` (operator+) | Invite member |

**Quota fields on Org**
```json
{
  "quota": {
    "max_dashboards": 50,
    "max_datasources": 20,
    "max_api_keys": 10,
    "max_alert_rules": 100,
    "max_slos": 50
  }
}
```

---

## API Keys

### GET /api/v1/api-keys
List API keys (values redacted).

### POST /api/v1/api-keys (operator+)
```json
{ "name": "ci-key", "role": "operator", "expires_in": "90d" }
```
**Response 201**
```json
{ "id": "ak_xxx", "name": "ci-key", "key": "ohe_<secret>", "expires_at": "2026-07-19T00:00:00Z" }
```
The `key` field is shown only once.

### DELETE /api/v1/api-keys/{id} (operator+)

---

## Audit Log (admin only)

### GET /api/v1/audit
**Query params:** `from`, `to` (RFC3339), `actor`, `action`, `limit`

**Response 200**
```json
[{
  "id": "evt_xxx",
  "ts": "2026-04-19T10:00:00Z",
  "actor": "alice",
  "action": "dashboard.create",
  "resource": "db_yyy",
  "org_id": "default"
}]
```

---

## Logs

### GET /api/v1/logs
Query stored logs.

**Query params:** `service`, `level`, `start`, `end`, `q` (full-text), `limit`

### GET /api/v1/logs/stream
Server-Sent Events stream of live log entries. `Content-Type: text/event-stream`

---

## Traces / APM

### GET /api/v1/traces
Search traces. **Query params:** `service`, `operation`, `min_duration_ms`, `start`, `end`.

### GET /api/v1/traces/{traceID}
Full trace with all spans.

### GET /api/v1/topology
Service dependency graph derived from trace spans.

---

## Fleet

### GET /api/v1/fleet
Aggregate summary across all hosts — latest Stress, Fatigue, HealthScore per host.

---

## Ingest (operator+)

### POST /api/v1/ingest
Agent push endpoint. Accepts a batch of metric points.

**Body**
```json
{
  "host": "web-01",
  "metrics": [
    { "name": "cpu_percent", "value": 72.5, "ts": "2026-04-19T10:00:00Z" }
  ]
}
```

---

## Query (QQL)

### POST /api/v1/query
Run a QQL (Query Query Language) expression.

**Body**
```json
{ "query": "stress{host='web-01'} | range(1h) | avg" }
```

---

## Remote Write (Prometheus)

### POST /api/v1/write
Accepts Prometheus `WriteRequest` (snappy + protobuf). Use the Prometheus `remote_write` config block.

---

## OTLP

| Method | Path | Format |
|--------|------|--------|
| POST | `/otlp/v1/traces` | OTLP JSON or protobuf |
| POST | `/otlp/v1/metrics` | OTLP JSON or protobuf |
| POST | `/otlp/v1/logs` | OTLP JSON or protobuf |
| POST | `/opentelemetry/api/v1/traces` | Alias for OTLP traces |

---

## Loki-Compatible

| Method | Path | Description |
|--------|------|-------------|
| POST | `/loki/api/v1/push` | Push log streams (JSON) |
| GET | `/loki/api/v1/query_range` | Query logs with LogQL |
| GET | `/loki/api/v1/labels` | List label names |
| GET | `/loki/api/v1/label/{name}/values` | List values for a label |

---

## Elasticsearch-Compatible

| Method | Path | Description |
|--------|------|-------------|
| POST | `/_bulk` | Bulk index documents (ndjson) |
| POST | `/{index}/_bulk` | Bulk index with explicit index |
| GET | `/_cat/indices` | List indices |
| GET/POST | `/_search` | Search all indices |
| GET/POST | `/{index}/_search` | Search specific index |

---

## Datadog-Compatible

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/series` | Ingest Datadog metric series |
| POST | `/api/v2/logs` | Ingest Datadog log entries |

---

## Error Responses

All error responses follow this envelope:

```json
{ "error": "resource not found", "code": "NOT_FOUND" }
```

| HTTP Status | Meaning |
|-------------|---------|
| 400 | Bad request / validation error |
| 401 | Missing or invalid token |
| 402 | Quota exceeded |
| 403 | Insufficient role |
| 404 | Resource not found |
| 409 | Conflict (duplicate name) |
| 429 | Rate limit exceeded |
| 500 | Internal server error |

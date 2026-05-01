# API Reference

Base URL: `http://<host>:8080/api/v2`

All requests require `Authorization: Bearer <api-key>` unless noted. Set the key via `RUPTURA_API_KEY` when starting Ruptura.

---

## Health & Readiness

### `GET /health`

Returns server health. **No auth required.**

```bash
curl http://localhost:8080/api/v2/health
```

```json
{"status":"ok","rupture_detection":"active","uptime_seconds":3842}
```

### `GET /ready`

Kubernetes readiness probe. Returns `200` when ready, `503` during startup. **No auth required.**

### `GET /metrics`

Prometheus scrape endpoint — Ruptura's own self-metrics. **No auth required.**

---

## Ingest

### `POST /write`

Prometheus remote_write (protobuf). Used by Prometheus `remote_write` config.

```yaml
# prometheus.yml
remote_write:
  - url: http://ruptura:8080/api/v2/write
    authorization:
      credentials: <your-api-key>
```

### OTLP (metrics, logs, traces)

!!! important "OTLP goes to port 4317, not 8080"
    Send OTLP to `http://ruptura:4317` (separate OTLP HTTP server). Posting to `/api/v2/v1/{metrics,logs,traces}` on port 8080 returns `421 Misdirected Request` with port guidance — this is intentional.

```yaml
# otel-collector exporters section
exporters:
  otlphttp:
    endpoint: http://ruptura:4317
    headers:
      Authorization: "Bearer <your-api-key>"
```

---

## Rupture Index

### `GET /rupture/{namespace}/{workload}` _(primary — WorkloadRef)_

Get the Fused Rupture Index for a Kubernetes workload.

```bash
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/rupture/default/payment-api
```

```json
{
  "workload": {
    "namespace": "default",
    "kind": "Deployment",
    "name": "payment-api"
  },
  "fused_rupture_index": 4.2,
  "health_score": 43,
  "state": "critical",
  "stress":    { "value": 0.72, "state": "stressed" },
  "fatigue":   { "value": 0.81, "state": "burnout_imminent" },
  "mood":      { "value": 0.31, "state": "sad" },
  "pressure":  { "value": 0.65, "state": "storm_approaching" },
  "humidity":  { "value": 0.48, "state": "very_humid" },
  "contagion": { "value": 0.58, "state": "spreading" },
  "timestamp": "2026-05-01T09:00:00Z"
}
```

States: `stable` · `elevated` · `warning` · `critical` · `emergency`

### `GET /rupture/{host}` _(legacy — host-based)_

Backward-compatible host-level view. Works for non-Kubernetes ingest or when `k8s.*` OTLP attributes are absent.

### `GET /ruptures`

List the current Fused Rupture Index for every known workload.

```json
{
  "ruptures": [
    { "workload": { "namespace": "default", "kind": "Deployment", "name": "payment-api" }, "fused_rupture_index": 4.2, "state": "critical" },
    { "workload": { "namespace": "default", "kind": "Deployment", "name": "order-svc"  }, "fused_rupture_index": 1.1, "state": "elevated" }
  ]
}
```

---

## KPI Signals

10 signals available: `stress` · `fatigue` · `mood` · `pressure` · `humidity` · `contagion` · `resilience` · `entropy` · `velocity` · `health_score`

### `GET /kpi/{signal}/{namespace}/{workload}` _(primary)_

```bash
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/kpi/fatigue/default/payment-api
```

```json
{
  "signal": "fatigue",
  "workload": {
    "namespace": "default",
    "kind": "Deployment",
    "name": "payment-api"
  },
  "value": 0.81,
  "state": "burnout_imminent",
  "timestamp": "2026-05-01T09:00:00Z"
}
```

### `GET /kpi/{signal}/{host}` _(legacy)_

Host-based variant. Same response shape with `host` field instead of `workload`.

---

## Forecast

### `POST /forecast`

Request a forecast for a metric. Body: `{"metric": "cpu_usage", "workload": "default/payment-api", "horizon": 3600}`.

### `GET /forecast/{metric}/{namespace}/{workload}`

Get the cached forecast for a workload's metric.

### `GET /forecast/{metric}/{host}`

Legacy host-based forecast.

---

## Anomalies

### `GET /anomalies`

List anomaly events across all workloads. Optional query: `?since=<RFC3339>`.

```bash
curl -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v2/anomalies?since=2026-05-01T00:00:00Z"
```

```json
{
  "anomalies": [
    {
      "id": "anm_abc",
      "host": "payment-api",
      "method": "ca_ilr",
      "severity": "critical",
      "value": 4.8,
      "consensus": true,
      "timestamp": "2026-05-01T08:45:00Z"
    }
  ]
}
```

`consensus: true` means ≥2 detection methods agreed — high-confidence event.

### `GET /anomalies/{host}`

Anomalies for a specific host/workload.

---

## Actions

### `GET /actions`

List pending and recently executed actions.

```json
{
  "actions": [
    {
      "id": "act_abc",
      "workload": "default/Deployment/payment-api",
      "type": "scale",
      "params": {"replicas": 5},
      "tier": 2,
      "status": "pending",
      "rupture_id": "r_abc123",
      "created_at": "2026-05-01T08:45:00Z"
    }
  ]
}
```

### `GET /actions/{id}`

Get a single action by ID.

### `POST /actions/{id}/approve`

Approve a Tier-2 pending action.

```bash
curl -X POST -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/actions/act_abc/approve
```

### `POST /actions/{id}/reject`

Reject a pending action (removes it from the queue).

### `POST /actions/{id}/rollback`

Request a rollback of a previously executed action.

### `POST /actions/emergency-stop`

Immediately halt all Tier-1 automated actions globally.

```bash
curl -X POST -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/actions/emergency-stop
```

---

## Suppressions (Maintenance Windows)

Create time-bounded windows where rupture alerts are recorded but not dispatched to the action engine — use during planned deploys to avoid alert fatigue.

### `POST /suppressions`

```bash
curl -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "workload": "default/Deployment/payment-api",
    "start": "2026-05-01T14:00:00Z",
    "end": "2026-05-01T14:30:00Z",
    "reason": "rolling deploy v2.4.1"
  }' \
  http://localhost:8080/api/v2/suppressions
```

Optional `signals` array to suppress only specific signals (e.g. `["pressure","contagion"]`).

### `GET /suppressions`

List all active and upcoming suppression windows.

### `DELETE /suppressions/{id}`

Remove a suppression window early.

---

## Explainability

### `GET /explain/{rupture_id}`

Full XAI trace for a rupture.

### `GET /explain/{rupture_id}/formula`

Formula and coefficient breakdown (lighter response for dashboards).

### `GET /explain/{rupture_id}/pipeline`

Which pipeline (metric / log / trace) contributed most to the rupture.

### `GET /explain/{rupture_id}/narrative`

**Human-readable causal narrative.** Returns a structured English explanation — the primary differentiator.

```bash
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/explain/r_abc123/narrative
```

```json
{
  "narrative": "payment-api has been accumulating fatigue for 72h (fatigue 0.81, burnout threshold 0.80). The Tuesday 14:30 deploy increased pressure to 0.74. At 16:45, a contagion wave from payment-db propagated via the payment-api→payment-db call edge and pushed FusedR from 1.8 to 4.2 in 18 minutes. This is a cascade rupture, not an isolated spike. Recommended action: scale payment-api by 2 replicas.",
  "severity": "critical",
  "primary_pipeline": "metric",
  "top_factor": "fatigue",
  "ttf_seconds": 1800
}
```

---

## Context Events

Context entries inform Ruptura of deploy events, maintenance, or configuration changes. They suppress false alarms and anchor the timeline for explain narratives.

### `POST /context`

```bash
curl -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "workload": "default/Deployment/order-processor",
    "event": "deploy",
    "description": "v2.4.1 rolling deploy",
    "timestamp": "2026-05-01T14:30:00Z"
  }' \
  http://localhost:8080/api/v2/context
```

### `GET /context`

List recent context events.

### `DELETE /context/{id}`

Remove a context event.

---

## Self-Metrics (Prometheus)

### `GET /metrics`

Ruptura's own metrics in Prometheus format. **No auth required.**

Primary series (all workloads, all signals):

```
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="fatigue"} 0.81
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="stress"}  0.72
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="health_score"} 43.0
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="fused_rupture_index"} 4.2
```

Legacy host-labelled series (still emitted for backward compatibility):

```
rpt_rupture_index{host="payment-api",metric="cpu_usage",severity="critical"} 4.2
rpt_time_to_failure_seconds{host="payment-api",metric="cpu_usage"}           1800
rpt_kpi_healthscore{host="payment-api"}                                       43.0
rpt_kpi_stress{host="payment-api"}                                            0.72
rpt_kpi_fatigue{host="payment-api"}                                           0.81
rpt_actions_total{type="scale",tier="2",outcome="approved"}                   3
rpt_ingest_samples_total{source="prometheus"}                                 840200
rpt_memory_bytes                                                               45678900
rpt_uptime_seconds                                                             3842
rpt_version_info{version="6.2.2"}                                             1
```

Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: ruptura
    scrape_interval: 15s
    static_configs:
      - targets: ["ruptura:8080"]
    metrics_path: /api/v2/metrics
```

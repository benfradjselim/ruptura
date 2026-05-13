# Ingest Guide

Ruptura accepts telemetry from three sources: **Prometheus remote_write**, **OTLP over HTTP**, and **DogStatsD UDP**. All three feed the same analysis pipeline — you can mix them freely from different workloads.

---

## Quick reference

| Source | Protocol | Port | Path |
|--------|----------|------|------|
| Prometheus remote_write | HTTP POST (protobuf) | 8080 | `/api/v2/write` |
| OTLP metrics | HTTP POST (JSON) | 4317 | `/otlp/v1/metrics` |
| OTLP logs | HTTP POST (JSON) | 4317 | `/otlp/v1/logs` |
| OTLP traces | HTTP POST (JSON) | 4317 | `/otlp/v1/traces` |
| DogStatsD | UDP | 8125 | — |
| Loki push | HTTP POST (JSON) | 4317 | `/loki/api/v1/push` |
| Elasticsearch bulk | HTTP POST (JSON) | 4317 | `/_bulk` |

!!! important "OTLP format"
    Ruptura's OTLP endpoint accepts **JSON encoding only** (no protobuf) and **no compression** (no gzip). Always configure exporters with `encoding: json, compression: none`.

---

## 1. Prometheus remote_write

The simplest path for Kubernetes clusters that already run Prometheus.

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

remote_write:
  - url: http://ruptura.ruptura-system.svc.cluster.local:80/api/v2/write
    authorization:
      type: Bearer
      credentials: <your-api-key>
    queue_config:
      max_samples_per_send: 1000
      batch_send_deadline: 5s
```

Ruptura reads standard Kubernetes labels (`namespace`, `pod`, `deployment`) from the metric labels and maps them to workload identities automatically.

---

## 2. OpenTelemetry Collector

The recommended path for production — the OTel Collector handles buffering, retry, and fan-out.

### Full pipeline (metrics + logs + traces)

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc: {endpoint: 0.0.0.0:4317}
      http: {endpoint: 0.0.0.0:4318}
  prometheus:
    config:
      scrape_configs:
        - job_name: k8s
          kubernetes_sd_configs:
            - role: pod

processors:
  batch:
    timeout: 5s
  k8sattributes:
    extract:
      metadata: [k8s.namespace.name, k8s.deployment.name, k8s.pod.name]

exporters:
  otlphttp/ruptura:
    endpoint: http://ruptura.ruptura-system.svc.cluster.local:4317
    encoding: json          # required — Ruptura does not accept protobuf
    compression: none       # required — Ruptura does not accept gzip
    headers:
      Authorization: "Bearer <your-api-key>"

service:
  pipelines:
    metrics:
      receivers: [prometheus, otlp]
      processors: [k8sattributes, batch]
      exporters: [otlphttp/ruptura]
    logs:
      receivers: [otlp]
      processors: [k8sattributes, batch]
      exporters: [otlphttp/ruptura]
    traces:
      receivers: [otlp]
      processors: [k8sattributes, batch]
      exporters: [otlphttp/ruptura]
```

### Resource attributes Ruptura reads

Ruptura uses these OTLP resource attributes to group telemetry into workloads:

| Attribute | Used for |
|-----------|---------|
| `k8s.namespace.name` | Namespace (required for workload identity) |
| `k8s.deployment.name` | Maps to Deployment workload |
| `k8s.statefulset.name` | Maps to StatefulSet workload |
| `k8s.daemonset.name` | Maps to DaemonSet workload |
| `k8s.job.name` | Maps to Job workload |
| `service.name` | Fallback workload name when no k8s.* present |
| `host.name` | Node-level identity |

Multiple pods from the same Deployment are automatically merged into a single health view.

---

## 3. Direct OTLP/HTTP (curl / scripts)

You can send OTLP JSON directly with `curl` — useful for testing, CI pipelines, or custom ingest scripts.

### Metrics

```bash
T=$(date +%s%N)

curl -X POST http://<host>:4317/otlp/v1/metrics \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <api-key>" \
  -d '{
    "resourceMetrics": [{
      "resource": {"attributes": [
        {"key": "service.name",       "value": {"stringValue": "payment-api"}},
        {"key": "k8s.namespace.name", "value": {"stringValue": "production"}}
      ]},
      "scopeMetrics": [{"metrics": [
        {
          "name": "process.cpu.utilization",
          "gauge": {"dataPoints": [{
            "timeUnixNano": "'$T'",
            "asDouble": 0.85
          }]}
        },
        {
          "name": "process.memory.utilization",
          "gauge": {"dataPoints": [{
            "timeUnixNano": "'$T'",
            "asDouble": 0.72
          }]}
        },
        {
          "name": "http.server.request.duration",
          "gauge": {"dataPoints": [{
            "timeUnixNano": "'$T'",
            "asDouble": 1240
          }]}
        },
        {
          "name": "http.server.error.rate",
          "gauge": {"dataPoints": [{
            "timeUnixNano": "'$T'",
            "asDouble": 0.42
          }]}
        }
      ]}]
    }]
  }'
```

**Metric names that influence KPI signals:**

| Metric name | Signal affected |
|-------------|----------------|
| `process.cpu.utilization` | stress, pressure |
| `process.memory.utilization` | fatigue, pressure |
| `http.server.request.duration` | velocity, stress |
| `http.server.error.rate` | mood, resilience |
| `runtime.gc.pause.duration` | fatigue |
| `process.open_file_descriptors` | entropy |
| Any gauge metric | General signal input |

### Logs

```bash
T=$(date +%s%N)

curl -X POST http://<host>:4317/otlp/v1/logs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <api-key>" \
  -d '{
    "resourceLogs": [{
      "resource": {"attributes": [
        {"key": "service.name",       "value": {"stringValue": "payment-api"}},
        {"key": "k8s.namespace.name", "value": {"stringValue": "production"}}
      ]},
      "scopeLogs": [{"logRecords": [
        {
          "timeUnixNano": "'$T'",
          "severityNumber": 9,
          "severityText": "INFO",
          "body": {"stringValue": "Payment tx_9921 authorized, latency=340ms"}
        },
        {
          "timeUnixNano": "'$((T+200000000))'",
          "severityNumber": 17,
          "severityText": "ERROR",
          "body": {"stringValue": "DB connection pool exhausted: 20/20 active"},
          "attributes": [
            {"key": "db.pool.active", "value": {"intValue": 20}},
            {"key": "error.type",     "value": {"stringValue": "PoolExhausted"}}
          ]
        },
        {
          "timeUnixNano": "'$((T+400000000))'",
          "severityNumber": 21,
          "severityText": "FATAL",
          "body": {"stringValue": "Circuit breaker OPEN — rejecting all requests"},
          "attributes": [
            {"key": "cb.state", "value": {"stringValue": "open"}}
          ]
        }
      ]}]
    }]
  }'
```

**OTLP severity numbers:**

| Range | Level | Effect |
|-------|-------|--------|
| 1–4 | TRACE | ignored |
| 5–8 | DEBUG | ignored |
| 9–12 | INFO | positive signal (+mood) |
| 13–16 | WARN | negative signal (−mood) |
| 17–24 | ERROR / FATAL | strong negative signal (−mood, −resilience) |

**JSON value types:**  
Use `{"stringValue": "..."}` for strings, `{"intValue": 42}` (JSON number) for integers, `{"boolValue": true}` for booleans. Do **not** quote integers — `{"intValue": "42"}` is invalid.

### Traces

```bash
T=$(date +%s%N)
TRACE_ID=$(cat /proc/sys/kernel/random/uuid | tr -d '-')         # 32 hex chars
SPAN_ID=$(cat /proc/sys/kernel/random/uuid | tr -d '-' | cut -c1-16)  # 16 hex chars

curl -X POST http://<host>:4317/otlp/v1/traces \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <api-key>" \
  -d '{
    "resourceSpans": [{
      "resource": {"attributes": [
        {"key": "service.name",       "value": {"stringValue": "order-service"}},
        {"key": "k8s.namespace.name", "value": {"stringValue": "production"}}
      ]},
      "scopeSpans": [{"spans": [
        {
          "traceId":          "'$TRACE_ID'",
          "spanId":           "'$SPAN_ID'",
          "name":             "POST /orders",
          "kind":             2,
          "startTimeUnixNano": "'$T'",
          "endTimeUnixNano":   "'$((T+320000000))'",
          "status": {"code": 1},
          "attributes": [
            {"key": "http.method",      "value": {"stringValue": "POST"}},
            {"key": "http.status_code", "value": {"intValue": 201}}
          ]
        }
      ]}]
    }]
  }'
```

**Span status codes:**

| Code | Meaning | Effect on FusedR |
|------|---------|-----------------|
| 0 | UNSET | neutral |
| 1 | OK | positive |
| 2 | ERROR | traceR += (error_rate × 5.0) → raises FusedR |

Ruptura tracks error rate per service in a sliding window. When ≥10 spans have arrived or 15 seconds have passed, the window flushes and `traceR` is computed as `0.6×errorRate + 0.4×(avgLatencyMS/200)`. This feeds directly into the Fused Rupture Index.

---

## 4. DogStatsD

Ruptura listens for DogStatsD metrics on UDP port 8125 (same format as Datadog Agent).

```python
# Python — datadog library
from datadog import initialize, statsd

initialize(statsd_host='<ruptura-host>', statsd_port=8125)

statsd.gauge('payment.latency_ms', 1240, tags=['service:payment-api', 'namespace:production'])
statsd.gauge('payment.error_rate', 0.42, tags=['service:payment-api'])
statsd.increment('payment.requests_total', tags=['service:payment-api'])
```

```bash
# netcat — raw DogStatsD wire format
echo "payment.latency_ms:1240|g|#service:payment-api,namespace:production" \
  | nc -u -w1 <ruptura-host> 8125
```

---

## 5. Application SDKs

### Go

```go
import (
    "github.com/benfradjselim/ruptura/sdk/go/ruptura"
    "context"
)

client := ruptura.New(ruptura.Config{
    URL:    "http://ruptura:8080",
    APIKey: os.Getenv("RUPTURA_API_KEY"),
})

// Read workload health
snap, err := client.GetRupture(ctx, "production", "payment-api")
fmt.Printf("health=%.0f  fused_r=%.2f\n", snap.HealthScore.Value, snap.FusedRuptureIndex)
```

### Python

```python
import ruptura

client = ruptura.Client(
    url="http://ruptura:8080",
    api_key=os.environ["RUPTURA_API_KEY"]
)

snap = client.get_rupture("production", "payment-api")
print(f"health={snap.health_score.value:.0f}  fused_r={snap.fused_rupture_index:.2f}")
```

---

## 6. What Ruptura does with ingested data

```
OTLP metrics ──► Metric Pipeline (CA-ILR + ARIMA + HW + MAD + EWMA)
                      │
                      ▼
               10 KPI Signals ────────────────────────────┐
               stress · fatigue · mood · pressure          │
               humidity · contagion · resilience           │
               entropy · velocity · health_score           │
                                                           ▼
OTLP logs ────► Sentiment Engine (pos/neg word scoring)  Fusion Engine
OTLP traces ──► Span Window (error_rate × latency)     → FusedR Index
                                                           │
                                                           ▼
                                                  anomaly events
                                                  action engine
                                                  explain narrative
```

**FusedR thresholds:**

| Value | State | Dashboard colour |
|-------|-------|-----------------|
| < 1.5 | normal | green |
| 1.5–2.5 | warning | yellow — event emitted |
| 2.5–4.0 | critical | orange — action recommended |
| > 4.0 | emergency | red — auto-action if `autopilot` edition |

Once calibration completes (~25 minutes of data, 100 analyzer ticks), Ruptura switches from fixed thresholds to adaptive per-workload baselines (z-score deviation from Welford rolling mean).

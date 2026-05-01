# Quickstart

Get Ruptura running and observing your workloads in under 5 minutes.

## Step 1 — Start Ruptura

=== "Docker (fastest)"

    ```bash
    export RUPTURA_API_KEY=$(openssl rand -hex 32)

    docker run -d \
      --name ruptura \
      -p 8080:8080 \
      -p 4317:4317 \
      -v ruptura-data:/var/lib/ruptura/data \
      -e RUPTURA_API_KEY=$RUPTURA_API_KEY \
      ghcr.io/benfradjselim/ruptura:6.2.2
    ```

=== "Kubernetes / Helm"

    ```bash
    export RUPTURA_API_KEY=$(openssl rand -hex 32)

    helm install ruptura workdir/deploy/helm/ruptura \
      --namespace ruptura-system \
      --create-namespace \
      --set auth.apiKey=$RUPTURA_API_KEY

    kubectl port-forward svc/ruptura 8080:80 -n ruptura-system &
    ```

## Step 2 — Verify health

```bash
curl http://localhost:8080/api/v2/health
```

Expected response:

```json
{"status":"ok","rupture_detection":"active","uptime_seconds":3}
```

## Step 3 — Configure authentication

```bash
# Set for the rest of the session
export API_KEY=$RUPTURA_API_KEY
```

All subsequent requests need: `Authorization: Bearer $API_KEY`

## Step 4 — Send metrics

=== "Prometheus remote_write"

    Add to `prometheus.yml`:

    ```yaml
    remote_write:
      - url: http://localhost:8080/api/v2/write
        authorization:
          credentials: <your-api-key>
    ```

=== "OTLP (OTel Collector)"

    ```yaml
    exporters:
      otlphttp:
        endpoint: http://localhost:4317
        headers:
          Authorization: "Bearer <your-api-key>"
    ```

    Ruptura reads `k8s.namespace.name`, `k8s.deployment.name`, etc. from OTLP resource attributes and groups signals by Kubernetes workload automatically.

=== "curl (test)"

    ```bash
    # Prometheus remote_write with a test payload requires protobuf encoding.
    # For quick testing, Ruptura auto-generates state from synthetic load after startup.
    # Use the /api/v2/ruptures endpoint to see discovered workloads.
    curl -s -H "Authorization: Bearer $API_KEY" \
      http://localhost:8080/api/v2/ruptures | python3 -m json.tool
    ```

## Step 5 — Query the Fused Rupture Index

```bash
# All workloads
curl -s -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/ruptures | python3 -m json.tool

# Specific workload (namespace/name)
curl -s -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/rupture/default/payment-api | python3 -m json.tool
```

Example response:

```json
{
  "workload": {
    "namespace": "default",
    "kind": "Deployment",
    "name": "payment-api"
  },
  "fused_rupture_index": 1.8,
  "health_score": 74,
  "state": "fair",
  "stress": { "value": 0.52, "state": "nervous" },
  "fatigue": { "value": 0.67, "state": "exhausted" },
  "timestamp": "2026-05-01T09:00:00Z"
}
```

## Step 6 — Query KPI signals

```bash
# All 10 signals for a workload
for sig in stress fatigue mood pressure humidity contagion resilience entropy velocity health_score; do
  echo -n "$sig: "
  curl -s -H "Authorization: Bearer $API_KEY" \
    "http://localhost:8080/api/v2/kpi/$sig/default/payment-api" | \
    python3 -c "import sys,json; d=json.load(sys.stdin); print(f\"{d.get('value','?'):.3f} ({d.get('state','?')})\")"
done
```

## Step 7 — View anomaly events

```bash
# All workloads
curl -s -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/anomalies | python3 -m json.tool

# Filter by time window
curl -s -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v2/anomalies?since=2026-05-01T00:00:00Z"
```

## Step 8 — Read the narrative explain

```bash
# Get a rupture ID from /api/v2/ruptures, then:
curl -s -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/explain/<rupture_id>/narrative
```

Response example:

```json
{
  "narrative": "payment-api has been accumulating fatigue for 72h (fatigue 0.81, burnout threshold 0.80). The Tuesday 14:30 deploy increased pressure to 0.74. At 16:45, a contagion wave from payment-db — which entered epidemic state — propagated via the payment-api→payment-db call edge and pushed FusedR from 1.8 to 4.2 in 18 minutes. This is a cascade rupture, not an isolated spike. Recommended action: scale payment-api by 2 replicas and circuit-break the payment-db dependency.",
  "severity": "critical",
  "primary_pipeline": "metric",
  "top_factor": "fatigue",
  "ttf_seconds": 1800
}
```

## Step 9 — Create a maintenance window

Suppress alerts during a planned deploy:

```bash
curl -s -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "workload": "default/Deployment/payment-api",
    "start": "2026-05-01T14:00:00Z",
    "end": "2026-05-01T14:30:00Z",
    "reason": "planned rolling deploy"
  }' \
  http://localhost:8080/api/v2/suppressions
```

---

## Grafana

Point Grafana at Ruptura's Prometheus endpoint:

```
Datasource URL: http://ruptura:8080/api/v2/metrics
```

Import the bundled dashboard:

```bash
# File path: workdir/deploy/grafana/dashboards/ruptura_overview.json
# Or enable via Helm: --set grafana.dashboards.enabled=true
```

The dashboard shows: HealthScore per workload · Stress + Fatigue timeseries · Fused Rupture Index · Pressure + Contagion · Throughput Collapse · active action queue.

---

## Next steps

- [Configuration reference →](configuration.md)
- [API reference →](../api/reference.md)
- [Composite Signals →](../concepts/composite-signals.md)
- [Action engine →](../concepts/action-engine.md)

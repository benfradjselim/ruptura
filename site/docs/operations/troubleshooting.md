# Troubleshooting

## Ruptura won't start

**Symptom:** Container exits immediately or logs show a startup error.

```bash
# Docker
docker logs ruptura

# Kubernetes
kubectl logs -n ruptura-system -l app=ruptura --previous
```

**Common causes:**

| Error message | Fix |
|--------------|-----|
| `failed to open BadgerDB` | Storage path not writable — check PVC mount and `fsGroup` security context |
| `bind: address already in use` | Port 8080 or 4317 already in use — change `--port` or `--otlp-port` |
| `permission denied: /var/lib/ruptura/data` | `runAsUser: 65532` cannot write to volume — check `fsGroup: 65532` in pod security context |

No secret is required to start — `RUPTURA_API_KEY` is optional (auth disabled if unset, fine for dev).

---

## FusedR is always 0

**Symptom:** `GET /api/v2/rupture/{namespace}/{workload}` returns `fused_rupture_index: 0`.

**Cause:** FusedR requires ≥2 signal sources (metricR + logR or traceR). The burst ILR window also needs ≥20 samples (5 minutes at 15s intervals).

**Fix:**

1. Wait 5–10 minutes after starting ingest
2. Verify metrics are being received:
   ```bash
   curl http://localhost:8080/api/v2/metrics | grep rpt_ingest_samples_total
   # Should be > 0
   ```
3. Send OTLP logs or traces in addition to metrics to populate the second source

---

## No workloads appear in /api/v2/ruptures

**Symptom:** `GET /api/v2/ruptures` returns an empty list.

**Cause:** Ruptura needs OTLP telemetry with `k8s.namespace.name` and `k8s.deployment.name` resource attributes (or Prometheus remote_write with a `deployment` or `workload` label).

**Check:**

```bash
# Look for any known hosts (fall-back view)
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/ruptures | python3 -m json.tool

# Confirm OTLP is being sent to the right port
# OTLP → port 4317, NOT port 8080
# Posting to /api/v2/v1/* on port 8080 returns 421 Misdirected Request
```

Configure your OTel Collector:
```yaml
exporters:
  otlphttp:
    endpoint: http://ruptura:4317   # port 4317
```

---

## KPI signal returns 404

**Symptom:** `GET /api/v2/kpi/fatigue/default/payment-api` returns 404.

**Fix:** Verify the namespace and workload name match exactly what Ruptura observed. Names are case-sensitive.

```bash
# List all known workloads
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/ruptures
```

For legacy host-based queries:
```bash
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/kpi/fatigue/web-01
```

---

## Actions not firing in `auto` mode

**Symptom:** FusedR > 5.0 but no Tier-1 actions execute.

**Check in order:**

1. **`execution_mode`** — must be `auto`, not `shadow` or `suggest`
2. **`confidence_thresholds.auto_action`** — ensemble confidence must be ≥ 0.85
3. **`namespace_allowlist`** — target namespace must be in the allowlist (empty list = all blocked)
4. **Rate limit** — check `rpt_actions_total{tier="1"}` — if at `rate_limit_per_hour`, actions are gated
5. **Emergency stop** — check if `POST /api/v2/actions/emergency-stop` was called
6. **Suppression window** — check if the workload is in a maintenance window

```bash
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/actions

curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/suppressions
```

---

## Too many alerts during deploys

**Symptom:** Every rolling deploy triggers warning/critical rupture alerts.

**Fix:** Create a maintenance window (suppression) before the deploy:

```bash
curl -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "workload": "default/Deployment/my-service",
    "start": "2026-05-01T14:00:00Z",
    "end": "2026-05-01T14:30:00Z",
    "reason": "rolling deploy v2.4.1"
  }' \
  http://localhost:8080/api/v2/suppressions
```

During the window, ruptures are recorded but action dispatch is suppressed.

---

## False alarms from batch jobs

**Symptom:** A cron job or batch workload triggers high stress/fatigue alerts despite running normally.

**Cause:** Adaptive baselines require ~24h of observation (96 × 15s intervals) before activating. During the learning period, global thresholds apply and batch jobs look "stressed."

**Fix:** Wait for the observation window to complete. After 24h, Ruptura learns the workload's normal pattern and the alerts stop. You can also create a suppression window for the batch job's scheduled run time while baselines are being learned.

---

## High memory usage

Ruptura typical usage: **22 MB** idle, up to ~256 MB at scale.

**Causes:**

- High cardinality: each workload maintains 2 ILR windows and 12 signal histories
- Long retention: check `storage.retention.kpis_days`

---

## OTLP sending to wrong port

**Symptom:** Posting to `http://ruptura:8080/api/v2/v1/metrics` returns `421 Misdirected Request`.

**Fix:** This is correct behavior — OTLP goes to port **4317**, not 8080.

```yaml
# otel-collector exporters
exporters:
  otlphttp:
    endpoint: http://ruptura:4317   # correct
    # endpoint: http://ruptura:8080  # wrong — 421 response
```

---

## Logs

```bash
# Kubernetes
kubectl logs -n ruptura-system deployment/ruptura --follow

# Docker
docker logs -f ruptura

# Set debug log level
docker run -e RUPTURA_LOG_LEVEL=debug ...
# or: kubectl set env deployment/ruptura RUPTURA_LOG_LEVEL=debug -n ruptura-system
```

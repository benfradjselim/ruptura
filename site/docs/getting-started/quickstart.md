# Quickstart

Get Kairo running and see the Rupture Index in under 5 minutes.

## Step 1 — Start Kairo

```bash
docker run -d \
  --name kairo \
  -p 8080:8080 \
  -v kairo-data:/var/lib/kairo \
  -e KAIRO_JWT_SECRET=dev-secret-change-in-prod \
  kairo-core:6.1.0
```

## Step 2 — Verify health

```bash
curl http://localhost:8080/api/v2/health
```

Expected response:

```json
{"status":"ok","rupture_detection":"active","uptime_seconds":3}
```

## Step 3 — Create an API key

```bash
curl -s -X POST http://localhost:8080/api/v2/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"dev-secret-change-in-prod"}' \
  | python3 -m json.tool
```

Copy the returned JWT or use `X-API-Key` after creating a key via `/api/v2/apikeys`.

## Step 4 — Send metrics (Prometheus remote_write)

```bash
# Push a sample metric payload
curl -s -X POST http://localhost:8080/api/v2/write \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/x-protobuf" \
  --data-binary @sample.prw
```

Or configure your Prometheus to remote_write to Kairo:

```yaml
# prometheus.yml
remote_write:
  - url: http://kairo-core:8080/api/v2/write
    basic_auth:
      password: <your-api-key>
```

## Step 5 — Query the Rupture Index

```bash
API_KEY=<your-api-key>

# Get rupture index for a host
curl -s -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/rupture/web-01 | python3 -m json.tool
```

Sample response:

```json
{
  "host": "web-01",
  "rupture_index": 1.2,
  "state": "elevated",
  "time_to_failure_seconds": null,
  "dominant_signal": "stress"
}
```

## Step 6 — Query composite signals

```bash
# healthscore (0–100 product of stress, fatigue, pressure, contagion)
curl -s -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v2/kpi/healthscore/web-01"

# All 8 signals
for sig in stress fatigue pressure contagion resilience entropy sentiment healthscore; do
  echo -n "$sig: "
  curl -s -H "Authorization: Bearer $API_KEY" \
    "http://localhost:8080/api/v2/kpi/$sig/web-01" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('value','?'))"
done
```

## Step 7 — Explain a prediction

```bash
curl -s -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/explain/<rupture_id> | python3 -m json.tool
```

This returns the formula, contributing signals, and recommended action.

---

## Next steps

- [Configuration reference →](configuration.md)
- [API reference →](../api/reference.md)
- [Adaptive ensemble →](../concepts/surge-profiles.md)
- [Action engine →](../concepts/action-engine.md)

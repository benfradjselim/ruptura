# Troubleshooting

## Kairo won't start

**Symptom:** Container exits immediately or logs show a startup error.

**Check:**

```bash
# Docker
docker logs kairo

# Kubernetes
kubectl logs -n kairo-system -l app=kairo-core --previous
```

**Common causes:**

| Error message | Fix |
|--------------|-----|
| `jwt_secret is required` | Set `KAIRO_JWT_SECRET` env var or `auth.jwt_secret` in config |
| `failed to open BadgerDB` | Storage path not writable — check PVC/volume permissions |
| `bind: address already in use` | Port 8080 or 9090 already in use — change `ingest.http_port` or `ingest.grpc_port` |

---

## Rupture Index always 0

**Symptom:** `GET /api/v2/rupture/{host}` returns `rupture_index: 0` for all hosts.

**Cause:** Kairo needs at least 20 samples (5 minutes at 15 s intervals) before the burst ILR window is populated.

**Fix:** Wait 5–10 minutes after starting ingest, then check again.

**Also check:** Confirm metrics are actually being ingested:

```bash
curl http://localhost:8080/api/v2/metrics | grep kairo_ingest_samples_total
# Should be > 0
```

---

## No composite signal data

**Symptom:** `GET /api/v2/kpi/stress/web-01` returns `404` or empty value.

**Fix:** Verify the host name matches exactly what is being ingested. Host names are case-sensitive.

```bash
# List all known hosts
curl -H "Authorization: Bearer $KEY" http://localhost:8080/api/v2/ruptures | python3 -m json.tool
```

---

## Actions not firing in `auto` mode

**Symptom:** R > 5.0 but no Tier-1 actions are executed.

**Check in order:**

1. **`execution_mode`** — must be `auto`, not `shadow` or `suggest`
2. **`confidence_thresholds.auto_action`** — ensemble confidence must be ≥ 0.85
3. **`namespace_allowlist`** — target namespace must be in the allowlist
4. **Rate limit** — check `kairo_actions_total{tier="1"}` — if it's at `rate_limit_per_hour`, actions are gated
5. **Emergency stop** — check if `POST /api/v2/actions/emergency-stop` was called

```bash
curl -H "Authorization: Bearer $KEY" http://localhost:8080/api/v2/actions
```

---

## Kafka / NATS eventbus not receiving events

**Symptom:** `kairo.rupture.*` topic is empty.

**Check config:**

```bash
# Verify eventbus driver is set
grep -A5 eventbus /etc/kairo/kairo.yaml
```

**Check connectivity:**

```bash
# NATS
nats pub kairo.test "hello" --server nats://localhost:4222

# Kafka
kafka-topics.sh --list --bootstrap-server localhost:9092
```

**Logs:**

```bash
kubectl logs -n kairo-system -l app=kairo-core | grep eventbus
```

---

## High memory usage

**Symptom:** Kairo uses much more than expected memory.

Kairo typical usage: **22 MB** idle, up to ~256 MB at scale.

**Causes:**

- Too many hosts — each host maintains 2 ILR windows and 8 composite signal histories
- Retention too long — check `storage.retention.kpis_days`
- BadgerDB compaction backlog — trigger manually:

```bash
curl -X POST -H "Authorization: Bearer $KEY" http://localhost:8080/api/v2/admin/compact
```

---

## gRPC ingest returning RESOURCE_EXHAUSTED

**Symptom:** gRPC clients receive `RESOURCE_EXHAUSTED` status.

**Cause:** Internal metric queue is full — the metric pipeline cannot keep up with ingest rate.

**Fix:**

1. Reduce scrape interval on the sending side
2. Increase `ingest.grpc_queue_size` in config (default: 10,000)
3. Check if fusion engine is slow — look at `kairo_api_requests_total` latency

---

## Logs

```bash
# Kubernetes
kubectl logs -n kairo-system deployment/kairo-core --follow

# Docker
docker logs -f kairo

# Set log level to debug in config
# kairo.yaml: log_level: debug
# or env: KAIRO_LOG_LEVEL=debug
```

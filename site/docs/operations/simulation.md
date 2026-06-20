# Simulating Workloads

Ruptura ships two ways to inject synthetic workloads: the **Python workload simulator** (`scripts/simulate.py`) for continuous multi-profile injection, and the **ruptura-sim API** for one-shot failure pattern injection.

---

## Python workload simulator (v7+)

`scripts/simulate.py` injects 5 synthetic workloads with distinct failure modes continuously via the `/api/v2/write` endpoint. Use it to:

- Demo Ruptura immediately after install — no real workloads needed
- Test alert routing and action engine rules before go-live
- Validate that the dashboard shows differentiated health scores per workload
- Train your team on what each failure profile looks like in the dashboard

### Usage

```bash
python3 scripts/simulate.py [--host HOST] [--port PORT] [--interval SEC]

# Default target: http://<node-ip>:31470
# Default interval: 5s between pushes
```

### Workload profiles

| Workload | Profile | What it shows |
|----------|---------|---------------|
| `gateway` | Stable/healthy | All signals green — CPU ~22%, err ~0.3%, latency ~95ms |
| `order-service` | Slow-burn CPU stress | CPU rises from 45% → 90% over 10 minutes, latency and error rate climb with it |
| `payment-api` | Error bursts | Error rate spikes from 8% → 43% every 2 minutes, latency explodes to 3s during bursts |
| `cache-worker` | Traffic spikes | Request rate surges from 150 → 1350 rps every 3 minutes; throughput and CPU spike together |
| `ml-inference` | Noisy/calibrating | High variance on all signals — simulates a new workload in calibration phase |

### Metric format

Each workload pushes metrics as JSON to `/api/v2/write`. The `host` label must match the workload key `namespace/Kind/name`:

```json
{
  "timeseries": [{
    "Labels": [
      {"Name": "__name__",   "Value": "cpu_percent"},
      {"Name": "host",       "Value": "default/Deployment/order-service"},
      {"Name": "namespace",  "Value": "default"},
      {"Name": "deployment", "Value": "order-service"}
    ],
    "Samples": [{"Value": 78.3, "Timestamp": 1715780000000}]
  }]
}
```

!!! warning "host label is required"
    The ingest pipeline uses the `host` label as the pipeline key. If `host` is missing, all metrics merge into a single `"unknown"` workload. Always set `host` to `namespace/Kind/name`.

### What you'll see in the dashboard

After ~30 seconds of simulation:

1. **Fleet view** — 5 workload cards appear with different health rings and signal bars
2. **order-service** — stress and fatigue signals rising; calibration bar fills as baselines form
3. **payment-api** — mood signal low (error sentiment); FusedR spikes during burst cycles
4. **cache-worker** — velocity and throughput bars surge on the 3-minute sawtooth
5. **ml-inference** — card shows "calibrating" badge; signals fluctuate erratically
6. **gateway** — all green, stable health score near 95+

---

## ruptura-sim API patterns

Four built-in failure patterns inject via the REST API for one-shot demos:

| Pattern | What it simulates | Signals affected |
|---------|------------------|-----------------|
| `memory-leak` | Gradual RAM accumulation over 30 minutes | fatigue ↑, mood ↓, health_score ↓ |
| `cascade-failure` | Upstream dependency fails, errors propagate | contagion ↑↑, FusedR spikes |
| `traffic-surge` | 10× request rate spike over 5 minutes | stress ↑↑, pressure ↑, velocity ↑ |
| `slow-burn` | Low-grade degradation over 2 hours | fatigue ↑, entropy ↑, recovery_debt++ |

### Via API

```bash
curl -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "pattern": "cascade-failure",
    "host": "default/Deployment/payment-api",
    "duration_minutes": 15
  }' \
  http://<node-ip>:31468/api/v2/sim/inject
```

---

## Walkthrough: cascade failure demo

**1. Start the Python simulator (continuous background injection):**

```bash
python3 scripts/simulate.py --host <node-ip> --port 31470 &
```

**2. Watch the Fleet dashboard** at `http://<node-ip>:31469/` — after one 15s analyzer tick, all 5 workloads appear.

**3. Inject a cascade failure on top of the running simulator:**

```bash
curl -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"pattern":"cascade-failure","host":"default/Deployment/payment-api","duration_minutes":10}' \
  http://<node-ip>:31468/api/v2/sim/inject
```

**4. Watch the rupture index climb:**

```bash
watch -n 5 'curl -s \
  -H "Authorization: Bearer $API_KEY" \
  http://<node-ip>:31468/api/v2/rupture/default/payment-api \
  | python3 -m json.tool | grep -E "fused_rupture|health_score|contagion|state"'
```

You'll see `contagion` rise first, then `FusedR` cross 1.5 (warning), then 3.0 (critical). The action engine queues a Tier-2 recommendation.

**5. Read the narrative explain:**

```bash
# Get the rupture ID from the rupture response, then:
curl -s -H "Authorization: Bearer $API_KEY" \
  http://<node-ip>:31468/api/v2/explain/<rupture_id>/narrative \
  | python3 -m json.tool
```

**6. Check the pending action:**

```bash
curl -s -H "Authorization: Bearer $API_KEY" \
  http://<node-ip>:31468/api/v2/actions | python3 -m json.tool
```

---

## Tips

**Combine patterns** for realistic scenarios. Run the Python simulator for 10 minutes first (builds baselines), then inject `cascade-failure`. Ruptura shows the compounding effect.

**Watch `recovery_debt` accumulate** by repeatedly injecting patterns that recover just below FusedR 1.0. After a few rounds, `business.recovery_debt` in the snapshot reflects the hidden instability.

**ml-inference calibration** — this workload stays in "calibrating" state for ~24h of real data. The simulator sends enough variance that baselines never fully settle — showing the calibration UI state indefinitely.

---

## Endpoint reference

```
POST /api/v2/sim/inject

Body:
  pattern          string  — "memory-leak" | "cascade-failure" | "traffic-surge" | "slow-burn"
  host             string  — workload key (e.g. "default/Deployment/payment-api")
  duration_minutes int     — how long the pattern runs (default: 10)

Response: 202 Accepted
  { "injection_id": "sim_abc123", "pattern": "cascade-failure", "started_at": "..." }
```

---

## Lab Setup (Civo / k3s)

The fastest way to get a live cluster with realistic workloads:

```bash
export KUBECONFIG=~/civo-lab-ruptura-kubeconfig
bash lab-setup/setup.sh
```

This deploys:

| Component | Purpose |
|-----------|---------|
| Prometheus + kube-state-metrics | Scrapes test apps → remote-write to Ruptura |
| OpenTelemetry Collector | Receives OTLP from test apps → forwards to Ruptura |
| Ruptura community engine | Core engine on NodePort 31468 |
| Ruptura UI | Dashboard on NodePort 31469 |
| `gateway` | Stable, healthy (2 replicas) |
| `order-service` | Degraded — CPU stress + 8 MB/min memory leak |
| `payment-api` | At-risk — 18% error rate + dependency failure contagion |
| `cache-worker` | Spike — burst every 120s (4.5× CPU, 8× latency) |
| `ml-inference` | Calibrating — new workload, no baseline yet |
| `db-proxy` | Healthy — very low resource, stable |

After ~10 minutes you will see all 6 states in the Fleet dashboard simultaneously.

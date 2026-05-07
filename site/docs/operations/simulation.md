# Simulating Ruptures with `ruptura-sim`

`ruptura-sim` is a companion binary that injects synthetic load patterns into a running Ruptura instance. Use it to:

- Demo Ruptura without waiting for real incidents
- Test alert routing and action engine rules before go-live
- Reproduce a specific failure pattern to validate a fix
- Train your team on what a cascade failure looks like in the dashboard

---

## Installation

`ruptura-sim` is included in the same Docker image and built alongside the main binary.

**Docker:**

```bash
# Run sim against a local Ruptura container
docker run --rm --network host \
  ghcr.io/benfradjselim/ruptura:6.7.0 \
  /ruptura-sim --help
```

**From source:**

```bash
cd ruptura/workdir
go build -o ruptura-sim ./cmd/ruptura-sim
```

---

## Patterns

Four built-in failure patterns are available:

| Pattern | What it simulates | Signals affected |
|---------|------------------|-----------------|
| `memory-leak` | Gradual RAM accumulation over 30 minutes | fatigue ↑, mood ↓, health_score ↓ |
| `cascade-failure` | Upstream dependency starts failing, errors propagate | contagion ↑↑, FusedR spikes |
| `traffic-surge` | 10× request rate spike over 5 minutes | stress ↑↑, pressure ↑, velocity ↑ |
| `slow-burn` | Low-grade degradation over 2 hours — never critical alone | fatigue ↑, entropy ↑, recovery_debt++ |

---

## Usage

Inject a pattern via the REST API or the CLI binary.

### Via API (any HTTP client)

```bash
curl -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "pattern": "cascade-failure",
    "host": "payment-api",
    "duration_minutes": 15
  }' \
  http://localhost:8080/api/v2/sim/inject
```

### Via CLI binary

```bash
./ruptura-sim \
  --pattern=cascade-failure \
  --host=payment-api \
  --ruptura-url=http://localhost:8080 \
  --api-key=$API_KEY \
  --duration=15m
```

---

## Walkthrough: cascade failure demo

This is the recommended first demo for anyone evaluating Ruptura.

**1. Start Ruptura locally:**

```bash
docker run -d --name ruptura \
  -p 8080:8080 -p 4317:4317 \
  -e RUPTURA_API_KEY=demo \
  ghcr.io/benfradjselim/ruptura:6.7.0
```

**2. Inject a cascade failure:**

```bash
curl -X POST \
  -H "Authorization: Bearer demo" \
  -H "Content-Type: application/json" \
  -d '{"pattern":"cascade-failure","host":"payment-api","duration_minutes":10}' \
  http://localhost:8080/api/v2/sim/inject
```

**3. Watch the Rupture Index climb:**

```bash
watch -n 5 'curl -s -H "Authorization: Bearer demo" \
  http://localhost:8080/api/v2/rupture/payment-api \
  | python3 -m json.tool | grep -E "fused_rupture|health_score|contagion|state"'
```

You will see `contagion` rise first, then `FusedR` cross 1.5 (warning), then 3.0 (critical). The action engine will queue a Tier-2 recommendation.

**4. Read the narrative explain:**

```bash
# Get the rupture ID from the rupture response, then:
curl -s -H "Authorization: Bearer demo" \
  http://localhost:8080/api/v2/explain/<rupture_id>/narrative \
  | python3 -m json.tool
```

Expected output:
```
"payment-api has been accumulating contagion from upstream dependencies for 8 minutes.
Error propagation via the payment-api→payment-db edge pushed FusedR from 1.2 to 4.1.
This is a cascade rupture, not an isolated spike. Recommended action: scale payment-api
by 2 replicas and investigate payment-db error rate."
```

**5. Check the pending action:**

```bash
curl -s -H "Authorization: Bearer demo" \
  http://localhost:8080/api/v2/actions \
  | python3 -m json.tool
```

---

## Tips

**Combine patterns** for realistic scenarios. Inject `slow-burn` for 30 minutes first, then `traffic-surge` — Ruptura will show the compounding effect of a fatigued workload hitting a traffic spike.

**Use a real workload name** matching your K8s namespace/workload key (e.g. `--host=default/Deployment/checkout`) so the simulated signals merge with real telemetry in the dashboard.

**Watch `recovery_debt` accumulate** by repeatedly injecting `slow-burn` patterns that recover below FusedR 1.0. After a few rounds, the `business.recovery_debt` counter in the snapshot will reflect the hidden instability.

---

## Endpoint reference

```
POST /api/v2/sim/inject

Body:
  pattern          string  — "memory-leak" | "cascade-failure" | "traffic-surge" | "slow-burn"
  host             string  — workload key (e.g. "payment-api" or "payments/Deployment/checkout")
  duration_minutes int     — how long the pattern runs (default: 10)

Response: 202 Accepted
  { "injection_id": "sim_abc123", "pattern": "cascade-failure", "started_at": "..." }
```

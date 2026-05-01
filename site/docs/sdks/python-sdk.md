# Python SDK

The Python SDK is published as `ruptura-client` on PyPI.

## Install

```bash
pip install ruptura-client
```

Requires Python 3.9+. Runtime dependency: `requests>=2.28`.

## Create a client

```python
from ruptura import RupturaClient

c = RupturaClient("http://ruptura:8080", api_key="your-api-key")

# Custom timeout
c = RupturaClient("http://ruptura:8080", api_key="your-api-key", timeout=10.0)
```

## Health check

```python
health = c.health()
print(health["status"])   # "ok"
```

## Rupture Index (WorkloadRef — primary)

```python
# Kubernetes workload: namespace + name
rupture = c.rupture_index("default", "payment-api")
print(f"FusedR={rupture['fused_rupture_index']:.2f}  state={rupture['state']}")
print(f"health_score={rupture['health_score']}")
```

## All ruptures

```python
ruptures = c.ruptures()
for r in ruptures:
    wl = r["workload"]
    print(f"{wl['namespace']}/{wl['name']}: FusedR={r['fused_rupture_index']:.2f}  state={r['state']}")
```

## KPI signals

```python
# Any of: stress, fatigue, mood, pressure, humidity, contagion,
#         resilience, entropy, velocity, health_score
kpi = c.kpi("fatigue", "default", "payment-api")
print(f"fatigue={kpi['value']:.2f}  state={kpi['state']}")

hs = c.kpi("health_score", "default", "payment-api")
print(f"health_score={hs['value']:.1f}")
```

## Narrative explain

```python
narrative = c.explain_narrative("r_abc123")
print(narrative["narrative"])
# "payment-api has been accumulating fatigue for 72h..."
print(f"severity={narrative['severity']}  top_factor={narrative['top_factor']}")
```

## Anomalies

```python
# All anomalies
anomalies = c.anomalies()

# For a specific host/workload
anomalies = c.anomalies("payment-api")
for a in anomalies:
    print(f"{a['host']}: method={a['method']}  severity={a['severity']}  consensus={a['consensus']}")
```

## Actions

```python
# List pending actions
actions = c.list_actions()

# Approve
c.approve_action("act_abc")

# Reject
c.reject_action("act_abc")

# Emergency stop
c.emergency_stop()
```

## Maintenance windows

```python
from datetime import datetime, timedelta, timezone

now = datetime.now(timezone.utc)
c.create_suppression({
    "workload": "default/Deployment/order-processor",
    "start": now.isoformat(),
    "end": (now + timedelta(minutes=30)).isoformat(),
    "reason": "rolling deploy v2.4.1"
})
```

## Error handling

```python
from ruptura.exceptions import RupturaError

try:
    rupture = c.rupture_index("default", "unknown-svc")
except RupturaError as e:
    print(f"HTTP {e.status_code}: {e}")
```

## Async usage

The client is synchronous (`requests`-based). For async usage:

```python
import asyncio
from ruptura import RupturaClient

c = RupturaClient("http://ruptura:8080", api_key="your-api-key")

async def get_rupture(namespace: str, workload: str):
    return await asyncio.to_thread(c.rupture_index, namespace, workload)
```

## Client reference

| Method | Description |
|--------|-------------|
| `health()` | Server health status |
| `rupture_index(namespace, workload)` | Fused Rupture Index for a workload |
| `ruptures()` | All active ruptures |
| `kpi(signal, namespace, workload)` | KPI signal value |
| `explain_narrative(rupture_id)` | Human-readable causal narrative |
| `anomalies(host="")` | Anomaly events (all or filtered by host) |
| `list_actions()` | Pending / recent actions |
| `approve_action(id)` | Approve a Tier-2 action |
| `reject_action(id)` | Reject a pending action |
| `emergency_stop()` | Halt all Tier-1 auto-actions |
| `create_suppression(window)` | Create maintenance window |
| `list_suppressions()` | List active suppressions |
| `delete_suppression(id)` | Remove a suppression |

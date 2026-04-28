# Python SDK

The Python SDK is published as `ruptura-client` on PyPI.

## Install

```bash
pip install ruptura-client
```

Requires Python 3.9+. The only runtime dependency is `requests>=2.28`.

## Create a client

```python
from ruptura import KairoClient

# API key auth (recommended for services)
c = KairoClient("http://ruptura:8080", api_key="ohe_abc123")

# Custom timeout
c = KairoClient("http://ruptura:8080", api_key="ohe_abc123", timeout=10.0)
```

## Health check

```python
health = c.health()
print(health["status"])   # "ok"
```

## Rupture Index

```python
rupture = c.rupture_index("web-01")
print(f"R={rupture['rupture_index']:.2f}  state={rupture['state']}")
```

## Composite signals

```python
# Any of: stress, fatigue, pressure, contagion,
#         resilience, entropy, sentiment, healthscore
kpi = c.kpi("stress", "web-01")
print(f"stress={kpi['value']:.2f}  state={kpi['state']}")

hs = c.kpi("healthscore", "web-01")
print(f"healthscore={hs['value']:.1f}")
```

## All ruptures

```python
ruptures = c.ruptures()
for r in ruptures:
    print(r["host"], r["state"], r["rupture_index"])
```

## Ingest metrics

```python
c.ingest_metrics([
    {"name": "cpu_usage", "value": 0.72, "host": "web-01"},
    {"name": "mem_usage", "value": 0.45, "host": "web-01"},
])
```

## Actions

```python
# List pending actions
actions = c.list_actions()

# Approve
c.approve_action("act_abc")

# Emergency stop
c.emergency_stop()
```

## Error handling

```python
from ruptura.exceptions import KairoError

try:
    rupture = c.rupture_index("unknown-host")
except RupturaError as e:
    print(f"HTTP {e.status_code}: {e}")
```

## Async usage

The client is synchronous (`requests`-based). For async usage, wrap calls with `asyncio.to_thread`:

```python
import asyncio
from ruptura import KairoClient

c = KairoClient("http://ruptura:8080", api_key="ohe_abc123")

async def get_rupture(host: str):
    return await asyncio.to_thread(c.rupture_index, host)
```

## Client reference

| Method | Description |
|--------|-------------|
| `health()` | Server health status |
| `ready()` | Readiness probe (raises on not-ready) |
| `rupture_index(host)` | Rupture Index for a host |
| `ruptures()` | All active ruptures |
| `kpi(signal, host)` | Composite signal value |
| `ensemble_weights(host)` | Model weights (v6.1) |
| `ingest_metrics(metrics)` | Push raw metrics |
| `list_actions()` | Pending / recent actions |
| `approve_action(id)` | Approve a Tier-2 action |
| `emergency_stop()` | Halt all Tier-1 auto-actions |

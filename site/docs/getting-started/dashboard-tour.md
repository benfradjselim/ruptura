# Dashboard Tour

Ruptura v7 ships a Svelte 4 SPA served by a dedicated `ruptura-ui` pod at `http://<host>:31469`. The UI connects to the `ruptura-engine` REST API via an nginx reverse proxy — no Grafana, no external tools.

---

## Fleet overview

The Fleet view is your starting point — a live grid of every tracked workload.

**What each card shows:**

| Area | What it shows |
|------|--------------|
| **Health ring** | Actual KPI value (0–100) with colour: green → yellow → red as health degrades |
| **10 signal mini-bars** | One bar per KPI signal (Stress, Fatigue, Mood, Pressure, Humidity, Contagion, Resilience, Entropy, Velocity, Throughput) |
| **FusedR badge** | Current Fused Rupture Index — the composite risk score |
| **Calibration progress** | `calibrating` badge appears during the first data-collection window; rupture predictions are suppressed until calibration completes |
| **Rupture warning** | `⚠ Critical in ~12m` when HealthScore forecast is heading toward critical |

The Fleet header shows the live rupture counter, updating over SSE without polling.

---

## KPI signals — per-workload detail

Click any workload card to open the detail panel. The **Signals** tab shows all 10 KPI gauges:

| Signal | Measures |
|--------|----------|
| **Stress** | CPU + latency burst |
| **Fatigue** | Cumulative baseline deviation (long-term wear) |
| **Mood** | Log error/warn sentiment ratio |
| **Pressure** | Memory + disk saturation |
| **Humidity** | Forecast variance — how predictable behavior is |
| **Contagion** | Error propagation from upstream services |
| **Resilience** | Recovery speed after spikes |
| **Entropy** | Internal signal disorder |
| **Velocity** | Request rate acceleration |
| **Throughput** | Data volume processed per cycle |

Each signal shows `value`, `state` (ok / warning / critical), and `trend` (rising / falling / stable).

Also visible on this tab: **PatternMatch** warning (cosine similarity ≥ 0.85 against prior rupture fingerprints), **Business Signals** (`slo_burn_velocity`, `blast_radius`, `recovery_debt`), and the **Explain** panel with narrative + formula breakdown.

---

## History tab

Switch to **History** to see how signals evolved over time. Toggle any of the 12 signal chips to overlay them on the Chart.js time-series. Useful for spotting the exact moment a cascade took hold versus a slow-burn pattern building gradually.

---

## Forecast tab

The **Forecast** tab projects HealthScore **+15 and +30 minutes** forward using the 5-model ensemble (CA-ILR, ARIMA, Holt-Winters, MAD, EWMA). When the projected score is heading toward critical, `critical_eta_minutes` is surfaced — the card shows "⚠ Critical in ~12m".

---

## Predictions tab

The **Predictions** tab shows per-metric ensemble outputs — individual model votes and the current model weights. Weights are re-computed every 60s based on actual prediction error.

---

## Events tab

Live SSE rupture/recovery log for the workload. Every FusedR threshold crossing fires in real time.

---

## Actions tab

Approve or reject pending Tier-2 actions (suggested by the action engine when FusedR is 3.0–5.0). Tier-1 actions (FusedR ≥ 5.0) execute automatically subject to safety gates.

---

## Kubernetes tab

Pod list, replica count, resource requests/limits, and labels for the workload's underlying Kubernetes resources.

---

## Topology view

The **Topology** page renders the service dependency graph derived from OTLP traces.

- **Click a node** → health bar + current FusedR for that service
- **Click an edge** → call rate, error rate, P99 latency for that service-to-service link

The graph updates as new trace data arrives — new services and edges appear automatically.

---

## Engine view

The **Engine** page shows Ruptura's own runtime stats:

- Analyzer state (last tick time, workload count)
- Ingest rates (metrics/s, logs/s, traces/s)
- Cumulative data flow counters
- BadgerDB storage usage and GC stats

---

## Cluster / Nodes view

The **Nodes** page shows Kubernetes node health — CPU, memory, and disk pressure per node, sourced from the k8smetrics poller.

---

## Alerts view

Active and resolved alert feed across all workloads, with severity, timestamp, and the FusedR value at trigger time.

---

## Settings

### Datasources tab

Register and test data sources:

- **Prometheus** — remote-write endpoint configuration, namespace scoping
- **OTLP** — configure the push endpoint (TCP connectivity test); incoming signals are attributed to the configured namespace

### Ingest Stats tab

Live totals: metrics received, logs received, traces received, parse errors, active workloads.

### Database tab

Per-signal-type data retention configuration (days). Purge controls:

- **Purge by type** — delete all data for a specific signal type
- **Purge by date** — delete data older than a given date
- **Purge all** — full data reset (requires confirmation)

---

## Quick install

Deploy Ruptura on Kubernetes and inject synthetic workloads in under two minutes:

```bash
helm install ruptura oci://ghcr.io/benfradjselim/charts/ruptura \
  --namespace ruptura-system \
  --create-namespace \
  --set apiKey=$(openssl rand -hex 32)

# Dashboard:   http://<node-ip>:31469/
# Engine API:  http://<node-ip>:31468/api/v2/health
# OTLP ingest: http://<node-ip>:31470/api/v2/write
```

Inject the five built-in synthetic workload profiles immediately:

```bash
python3 scripts/simulate.py
# Sends 5 workloads every 5s:
#   gateway        — stable/healthy
#   order-service  — slow-burn CPU stress
#   payment-api    — error bursts every 2 min
#   cache-worker   — traffic spikes every 3 min
#   ml-inference   — noisy/calibrating new workload
```

→ Full reference: [Architecture →](../architecture/overview.md)

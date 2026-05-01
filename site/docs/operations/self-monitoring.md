# Self Monitoring

Ruptura exports Prometheus metrics about its own health and performance. Scrape at `GET /api/v2/metrics` (no auth required).

## Primary metric: ruptura_kpi

The main workload-level gauge — one series per signal per workload:

```
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="stress"}              0.72
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="fatigue"}             0.81
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="mood"}                0.31
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="pressure"}            0.65
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="humidity"}            0.48
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="contagion"}           0.58
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="resilience"}          0.42
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="entropy"}             0.33
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="velocity"}            0.21
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="health_score"}        43.0
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="fused_rupture_index"} 4.2
ruptura_kpi{namespace="default",kind="Deployment",workload="payment-api",signal="throughput"}          0.65
```

Use this metric for Grafana dashboards and Alertmanager rules.

## Legacy host-labelled metrics

Still emitted for backward compatibility with existing dashboards:

| Metric | Type | Labels |
|--------|------|--------|
| `rpt_rupture_index` | Gauge | `host, metric, severity` |
| `rpt_time_to_failure_seconds` | Gauge | `host, metric` |
| `rpt_predicted_value` | Gauge | `host, metric, horizon` |
| `rpt_confidence` | Gauge | `host` |
| `rpt_kpi_stress` | Gauge | `host` |
| `rpt_kpi_fatigue` | Gauge | `host` |
| `rpt_kpi_healthscore` | Gauge | `host` |
| `rpt_actions_total` | Counter | `type, tier, outcome` |
| `rpt_ingest_samples_total` | Counter | `source` |
| `rpt_memory_bytes` | Gauge | — |
| `rpt_uptime_seconds` | Gauge | — |
| `rpt_version_info` | Gauge | `version` |

## Prometheus scrape config

```yaml
scrape_configs:
  - job_name: ruptura-self
    scrape_interval: 15s
    static_configs:
      - targets: ["ruptura:8080"]
    metrics_path: /api/v2/metrics
```

No auth required on `/api/v2/metrics`.

## Useful Grafana queries

```promql
# Workloads with Fused Rupture Index above warning threshold
ruptura_kpi{signal="fused_rupture_index"} > 1.5

# Average health score across all workloads
avg(ruptura_kpi{signal="health_score"})

# Most fatigued workloads (top 5)
topk(5, ruptura_kpi{signal="fatigue"})

# Workloads in burnout (fatigue > 0.8)
ruptura_kpi{signal="fatigue"} > 0.8

# Contagion spreading (trending up)
delta(ruptura_kpi{signal="contagion"}[10m]) > 0.1

# Actions fired per hour
rate(rpt_actions_total[1h]) * 3600

# Ingest throughput
rate(rpt_ingest_samples_total[5m])
```

## Recommended alerts

```yaml
- alert: RupturaDown
  expr: absent(rpt_uptime_seconds)
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Ruptura is unreachable — rupture detection offline"

- alert: RupturaHighActionRate
  expr: rate(rpt_actions_total{tier="1"}[10m]) * 600 > 3
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Ruptura firing >3 Tier-1 actions per 10 min — verify safety gates"

- alert: RupturaHighMemory
  expr: rpt_memory_bytes > 450000000
  labels:
    severity: warning
  annotations:
    summary: "Ruptura memory approaching 512MB limit"
```

## The Grafana dashboard

The bundled dashboard (`deploy/grafana/dashboards/ruptura_overview.json`) includes:

- **Panel 1**: HealthScore gauge per workload (0–100, green/yellow/red bands)
- **Panel 2**: Stress + Fatigue time series overlay
- **Panel 3**: Fused Rupture Index per workload (queries `ruptura_kpi{signal="fused_rupture_index"}`)
- **Panel 4**: Pressure + Contagion time series
- **Panel 5**: Anomaly event annotations
- **Panel 6**: Throughput Collapse signal

Enable via Helm: `--set grafana.dashboards.enabled=true` (creates a ConfigMap with `grafana_dashboard: "1"` label for auto-import).

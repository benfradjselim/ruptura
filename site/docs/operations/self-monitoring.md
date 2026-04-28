# Self Monitoring

Kairo exports 14 Prometheus metric series about its own health and performance. Scrape at `GET /api/v2/metrics`.

## Exported metrics

| Metric | Type | Description |
|--------|------|-------------|
| `kairo_rupture_index{host}` | Gauge | Current Rupture Index per host |
| `kairo_time_to_failure_seconds{host}` | Gauge | Predicted seconds until failure (NaN if stable) |
| `kairo_kpi_healthscore{host}` | Gauge | Health score 0–100 per host |
| `kairo_kpi_stress{host}` | Gauge | Stress signal 0–1 per host |
| `kairo_kpi_fatigue{host}` | Gauge | Fatigue signal 0–1 per host |
| `kairo_kpi_pressure{host}` | Gauge | Pressure signal 0–1 per host |
| `kairo_kpi_contagion{host}` | Gauge | Contagion signal 0–1 per host |
| `kairo_ensemble_weight{host,model}` | Gauge | Per-model ensemble weight (v6.1) |
| `kairo_actions_total{tier,result}` | Counter | Actions executed by tier and outcome |
| `kairo_ingest_samples_total{source}` | Counter | Samples ingested per source |
| `kairo_storage_bytes` | Gauge | BadgerDB on-disk size |
| `kairo_storage_kpis_total` | Gauge | Total KPI records stored |
| `kairo_uptime_seconds` | Counter | Seconds since start |
| `kairo_api_requests_total{method,path,status}` | Counter | REST API request count |

## Example Grafana queries

```promql
# Hosts with Rupture Index above warning threshold
kairo_rupture_index > 1.5

# Average health score across all hosts
avg(kairo_kpi_healthscore)

# Ensemble weight drift — how much CA-ILR is dominating
kairo_ensemble_weight{model="ca_ilr"}

# Actions per hour (Tier-1)
rate(kairo_actions_total{tier="1"}[1h]) * 3600

# Ingest throughput
rate(kairo_ingest_samples_total[5m])

# API error rate
rate(kairo_api_requests_total{status=~"5.."}[5m])
  / rate(kairo_api_requests_total[5m])
```

## Prometheus scrape config

```yaml
scrape_configs:
  - job_name: kairo-self
    scrape_interval: 15s
    static_configs:
      - targets: ["kairo-core:8080"]
    metrics_path: /api/v2/metrics
    bearer_token: "<api-key>"
```

## Recommended alerts

```yaml
- alert: KairoDown
  expr: up{job="kairo-self"} == 0
  for: 1m
  labels:
    severity: critical

- alert: KairoHighActionRate
  expr: rate(kairo_actions_total{tier="1"}[10m]) * 600 > 3
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Kairo is executing Tier-1 actions at >3 per 10 min — check safety gates"

- alert: KairoStorageHigh
  expr: kairo_storage_bytes > 8e9
  labels:
    severity: warning
  annotations:
    summary: "Kairo BadgerDB storage approaching 10 GB limit"
```

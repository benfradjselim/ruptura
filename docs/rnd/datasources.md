# Data Sources & Ingestion

OHE acts as a central observability hub, capable of collecting data from various sources and proxying queries to external telemetry systems.

## Data Ingestion
OHE supports native ingestion protocols:
- **OHE Agent:** DaemonSet on nodes pushing to `/api/v1/ingest`.
- **OTLP:** Standard OpenTelemetry HTTP/gRPC ingestion for traces, metrics, and logs.
- **Loki:** Compatible ingestion for log streams (`/loki/api/v1/push`).
- **Elasticsearch:** Bulk API compatibility for log ingestion (`/_bulk`).
- **Datadog:** Compatibility for Datadog Agent metrics and logs.
- **DogStatsD:** UDP listener for lightweight metric pushes.

## External Data Sources
OHE can be configured to interact with external telemetry systems:
- **Registration:** Datasources are registered via `/api/v1/datasources`.
- **PromQL Proxy:** OHE can proxy PromQL queries to external systems like Prometheus, Thanos, or VictoriaMetrics using `/api/v1/datasources/{id}/proxy`.
- **SSRF Hardening:** Trusted datasource hosts must be defined in `OHE_TRUSTED_DATASOURCE_HOSTS` to prevent SSRF vulnerabilities.

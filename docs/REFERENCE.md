# Engine configuration

The operator sets these from a `RupturaInstance`. Set them yourself only when you run the engine without the operator.

| Variable | Effect |
|---|---|
| `RUPTURA_THANOS_URL` | Prometheus or Thanos query endpoint. Without it only `crash_loop` is reported. |
| `RUPTURA_API_KEY` | API key required on `/api/v2` requests, except probes. |
| `RUPTURA_LEVEL_SCOPE` | `pod`, `workload` or `namespace`: how results are reported. |
| `RUPTURA_LEVEL_SELECTOR` | `key=value,key=value`: only matching workloads are reported and alerted. |
| `RUPTURA_INPUT_PROFILE` | `minimal` or `standard`: how much telemetry is collected. |
| `RUPTURA_SENSITIVITY` | `conservative`, `balanced` or `aggressive`. |
| `RUPTURA_HORIZON` | Forecast horizon, for example `30m`. |
| `RUPTURA_PROMETHEUS_AUTH` | `serviceAccount` (default), `bearer`, `basic` or `none`. |
| `RUPTURA_PROMETHEUS_CA_FILE`, `RUPTURA_PROMETHEUS_CERT_FILE`, `RUPTURA_PROMETHEUS_KEY_FILE` | Extra CA and client certificate for Prometheus. |
| `RUPTURA_PROMETHEUS_SERVER_NAME` | Name checked in the Prometheus certificate. |
| `RUPTURA_PROMETHEUS_TOKEN_FILE`, `RUPTURA_PROMETHEUS_USERNAME_FILE`, `RUPTURA_PROMETHEUS_PASSWORD_FILE` | Credentials, read at each request. |
| `RUPTURA_OTLP_TLS_CERT_FILE`, `RUPTURA_OTLP_TLS_KEY_FILE` | Serve OTLP over TLS. The engine does not start if they cannot be loaded. |
| `RUPTURA_OTLP_TLS_CLIENT_CA_FILE` | Require OTLP client certificates signed by this CA. |
| `RUPTURA_ALERTMANAGER_URLS` | Comma separated Alertmanager URLs. |
| `RUPTURA_ALERTMANAGER_TOKEN_URLS` | Comma separated Alertmanager URLs that receive the service account token. |
| `RUPTURA_ALERTMANAGER_CA_FILE` | Extra CA file trusted for Alertmanager, such as the OpenShift service CA. |
| `RUPTURA_INGEST_RPS` | Rate limit of the OTLP receiver, requests per second. |
| `RUPTURA_EDITION` | Keep `community`. |
| `LOG_LEVEL` | `debug`, `info`, `warn` or `error`. |

Flags: `--port` for the HTTP API (default 8080), `--storage` for the data directory. OTLP is received on port 4317.

See the [docs site](https://benfradjselim.github.io/ruptura/) for the `RupturaInstance` reference, failure modes and API.

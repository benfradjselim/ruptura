# Lot 1: engine prediction

Gate: leak warned at least 10 minutes before OOM on 3 cycles, 0 false alarms on
stable-web, alerts reduced by at least 90 percent.

Stop rule: 2 attempts with no score change means stop and re-diagnose.

| ID | Size | Defect | File |
|---|---|---|---|
| M1 | S | FRI is 0 with one signal source | `internal/fusion/fusion.go` |
| M5 | S | Forecast GET loses the workload | `internal/api/handlers_extra.go` |
| M2 | S | Memory is not measured against the limit | `cmd/ruptura/main.go` |
| M3 | M | Only one primary metric is analyzed | `cmd/ruptura/main.go` |
| M4 | M | Ensemble weights use one-step error | `internal/predictor/ensemble.go` |
| M6 | M | Critical uses lifetime restart count | `internal/api/handlers_ui.go` |
| M7 | M | Summary ignores OOMKilled | `internal/api` |
| M8 | M | Alerts fire on negligible changes | `internal/predictor/anomaly_mad.go` |

Also found: the UI image listens on port 80 and needs port 8080 for OpenShift (installation lot).

Superseded: M3 and M4 move to the failure-mode design. See `docs/graph/components.yaml`.

Packaging defects found on the lab, for P3: the engine Deployment and Service selectors also match UI pods, because UI labels are a superset.

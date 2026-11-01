# Architecture

## Repository layout

```
.
├── workdir/          Go module and engine source
│   ├── cmd/          ruptura (engine), ruptura-ctl (CLI), ruptura-sim (simulator)
│   ├── internal/     engine packages
│   ├── pkg/          public Go packages
│   ├── operator/     Kubernetes operator, a separate Go module
│   └── ui/           Svelte web UI
├── helm/             Helm chart
├── install/          Single file install manifest
├── bundle/ catalog/ operators/   OperatorHub packaging
├── site/             Documentation website
├── docs/             API and configuration reference
└── scripts/          Simulator and repository tooling
```

The Go module lives in `workdir/` for historical reasons.

## Engine pipeline

```
Prometheus · OTLP · Kubernetes API
  → sources, ingest    collect telemetry
  → normalize          per target data, explicit missing values
  → pipeline, predictor  forecasts and anomaly detection
  → analyzer           per workload signals
  → failuremodes       known failures, predicted or detected
  → api                REST API under /api/v2
```

| Package | Role |
|---|---|
| `internal/sources` | Metric catalog and Prometheus queries |
| `internal/ingest`, `internal/receiver` | OTLP and remote write ingestion |
| `internal/normalize` | Frames per target, with missing data listed |
| `internal/pipeline` | Forecasting and anomaly detection |
| `internal/analyzer` | Workload signals and health |
| `internal/failuremodes` | Failure recipes |
| `internal/fusion` | Risk fusion |
| `internal/discovery` | Kubernetes workload discovery |
| `internal/collector` | Infrastructure collectors |
| `internal/actions` | Remediation engine |
| `internal/storage` | BadgerDB storage |
| `internal/api` | REST handlers |

## Rules

- Missing data is never a zero.
- A stage reads only from earlier stages.
- `/api/v2` is the contract for external clients.

See [docs/REFERENCE.md](docs/REFERENCE.md) for the API and configuration reference.

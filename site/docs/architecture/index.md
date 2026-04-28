# Architecture

Ruptura ships as a **single Go binary** with BadgerDB embedded — no external database, no sidecar, no agent fleet required.

## System diagram

```mermaid
graph TD
    subgraph Ingest
        A1[gRPC :9090]
        A2[OTLP/HTTP]
        A3[Prom remote_write]
        A4[DogStatsD]
    end

    subgraph Pipelines
        B[Metric / Log / Trace pipelines]
    end

    subgraph Fusion["Fusion Engine"]
        C1[8 Composite signals]
        C2[Adaptive Ensemble v6.1]
        C3[Rupture Detector CA-ILR]
    end

    subgraph Outputs
        D1[REST API v2\n44 endpoints / XAI]
        D2[Action Engine\nK8s · Webhook · AM · PD]
    end

    subgraph Infra
        E1[NATS / Kafka eventbus\noptional v6.1]
        E2[BadgerDB embedded\n7d metrics · 400d KPIs]
        E3[K8s Operator\nRupturaInstance CRD]
    end

    Ingest --> Pipelines
    Pipelines --> Fusion
    Fusion --> Outputs
    D2 --> E1
    Fusion --> E2
    E3 -.manages.-> D1
```

## Packages

| Package | Responsibility |
|---------|---------------|
| `cmd/ruptura` | Binary entry point, flag parsing |
| `internal/ingest` | OTLP, gRPC, DogStatsD receivers |
| `internal/pipeline` | Metric / log / trace pipelines |
| `internal/fusion` | Signal fusion, composites, rupture detection |
| `internal/pipeline/metrics` | Adaptive ensemble engine (v6.1) |
| `internal/actions` | Action execution, safety gates |
| `internal/api` | REST API v2 handlers (44 endpoints) |
| `internal/storage` | BadgerDB wrapper, tiered compaction |
| `internal/eventbus` | NATS / Kafka driver (v6.1) |
| `internal/grpcserver` | gRPC ingest server (v6.1) |
| `internal/operator` | RupturaInstance CRD reconciler (v6.1) |
| `internal/explain` | XAI trace generation |
| `pkg/rupture` | Rupture Index™ core maths |
| `pkg/composites` | Composite signal formulas |
| `sdk/go` | Official Go client (`ohe` package) |
| `sdk/python` | Official Python client (`ruptura-client`) |

## Detailed pages

- [Pipelines →](pipelines.md)
- [Fusion Engine →](fusion-engine.md)
- [Kubernetes Operator →](operator.md)

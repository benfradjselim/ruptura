# Architecture

Kairo Core ships as a **single Go binary** with BadgerDB embedded — no external database, no sidecar, no agent fleet required.

## System diagram

```
┌──────────���───────────────────────────────────────────────────┐
│                        kairo-core                            ��
│                                                              │
��  ┌──────────��───────────────────���──────────────────────┐    │
│  │  Ingest layer                                        │    │
│  │  gRPC :9090 · OTLP/HTTP · Prom remote_write · DogSD │    │
│  └──────────────────────┬────────────────────���─────────┘    │
│                         │                                    │
│                         ▼                                    │
│  ┌────────────���──────────────────────────────────────���──┐   │
│  │  Pipeline layer                                      │   │
│  │  Metric pipeline · Log pipeline · Trace pipeline     │   │
│  └──────────────────────┬───────────────────────────────┘   │
│                         │                                    │
│                         ▼                                    │
│  ┌───────────────��──────────────────────────────────────┐   │
│  │  Fusion Engine                                       │   │
│  │  8 Composite signals · Adaptive Ensemble (v6.1)      │   │
│  │  Rupture Detector (CA-ILR dual-scale)                │   │
│  └──────────────────────┬─────────────────────���─────────┘   │
│                         │                                    │
│              ┌──────────┴──────────┐                        │
│              ▼                     ▼                         │
│  ┌───────────────────┐  ┌──────────────────────────┐        │
│  │  REST API v2      │  │  Action Engine           │        │
│  │  44 endpoints     │  │  K8s · Webhook · AM · PD │        │
│  │  XAI explain      │  │  Tier-1/2/3 + safety     │        │
│  └───────────────────┘  └─────────��────────────────┘        │
│                                    │                         │
│              ┌─────────────────────┘                        │
│              ▼                                               │
│  ┌───────────────────────────┐                              │
│  │  NATS / Kafka eventbus    │  (v6.1 — optional)           │
│  │  kairo.rupture.{host}     │                              │
│  │  kairo.actions.tier1      │                              │
│  └────────��──────────────────┘                              │
│                                                              │
│  ┌────────���──────────────────┐                              │
│  │  BadgerDB (embedded)      │  7d metrics · 30d logs        │
│  │  400-day KPI retention    │  Tiered compaction           │
│  └─────────────────────────��─┘                              │
│                                                              │
│  ┌─────────────────���─────────┐                              │
│  │  K8s Operator (v6.1)      │  KairoInstance CRD           │
│  │  controller-runtime       │  Deployment + Service + PVC  │
│  └────────────────────────���──┘                              │
└──────────────────────���───────────────────────────────────────┘
```

## Packages

| Package | Responsibility |
|---------|---------------|
| `cmd/kairo-core` | Binary entry point, flag parsing |
| `internal/ingest` | OTLP, gRPC, DogStatsD receivers |
| `internal/pipeline` | Metric / log / trace pipelines |
| `internal/fusion` | Signal fusion, composites, rupture detection |
| `internal/pipeline/metrics` | Adaptive ensemble engine (v6.1) |
| `internal/actions` | Action execution, safety gates |
| `internal/api` | REST API v2 handlers (44 endpoints) |
| `internal/storage` | BadgerDB wrapper, tiered compaction |
| `internal/eventbus` | NATS / Kafka driver (v6.1) |
| `internal/grpcserver` | gRPC ingest server (v6.1) |
| `internal/operator` | KairoInstance CRD reconciler (v6.1) |
| `internal/explain` | XAI trace generation |
| `pkg/rupture` | Rupture Index™ core maths |
| `pkg/composites` | Composite signal formulas |
| `sdk/go` | Official Go client (`ohe` package) |
| `sdk/python` | Official Python client (`kairo-client`) |

## Detailed pages

- [Pipelines →](pipelines.md)
- [Fusion Engine →](fusion-engine.md)
- [Kubernetes Operator →](operator.md)

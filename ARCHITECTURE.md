# Architecture

## Why `workdir/` exists

The Go module root, `cmd/`, `internal/`, `pkg/`, and the embedded UI all live
under `workdir/` instead of the repository root. This is historical: early
prototyping happened alongside non-Go tooling (lab automation, the OLM
bundle, the marketing site) at the repo root, and `workdir/` was the
boundary that kept `go build ./...` from trying to walk those trees.

Moving `workdir/` contents up to the repo root would change the Go module's
canonical import paths for every package under it — a breaking change for
anything vendoring or importing this module — for a cosmetic improvement.
That churn is not worth it right now, so the layout stays as-is and is
documented here instead.

## Repository layout

```
.
├── workdir/              Go module root — the actual engine source
│   ├── cmd/ruptura/      main binary entrypoint (HTTP API, OTLP/Prometheus receivers)
│   ├── cmd/ruptura-ctl/  CLI client
│   ├── cmd/ruptura-sim/  synthetic workload generator (used by scripts/simulate.py's Go path)
│   ├── internal/         everything not part of the public API:
│   │   ├── analyzer/         composite KPI → Fused Rupture Index scoring
│   │   ├── collector/infra/  v8 dual-axis infrastructure collectors (Object-Group model)
│   │   ├── fusion/           the 5-model adaptive prediction ensemble
│   │   ├── actions/          3-tier action engine (auto / approve / alert-only)
│   │   ├── api/              HTTP handlers for /api/v2/*
│   │   └── storage/          BadgerDB-backed time-series + key-prefix registry
│   ├── pkg/               stable, externally-referenceable types (client, models, rupture)
│   ├── operator/          OLM operator controller (reconciles RupturaInstance CRs)
│   └── ui/                Svelte 4 + Vite 5 dashboard, embedded into the Go binary
├── helm/                 Helm chart (the primary install path)
├── bundle/, catalog/     OLM bundle + catalog for OperatorHub distribution
├── docs/                 REFERENCE.md (API + config surface), fable.md (roadmap), history/
├── site/                 GitHub Pages marketing/docs site
├── lab-setup/            scripts for the maintainer's live validation cluster
├── testapp/              a sample app used to exercise the full pipeline end-to-end
└── scripts/              simulate.py and other operator conveniences
```

## Runtime shape

```
ruptura (community engine, this repo)
  ├── OTLP + Prometheus remote-write receivers  → ingest telemetry
  ├── analyzer/fusion                            → per-workload Fused Rupture Index
  ├── actions engine                             → Tier-1/2/3 remediation (edition-gated)
  └── /api/v2/*                                  → the only contract other services rely on

Ruptura Autopilot (private repo) is a separate process that talks to this
engine over /api/v2/* as an authenticated HTTP client — it does not import
any package from this repo. See that repo's CLAUDE.md SS3.3 for why.
```

See `docs/REFERENCE.md` for the full API surface and configuration reference,
and `docs/fable.md` for the current roadmap and what's still in flight.

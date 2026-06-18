# Claude Code — Project Instructions for `ruptura`

## Language
All comments, docstrings, inline documentation, and code annotations must be in **English only**.
No Spanish, French, or other languages anywhere in source files — including folder tree comments, inline notes, or variable names.

## What this project is
Ruptura is a predictive AIOps engine for Kubernetes written in Go.
It ingests OTLP + Prometheus metrics and computes a Fused Rupture Index (FRI) using a 5-model ML ensemble to detect workload failures before they happen.

**Community edition** (this repo) — Apache 2.0, single-tenant, frozen at v7.0.25 feature-set.
**Paid edition** — private repo `benfradjselim/Ruptura-autopilot`, imports this as a Go module.

## Architecture — read before touching anything
```
workdir/
  cmd/ruptura/main.go          ← entry point, edition gate, version const
  internal/pipeline/metrics/   ← CA-ILR, ARIMA, Holt-Winters ensemble (core engine)
  internal/fusion/             ← weighted fusion of metric/log/trace R values
  internal/analyzer/           ← per-workload KPI computation (stress, fatigue, etc.)
  internal/storage/            ← BadgerDB wrapper + 3-tier compaction (raw→5m→1h)
  internal/api/                ← REST handlers, auth middleware
  internal/actions/            ← 3-tier action engine + Kubernetes actuator
  ui/src/                      ← ACTIVE Svelte UI (this is what gets built and served)
ui/                            ← ARCHIVED old UI (do not touch)
```

## Critical rules

### Storage key prefixes — NEVER change these without updating retention.go
```
m:{host}:{metric}:{ts_ns}       raw metrics    TTL 7d
kpi:{name}:{host}:{ts_ns}       raw KPIs       TTL 7d
r5:{host}:{metric}:{ts_ns}      5m rollup      TTL 35d
kr5:{name}:{host}:{ts_ns}       5m KPI rollup  TTL 35d
r1h:{host}:{metric}:{ts_ns}     1h rollup      TTL 400d
kr1h:{name}:{host}:{ts_ns}      1h KPI rollup  TTL 400d
```

### Edition gate — never remove or weaken
The `cfg.Edition == "autopilot"` check in `handlers_extra.go` is the commercial boundary.
Tier-1 approve and rollback must return HTTP 402 in community edition.
Do NOT remove or bypass this gate.

### API versioning
All routes are under `/api/v2/`. Do not add `/api/v1/` routes.
Probe endpoints (`/api/v2/health`, `/api/v2/ready`) are registered on the ROOT router, not the auth subrouter — they must remain unauthenticated for k8s probes.

### Auth/login endpoints
`/api/v2/auth/login`, `/api/v2/auth/setup`, `/api/v2/auth/logout`, `/api/v2/auth/refresh`
are registered on the ROOT router. They must NOT be behind authMiddleware.

## Code style
- Idiomatic Go, minimum Go 1.18 (no 1.21+ features — module requires go 1.18)
- Table-driven tests for all algorithmic code
- Every exported symbol must have a godoc comment
- `font-variant-numeric: tabular-nums` on all numeric UI elements
- No external dependencies without strong justification — keep binary small

## Before shipping any change
1. `go build ./...` must pass
2. `go test ./...` must pass
3. UI: `npm run build` must produce a clean dist in `workdir/web/`
4. Update `workdir/ui/public/version.json` to match `const version` in main.go

# DEV-GUIDE.md — Kairo Core v6.0.0 Developer Guide

Document ID: KC-DEV-001
Date: April 2026
Status: Canonical — Phase 1 Output
Produced by: Orchestrator (Claude Code)

---

## 1. Quick Start

```bash
# From workdir/
go build ./...
go test -race -timeout=120s ./...
go test -cover -timeout=120s ./...
go test -coverprofile=/tmp/cov.out ./... && go tool cover -func=/tmp/cov.out | grep total
go vet ./...
go test -run TestRuptureIndex ./pkg/rupture/...
```

---

## 2. Repository Layout

```
workdir/
  cmd/kairo-core/        # Binary entry point (FOXTROT)
  internal/
    ingest/              # BRAVO — Prom/OTLP/DogStatsD/gRPC
    pipeline/
      metrics/           # ALPHA — CA-ILR, ensemble, surge profiles
      logs/              # BRAVO — 4 extractors
      traces/            # BRAVO — 4 analyzers + topology
    fusion/              # CHARLIE — signal fusion
    composites/          # CHARLIE — 8 composite signals
    context/             # ECHO — 4-layer context awareness
    actions/
      engine/            # DELTA — rule engine, tier determination
      providers/         # DELTA — k8s, webhook, alertmanager, pagerduty
      arbitration/       # DELTA — dedup, priority
      safety/            # DELTA — rate limit, cooldown, rollback, kill switch
    explain/             # DELTA — XAI traces, formula audit
    api/                 # ECHO — all 44 v2 endpoints
    storage/             # ECHO — BadgerDB, single-tenant schema
    telemetry/           # ECHO — /metrics, /health, /ready
    eventbus/            # FOXTROT — internal pub/sub
    vault/               # FOXTROT — Vault integration
  pkg/
    rupture/             # ALPHA — public Rupture Index formula
    composites/          # CHARLIE — public composite formulas
    client/              # FOXTROT — Go SDK
    logger/              # zero-dep structured logger
    models/              # domain types
    utils/               # shared utilities
  docs/v6.0.0/           # All governance documents
  configs/               # kairo.yaml examples
  deploy/helm/           # Helm chart
```

---

## 3. Key Conventions

**Go version:** 1.18 only. No `any`, `min/max` builtins, `slices` package, or `log/slog`.

**Errors:** `fmt.Errorf("context: %w", err)`

**Interfaces:** accept interfaces, return concrete structs.

**Tests:** table-driven, `-race` always, `_test` package suffix for black-box.

**Files:** max 400 LOC; split when approaching limit.

**Commit prefixes** (one per agent):
```
[ALPHA]   [BRAVO]   [CHARLIE]   [DELTA]   [ECHO]   [FOXTROT]
```

---

## 4. Debugging CI Failures

When a CI job fails on a specific package:

**Step 1 — Identify the failing spec item.**
Look up the package in `TRACEABILITY.md`. Find the WP section and SPECS.md entry for the failing test.

**Step 2 — Check the formula.**
Every numeric computation maps to an exact formula in `SPECS.md §3–4`.
Compare the test assertion against the formula. Example:

```
TestFatigue_halflife fails
→ TRACEABILITY.md §6: internal/composites/fatigue.go
→ SPECS.md §4.2: t_half = ln(2) / lambda
→ For lambda=0.05: t_half ≈ 13.86 intervals
→ Verify test uses correct lambda and interval count.
```

**Step 3 — Check WP gaps.**
If formula output is unexpected, check `SPECS.md §18` (UNDEFINED — WP gaps).
The Bayesian fusion algorithm is a known gap — v6.0 uses weighted average.

**Step 4 — Interface mismatch.**
If `does not implement interface`, check `AGENTS.md` for the interface definition.
Interfaces are the contract. Never change without updating `AGENTS.md` first.

**Step 5 — Coverage gate failure.**
```bash
go test -coverprofile=/tmp/cov.out ./internal/composites/...
go tool cover -html=/tmp/cov.out -o /tmp/cov.html
# Red lines = uncovered — add table-driven cases
```

---

## 5. Agent Mission Protocol

### Creating a mission for OpenCode

Write `/tmp/kairo_mission_{ROLE}_{N}.md`:

```markdown
# Mission for OpenCode — Role {ROLE} — Mission {N}

## Context
Whitepaper    : /root/Mlops_crew_automation/docs/v6.0.0/whitepaper.md
SPECS         : /root/Mlops_crew_automation/docs/v6.0.0/SPECS.md
AGENTS        : /root/Mlops_crew_automation/docs/v6.0.0/AGENTS.md
AUDIT         : /root/Mlops_crew_automation/docs/v6.0.0/AUDIT.md
MIGRATION     : /root/Mlops_crew_automation/docs/v6.0.0/MIGRATION.md
TRACEABILITY  : /root/Mlops_crew_automation/docs/v6.0.0/TRACEABILITY.md

## Your role
You are agent {ROLE}. Branch: v6_{role}.
Working directory: /root/Mlops_crew_automation/workdir

## Packages to implement
- {package path} — {description from AGENTS.md}

## Specific task
{exact detail}

## Constraints
- Go 1.18 only (no 1.21+ features)
- Formulas EXACTLY as in SPECS.md
- Coverage >= {target from AGENTS.md}
- Commit prefix: [{ROLE}]
- Push to branch v6_{role}

## After completion
Write summary to /tmp/kairo_mission_{ROLE}_{N}_done.md
```

Launch in tmux:
```bash
tmux new-session -d -s kairo_{role} \
  'sudo su ohe -c "cd /root/Mlops_crew_automation && opencode --mission /tmp/kairo_mission_{ROLE}_{N}.md"'
```

Wait for done file:
```bash
watch -n 10 ls /tmp/kairo_mission_{ROLE}_{N}_done.md
```

### Handling CI failures

1. Read GitHub Actions log
2. Find package + test name
3. Look up in `TRACEABILITY.md` → `SPECS.md`
4. Write `/tmp/kairo_mission_{ROLE}_{N}_fix.md`:

```markdown
# Fix Mission — {ROLE} — Fix {N}

## CI Failure
Job: {job name}
Error: {exact error}
Package: {package}
Test: {test name}

## Root cause (from SPECS.md)
{formula or constraint}

## Required fix
{exact correction}
```

5. Relaunch OpenCode with the fix mission.

---

## 6. Interface Contracts (Summary)

All interfaces defined in `AGENTS.md`. Quick reference:

| Interface | Owner | Consumers |
|-----------|-------|-----------|
| `MetricPipeline` | ALPHA | CHARLIE, ECHO |
| `pkg/rupture.Index()` | ALPHA | CHARLIE, DELTA |
| `Ingestor` | BRAVO | FOXTROT |
| `LogPipeline` | BRAVO | CHARLIE, ECHO |
| `TracePipeline` | BRAVO | CHARLIE, ECHO |
| `FusionEngine` | CHARLIE | ECHO, DELTA |
| `CompositeEngine` | CHARLIE | ECHO, DELTA |
| `ActionEngine` | DELTA | ECHO, FOXTROT |
| `Explainer` | DELTA | ECHO |
| `ContextManager` | ECHO | FOXTROT |
| `Store` | ECHO | FOXTROT, all read paths |

**Rule:** Never change an interface without updating `AGENTS.md` first.

---

## 7. Minimal Config

```yaml
# configs/kairo.yaml
mode: connected

ingest:
  http_port: 8080
  grpc_port: 9090

predictor:
  stable_window: 60m
  burst_window: 5m
  rupture_threshold: 3.0

composites:
  fatigue:
    lambda: 0.05
    r_threshold: 0.3

storage:
  path: /tmp/kairo-data

auth:
  jwt_secret: "change-me"

telemetry:
  metrics:
    enabled: true
```

---

## 8. CI Pipeline Stages

See `.github/workflows/ci.yml` for the authoritative definition.

| Stage | Jobs | Blocks merge |
|-------|------|-------------|
| 1 | go build + go vet | Yes |
| 2 | go test -race | Yes |
| 3 | coverage gate >= 70% | Yes |
| 4 | golangci-lint | Yes |
| 5 | Docker buildx (tag only) | Yes (release) |
| 6 | cosign sign + Helm push (tag only) | Yes (release) |

Runs on every push to any `v6_*` branch and on PRs to `v6_main`.

---

Produced: 2026-04-24

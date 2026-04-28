# ROADMAP.md — Kairo Core Development Phases

Document ID: KC-ROAD-001
Date: April 2026
Status: Living document — v6.1.0 RELEASED 2026-04-27
Produced by: Orchestrator (Claude Code)

---

## 1. Dependency DAG

```
Phase -1: AUDIT + MIGRATION + SPECS  (Orchestrator — DONE)
    |
Phase 0:  Cleanup + Rename + Structure (Orchestrator)
    |
Phase 1:  Governance + CI/CD           (Orchestrator — IN PROGRESS)
    |
Phase 2a: ALPHA — Core Engine
    |          (predictor -> pipeline/metrics, pkg/rupture)
    |
    +——————————————————+
    |                  |
Phase 2b: BRAVO     Phase 2c: CHARLIE   (parallel after ALPHA green)
  Pipelines           Fusion + Composites
  (ingest,            (fusion, composites,
  pipeline/logs,      pkg/composites)
  pipeline/traces)
    |                  |
    +——————————————————+
               |
          Phase 3: DELTA
          Actions + Explainability
          (actions/*, explain)
               |
          Phase 4: ECHO
          API + Context + Telemetry + Storage
               |
          Phase 5: FOXTROT
          cmd/kairo-core + SDK + Final integration
               |
          Phase 6: Release
          Tag v6.0.0 + ghcr.io + Helm
```

**Parallelism rules:**
- BRAVO and CHARLIE run in parallel — they share no internal dependencies
- DELTA starts only when ALPHA + BRAVO + CHARLIE are all CI-green
- ECHO starts only when DELTA is CI-green
- FOXTROT starts only when ECHO is CI-green

---

## 2. Phase Details

### Phase -1 — Audit ✅ COMPLETE
**Owner:** Orchestrator
**Outputs:** `docs/v6.0.0/AUDIT.md`, `docs/v6.0.0/MIGRATION.md`
**Exit criteria:** All packages categorized (RÉUTILISER / RÉÉCRIRE / JETER / NEW)

### Phase 0 — Cleanup & Structure
**Owner:** Orchestrator
**Branch:** `v6_main`
**Tasks:**
1. Create branch `v6_main` from current HEAD
2. Update `go.mod` module path to `github.com/benfradjselim/kairo-core`
3. Rename `cmd/ohe/` → `cmd/kairo-core/`
4. Delete JETER packages: `internal/collector/`, `internal/billing/`, `internal/cost/`
5. Delete JETER files: `handlers_audit.go`, `handlers_rbac.go`, `handlers_proxy.go`
6. Create empty skeleton dirs for all NEW packages (SPECS.md §2)
7. Global rename: OHE→Kairo, ohe→kairo, version string→"6.0.0"
8. Verify `go build ./...` passes

**Exit criteria:** `go build ./...` green on `v6_main`

### Phase 0.5 — SPECS Extraction ✅ COMPLETE
**Owner:** Orchestrator
**Output:** `docs/v6.0.0/SPECS.md`

### Phase 1 — Governance + CI/CD ✅ COMPLETE
**Owner:** Orchestrator
**Outputs:**
- `docs/v6.0.0/ROADMAP.md` (this file)
- `docs/v6.0.0/AGENTS.md`
- `docs/v6.0.0/TRACEABILITY.md`
- `docs/v6.0.0/DEV-GUIDE.md`
- `.github/workflows/ci.yml`
- `Dockerfile`
- `.golangci.yml`
- `deploy/helm/kairo-core/`
- `CODEOWNERS`

**Exit criteria:** CI pipeline runs on `v6_main` skeleton (build + vet pass)

### Phase 2a — ALPHA: Core Engine
**Owner:** Orchestrator (light — heritage code reuse)
**Branch:** `v6_alpha`
**Packages:**
- `internal/pipeline/metrics/` — from `internal/predictor/` + `internal/processor/`
- `pkg/rupture/` — public Rupture Index + TTF formulas

**Exit criteria:**
- CI green on `v6_alpha`
- `internal/pipeline/metrics` coverage >= 80%
- `pkg/rupture` coverage >= 85%

### Phase 2b — BRAVO: Signal Pipelines
**Owner:** OpenCode
**Branch:** `v6_bravo`
**Packages:**
- `internal/ingest/` — Prom remote_write + OTLP + DogStatsD + gRPC (merged from receiver + grpcserver)
- `internal/pipeline/logs/` — 4 extractors: error_rate, keyword_counter, burst_detector, novelty_score
- `internal/pipeline/traces/` — 4 analyzers: latency_propagation, bottleneck_score, error_cascade, fanout_pressure

**Exit criteria:**
- CI green on `v6_bravo`
- All three packages coverage >= 80%

### Phase 2c — CHARLIE: Fusion + Composites
**Owner:** OpenCode
**Branch:** `v6_charlie`
**Packages:**
- `internal/fusion/` — weighted Bayesian signal fusion (WP gap: use weighted average as pragmatic default)
- `internal/composites/` — all 8 composite signals: Stress, Fatigue, Pressure, Contagion, Resilience, Entropy, Sentiment, HealthScore
- `pkg/composites/` — public composite formulas (importable)

**Exit criteria:**
- CI green on `v6_charlie`
- All three packages coverage >= 80%

### Phase 3 — DELTA: Actions + Explainability
**Owner:** OpenCode
**Branch:** `v6_delta`
**Packages:**
- `internal/actions/engine/` — rule evaluation, tier determination (Tier 1/2/3)
- `internal/actions/providers/` — Kubernetes, Webhook, Alertmanager, PagerDuty
- `internal/actions/arbitration/` — conflict detection, deduplication
- `internal/actions/safety/` — rate limiting, cooldown, rollback, emergency stop
- `internal/explain/` — XAI trace, formula audit, pipeline debug

**Exit criteria:**
- CI green on `v6_delta`
- `internal/actions/*` coverage >= 75%
- `internal/explain` coverage >= 75%

### Phase 4 — ECHO: API + Integrations
**Owner:** OpenCode
**Branch:** `v6_echo`
**Packages:**
- `internal/api/` — full v2 API; v1 behind `--compat-ohe-v5`; wire all handlers
- `internal/context/` — 4-layer context awareness
- `internal/telemetry/` — self-monitoring, `/metrics`, health endpoint
- `internal/storage/` — strip org isolation; single-tenant key schema

**Exit criteria:**
- CI green on `v6_echo`
- `internal/api` coverage >= 70%
- `internal/context`, `internal/telemetry` coverage >= 75%
- `internal/storage` coverage >= 70%

### Phase 5 — FOXTROT: Integration & Operability
**Owner:** Orchestrator
**Branch:** `v6_foxtrot`
**Packages:**
- `cmd/kairo-core/` — main binary; wire all packages; `kairo.yaml` parsing
- `internal/vault/` — keep as-is
- `internal/eventbus/` — extend for rupture events
- `sdk/go/` — rename + update to v2 API
- `sdk/python/` — rename + update to v2 API

**Exit criteria:**
- CI green on `v6_foxtrot`
- Binary size <= 25 MB (`go build -o kairo-core && du -sh kairo-core`)
- Total coverage >= 70%
- Smoke test: start binary, POST /api/v2/write, GET /api/v2/health returns 200

### Phase 6 — Release ✅ COMPLETE (2026-04-25)
**Owner:** Orchestrator
**Tasks:**
1. Merge all PRs: v6_alpha → v6_main, then BRAVO+CHARLIE (parallel), then DELTA, ECHO, FOXTROT
2. Final coverage check: total >= 70%
3. `git tag v6.0.0 && git push origin v6.0.0`
4. CI Stage 4-5-6 auto-triggers: Docker buildx + push ghcr.io + cosign signature
5. Helm chart package and publish

**Exit criteria:** Image live on `ghcr.io/benfradjselim/kairo-core:v6.0.0`; Helm chart installable

---

## v6.1.0 ✅ RELEASED 2026-04-27

| Item | Spec | Agent | Branch | PR | Coverage |
|------|------|-------|--------|----|---------|
| Real gRPC ingest server | §23 | GOLF | v6.1_golf | #8 | 83.2% |
| NATS/Kafka eventbus (JetStream + franz-go) | §24 | HOTEL | v6.1_hotel | #9 | 88.0% |
| Adaptive ensemble weighting (online MAE) | §25 | INDIA | v6.1_india | #10 | 89.2% |
| Kubernetes operator (KairoInstance CRD) | §26 | JULIET | v6.1_juliet | #11 | 85.1% |
| Go SDK kairo-client-go (full v2 coverage) | — | Orchestrator | v6.1 | direct | — |

---

## v6.2.0 — PLANNED (Q2 2026)

Theme: **Operability + SaaS foundations**

| Item | Spec | Agent | Priority |
|------|------|-------|---------|
| `kairoctl` CLI (Go binary, full v2 API) | §27 | PAPA | P0 |
| Multi-tenant opt-in flag (`--multi-tenant`) | §28 | QUEBEC | P0 |
| Web dashboard v2 (eventbus, adaptive weights, operator status) | §29 | ROMEO | P1 |
| Helm chart v2 (operator CRD + eventbus config) | §30 | SIERRA | P1 |
| Alerting templates (Alertmanager, PagerDuty pre-built rules) | §30 | SIERRA | P1 |
| Python SDK v6.1 (eventbus, adaptive, operator methods) | — | LIMA | P0 |
| Go SDK v6.1 (eventbus, adaptive, operator methods) | — | KILO | P0 |

---

## v6.3.0 — PLANNED (Q3 2026)

Theme: **Commercial SaaS + CNCF**

| Item | Priority |
|------|---------|
| Self-serve onboarding (sign-up → org → API key) | P0 |
| Billing integration (Stripe, metering hooks already exist) | P0 |
| Managed cloud deployment (Fly.io / Railway free tier) | P0 |
| Marketing + documentation site | P1 |
| GOVERNANCE.md, MAINTAINERS.md, CODE_OF_CONDUCT.md, SECURITY.md | P0 |
| Multi-arch Docker builds (amd64 + arm64) | P1 |
| CNCF Sandbox application | P1 |

---

## 3. Exit Criteria Summary Table

| Phase | Hard Gate | Coverage Gate | Result |
|-------|-----------|--------------|--------|
| 0 | `go build ./...` green | N/A | ✅ DONE |
| 1 | CI pipeline runs (build+vet) | N/A | ✅ DONE |
| 2a ALPHA | CI green | pipeline/metrics >= 80%, pkg/rupture >= 85% | ✅ 89.2% — MERGED PR#1 |
| 2b BRAVO | CI green | ingest >= 80%, pipeline/logs >= 80%, pipeline/traces >= 80% | ✅ 85-86% — MERGED PR#4 |
| 2c CHARLIE | CI green | fusion >= 80%, composites >= 80%, pkg/composites >= 85% | ✅ 85-93% — MERGED PR#3 |
| 3 DELTA | CI green | actions/* >= 75%, explain >= 75% | ✅ 83-100% — MERGED PR#5 |
| 4 ECHO | CI green | api >= 70%, context >= 75%, storage >= 70% | ✅ 72-95% — MERGED PR#6 |
| 5 FOXTROT | CI green + binary <= 25MB | total >= 70% | ✅ 63-88%* — MERGED PR#7 |
| 6 Release | image on ghcr.io | total >= 70% | ✅ TAGGED v6.0.0 — 2026-04-25 |

*cmd/kairo-core à 63% — sous le seuil de 70%, correction planifiée en v6.1.

---

## 4. Branch Strategy

| Branch | Owner | PR Target |
|--------|-------|-----------|
| `v6_main` | Orchestrator | base branch |
| `v6_alpha` | Orchestrator | → v6_main |
| `v6_bravo` | OpenCode | → v6_main |
| `v6_charlie` | OpenCode | → v6_main |
| `v6_delta` | OpenCode | → v6_main |
| `v6_echo` | OpenCode | → v6_main |
| `v6_foxtrot` | Orchestrator | → v6_main |

Merge order enforced by CI required checks:
ALPHA → (BRAVO || CHARLIE) → DELTA → ECHO → FOXTROT → tag v6.0.0

---

Produced: 2026-04-24
Last updated: 2026-04-27 — v6.1.0 released; v6.2 and v6.3 planned

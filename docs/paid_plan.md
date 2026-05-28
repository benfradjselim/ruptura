# Ruptura Paid Edition — Master Plan

> **Purpose of this file**: This is a briefing document for future Claude sessions.
> Read this before doing ANY work on Ruptura paid/autopilot features.
> It captures all business decisions, architecture decisions, priorities, and technical context.

---

## 1. What Ruptura Is

Ruptura is a self-hosted Kubernetes workload health and rupture-detection engine.
It observes metrics, logs, and traces from workloads and computes a **FusedRuptureIndex (FusedR)**
— a 0–10 score that signals when a workload is degrading before it fully fails.

The engine is written in **Go**, the dashboard is **Svelte 4 + nginx** (separate pod).
Storage is **BadgerDB** (embedded). OTLP ingest on port 4317, API on port 8080.

**Live cluster**: `185.229.225.115`
- Engine NodePort: `31468` (API + metrics)
- UI NodePort: `31469` (dashboard)
- OTLP NodePort: `31470` (push endpoint)
- API key: `8ef7320fdd90a69ca83333148938ae0ecdc12abd612e59cf7eeb0b525a91dc81`

**GitHub**: `https://github.com/benfradjselim/ruptura` (public, community edition)

---

## 2. Business Model — Open-Core

**Community edition** is public on GitHub, free, frozen at v7.0.25 capabilities.
**Autopilot edition** is private, paid, with advanced features.

Community serves as the acquisition channel (developers discover it, use it, want more).
Autopilot is sold to teams and enterprises that need automation, multi-cluster, and compliance.

### What "frozen at v7.0.25" means

No new features go into the public community repo after v7.0.25.
Security patches and critical bug fixes can still be backported.
The community branch stays at `main` on `github.com/benfradjselim/ruptura`.

### The Teaser Pattern

Some paid features are **visible but locked** in the community binary.
When a community user hits a locked feature (e.g. Tier-1 approve), the API returns:

```json
HTTP 402
{
  "error": "action execution requires the Autopilot edition",
  "upgrade": "set RUPTURA_EDITION=autopilot to enable automated and manual action approval"
}
```

This is intentional — it shows users what they're missing and drives upgrade intent.
**Do not remove the 402 gate or make it silent in community.**

Currently locked in community:
- Tier-1 auto-execute goroutine (code exists, guarded by `cfg.Edition == "autopilot"`)
- Manual approve of any action recommendation (returns 402 in community)

---

## 3. Community Edition — Current State (v7.0.25)

### Key capabilities shipped

| Feature | Status |
|---------|--------|
| FusedR computation (0–10 scale, capped) | Done |
| 5 KPI signals: health_score, stress, fatigue, mood, contagion | Done |
| Svelte 4 dashboard (separate UI pod) | Done |
| OTLP ingest (logs + traces + metrics) | Done |
| Prometheus + direct scraper datasources | Done |
| Action recommendations (Tier 1/2/3 engine) | Done — read-only in community |
| Tier-1 approve locked behind 402 | Done |
| BadgerDB persistence + snapshot restore | Done |
| K8s auto-discovery (Deployments/StatefulSets/DaemonSets) | Done |
| Helm chart (community values) | Done |
| MkDocs site deployed to GitHub Pages | Done |

### Key files to know

```
workdir/
  cmd/ruptura/main.go              — entrypoint, version const, Tier-1 goroutine gate
  internal/actions/engine/engine.go — ActionRecommendation struct (has JSON tags), tiers
  internal/actions/providers/kubernetes.go — KubernetesActuator (scale/restart/cordon)
  internal/api/handlers_extra.go   — handleActions, 402 gate, actionResp wire type
  internal/api/handlers_engine.go  — engine status (includes edition field)
  internal/storage/storage.go      — BadgerDB, LoadSnapshots, sanitizeLoadedSnapshot
  internal/scraper/manager.go      — datasource manager (Prometheus, direct, OTLP types)
  internal/fusion/engine.go        — FusedR computation
  pkg/rupture/rupture.go           — rupture.Index(), capped at 10.0
  web/index.html                   — points to current Svelte bundle hashes
  web/assets/                      — built Svelte bundles (copy after `npm run build`)

ui/
  src/routes/Fleet.svelte          — main dashboard (signals, history, actions, k8s tabs)
  src/lib/api.ts                   — all API types and fetch functions

helm/
  values.yaml                      — edition: community (default)
  templates/deployment.yaml        — RUPTURA_EDITION env var wired
  templates/rbac.yaml              — autopilot edition adds patch/update verbs

site/docs/                         — MkDocs source (deployed to GitHub Pages)
docs/                              — internal planning docs (this file lives here)
```

### Version history

- `7.0.20` — CAILR cap fix (FusedR no longer exceeds 10.0 in predictor path)
- `7.0.24` — Signal state "undefined" fix in UI, rupture.Index() capped at 10.0
- `7.0.25` — Actions JSON tags fix, Tier-1 teaser (locked in community), Helm RBAC

---

## 4. Paid Autopilot Edition — Feature Plan

### 4.1 Repo Strategy

**Use Option B: private repo extends public.**

Create a new **private** GitHub repository (suggested: `github.com/benfradjselim/ruptura-enterprise`
or under a new org `ruptura-io/ruptura-autopilot`).

The private repo imports the community engine as a Go module dependency:
```go
require github.com/benfradjselim/ruptura v7.0.25
```

Paid features are built on top — they call into the community engine's exported interfaces.
Community code is **never modified** to add paid features. Only the licensed binary is different.

This means:
- Community repo stays clean and trustworthy
- Paid binary ships community engine + paid extensions compiled together
- If community repo is forked by a competitor, they still can't get paid features

### 4.2 Licensing

**Use offline JWT license files** (recommended for enterprise/air-gapped environments).

How it works:
1. You generate a license keypair (EC P-256 or RSA-2048, kept secret)
2. Customer purchases → you sign a JWT with their org name, feature flags, expiry, cluster fingerprint
3. Customer sets `RUPTURA_LICENSE_KEY=<jwt>` in their deployment
4. Engine verifies signature at startup and periodically (every 60s)
5. If license missing or expired → fall back to community behavior (not crash)

JWT payload example:
```json
{
  "sub": "acme-corp",
  "iss": "ruptura.io",
  "iat": 1716000000,
  "exp": 1747536000,
  "features": ["autopilot", "multi_tenant", "multi_cluster"],
  "max_clusters": 5,
  "max_workloads": 500
}
```

No phone-home required. Works fully air-gapped (OCP requirement).

Implementation location: `internal/license/license.go` in private repo.

### 4.3 Feature Priority List

Work items in order. Do NOT skip ahead — each tier is a sellable milestone.

---

#### TIER A — Launch blockers (must have before first paid sale)

**A1. Licensing layer**
- Offline JWT verification at startup
- Feature flag helper: `license.Has("autopilot")`, `license.Has("multi_tenant")`
- Grace period: 7-day warning before expiry, 3-day hard cutoff
- CLI: `ruptura license info` to inspect current license

**A2. Tier-1 autopilot execution** (already implemented in community as teaser)
- Move goroutine from community `main.go` into paid extension
- Remove from community entirely (or keep as 402-gated teaser — user decision TBD)
- Expose execution history endpoint: `GET /api/v2/actions/history`
- Rollback support: `POST /api/v2/actions/{id}/rollback`

**A3. Audit log**
- Every action taken (auto or manual), every approve/reject, every config change
- Append-only log stored in BadgerDB with `auditlog:` prefix
- Endpoint: `GET /api/v2/audit?from=&to=&limit=`
- This is required by enterprise security teams before they'll buy anything

---

#### TIER B — Multi-tenant (required for SaaS and shared-platform customers)

**B1. Tenant isolation**
- Each tenant has: a unique API key, a namespace prefix, isolated BadgerDB keyspace
- All existing endpoints respect tenant context from bearer token
- Admin API (master key only) to create/delete tenants

**B2. Per-tenant RBAC**
- Roles: `admin`, `operator`, `viewer`
- Admin: full access including action approval
- Operator: can approve/reject, cannot change config
- Viewer: read-only

**B3. Tenant-scoped dashboard**
- UI shows only the tenant's workloads
- No cross-tenant data leakage at API level

---

#### TIER C — Multi-cluster

**C1. Federation agent**
- Lightweight sidecar agent runs inside each remote cluster
- Sends aggregated FusedR + KPI snapshots to a central Ruptura instance
- Uses OTLP-compatible push (reuse existing ingest path)
- Agent: `cmd/ruptura-agent/` in private repo

**C2. Central fleet view**
- New UI route: `/clusters` — shows all registered clusters
- Per-cluster: count of workloads, worst FusedR, active ruptures
- Drill-down into remote cluster's workloads (proxied through central engine)

**C3. Cross-cluster correlation**
- If service A in cluster-1 and service B in cluster-2 both rupture simultaneously, surface the correlation
- Extend topology builder to include cross-cluster edges (via manually configured service map or auto-detected via trace propagation)

---

#### TIER D — Premium dashboard + catalog

**D1. Dashboard catalog**
- Prebuilt dashboard templates: "K8s Node Health", "Microservices Latency", "Database Saturation", "ML Inference Health"
- JSON-defined panels, importable via UI
- Each template maps named datasource queries to FusedR KPI panels

**D2. Custom dashboard builder**
- Drag-and-drop panel layout
- Panel types: time-series, gauge, table, heatmap, topology map
- Variable support (e.g. `$namespace`, `$workload`)
- Saved to BadgerDB, exportable as JSON

**D3. Premium UI theme**
- Distinct visual style from community (shows clear differentiation)
- Dark enterprise theme + light mode toggle
- White-labeling option (logo/color overrides via Helm values)

---

#### TIER E — Extended data sources

**E1. Datadog metrics pull** — poll Datadog API, ingest into pipeline
**E2. Dynatrace pull** — same pattern
**E3. Loki logs pull** — connect to Grafana Loki, feed into log burst detector
**E4. CloudWatch** — AWS-native for teams running EKS
**E5. Azure Monitor** — for AKS teams

Each datasource follows the existing `scraper.DatasourceConfig` pattern.
Add a new `Type` constant per datasource and implement the `runScrape()` switch case.

---

#### TIER F — OCP (OpenShift) operator

**F1. Operator scaffold** — use `operator-sdk` or `kubebuilder`
**F2. CRD: RupturaInstance** — declarative config for the engine
**F3. OLM integration** — ship via OperatorHub for OCP 4.x
**F4. SCC (Security Context Constraints)** — OCP requires explicit SCC instead of PodSecurityPolicy

---

#### TIER G — Website + marketing

**G1. Landing page** — `ruptura.io` (or subdomain) dedicated to autopilot edition
- Above fold: "Self-healing Kubernetes, automated" — FusedR explainer, Tier-1 demo GIF
- Pricing page: tiers (team / enterprise / unlimited)
- Comparison table: community vs autopilot

**G2. Docs site (paid)** — private docs subdomain or private Docusaurus site
- Advanced feature docs (multi-cluster setup, licensing, audit log)
- Architecture deep-dives (why FusedR, how CAILR works, rupture lifecycle)

**G3. Blog / content** — "How we detect OOMKills before they happen", case studies

---

## 5. Architecture Decisions Locked In

| Decision | Choice | Reason |
|----------|--------|--------|
| Repo model | Option B: private extends public | No community code contamination; clean OSS story |
| Licensing | Offline JWT | Air-gap compatible; low ops; enterprise-ready |
| Storage | BadgerDB (keep) | No external dep; pod-local; good enough up to ~50k workloads |
| Multi-tenant isolation | Key prefix in BadgerDB | Avoid running multiple engines per tenant (too expensive) |
| Multi-cluster comm. | OTLP push from agent to central | Reuses existing ingest path; stateless agent |
| UI framework | Svelte 4 (keep) | No reason to change; fast and small bundle |
| Action execution | K8s ServiceAccount in-cluster | Already implemented in KubernetesActuator |
| Tier-1 teaser | Visible but 402-locked in community | Drives upgrade intent; already shipped |

---

## 6. What NOT to Do

- **Do not add paid features to the public community repo** — not even as dead code
- **Do not remove the 402 gate** — it is the upgrade trigger
- **Do not change the community version number** past v7.0.25 for feature work
- **Do not use phone-home licensing** — enterprises will not accept it
- **Do not build the dashboard catalog before licensing** — you can't sell without a paywall

---

## 7. How to Start Next Session

1. Read this file first
2. Check `workdir/cmd/ruptura/main.go` for current version constant
3. Check `workdir/internal/actions/engine/engine.go` for ActionRecommendation (has JSON tags since v7.0.25)
4. The private repo does not exist yet — first task is to create it and set up the Go module extension architecture
5. Then implement Tier A (licensing layer) before any other paid feature

If the private repo already exists when you read this, look for `internal/license/license.go` — that's the canonical licensing module location.

---

## 8. Customer Segments to Target

1. **Platform engineering teams** (10–200 engineers) running multi-service K8s — want Tier-1 automation to reduce 3am pages
2. **SRE teams at scale-ups** — want multi-cluster visibility without paying for Datadog
3. **Regulated industries** (fintech, healthtech) — need audit log, air-gapped licensing, OCP support
4. **MSPs / platform providers** — need multi-tenant to resell managed Ruptura to their customers

---

---

## 9. What Has Already Been Done in the Private Repo

> **Read this before writing any code in `Ruptura-autopilot`.** Everything below exists
> and is pushed to `main`. Do not re-implement — continue from here.

### Private repo: `github.com/benfradjselim/Ruptura-autopilot`

Clone side-by-side with the community repo:
```bash
git clone https://github.com/benfradjselim/ruptura
git clone https://github.com/benfradjselim/Ruptura-autopilot
```

The `go.mod` uses a `replace` directive so the autopilot module imports the community
engine locally:
```
replace github.com/benfradjselim/ruptura => ../ruptura/workdir
```

**Important architectural note discovered during setup:**
Go's `internal/` package visibility rule prevents the autopilot module from directly
importing `github.com/benfradjselim/ruptura/internal/...` packages. The executor
therefore communicates with the community engine via its **HTTP API** rather than
in-process function calls. This is actually a better design — the autopilot process
can run as a sidecar alongside any community engine instance.

---

### Tier A1 — Licensing layer ✅ DONE

**File:** `internal/license/license.go`

- Offline JWT verification using **EC P-256** (air-gap compatible, no phone-home)
- Feature flags: `autopilot`, `multi_tenant`, `multi_cluster`, `dash_catalog`, `ocp_operator`
- `license.Has("autopilot")` — check before enabling any paid feature
- `license.OrgName()` — returns licensed org or `"community"` if unlicensed
- `license.ExpiresAt()`, `license.DaysUntilExpiry()` — expiry tracking
- `license.MaxClusters()` — cluster count limit from JWT
- Expiry warnings at ≤7 days; hard fail at 0 days
- Reads from `RUPTURA_LICENSE_KEY` env var

**File:** `internal/license/license_test.go`

4 passing tests: valid license, expired license, tampered token, no license.

**Real keypair is generated and embedded.** `deploy/license.pub` is committed.
`deploy/license.key` is gitignored — store it in a secrets manager.

---

### Tier A1 — License issuer tool ✅ DONE

**File:** `cmd/ruptura-issue-license/main.go`

Go-native JWT signer — replaces `generate-license.sh` when the `step` CLI is unavailable.

```bash
go run ./cmd/ruptura-issue-license \
  -org "acme-corp" \
  -features "autopilot,multi_tenant,multi_cluster" \
  -key ./deploy/license.key \
  -clusters 5 \
  -days 365
```

Outputs the signed JWT to stdout. Pass it as `RUPTURA_LICENSE_KEY`.

---

### Tier A2 — Tier-1 auto-execution ✅ DONE (HTTP-API mode)

**File:** `internal/autopilot/executor.go`

- Polls `GET /api/v2/actions` every 15 seconds
- Finds Tier-1 actions where `state == "pending"` and `approved == false`
- Calls `POST /api/v2/actions/{id}/approve` for each
- License-gated: `New()` returns error if `license.Has("autopilot") == false`
- Configured via `RUPTURA_ENGINE_URL` (default `http://localhost:8080`) and `RUPTURA_API_KEY`

Note: execution of the actual K8s action (scale/restart/cordon) still happens inside
the community engine after the approve call — the engine's `RUPTURA_EDITION=autopilot`
goroutine fires. For full autopilot-only execution bypassing the edition gate, the
next step is to add a dedicated execute endpoint to the community engine that the
autopilot sidecar can call directly.

---

### Tier B1 — Multi-tenant reverse proxy ✅ DONE (auth isolation)

**Files:** `internal/multitenant/tenant.go`, `internal/proxy/proxy.go`

- Reverse proxy listens on `:8090` (env `RUPTURA_PROXY_ADDR`)
- Resolves `Authorization: Bearer <tenant-key>` → rewrites to master key → forwards to community engine
- Master key passes through directly (operators can use the proxy for all API calls)
- Adds `X-Ruptura-Tenant` / `X-Ruptura-Tenant-Name` headers for future storage hooks
- Admin endpoints (master key only):
  - `POST /api/v2/tenants` — create tenant, returns generated API key
  - `GET  /api/v2/tenants` — list tenants
  - `DELETE /api/v2/tenants/{id}` — remove tenant
- License-gated: `multi_tenant` feature required
- Started automatically by `ruptura-autopilot` when licensed

**Data isolation note:** full BadgerDB key-prefix isolation is deferred. Current
isolation is at the auth layer only. `StoragePrefix(tenantID)` is wired and ready
for a future storage hook in the community engine.

---

### Tier C1 — Multi-cluster federation agent ✅ DONE

**Files:** `internal/federation/agent.go`, `cmd/ruptura-agent/main.go`

Standalone binary that runs as a sidecar inside each **remote** cluster.

- Polls `GET /api/v2/fleet` from the local engine every 30s (env `RUPTURA_POLL_INTERVAL`)
- Pushes FusedR + KPI snapshots to the central engine via `POST /otlp/v1/metrics`
- Prefixes each workload's `host.name` with `RUPTURA_CLUSTER_NAME/` so the central
  engine stores remote workloads separately (e.g. `eu-west-1/default/Deployment/api`)
- Also sets `ruptura.cluster` and `ruptura.workload` resource attributes
- License-gated: `multi_cluster` feature required
- No K8s permissions needed; no shared storage with the engine

**Env vars:**
```
RUPTURA_LICENSE_KEY      — must include multi_cluster
RUPTURA_CLUSTER_NAME     — unique name for this cluster (required)
RUPTURA_CENTRAL_URL      — base URL of the central engine (required)
RUPTURA_CENTRAL_API_KEY  — API key for the central engine
RUPTURA_LOCAL_URL        — local engine URL (default http://localhost:8080)
RUPTURA_POLL_INTERVAL    — push interval in seconds (default 30)
```

---

### Tier C2 — Central fleet view ✅ DONE

**Files:** `internal/proxy/clusters.go`, `internal/proxy/ui/clusters.html`

Served by the autopilot proxy at `:8090`.

- `GET /clusters` — standalone HTML dashboard (dark theme, no build step, embedded in binary)
  - API key input stored in `sessionStorage`
  - Status bar: cluster count, total workloads, healthy/degraded/critical, worst FusedR
  - Cluster cards with FusedR bar (green <3 / yellow 3–6 / red 6+) and state pills
  - Click card → workload drill-down table with full KPI columns
- `GET /api/v2/clusters` — JSON cluster summaries (master key required)
- `GET /api/v2/clusters/{name}/workloads` — JSON workload list for one cluster

**Detection logic:** a host pushed by `ruptura-agent` has `clusterName/namespace/kind/name`
(3+ slashes). Local workloads have at most 2 slashes. Local workloads appear as `_local`.

3 unit tests covering `remoteCluster`, `groupByCluster`, `workloadsForCluster`.

---

### Entrypoint ✅ DONE

**File:** `cmd/ruptura-autopilot/main.go`

- `./ruptura-autopilot` — starts with license check; launches autopilot executor if
  `autopilot` licensed; starts multi-tenant proxy if `multi_tenant` licensed
- `./ruptura-autopilot license` — prints license info (org, features, expiry, max clusters)
- Reads `RUPTURA_ENGINE_URL`, `RUPTURA_API_KEY`, `RUPTURA_LICENSE_KEY`, `RUPTURA_PROXY_ADDR`

---

### Tooling ✅ DONE

**`deploy/generate-keypair.sh`** — generates EC P-256 signing keypair.
Run once on a secure machine. Outputs `license.key` (private, never commit) and
`license.pub` (embed into `internal/license/license.go`). **Already run — real key embedded.**

**`deploy/generate-license.sh`** — issues signed JWT licenses for customers.
Requires `step` CLI. Use `cmd/ruptura-issue-license` instead if `step` is unavailable.

**`deploy/license.pub`** — committed. The matching private key is gitignored.

**`.github/workflows/ci.yml`** — runs `go test ./...` + cross-compiles for
`linux/amd64`, `linux/arm64`, `darwin/arm64` on every push to `main`.
`COMMUNITY_REPO_TOKEN` Actions secret is set.

---

### What to Do Next (in priority order)

**Step 1 — Tier B2: per-tenant RBAC**
Add `admin` / `operator` / `viewer` roles to the tenant model.
- `admin`: full access including action approval
- `operator`: can approve/reject, cannot change config
- `viewer`: read-only
Wire role enforcement into the proxy middleware.

**Step 2 — Tier B3: tenant-scoped dashboard**
The community dashboard at `:31469` shows all workloads. A tenant operator
should only see their own. Options:
- Serve a filtered UI from the proxy (easiest — query fleet and strip cross-tenant hosts)
- Add a UI route `/tenant-dashboard` served by the proxy, similar to `/clusters`

**Step 3 — Tier A3: audit log**
Every action taken (auto or manual), every approve/reject, every config change.
Append-only, stored in BadgerDB with `auditlog:` prefix.
`GET /api/v2/audit?from=&to=&limit=` endpoint in the proxy.
Required by enterprise security teams before first paid sale.

**Step 4 — Tier C3: cross-cluster correlation**
If service A in cluster-1 and service B in cluster-2 rupture simultaneously, surface it.
Extend the central engine's topology builder with cross-cluster edges.

**Step 5 — Tier D: premium dashboard catalog**
Prebuilt dashboard templates (JSON-defined panels). Drag-and-drop builder. White-labeling.

---

*Last updated: 2026-05-28 | Community frozen at: v7.0.25 | Private repo: github.com/benfradjselim/Ruptura-autopilot @ main*

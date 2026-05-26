# Dashboard Architecture

Ruptura v7 ships the dashboard as a **separate `ruptura-ui` pod** — a Svelte 4 SPA served by nginx. This decouples UI deploys from engine deploys and eliminates the `go:embed` rebuild cycle that v6 required.

---

## Two-pod separation

```
┌─────────────────────────────────────────────────────────────────┐
│                        ruptura-system                            │
│                                                                  │
│   ┌─────────────────────────┐   ┌──────────────────────────┐    │
│   │     ruptura-engine      │   │       ruptura-ui          │    │
│   │      (Go binary)        │   │   (Svelte 4 + nginx)      │    │
│   │                         │   │                            │    │
│   │  :8080  REST API        │◄──│  /api/* → proxy_pass       │    │
│   │  :4317  OTLP gRPC       │   │  injects Authorization     │    │
│   │  :8080  OTLP HTTP       │   │  :80  SPA (index.html)     │    │
│   └─────────────────────────┘   └──────────────────────────┘    │
│          NodePort 31468                 NodePort 31469            │
│          NodePort 31470 (OTLP)                                    │
└─────────────────────────────────────────────────────────────────┘
```

| Service | Port | What it serves |
|---------|------|----------------|
| `ruptura-engine` | 31468 | REST API — `/api/v2/*` |
| `ruptura-ui` | 31469 | Svelte SPA + nginx proxy |
| `ruptura-engine` | 31470 | OTLP ingest — `/api/v2/write`, `/otlp/v1/*` |

The UI pod never talks to the engine directly from the browser. All API calls from the browser go to `http://<node-ip>:31469/api/...`, which nginx proxy-passes to `http://ruptura-engine:8080/api/...` inside the cluster. This means:

- No CORS issues — the browser always talks to the same origin
- The API key is injected by nginx as a `proxy_set_header Authorization` when the Helm `apiKey` value is set
- The engine never needs to be NodePort-exposed to the browser

---

## nginx configuration

The nginx config (`helm/templates/ui-configmap.yaml`) has two routing rules:

```nginx
# SSE stream — disable buffering so events flow in real time
location /api/v2/events {
    proxy_pass         http://ruptura-engine:8080;
    proxy_buffering    off;
    proxy_cache        off;
    proxy_read_timeout 3600s;
    proxy_set_header   Authorization "Bearer ${API_KEY}";
}

# All other API calls
location /api/ {
    proxy_pass       http://ruptura-engine:8080;
    proxy_set_header Authorization "Bearer ${API_KEY}";
}

# SPA — serve index.html for all non-asset paths
location / {
    root       /usr/share/nginx/html;
    try_files  $uri $uri/ /index.html;
}
```

The `proxy_buffering off` on the SSE route is what makes the live rupture counter and the Events tab work — nginx cannot buffer a never-ending stream.

---

## Svelte 4 SPA structure

The UI is a standard Vite + Svelte 4 build. Built assets are copied into the nginx image at `COPY dist/ /usr/share/nginx/html/` during the Docker build.

Key routing and state:

| Route | Component | Data source |
|-------|-----------|-------------|
| `/` (Fleet) | `FleetView.svelte` | `GET /api/v2/fleet` every 15 s |
| `/workload/:ns/:kind/:name` | `WorkloadDrawer.svelte` | `GET /api/v2/kpi/snapshot/:ns/:workload` |
| `/topology` | `TopologyMap.svelte` | `GET /api/v2/topology` |
| `/engine` | `EngineView.svelte` | `GET /api/v2/health`, `GET /api/v2/engine/stats` |
| `/alerts` | `AlertsView.svelte` | `GET /api/v2/alerts` |
| `/nodes` | `NodesView.svelte` | `GET /api/v2/nodes` |
| `/settings` | `SettingsView.svelte` | `GET /api/v2/datasources` |

SSE connection (`GET /api/v2/events`) is opened once at app boot and kept alive. The header rupture counter increments in real time from the SSE stream without any additional polling.

---

## Workload lifecycle phases in the UI

The UI reflects three distinct engine states for each workload:

### Phase 1 — Calibrating

When a workload is first seen, the engine needs ~30 minutes of signal history to build adaptive Welford baselines. During this window:

- The health ring is **gray-bordered** regardless of the current KPI value
- A progress bar and ETA render under the workload name: `Calibrating… 45% · ~13 min`
- The FusedR badge is **gray**
- All 10 signal bars are visible and updating every 15 s
- Rupture alerts are **suppressed** — a single startup spike cannot page anyone

The API fields driving this state:

```json
{
  "status": "calibrating",
  "calibration_progress": 45,
  "calibration_eta_minutes": 13
}
```

### Phase 2 — Active (alerting enabled)

Once calibration completes, the full prediction stack is live:

- Health ring border changes to **green / yellow / red** based on HealthScore
- FusedR badge colors: green < 1.5 · yellow 1.5–3.0 · orange 3.0–5.0 · red ≥ 5.0
- `⚠ Critical in ~Xm` appears on the card when `critical_eta_minutes` is set
- Tier-2 and Tier-1 actions can fire
- Pattern-match warnings appear on the Signals tab when cosine similarity ≥ 0.85 against a prior rupture fingerprint

### Phase 3 — Rupture

FusedR crossed a threshold and held:

- Card background turns red
- A rupture event is logged in the Events tab (SSE fires immediately)
- The action engine evaluates and executes or suggests remediation
- The Signals tab shows the full named-state vocabulary (`panic`, `burnout`, `pandemic`, etc.)

---

## Signal state vocabulary

Each of the 10 KPI signals maps its numeric value (0–1) to a named state. These names appear in tooltips, the Signals tab, and the Events feed.

| Signal | States (low → high) |
|--------|---------------------|
| Stress | `calm` · `nervous` · `stressed` · `panic` |
| Fatigue | `fresh` · `tired` · `exhausted` · `burnout` |
| Mood | `happy` · `neutral` · `unhappy` · `depressed` |
| Contagion | `contained` · `spreading` · `epidemic` · `pandemic` |
| Resilience | `strong` · `recovering` · `fragile` · `critical` |
| Pressure | `comfortable` · `rising` · `heavy` · `critical` |
| Humidity | `dry` · `humid` · `stormy` |
| Entropy | `ordered` · `mixed` · `chaotic` |
| Velocity | `steady` · `accelerating` · `surging` |
| Throughput | `low` · `normal` · `high` · `flood` |

---

## Light / dark mode

The Svelte app stores the preference in `localStorage` and toggles a CSS class on `<html>`. nginx serves a static asset — no server involvement. The toggle is in the top-right corner of the header.

---

## SSE live events

```
GET /api/v2/events
Authorization: Bearer <key>

data: {"type":"rupture","workload":"payment-api","fused_r":21.97,"threshold":"emergency","ts":"2026-05-25T14:02:11Z"}

data: {"type":"recovery","workload":"order-service","fused_r":0.82,"ts":"2026-05-25T14:08:55Z"}
```

The engine emits one event per rupture/recovery. The UI header counter and the Events tab both consume this stream. No WebSocket — SSE is unidirectional and reconnects automatically on disconnect.

---

## Air-gap compatibility

Because nginx serves the built Svelte bundle from the container image and all API calls are same-origin proxied, there are no external CDN dependencies. The dashboard works in fully air-gapped clusters with no egress.

Fonts are bundled inside the Docker image at build time — no Google Fonts fetch at runtime.

---

## Build pipeline

```
workdir/web/          ← Svelte 4 source
  src/
    lib/              ← reusable components
    routes/           ← page components
    stores/           ← Svelte stores (fleet state, SSE)
  vite.config.ts

docker build          → copies dist/ into nginx:alpine image
helm package          → ruptura-ui Deployment references ghcr.io/benfradjselim/ruptura-ui:<tag>
```

The engine image and UI image are built and pushed independently. A UI-only change (CSS fix, new chart) does not require rebuilding the Go binary.

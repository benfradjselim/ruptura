# Ruptura

**The Predictive Action Layer for Cloud-Native Infrastructure.**

Ruptura detects workload ruptures before they cause outages — using the Fused Rupture Index™, 10 composite KPI signals with adaptive per-workload baselines, and an action engine that responds automatically with safety gates.

→ **[Technical documentation & quickstart](workdir/README.md)**
→ **[Website](https://benfradjselim.github.io/ruptura/)**
→ **[API Specification](docs/openapi.yaml)**

---

## Project Status

| Version | Date | Status |
|---------|------|--------|
| v6.2.2 | 2026-04-30 | ✅ Released — anomaly REST endpoints, GAP-04 closed, all v6.x gaps resolved |
| v6.2.1 | 2026-04-30 | ✅ Released — FusedR exposed in API, Grafana dashboard corrected |
| v6.2.0 | 2026-04-30 | ✅ Released — WorkloadRef, adaptive baselines, narrative explain, topology contagion, maintenance windows |
| v6.1.0 | 2026-04-27 | ✅ Released — gRPC, eventbus, adaptive ensemble, K8s operator |
| v6.0.0 | 2026-04-25 | ✅ Released — full clean rewrite |

**Active branch:** `v6.1` · **Module:** `github.com/benfradjselim/ruptura`

---

## What's Inside

```
workdir/                  Ruptura Go source (v6.2.2)
  cmd/ruptura/            Main binary
  internal/               Engine, pipelines, API, storage, actions
  pkg/                    Public Go packages (rupture, composites, client)
  deploy/
    helm/ruptura/         Helm chart (v0.2.0)
    *.yaml                Kustomize manifests
    grafana/              Grafana dashboard JSON + provisioning
  ohe/operator/           Kubernetes operator (RupturaInstance CRD)

sdk/
  go/                     ruptura-go (Go SDK)
  python/                 ruptura-client (Python SDK)

docs/
  v6.0.0/                 SPECS, AGENTS, ROADMAP, whitepaper, DEV-GUIDE
  v6.1.0/                 v6.1 delta specs
  judgment.md             Design conscience — gaps, milestones, version log
```

---

## Install in 60 seconds

```bash
helm install ruptura workdir/deploy/helm/ruptura \
  --namespace ruptura-system \
  --create-namespace \
  --set auth.apiKey=$(openssl rand -hex 32)

kubectl port-forward svc/ruptura 8080:80 -n ruptura-system
curl http://localhost:8080/api/v2/health
```

---

## Roadmap

```
v6.2.x ✅  Fused Rupture Index · workload-level signals · adaptive baselines
            narrative explain · topology contagion · maintenance windows
v6.1.0 ✅  gRPC ingest · NATS/Kafka eventbus · adaptive ensemble · K8s operator
v7.0.0 ⏳  ruptura-ctl CLI · web dashboard v2 · multi-tenant opt-in (X-Org-ID)
```

---

## CNCF

Ruptura targets alignment with CNCF sandbox criteria: Apache 2.0 license, open governance ([GOVERNANCE.md](GOVERNANCE.md)), documented security policy ([SECURITY.md](SECURITY.md)), public roadmap. A sandbox application requires demonstrable production adoption — contributions and production feedback are the path there.

---

## License

Apache 2.0

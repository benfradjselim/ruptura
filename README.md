# Ruptura

**The Predictive Action Layer for Cloud-Native Infrastructure.**

Ruptura detects infrastructure failures before they happen — using the Rupture Index™, an adaptive ensemble of 5 ML models, and an action engine that responds automatically with safety gates.

→ **[Technical documentation & quickstart](workdir/README.md)**
→ **[Whitepaper](docs/v6.0.0/whitepaper.md)**
→ **[API Specification](docs/v6.0.0/SPECS.md)**

---

## Project Status

| Version | Date | Status |
|---------|------|--------|
| v6.1.0 | 2026-04-27 | ✅ Released — gRPC, eventbus, adaptive ensemble, K8s operator |
| v6.0.0 | 2026-04-25 | ✅ Released — full clean rewrite |
| v5.1.0 (OHE) | 2026-04-19 | ✅ Released — SDKs, Vault, plugin system |

**Active branch:** `v6.1` · **Module:** `github.com/benfradjselim/ruptura`

---

## What's Inside

```
workdir/               Ruptura Go source (v6.1.0)
  cmd/ruptura/      Main binary
  internal/            Engine, pipelines, API, storage, actions
  pkg/                 Public Go packages (rupture, composites, client)
  ohe/operator/        Kubernetes operator (RupturaInstance CRD)
  sdk/                 (legacy — see sdk/ at root)

sdk/
  go/                  ruptura-go (Go SDK)
  python/              ruptura-client (Python SDK)

docs/
  v6.0.0/             SPECS, AGENTS, ROADMAP, whitepaper, DEV-GUIDE
  v6.1.0/             v6.1 delta specs (§23–§26)

helm/                  Helm chart
deploy/                Raw Kubernetes manifests
```

---

## Roadmap

```
v6.1.0 ✅  gRPC ingest · NATS/Kafka eventbus · adaptive ensemble · K8s operator
v6.2.0 ⏳  ruptura-ctl CLI · web dashboard v2 · multi-tenant opt-in
v6.3.0 ⏳  SaaS self-serve · billing · managed cloud deployment
```

Full roadmap: [docs/v6.0.0/ROADMAP.md](docs/v6.0.0/ROADMAP.md)

---

## License

Apache 2.0

# Ruptura — CNCF Sandbox Proposal

## Name of Project
Ruptura

## Project Description

Ruptura is an open-source predictive AIOps engine for Kubernetes that detects workload failure *before it happens*. Unlike reactive monitoring tools (Prometheus, Grafana) that alert after thresholds are breached, Ruptura continuously models workload behavior using a 5-model ML ensemble and emits a **Fused Rupture Index (FRI)** — a single composite risk score that quantifies divergence from baseline up to 2 hours in advance.

Ruptura ingests OTLP and Prometheus metrics, computes 10 KPI signals (stress, fatigue, mood, pressure, humidity, contagion, resilience, entropy, velocity, throughput), and generates actionable recommendations through a 3-tier action engine (Monitor / Warn / Act). It ships as a single Go binary with a Svelte 4 SPA, deploys via Helm or OLM, and exposes a `ruptura-ctl` CLI for SRE workflows.

## Alignment with CNCF

Ruptura addresses the observability and reliability gap in the CNCF landscape:

- **Complements existing CNCF projects**: Ingest from OpenTelemetry (CNCF graduated), expose metrics to Prometheus (CNCF graduated), deploy via Helm (CNCF graduated), operate on Kubernetes (CNCF graduated)
- **Fills a gap**: Predictive failure detection is not addressed by any current CNCF project. Existing solutions are reactive; Ruptura is proactive.
- **Cloud-native design**: Single binary, stateless beyond BadgerDB storage, Helm-first deployment, Kubernetes operator, OLM bundle published to OperatorHub

## Project Maturity

- **Current version**: v7.1.0 (community) / v7.2.0 (autopilot commercial)
- **Kubernetes operator**: Published to OperatorHub (community-operators PR #8246, OCP PR #9872)
- **License**: Apache License 2.0
- **Languages**: Go 1.22, TypeScript/Svelte 4
- **Test coverage**: Unit tests across all packages (`go test ./...` passes)
- **CI/CD**: GitHub Actions — build, test, Helm lint, OLM bundle validate, smoke test on k3d

## Sponsor from TOC

*Seeking a TOC sponsor — project is at the point of first public promotion and CNCF Sandbox would provide the neutral governance home needed to build a contributor community beyond the founding author.*

## Preferred Maturity Level

Sandbox

## License

Apache License 2.0 — confirmed in `LICENSE` file at repo root.

## Source Control

https://github.com/benfradjselim/ruptura

## External Dependencies

All dependencies are listed in `workdir/go.mod`. Key runtime dependencies:
- `dgraph-io/badger/v4` — embedded time-series storage (Apache 2.0)
- `gorilla/mux` — HTTP routing (BSD-3)
- `open-telemetry/opentelemetry-go` — OTLP ingest (Apache 2.0)
- `google.golang.org/grpc` — gRPC server (Apache 2.0)

No GPL or LGPL dependencies.

## Maintainers

| Name | GitHub | Affiliation | Role |
|------|--------|-------------|------|
| Selim Benfradj | @benfradjselim | Independent | Founding Maintainer |

*The project is actively seeking additional maintainers. CNCF Sandbox acceptance would accelerate community building.*

## Infrastructure Requests (from CNCF)

- [ ] GitHub repository transfer to CNCF org (or remain in personal org with CNCF topic tag)
- [ ] Inclusion in CNCF landscape
- [ ] Access to CNCF Slack workspace (#ruptura channel)
- [ ] CNCF blog post / announcement support

## Communication Channels

- **Issues**: https://github.com/benfradjselim/ruptura/issues
- **Discussions**: https://github.com/benfradjselim/ruptura/discussions
- **Security**: security@ruptura.dev

## Website

https://ruptura.dev

## Release Process

Releases are triggered by semver tags (`vX.Y.Z`) via GitHub Actions:
1. Go binary built for linux/amd64 and linux/arm64
2. Container image pushed to `ghcr.io/benfradjselim/ruptura`
3. Helm chart published to GitHub Pages
4. `ruptura-ctl` CLI binaries published as GitHub Release assets
5. OLM bundle and FBC catalog updated for OperatorHub

## Social Media

*None currently — will establish @ruptura_io on Twitter/X upon CNCF acceptance.*

## Existing Sponsorship

None — fully self-funded and open source.

## Statement on Alignment with CNCF Mission

Ruptura advances the CNCF mission of "making cloud native computing ubiquitous" by reducing the operational burden on SRE teams managing Kubernetes workloads. Predictive failure detection democratizes reliability practices that today only large companies with ML teams can afford. By making this capability open-source and Kubernetes-native, Ruptura contributes to the maturation of the cloud-native operations ecosystem.

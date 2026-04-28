# About

**Kairo Core** is a predictive failure detection engine for cloud-native infrastructure, built by Selim Benfradj.

## Origins

Kairo Core is the v6.0 clean-room rewrite of the **Observability Holistic Engine (OHE)**, a project that began in 2025 with a core insight: existing observability tools answer *"What is broken?"* — none answer *"When will it break, and why?"*

OHE v5.0 proved the thesis with a single Go binary delivering dual-scale CA-ILR predictions, dissipative fatigue formulas, and the METRICS.md explainability standard. Kairo Core (v6.0+) extends this into a production-grade platform with a Kubernetes operator, gRPC ingest, eventbus integration, and official SDKs.

## Philosophy

> **Prevention is better than cure.**

Three principles that have been non-negotiable since v4.0:

1. **Transparent AI** — every prediction traceable to a published formula. No black boxes.
2. **Sovereign deployment** — a single static binary, no external database, runs on a Raspberry Pi 4.
3. **Auditable by design** — KPI formulas are versioned release artifacts. CISOs, auditors, and SREs can challenge any decision.

## Author

**Selim Benfradj** — Architect & Founder

- GitHub: [@benfradjselim](https://github.com/benfradjselim)
- Email: benfradjselim@gmail.com

## License

Apache License 2.0 — see [License →](license.md)

## Whitepaper

Technical and strategic background in the [Whitepaper →](whitepaper.md)

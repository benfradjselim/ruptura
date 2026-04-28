# Operations

Guides for running Kairo Core in production.

| Guide | What it covers |
|-------|---------------|
| [Deployment](deploy.md) | Kubernetes, Helm, Docker, bare-metal production configs |
| [Self Monitoring](self-monitoring.md) | Prometheus metrics Kairo exports about itself |
| [Troubleshooting](troubleshooting.md) | Common failure modes and how to resolve them |

## Production checklist

- [ ] `auth.jwt_secret` set via environment variable, not hardcoded in config
- [ ] `actions.execution_mode` set to `suggest` or `auto` (not `shadow`) when ready
- [ ] `actions.safety.namespace_allowlist` populated with production namespaces
- [ ] `storage.path` backed by a persistent volume (PVC in K8s)
- [ ] Scrape `GET /api/v2/metrics` in Prometheus for Kairo self-metrics
- [ ] Alert on `kairo_rupture_index > 3.0` in your Alertmanager
- [ ] Emergency stop documented in your runbook: `POST /api/v2/actions/emergency-stop`

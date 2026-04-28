# Action Engine

The Kairo Action Engine translates rupture detections into concrete remediation steps. It supports three execution tiers, four integration targets, and a suite of safety gates to prevent runaway automation.

## Execution tiers

| Tier | Mode | Trigger | Who acts |
|------|------|---------|---------|
| **Tier-1** | Automatic | R ≥ 5.0 + confidence ≥ 0.85 | Kairo (no human needed) |
| **Tier-2** | Suggested | R ≥ 3.0 + confidence ≥ 0.60 | Human approves via API |
| **Tier-3** | Alert only | R ≥ 1.5 | Human decides |

Configure the execution mode in `kairo.yaml`:

```yaml
actions:
  execution_mode: suggest   # shadow | suggest | auto
```

## Integration targets

### Kubernetes

```yaml
# Available K8s actions
- scale:    increase replica count on target Deployment
- restart:  rolling restart of target Deployment
- cordon:   mark Node unschedulable
- drain:    evict all Pods from a Node
- isolate:  apply NetworkPolicy to block ingress/egress
```

### Webhook

Send an HTTP POST to any URL with the rupture payload. Useful for triggering CI/CD pipelines, Slack notifications, or custom scripts.

### Alertmanager

Raise or resolve alerts in Prometheus Alertmanager. Kairo generates compatible alert payloads with labels, annotations, and `generatorURL`.

### PagerDuty

Create or update PagerDuty incidents with severity, rupture context, and XAI explanation link.

## Safety gates

Kairo enforces multiple safety gates before executing any Tier-1 action:

| Gate | Default | Description |
|------|---------|-------------|
| Rate limit | 6 / hour | Max Tier-1 actions per target per hour |
| Cooldown | 300 s | Minimum gap between two actions on the same target |
| Namespace allowlist | `[]` (all blocked) | Only act on pods in listed namespaces |
| Confidence threshold | 0.85 | Ensemble confidence required for auto-execution |
| Emergency stop | off | `POST /api/v2/actions/emergency-stop` halts all Tier-1 actions globally |

Configuration:

```yaml
actions:
  execution_mode: auto
  safety:
    rate_limit_per_hour: 6
    cooldown_seconds: 300
    namespace_allowlist:
      - production
      - staging
```

## Action lifecycle

```
Rupture detected (R ≥ threshold)
        │
        ▼
Safety gates evaluated
        │
   ┌────┴────┐
  Pass      Fail → Log + skip
   │
   ▼
execution_mode?
   ├── shadow  → Log action, do nothing
   ├── suggest → POST to /api/v2/actions (pending approval)
   └── auto    → Execute immediately (Tier-1) or queue (Tier-2)
        │
        ▼
Emit event: kairo.actions.tier1 (eventbus)
Emit metric: kairo_actions_total{tier="1",result="ok"}
```

## Approving a suggested action

```bash
# List pending actions
GET /api/v2/actions

# Approve a specific action
POST /api/v2/actions/{id}/approve

# Emergency stop all Tier-1 auto-actions
POST /api/v2/actions/emergency-stop
```

## Eventbus events

When an eventbus is configured (`nats` or `kafka`), every Tier-1 action publishes:

```
kairo.actions.tier1   → { action_id, host, type, params, rupture_id, timestamp }
kairo.rupture.{host}  → on every rupture state change
```

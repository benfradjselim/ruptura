# Action Engine

The Ruptura Action Engine translates rupture detections into concrete remediation steps. It supports three execution tiers, four integration targets, and a suite of safety gates to prevent runaway automation.

## Execution tiers

| Tier | Mode | Trigger | Who acts |
|------|------|---------|---------|
| **Tier-1** | Automatic | FusedR ≥ 5.0 + confidence ≥ 0.85 | Ruptura (no human needed) |
| **Tier-2** | Suggested | FusedR ≥ 3.0 + confidence ≥ 0.60 | Human approves via API |
| **Tier-3** | Alert only | FusedR ≥ 1.5 | Human decides |

Configure the execution mode via `RUPTURA_ACTIONS_EXECUTION_MODE` or in `ruptura.yaml`:

```yaml
actions:
  execution_mode: suggest   # shadow | suggest | auto
```

**Recommended for first deployment**: start with `suggest` to review Ruptura's recommendations for a week before enabling `auto`.

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

The action engine uses the same service account as Ruptura itself (see RBAC in the Helm chart). The ClusterRole grants `get/list/watch` on Deployments, StatefulSets, Pods, Nodes — **write permissions are not included by default**. To enable K8s remediation actions, add `update/patch` verbs to the ClusterRole.

### Webhook

Send an HTTP POST to any URL with the rupture payload. Useful for triggering CI/CD pipelines, Slack notifications, or custom scripts.

### Alertmanager

Raise or resolve alerts in Prometheus Alertmanager. Ruptura generates compatible alert payloads with labels, annotations, and `generatorURL`.

### PagerDuty

Create or update PagerDuty incidents with severity, rupture context, and link to the narrative explain.

## Safety gates

Ruptura enforces multiple safety gates before executing any Tier-1 action:

| Gate | Default | Description |
|------|---------|-------------|
| Rate limit | 6 / hour | Max Tier-1 actions per target per hour |
| Cooldown | 300 s | Minimum gap between two actions on the same target |
| Namespace allowlist | `[]` (all blocked) | Only act on pods in listed namespaces |
| Confidence threshold | 0.85 | Ensemble confidence required for auto-execution |
| Emergency stop | off | `POST /api/v2/actions/emergency-stop` halts all Tier-1 globally |

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
FusedR ≥ threshold (workload enters Warning/Critical/Emergency)
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
   ├── suggest → Enqueue in /api/v2/actions (pending approval, max 256 entries)
   └── auto    → Execute immediately (Tier-1) or queue (Tier-2)
        │
        ▼
Emit metric: rpt_actions_total{type,tier,outcome}
```

## Approving a suggested action

```bash
# List pending actions
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/actions

# Approve
curl -X POST -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/actions/act_abc/approve

# Reject
curl -X POST -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/actions/act_abc/reject

# Emergency stop all Tier-1 auto-actions
curl -X POST -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/actions/emergency-stop
```

## Maintenance windows

To suppress action dispatch during planned deploys (preventing false alarms):

```bash
curl -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "workload": "default/Deployment/order-processor",
    "start": "2026-05-01T14:00:00Z",
    "end": "2026-05-01T14:30:00Z",
    "reason": "rolling deploy v2.4.1"
  }' \
  http://localhost:8080/api/v2/suppressions
```

During the window, ruptures are still recorded and the narrative explain is updated — only action dispatch is suppressed. After the window, Ruptura compares pre/post baselines and reports the health delta.

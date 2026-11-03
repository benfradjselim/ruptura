# API

The engine serves HTTP on port 8080. Requests need the API key: `Authorization: Bearer <key>`. The operator stores it in the Secret `<instance>-api-key`.

| Endpoint | Returns |
|---|---|
| `GET /api/v2/failures` | Active failures, one result per failure |
| `GET /api/v2/fleet` | Every workload with its state and failure |
| `GET /api/v2/inputs` | Inputs measured per workload, and those missing |
| `GET /api/v2/coverage` | What the instance can and cannot see |
| `GET /api/v2/track-record` | How early past failures were announced |
| `GET /api/v2/event-history` | Recent changes and failures, for the timeline |
| `GET /api/v2/health`, `GET /api/v2/ready` | Probes, no API key |
| `GET /api/v2/version` | Engine version |

The engine also serves older endpoints used by the UI. They are not part of this reference and may change.

## Result

```json
{
  "level": "workload",
  "target": "demo-shop/Deployment/missing-mount",
  "pod": "demo-shop/missing-mount-776f69cf9d-zff9l",
  "failureMode": "startup_stuck",
  "kind": "detected",
  "state": "critical",
  "recommendedAction": "Read the pod events and fix the FailedMount cause",
  "evidence": ["Kubernetes reports FailedMount"]
}
```

| Field | Meaning |
|---|---|
| `target` | `namespace/Kind/name` |
| `failureMode` | See [failure modes](../failures.md) |
| `kind` | `predicted` or `detected` |
| `etaSeconds` | Predicted only: time left |
| `evidence` | The measurement behind the result |

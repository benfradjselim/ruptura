# Failure modes

Each result names one failure mode. A **predicted** failure has not happened yet and carries `etaSeconds`. A **detected** failure is happening now.

| Failure mode | Kind | Raised when |
|---|---|---|
| `saturation` | predicted | Memory of a container is on course to reach its limit within the horizon. Needs a memory limit and at least 10 minutes of samples. |
| `recurrent_failure` | predicted | A workload failed at least 6 times at regular intervals; the next failure is expected one interval after the last. |
| `oom_killed` | detected | The last exit of a container was `OOMKilled`, in the last 30 minutes. |
| `crash_loop` | detected | At least 3 restarts in the last 30 minutes. |
| `startup_stuck` | detected | Kubernetes reports a pod that cannot start: image pull failure or `FailedMount`. |
| `replicas_unavailable` | detected | Deployment, StatefulSet or DaemonSet missing replicas for over 5 minutes. |
| `rollout_stuck` | detected | A Deployment passed its progress deadline. |
| `cpu_throttling` | detected | A container throttled at least 25% of the time for 10 minutes. Clears after 5 minutes below. |

## State

| State | Meaning |
|---|---|
| `critical` | The failure is happening or close |
| `degraded` | The failure is expected, with time to act |
| `calibrating` | Not enough history yet to judge |

## What Ruptura cannot see

A workload without a memory limit cannot be forecast for `saturation`. `GET /api/v2/coverage` lists what each instance can and cannot measure on your cluster.

# Failure modes

Each result names one failure mode. A **predicted** failure has not happened yet and carries `etaSeconds`. A **detected** failure is happening now.

A predicted `saturation`, `release_regression` or `replica_divergence` also carries `etaEarlySeconds` and `etaLateSeconds`: the earliest and latest failure, from the first and third quartiles of the growth rates the readings show. `GET /api/v2/failures/series?workload=<namespace/kind/name>` returns the memory readings behind the forecast, which the web console draws with the trend and the range.

| Failure mode | Kind | Raised when |
|---|---|---|
| `saturation` | predicted | Memory of a container is on course to reach its limit. Needs a memory limit and at least 10 minutes of samples. Not raised when the climb is slowing down (a new version warming up), nor when usage already came back down by itself at least twice and stays within its earlier peaks (a load that repeats). |
| `memory_beyond_usual` | predicted | A pod with no memory limit is on course to grow past the usual level of its workload: the namespace default (`spec.analysis.defaultMemoryLimits`), else the 7-day 99th percentile plus 20%. Nothing kills the pod there, so it is never reported as an OOM kill. |
| `release_regression` | predicted | A new version of a Deployment grows toward its memory limit at least 3 times faster than the version it replaced, for 5 minutes past a 6 minute warm-up. Reported up to 2 hours ahead, with both rates and a roll back as action. |
| `replica_divergence` | predicted | One of at least 3 replicas of a version grows at least 3 times faster than the others, for 5 minutes. Reported up to 2 hours ahead and names the pod. Silent when all replicas grow together. |
| `recurrent_failure` | predicted | A workload failed at least 6 times at regular intervals; the next failure is expected one interval after the last. |
| `oom_killed` | detected | The last exit of a container was `OOMKilled`, in the last 30 minutes. A recent OOM kill names the failure even when the pod also crash loops or misses replicas. |
| `crash_loop` | detected | Kubernetes reports a back-off restarting a failed container, or at least 3 restarts in the last 30 minutes. A loop of OOM kills with a regular rhythm is reported as `recurrent_failure`, predicted, with the next kill. |
| `startup_stuck` | detected | Kubernetes reports a pod that cannot start: image pull failure or `FailedMount`. |
| `replicas_unavailable` | detected | Deployment, StatefulSet or DaemonSet missing replicas for over 5 minutes, or at once when the scheduler reports it cannot place a pod (`FailedScheduling`). |
| `rollout_stuck` | detected | A Deployment passed its progress deadline. |
| `cpu_throttling` | detected | A container throttled at least 25% of the time for 10 minutes. Clears after 5 minutes below. |

## What a failure can take down

The engine reads which workloads call which from what the cluster declares: a container environment value that names a Service, and Routes or Ingresses that send traffic to one. `GET /api/v2/callgraph` returns the graph. Each failure lists in `affects` the workloads that call it, directly or not, adds an evidence line, and sends an `impact` annotation to Alertmanager. These are declared calls; calls seen in traces or network flows come later.

## State

| State | Meaning |
|---|---|
| `critical` | The failure is happening or close |
| `degraded` | The failure is expected, with time to act |
| `calibrating` | Not enough history yet to judge |

Pods that are terminating, or that belong to a ReplicaSet a Deployment has replaced, raise no finding.

## What Ruptura cannot see

A workload without a memory limit cannot be forecast for `saturation`; it is watched for `memory_beyond_usual` instead. `GET /api/v2/coverage` lists what each instance can and cannot measure on your cluster.

How each mode decides, and what is measured, is in [How Ruptura decides](design.md).

# How Ruptura decides

Ruptura names a failure only when a fixed rule, written down here, says so. There is no learned model to trust: each failure mode reads a few signals your cluster already has and applies its own test. This page describes those tests, the guards against false warnings, and their known limits.

## Inputs

| Source | What is read |
|---|---|
| Prometheus or Thanos | `container_memory_working_set_bytes`, `kube_pod_container_resource_limits`, `container_cpu_cfs_throttled_periods_total` and `container_cpu_cfs_periods_total`, `kube_pod_container_status_restarts_total` |
| Kubernetes API | Pods, ReplicaSets, Deployments, StatefulSets, DaemonSets, events (`BackOff`, `FailedMount`, image pull errors, `FailedScheduling`), Services, Routes and Ingresses for the call graph |

The engine polls every 30 seconds. Every failure it lists comes from one of these inputs; when an input is missing, `GET /api/v2/coverage` says so instead of reporting a healthy cluster.

## Predicted failures

**Saturation.** For each container with a memory limit, the engine keeps up to 30 minutes of readings and fits a robust slope (Theil-Sen: the median of the slopes between every pair of points, so one spike cannot decide). It forecasts the time at which the limit is reached only when:

- the readings span at least 10 minutes, so one ramp of a sawtooth is not read as a trend;
- at least 80 percent of the pairwise slopes that are not flat show growth;
- the last third of the window still grows at least half as fast as the whole window. A new version that warms up and then levels off fails this test; a leak keeps its pace and passes it;
- usage has not already fallen back by itself, by at least 10 percent of the limit, twice in the last 6 hours while staying no more than 5 percent of the limit above its earlier peaks. A load that rises and falls every hour is a cycle, not a leak. The guard needs two such falls to learn the cycle, so the first cycles can still raise a forecast.

The forecast carries a range (`etaEarlySeconds`, `etaLateSeconds`) from the first and third quartiles of the slopes.

**Memory beyond usual.** A pod with no memory limit has no point of failure to forecast. Ruptura compares it with a ceiling instead: the namespace default set in the `RupturaInstance`, else the 99th percentile of the workload's memory over 7 days plus 20 percent. The same trend test applies. Since nothing kills the pod at that ceiling, the result is reported as unusual growth.

**Release regression.** A pod of a new Deployment version, between 6 minutes and 2 hours old, that grows at least 3 times faster than the previous version did when steady, for 5 minutes. The comparison with the previous version makes a forecast up to 2 hours ahead credible, with a roll back as the action.

**Replica divergence.** One replica of at least 3 growing at least 3 times faster than the others for 5 minutes. When all replicas grow together, this mode stays silent and saturation judges the trend.

**Recurrent failure.** A workload that failed at least 6 times at regular intervals (spread under 20 percent of the median interval) is expected to fail again one interval after the last failure. The rhythm counts as broken once a failure is 1.5 intervals overdue.

## Detected failures

Detected failures are happening now: `oom_killed`, `crash_loop`, `startup_stuck`, `replicas_unavailable`, `rollout_stuck` and `cpu_throttling`. Their rules are in [Failure modes](failures.md). When several apply to one workload, the cause outranks the symptom: a recent OOM kill is named `oom_killed` even when the pod also crash loops.

## Guards

- **Pods on their way out.** A terminating pod, or one from a ReplicaSet the Deployment has replaced, raises no finding. A rollback therefore stops warning about the bad version.
- **Engine restarts.** The recent readings are saved every minute and at shutdown, and restored at start when they are less than 30 minutes old. A forecast in progress survives a restart; after a longer outage the engine starts cold and needs about 10 minutes of readings.

## How it is measured

Each change to these rules is measured by a test written before its run, against `predict_linear` alerts and the rules OpenShift ships, and the result is published whether it passes or fails. The full benchmark, scripts and raw results are in [`benchmark/`](https://github.com/benfradjselim/ruptura/tree/main/benchmark) and the notes of each release.

## Known limits

- On a release that leaks fast, `predict_linear` warns a few minutes earlier, at the cost of firing on many healthy pods.
- Right after a rollback, the engine can still list the version being removed until its next poll, up to 30 seconds.
- Throttling is detected, not predicted, and only CPU and memory are covered. Disk, network and node pressure are not yet.
- Calls between workloads come from what the cluster declares (environment values naming a Service, Routes, Ingresses), not from traffic.

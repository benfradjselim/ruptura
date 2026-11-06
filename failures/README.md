# Failure catalog

An open catalog of the failures that hit Kubernetes and OpenShift clusters, one card per failure. Each card says what the failure looks like, how to detect it, whether it can be seen coming, what it hits next, what to do, and where Ruptura stands on it.

The catalog is plain YAML, one file per level. Anyone can add a card or correct one.

## Card

```yaml
- id: memory-leak-to-oom            # unique, kebab-case
  title: Memory leak ends in an OOM kill
  level: container                  # container, pod, workload, namespace, node, network, storage, control-plane, cluster, application
  symptom: Container memory grows until the kernel kills it at its limit
  impact: The container restarts, in-flight requests fail
  detect:
    inputs: [container_memory_working_set_bytes, kube_pod_container_resource_limits]
    rule: Last termination reason OOMKilled
  predict:
    possible: yes                   # yes, partial, no
    precursor: Steady or accelerating growth toward the limit
  propagates_to: [callers of the workload]
  action: Raise the memory limit or fix the leak
  sources:                          # alert rules, Kubernetes reasons, documentation
    - kubernetes-mixin KubePodCrashLooping
  ruptura:
    status: tested                  # tested, implemented, planned, not-covered
    mode: saturation                # the engine failure mode, when there is one
```

`status` reads:

- `tested`: implemented and measured by a pre-registered test on the lab. Results are published with each benchmark.
- `implemented`: the engine reports it, without a pre-registered test yet.
- `planned`: on the roadmap.
- `not-covered`: known, not handled.

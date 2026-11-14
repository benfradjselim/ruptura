# Ruptura benchmark

Scripts used to measure Ruptura against the alert rules a team already has:
Prometheus `predict_linear` on container memory and a detection rule set with their own `for`
delays. They run on a single-node OpenShift lab with injected faults.

## Rules

- Every test was written, with its pass criteria in the script header, before any of its data.
- A setting is never changed after a result. A change needs a dated amendment written before the
  window it affects.
- A run is void if the engine restarts inside its window or a script fails. A void run is rerun
  once with the same rules, never rescored with other rules.
- Each run is scored once. Losses are published like wins.

The files here are byte-identical to the frozen versions used for scoring:

| File | sha256 (first 16) | Part |
|---|---|---|
| `checkpoint.py` | `bf7bf83cd4dc0d12` | A, memory failures against `predict_linear` 30m/30m and 10m/30m |
| `checkpoint-preflight.sh` | `93c3c4ec821c5029` | Terrain check before a window |
| `benchmark-detect.py` | `cb383299492c50b1` | B, detection of 6 faults over 3 rounds |
| `opponent-rules.yaml` | `e058772596035bf2` | B and C, the rules a user writes without Ruptura |
| `benchmark-rollout.sh`, `score-rollout.py` | `8a3bd91b64c0683b`, `df7512dd65eb612a` | B, stuck rollout |
| `quiet.py` | `9bc9ed7a32fa764d` | C, findings on healthy workloads; D, engine cost |
| `benchmark-replica.sh` | `1f0104eef6296c9c` | F1, one replica of three diverges |
| `benchmark-release.sh` | `094969d755d3144d` | F2, a bad release beside healthy and heavier ones |
| `score-variant.py` | `6936953635bb5d97` | F1 and F2 scoring |
| `benchmark-faults.yaml` | `3a1e78a193f6acf3` | Fault workloads (leaks, periodic spike, crash loop, bad image, missing mount, throttling), amended 14 Nov, see below |
| `shop-app/` | | A small three-tier app that runs beside the faults |

### Amendment, 14 November 2026, before the main benchmark day

The faults that restart after an OOM kill wrote files to a memory-backed `emptyDir`. Those files
outlive the killed container, so the pod's memory cgroup stays full and the kernel kills the container
runtime while it creates the next container; the pod then sticks in `CreateContainerError`. This
happened three times on 14 November and would void a 12-hour window. The five restarting faults
(step-leak, oom-spike, accel-leak, periodic-spike, leak-orders) now hold anonymous Python memory with
the same sizes, pace, timing and limits; the process starts about 10 MiB higher than a shell. The
healthy control and all scoring scripts are unchanged. Previous version of the file: sha256
`aa62b44f704aaf15...`.

Results and raw outputs are added to this folder when the benchmark is frozen, on 17 November 2026.

## Running it on your cluster

The scripts are written for an OpenShift node shell (in our lab, `/root/lab.sh` sends a script
to the node over ssh). They expect:

- `oc` logged in with cluster-reader rights, and Prometheus or the Thanos querier reachable as
  `localhost:9090` (for example with `oc port-forward`).
- Ruptura installed, its API reachable, and an API key in `/var/tmp/.k`.
- Edit the engine Service address at the top of each script for your cluster.

Apply `benchmark-faults.yaml`, let every fault run for a few hours so it has history, run
`checkpoint-preflight.sh`, then each part as described in its header.

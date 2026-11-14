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
| `benchmark-faults.yaml` | `aa62b44f704aaf15` | Fault workloads (leaks, periodic spike, crash loop, bad image, missing mount, throttling) |
| `shop-app/` | | A small three-tier app that runs beside the faults |

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

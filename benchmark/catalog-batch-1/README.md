# Catalog batch 1: workload, node and namespace cards

Seven failure cards that take Ruptura beyond pods: autoscalers, disruption budgets, Jobs, node filesystems, node memory, node conditions and namespace quotas. Each card is data in the engine catalog (`workdir/internal/sources/catalog.yaml`, section `cards`), read without card-specific code. This page holds the lab tests of those cards, run on 25 November 2026 on a single-node OpenShift lab, under the rules of the [benchmark](../README.md): criteria written before any data, each run scored once, losses published like wins.

## Cards

| Card | Failure mode | Level | Kind | Rule |
|---|---|---|---|---|
| `hpa-maxed-out` | `hpa_maxed_out` | workload | detected | Current replicas at or above the HPA maximum for 10 min; clears after 5 min below |
| `pdb-blocks-drain` | `pdb_blocks_drain` | workload | detected | PDB allows 0 disruptions while it expects at least one pod, for 10 min; clears after 2 min |
| `job-failed` | `job_failed` | workload | detected | Job has its `Failed` condition; clears when the Job is gone |
| `node-filesystem-filling` | `node_filesystem_filling` | node | predicted | Used bytes of a node filesystem toward its size; horizon 6 h |
| `node-memory-exhaustion` | `node_memory_exhaustion` | node | predicted | Node memory in use toward its total; horizon 1 h |
| `node-condition` | `node_not_ready`, `node_*_pressure` | node | detected | `Ready` false, or a pressure condition true, for 2 min |
| `resource-quota-exhausted` | `quota_exhaustion` | namespace | predicted | Used toward hard, per quota and resource; horizon 6 h |

Node memory exhaustion and node conditions would take a single-node lab down, so they are not tested here.

## Tests and criteria

Written before the code. A card is `tested` only when every check of its test passes. The engine is polled every 30 s through `GET /api/v2/failures`.

| Test | Setup | Pass criteria |
|---|---|---|
| T1 HPA | Deployment with a CPU loop, HPA min 1 max 2 at 50 % CPU; control HPA min 1 max 3 on an idle Deployment | Raised 10 to 11 min after current replicas reach max; cleared within 6 min after they drop below max; control silent |
| T2 PDB | Deployment of 1 replica with PDB `minAvailable: 1`; control Deployment of 2 replicas with PDB `maxUnavailable: 1` | Raised 10 to 11 min after the PDB is created; cleared within 3 min after it is deleted; control silent |
| T3 Job | Job that exits 1 with `backoffLimit: 0`; control Job that exits 0 | Raised within 2 min of the `Failed` condition; cleared within 2 min after the Job is deleted; control silent |
| T4 node filesystem | 512 MiB loop filesystem on the node, filled 2 MiB every 15 s, fill stops at 95 %; control loop filesystem with no writes | First forecast at least 15 min before full; every forecast from 20 min before full has the true time inside its early-late range or within 25 % of the time left; control silent |
| T5 quota | ResourceQuota `configmaps: 60`, one ConfigMap every 30 s; control quota with 5 static ConfigMaps | First forecast at least 10 min before full; ETA within 25 % from 15 min before full; control silent |

Amendment before run 1 and its data: T1 clear is measured from the drop below max, because the HPA scale-down stabilisation window delays it; T3 allows 2 min instead of 1, because kube-state-metrics is scraped about once a minute (on this lab it turned out to be every 2 min); T5 uses its own namespaces so other test objects do not count against the quota.

The scorer defines "full" for T4 as `df` used bytes reaching the filesystem size, extrapolated by a least-squares line, and for T5 as the first poll with 60 ConfigMaps.

## Results

### Run 1: 10 of 15

Engine from commit `bd6d909`.

| Check | Result |
|---|---|
| T1 raise | FAIL, 11.3 min |
| T1 clear | FAIL, 6.1 min |
| T2 raise | FAIL, 12.3 min |
| T3 raise and clear | PASS, 1.9 and 1.2 min |
| T4 first forecast | PASS, 49.2 min before full |
| T4 accuracy | FAIL, ETA frozen at 202 s for the last 5 samples, after the fill stopped |
| T5 first forecast | PASS, 17.0 min before full |
| T5 accuracy | FAIL, ETA 26 to 40 % late |
| T2 clear | PASS |
| All controls | PASS |

Cause, read from the data and the lab configuration after scoring: OpenShift scrapes kube-state-metrics every 2 minutes, so the engine read HPA, PDB and quota values up to 2 minutes old and dated them by its own poll. Separately, the disk forecast kept its slope after writes stopped.

### Rerun 1: void for T1 to T3, 10 of 16

Engine fix [`6f20fff`](https://github.com/benfradjselim/ruptura/commit/6f20fff), registered before the rerun with the same criteria and one added check (T4 stop: once the fill stops, the forecast is withdrawn within 2 min and stays withdrawn). The fix dates each reading by its source sample (`timestamp()` in PromQL), places a state change halfway between samples, counts forecasts down from the sample time, and withdraws a trend that pauses longer than it did while rising.

T1, T2 and T3 did not run: the run script did not create its test namespace (`results/rerun1-run.log`), so their passing clear and control checks count for nothing. The script now creates it and stops if it cannot.

| Check | Result |
|---|---|
| T4 first forecast | PASS, 49.7 min |
| T4 accuracy | FAIL, last 4 samples about 80 s early |
| T4 stop | PASS |
| T5 first forecast | PASS, 18.6 min |
| T5 accuracy | FAIL on the last sample only: ETA 0 s, 1 s left |
| T4 and T5 controls | PASS |

### Rerun 2: 13 of 16

Engine [`70d0757`](https://github.com/benfradjselim/ruptura/commit/70d0757) (the fix above plus review follow-ups), all five tests, same criteria and scorer, registered before the run.

| Check | Result |
|---|---|
| T1 raise | PASS, 10.9 min |
| T1 clear | PASS, 4.7 min |
| T2 raise | FAIL, 11.9 min |
| T2 clear | PASS, 2.4 min |
| T3 raise and clear | PASS, 1.9 and 1.3 min |
| T4 first forecast | PASS, 50.2 min |
| T4 accuracy | FAIL, last 5 samples about 80 s early |
| T4 stop | PASS |
| T5 first forecast | PASS, 17.3 min |
| T5 accuracy | FAIL on the last sample only: ETA 3 s, 1 s left |
| All controls | PASS |

No control raised a finding in any run.

### Reading, after scoring

- T2: the PDB was blocking 30 s after creation; the engine raised 11.4 min after that. With a 2-minute scrape, when a state changed is known only to within about a minute, which can miss a 1-minute window. This limit was stated before rerun 2.
- T4: the engine forecasts when free space reaches zero, which is when writes fail. On this ext4 filesystem, used plus available is 10.7 MB below the size (metadata, no reserved blocks), about 77 s at this fill rate: the size of the error. The scorer's definition of full makes the gap; the FAIL stands as registered.
- T5: every reading is within 25 % except the last one, a second before the quota filled.

### Card status

| Card | Status |
|---|---|
| `job-failed` | tested |
| `hpa-maxed-out` | tested |
| `pdb-blocks-drain` | implemented, raise outside its window |
| `node-filesystem-filling`, `resource-quota-exhausted` | implemented, accuracy fails on the last readings |
| `node-memory-exhaustion`, `node-condition` | implemented, not testable on a single node |

## Files

| File | sha256 (first 16) | Use |
|---|---|---|
| `batch1.sh` | `6ede0f1ac93d53a4` | Run script, as used for rerun 2. Run 1 and rerun 1 used the earlier version `94d72f696f4445a9`, which did not create the test namespace |
| `score-batch1.py` | `e41a1f94d93bd3ef` | Scorer, as used for both reruns. Run 1 used the earlier version `9ea82370a0d87831`, without the T4 stop check |
| `results/<run>-api.jsonl` | | Every `/api/v2/failures` answer, every 30 s |
| `results/<run>-truth.jsonl` | | Ground truth read from the cluster at each poll |
| `results/<run>-events.jsonl` | | What the run script did and when |
| `results/<run>-score.txt` | | Scorer output |

The lab also runs other workloads. The `api` files keep only findings about the test objects, which are the only ones the scorer reads, and the node name is replaced by `lab-node`. Scoring these files gives the same result as the full ones, line for line.

To score a run: copy its three files into a folder as `api.jsonl`, `truth.jsonl` and `events.jsonl`, then run `python3 score-batch1.py <folder>`.

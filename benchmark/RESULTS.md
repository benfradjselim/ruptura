# Ruptura benchmark results, 16 November 2026

## In plain words

**The question.** When an application on Kubernetes is about to crash, can a tool warn you *before* it
happens, with enough time to act, and without crying wolf? We compared Ruptura with what most teams
already have for free: alert rules in Prometheus, the standard monitoring tool (`predict_linear`).

**How.** On a test cluster we caused real failures again and again for 12 hours (programs that slowly use
up their memory until they crash, failures that come back every 45 minutes, a new version that leaks,
pods that cannot start). We wrote the rules of the test before running it and published them, so we could
not adjust anything afterwards. Losses are published like wins.

**What Ruptura wins**
- **Memory running out**, the most common crash: Ruptura warned **15 minutes** ahead on average, the
  classic rule **7 minutes**, and Ruptura raised **no false alarm**. Tuned to be faster, the classic rule
  warns earlier but raised **67 false alarms** in 12 hours.
- **Failures that come back on a schedule** (every 45 minutes): Ruptura saw **14 out of 14** coming.
  The classic rules saw **none**.
- **One copy of an application leaking while its twins stay healthy**: Ruptura warned **57 minutes**
  before the crash, the classic rules **30 minutes**.
- **Applications that cannot start** (missing image, missing file, crash at start): spotted in
  **about 30 seconds**, against **16 to 18 minutes** for the classic rules.
- **A stuck software update**: spotted every time, within seconds to two minutes.

**What Ruptura loses**
- **A new version that leaks fast**: the classic rule warned about **10 minutes earlier**, and Ruptura
  raised some false alarms during that test.
- **Two failures named wrong**: Ruptura saw them, even earlier than the rules, but gave them the wrong name.
- **Healthy applications**: Ruptura flagged a few more real CPU slowdowns than the classic rules did.

**In one sentence.** On what Ruptura is built for, warning before a crash, it beats the free tool everyone
already has, without false alarms; its losses are specific, known, and being fixed.

---

## Detailed results

Ruptura against the alert rules a team already has, on the same OpenShift lab, the same injected
faults and the same windows. Rules, scripts and criteria were published before the runs
(see [README](README.md), including the 14 November amendment). Each part was scored once.
Losses are reported like wins.

## Summary

| Part | What | Verdict | One line |
|---|---|---|---|
| A | Memory failures against `predict_linear` | **PASS** (C1, C2, C3) | 65 of 73 leak cycles warned, median 15.4 min vs 7.0 for the quiet rule, 0 false; 14 of 14 periodic spikes vs 0 |
| B | Detection of 6 faults, 3 rounds, plus a stuck rollout | **FAIL** (4 of 6 faults pass; stuck rollout PASS) | Startup faults in 18 to 35 s vs 16 to 18 min for the rules; replicas unavailable and OOM kill named under another mode |
| C | Findings on healthy workloads, 12 h | **FAIL** | 8 findings vs 5 for the rules; all 8 real CPU throttling, 0 false |
| D | Engine and operator cost | report only | Engine CPU 46m mean, 96m peak; 442 MiB peak memory |
| E | What a user writes | report only | 1 PrometheusRule (8 rules, 53 lines) + 2 cluster settings vs 1 RupturaInstance (22 lines) |
| F1 | One replica of three leaks, run 13 Nov | **PASS** (R1, R2, R3) | Warned 7.6 to 8.1 min after each leak start; median lead 57.2 min against 29.8 for `predict_linear`; 0 false |
| F2 | A bad release beside healthy and heavier ones, run 14 Nov | **FAIL** (R1 pass, R2 fail, R3 fail) | Warned 11 to 12 min after each bad rollout, but `predict_linear` led by 9.6 min more; about 14 min of false warnings on heavy rollouts and rollbacks |

## Run facts

- Cluster: single-node OpenShift 4.22 lab. Engine: Ruptura 8.7.8, published image, started
  2026-11-11 22:31 UTC, no restart during any scored window.
- Part A, C, D window: 2026-11-16 00:00 to 12:00 UTC. Preflight READY at 07:33 UTC (planned 23:45 UTC on 15 November; it ran late, the window was kept as registered and checked undisturbed) and at 12:10 UTC.
- Part B: 12:12 to 13:40 UTC. Stuck rollout: 14:17 to 15:17 UTC.
- Events during windows: none found. No engine restart, no API server revision (last one 15 November 10:19 UTC), no blocked or evicted fault pod, fleet log continuous.

## A. Memory failures

Contestants: `predict_linear` on working set against the limit, 30 min window and 30 min horizon,
and 10 min window and 30 min horizon; Ruptura with saturation forecasts only, and with its failure
memory added. Fresh lead (primary): the first warning inside each cycle. A lead under 5 min counts as missed.
Scored cycles: leak-orders 21, step-leak 20, accel-leak 32, oom-spike 68 (reported, not judged), periodic-spike 14.

| Contestant | leak-orders | step-leak | accel-leak | oom-spike | periodic-spike | Median fresh lead | False episodes, cluster |
|---|---|---|---|---|---|---|---|
| `predict_linear` 30m/30m | 18 of 21 | 20 of 20 | 0 of 32 | 0 of 68 | 0 of 14 | 7.0 min | 0 |
| `predict_linear` 10m/30m | 21 of 21 | 20 of 20 | 32 of 32 | 0 of 68 | 0 of 14 | 21.5 min | 67 on 3 workloads (healthy sawtooth-batch 61) |
| Ruptura saturation | 21 of 21 | 18 of 20 | 26 of 32 | 0 of 68 | 0 of 14 | 15.4 min | 0 |
| Ruptura with failure memory | 21 of 21 | 20 of 20 | 32 of 32 | 14 of 68 | 14 of 14 | 18.3 min | 2 on 2 workloads |

| Criterion | Measured | Verdict |
|---|---|---|
| C1 trends | Ruptura saturation 65 of 73, median 15.4 min; `predict_linear` 30m/30m 38 of 73, 7.0 min (needed as many and 5 min more) | PASS |
| C2 noise | Ruptura with failure memory 2; `predict_linear` 10m/30m 67 (at most half); 30m/30m 0 (at most 2 more) | PASS |
| C3 periodic spike | Ruptura with failure memory 14 of 14 (at least 60 %); each `predict_linear` 0 (at most 1) | PASS |

## B. Detection

| Fault | Ruptura median | Rule median | Expected mode in 3 of 3 rounds | Verdict |
|---|---|---|---|---|
| crash loop | 35 s | 1093 s | yes | pass |
| image not found | 34 s | 968 s | yes | pass |
| missing mount | 18 s | 968 s | yes | pass |
| replicas unavailable | 1223 s | 968 s | no, startup_stuck listed | FAIL |
| CPU throttling | 742 s | 1015 s | yes | pass |
| OOM kill | 192 s | 667 s | no, crash_loop twice and replicas_unavailable once | FAIL |
| stuck rollout (4 criteria) | rollout_stuck 31, 16, 123 s after Progressing=False; cleared 16, 0, 91 s after undo | | healthy rollouts silent | PASS |

Known before the run: the replicas fault (one replica unschedulable) was listed as `startup_stuck`
in the dry run, not `replicas_unavailable`; the OOM rule has no `for` delay, so a tie was the best case.

## C. Quiet

Findings on every workload with no injected fault, all namespaces, same window as A.
Ruptura 8 findings on 3 workloads (cpu_throttling: boutique/inventory 1, openshift-network-operator/iptables-alerter 4,
ruptura-h/otel 3); rule set 5 on 2 workloads (ruptura-h/otel CPUThrottlingHigh 4, an image build pod KubeContainerWaiting 1).
Classified by hand from the share of CPU periods throttled over the window: 37.7 %, 99.9 % and 37.0 % at peak, above
the 25 % of the shipped OpenShift rule, so all 8 are real and 0 false (at most 3: met). The first criterion, no more
findings than the rules, fails: 8 against 5.

## D. Cost

Engine: CPU mean 46m, highest 5 min rate 96m; working set highest 442 MiB (limit 512 MiB). Operator: not measured,
the benchmark engine runs without the operator on this lab.

## E. Configuration effort

Without Ruptura: 1 PrometheusRule holding 8 rules, 53 lines, plus 2 cluster settings (user workload monitoring
enabled, an Alertmanager route). With Ruptura: 1 RupturaInstance, 22 lines, 3 of them optional (console plugin).

## F1. Diverging replica (13 November)

Three replicas, one starts leaking, three rounds, 08:27 to 16:45 UTC.

| | Ruptura | `predict_linear` 30m/30m and 10m/30m |
|---|---|---|
| Warned after the leak start | 8.1, 7.6, 8.1 min, mode replica_divergence, naming the leaking pod | 35.0, 34.5, 35.5 min |
| Lead before the OOM kill (rounds 2 and 3; round 1 reached no OOM in the run) | 57.7, 57.2 min | 30.7, 29.8 min |
| False | 0 | fired on 3 calm pods |

R1 (warned within 15 min, naming the pod), R2 (median lead at least 20 min longer), R3 (never listed
on calm phases): PASS. Same picture as the 4 November run (57.8 against 30.4 min).

## F2. Bad release (14 November)

Per round: a healthy version, a heavier but stable version, back to v1, then a version that leaks
until both pods are OOM killed, then rollback. Three rounds, 06:00 to 13:42 UTC.

| | Ruptura | `predict_linear` 30m/30m and 10m/30m |
|---|---|---|
| Warned after the bad rollout | 12.2, 11.2, 11.6 min (release_regression, then saturation) | 2.0, 1.0, 2.0 min |
| Median lead before the OOM kill | 55.2 min | 64.8 min |
| False | 28 samples of 30 s: saturation on heavy rollouts (13), saturation and release_regression during rollbacks (15) | fired on 38 and 32 calm pods |

R1 (warned within 15 min): PASS. R2 (median lead at least 20 min longer): FAIL. R3 (no false warning): FAIL.
A trend rule reacts to every new version's warm-up, so it is first on a bad release and wrong on most
healthy ones. Ruptura's own false warnings come from a heavy version's warm-up and from forecasts left
up during a rollback; both are being fixed and will be measured again under a new pre-registered test.

## Raw files

In `results/`: `checkpoint-20261116.txt` (A), `quiet-20261116.txt` (C, D), `detect-20261116.jsonl` (B),
`rollout-20261116.jsonl`, `rollout-phases-20261116.log`, `score-rollout-20261116.txt` (stuck rollout),
`replica-20261113.jsonl`, `replica-phases-20261113.log`, `score-f1-20261113.txt` (F1),
`release-20261114.jsonl`, `release-phases-20261114.log`, `score-f2-20261114.txt` (F2).

# STATE

Read this file and the active lot file. Do not read the whole repository.

## Handoff

This section is how sessions pass work to each other. Read it first, continue from "Next action".

Rewrite it before any wait over 10 minutes, after each deploy, and at the end of a session. A session can stop without warning, so never leave this for the end. Replace the old content and keep it short. Numbers must come from a real measure.

- Updated: 2026-11-01 02:10 UTC
- Direction from the owner 2026-11-01: reanalyse the formulas, keep only signals proven useful, consume broadly, clear system design. Read `docs/engine/DESIGN.md`, draft awaiting owner review, its phases replace the failure mode list below.
- Plan: fix the engine first. Other components (Helm, operator, UI, CLI, site) start only once the engine passes its gate. Deadline 2026-11-24, Red Hat Summit Paris.
- Work location: `/root/ruptura-fix`, branch `fix/engine-prediction`, pushed. Push with `git -c credential.helper= -c 'credential.helper=!gh auth git-credential' push`, plain push has no credentials.
- Lab: OpenShift, reach it with `/root/lab.sh`, not the Civo kubeconfig. Deploy with `/root/redeploy.sh`, which deletes `/var/tmp/fleet.jsonl`, so copy that file first.
- `gh api` and `gh run` are blocked by policy. Do not bypass this, verify locally.
- Active gate: LOT-1, alerts reduced by at least 90 percent. Baseline is at least 30 alerts per minute (capped 1000 in 33 min, `14a97f1`), so the target is 3 per minute or less.
- Measures: `14a97f1` 30 per minute or more. `7793f07` 7.1 per minute (320 in 45 min).
- `dc971d6` measured: 667 alerts in 45 min, 14.8 per minute, worse than `7793f07`. 568 of 667 from `network.io` and `cpu.time`. Last hour on the lab (19:32 to 20:31 UTC): 1000 alerts, API cap reached, 885 on those counters, 89 series, 609 distinct episodes.
- Findings so far: one observation can publish up to 3 alerts, one per method. The 5 minute cooldown is the main limit on the flow, gaps peak at 5 minutes. 771 of 885 values are above expected, 53 are zero. Noisiest workloads have one pod, so replica mixing is not their cause.
- Cause found: bursts are real, not an artifact. Kubelet counters sampled 15 s for 12 min on the 4 noisiest workloads: rx medians 27 to 990 B/s match the engine's expected values, and 44 of 172 rx samples exceed 5 times the median. The 15 s rate of idle pods is heavy tailed, so the MAD and z-score tests fire on normal traffic. The rate code itself is correct.
- Side defect: `handleOTLPMetrics` drops data point attributes, so rx and tx share one key and `counterRates` drops the second point at the same timestamp. Only one direction is scored.
- Fix: `0f6335d` counters scored on a 5 minute rate, one alert per observation, deployed 21:53 UTC, not pushed. Previous data saved as `/var/tmp/*-20261031-dc971d6.*` on the lab.
- `0f6335d` measured 21:53 to 22:39 UTC: 142 alerts, 3.16 per minute, about 2.4 per minute after 22:20 (45 alerts in 19 min). Seasonal detector has no alerts yet because it needs 1 hour of warm-up, so this figure is incomplete and the gate is not passed.
- Demo checks passed over the run: crash-pay critical, stable-web healthy, leak-orders reached degraded and critical with etf 1278 to 48 s, failure_mode saturation.
- `0f6335d` warm hour 22:54 to 23:54 UTC: 233 alerts, 3.88 per minute, gate failed. 86 came from the seasonal method alone. Demo states still correct.
- `dropUnconfirmedSeasonal` commit deployed 00:37 UTC, pushed. Expected near 2.45 per minute if the same alerts recur, not measured yet. Data saved as `/var/tmp/*-20261101-0f6335d.*` on the lab.
- Next action: after 02:37 UTC run `/root/lab.sh < scripts/lab-eval/measure-alerts.sh`, it counts the second hour after the deploy. Target 3 per minute or less, and 0 false alarms on stable-web, which is part of the LOT-1 gate: it had 3 anomaly alerts in the 22:54 hour, seasonal 2 and zscore 1, so check it explicitly. Also confirm leak warned at least 10 min before OOM on 3 cycles. Then metric coverage review, then the audit steps in order.
- Then: review metric coverage, owner requires it before leaving the engine. Inventory done 2026-11-01, no code changed: the lab Prometheus exposes 2594 metric names, `sources/catalog.yaml` reads 5 of them (0.2 percent), plus kubeletstats over OTLP. Most names are internal and not useful.
- Signals finding 2026-11-01, one reading of `/api/v2/metrics` on 98 workloads: contagion, fatigue, humidity, mood, resilience, throughput are 0 everywhere, pressure is 0.5 everywhere. Alive: stress (max 0.30), entropy, velocity, health_score (min 0.79, above the 0.70 degraded limit). The analyzer reads `error_rate`, `timeout_rate`, `latency`, `load_avg_1`, but `mapToAnalyzerSignals` in `cmd/ruptura/main.go` maps only cpu and memory, and the lab collector sends only kubeletstats, no traces or logs. Degraded and critical on the demos appear to come from failure modes and restarts, not from these signals, to confirm in code. FRI also disagrees: `/metrics` shows 10 on 25 workloads while `/fleet` shows 3 nonzero. Owner expects the 10 signals to be the core of the product, so feeding them comes first in the coverage work, before any new failure mode.
- Proposed failure modes, by priority, pending owner approval: volume full (`kubelet_volume_stats_*`, 3 names), container stuck waiting (`kube_pod_container_status_*`, 8), node pressure (`kube_node_status_condition`), ephemeral disk (`container_fs_*`, 17), HTTP errors (`haproxy_backend_http_*`, 4), etcd slowdown (`etcd_disk_*`, 17). Each needs its own gate.

## Maintainability audit 2026-10-31
Owner approved 2026-10-31: once LOT-1 passes, do these in order, after the metric coverage review, one commit per step:
- Merge the near identical `internal/predictor` and `internal/pipeline/metrics` (7 files differ by 2 to 15 lines).
- Fix docs that disagree with code: README 5 models (verify EWMA and the 60 s reweighting), ARCHITECTURE places the ensemble in `fusion/`, `components.yaml` lists a missing `internal/normalize`.
- Raise tests on `internal/actions/providers` 34.8 percent and `internal/api` 39.1 percent. `gofmt` flags 87 files.
- Check `processor`, `notifier`, `billing`, `vault`: no import in this repository, Autopilot may use them.
- One maintainer listed. CI status unverified because `gh run` is blocked.

## History
- Baseline 2026-10-30: leak predicted 0/2, FRI > 0 on 0/98 hosts, control false alarms 0.
- Latest: FRI > 0 on 18/98 hosts after M1. M2 and M5 are merged here, forecast GET returns 200 on the lab.
- P1 gate passed on OpenShift 2026-10-30: 3 of 3 OOM warned, leads 17, 20, 20.1 min, 0 false alarms, 0 engine restarts, engine 172 Mi.
- Active phase: P2 and P3.
- Offline backtest: linear time-to-limit warned 21.8 min before an OOM that came at 21.5 min, 1 eligible cycle.
- P0 done: `cd workdir && go test ./internal/replay -v` runs in under 5 s. Baseline Predicted 0, Eligible 1, FalseAlarms 0.
- Replay score with saturation detector: Predicted 1 of 1, lead 24.8 min, FalseAlarms 0. One cycle, synthetic linear leak. Not wired into the engine yet.
- Engine reads Thanos, API exposes etf_seconds and failure_mode. Saturation hardened on 6 realistic scenarios, 21.5 min replay lead. CPU throttling mode added. Helm fixed for OpenShift, not yet installed live.
- Known gaps: alert API still returns 1000 items (M8), UI does not show ETF, images not published.
- Score command: `python3 scripts/lab-eval/score.py <fleet.jsonl>`

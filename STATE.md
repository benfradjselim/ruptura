# STATE

Read this file and the active lot file. Do not read the whole repository.

## Handoff

This section is how sessions pass work to each other. Read it first, continue from "Next action".

Rewrite it before any wait over 10 minutes, after each deploy, and at the end of a session. A session can stop without warning, so never leave this for the end. Replace the old content and keep it short. Numbers must come from a real measure.

- Updated: 2026-10-31 22:42 UTC
- Plan: fix the engine first. Other components (Helm, operator, UI, CLI, site) start only once the engine passes its gate. Deadline 2026-11-24, Red Hat Summit Paris.
- Work location: `/root/ruptura-fix`, branch `fix/engine-prediction`, pushed at `dc971d6`.
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
- Next action: after 23:54 UTC run `/root/lab.sh < scripts/lab-eval/measure-alerts.sh`. It counts 22:54 to 23:54 UTC, all 3 detectors active. Target 3 per minute or less. Do not redeploy before that. Update START and END in the script after any later deploy.
- Then: review metric coverage. The engine uses under 5 percent of available metrics, and the owner requires this addressed before leaving the engine.

## Maintainability audit 2026-10-31
No code changed. Priorities once LOT-1 passes:
- Merge the near identical `internal/predictor` and `internal/pipeline/metrics` (7 files differ by 2 to 15 lines).
- Fix docs that disagree with code: README 5 models, ARCHITECTURE places the ensemble in `fusion/`, `components.yaml` lists a missing `internal/normalize`.
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

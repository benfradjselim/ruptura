# STATE

Read this file and the active lot file. Do not read the whole repository.

## Handoff

This section is how sessions pass work to each other. Read it first, continue from "Next action".

Rewrite it before any wait over 10 minutes, after each deploy, and at the end of a session. A session can stop without warning, so never leave this for the end. Replace the old content and keep it short. Numbers must come from a real measure.

- Updated: 2026-10-31 15:05 UTC
- Plan: fix the engine first. Other components (Helm, operator, UI, CLI, site) start only once the engine passes its gate. Deadline 2026-11-24, Red Hat Summit Paris.
- Work location: `/root/ruptura-fix`, branch `fix/engine-prediction`, pushed at `dc971d6`.
- Lab: OpenShift, reach it with `/root/lab.sh`, not the Civo kubeconfig. Deploy with `/root/redeploy.sh`, which deletes `/var/tmp/fleet.jsonl`, so copy that file first.
- `gh api` and `gh run` are blocked by policy. Do not bypass this, verify locally.
- Active gate: LOT-1, alerts reduced by at least 90 percent. Baseline is at least 30 alerts per minute (capped 1000 in 33 min, `14a97f1`), so the target is 3 per minute or less.
- Measures: `14a97f1` 30 per minute or more. `7793f07` 7.1 per minute (320 in 45 min).
- In progress: `dc971d6` deployed 14:59 UTC, 45 minute measure due 15:46 UTC.
- Next action: read `GET /api/v2/alerts` on the lab, compute alerts per minute. Then recheck leak warning, crash-pay critical and stable-web healthy, because the P1 gate predates these changes.
- Then: review metric coverage. The engine uses under 5 percent of available metrics, and the owner requires this addressed before leaving the engine.

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

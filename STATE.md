# STATE

Read this file and the active lot file. Do not read the whole repository.

- Baseline 2026-10-30: leak predicted 0/2, FRI > 0 on 0/98 hosts, control false alarms 0.
- Latest: FRI > 0 on 18/98 hosts after M1. M2 and M5 are merged here, forecast GET returns 200 on the lab.
- Active phase: P1, first failure mode.
- Offline backtest: linear time-to-limit warned 21.8 min before an OOM that came at 21.5 min, 1 eligible cycle.
- P0 done: `cd workdir && go test ./internal/replay -v` runs in under 5 s. Baseline Predicted 0, Eligible 1, FalseAlarms 0.
- Replay score with saturation detector: Predicted 1 of 1, lead 24.8 min, FalseAlarms 0. One cycle, synthetic linear leak. Not wired into the engine yet.
- Next action: wire C4 into the engine through a Thanos source and catalog, then prove 3 cycles on the lab.
- Score command: `python3 scripts/lab-eval/score.py <fleet.jsonl>`

Update this file at the end of every session.

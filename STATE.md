# STATE

Read this file and the active lot file. Do not read the whole repository.

- Baseline 2026-10-30: leak predicted 0/2, FRI > 0 on 0/98 hosts, control false alarms 0.
- Latest: FRI > 0 on 18/98 hosts after M1. M2 and M5 are merged here, forecast GET returns 200 on the lab.
- Active lot: 1 (engine). See `docs/lots/LOT-1.md`.
- Offline backtest: linear time-to-limit warned 21.8 min before an OOM that came at 21.5 min, 1 eligible cycle.
- Next action: await owner approval of the v9 design, then phase P0.
- Score command: `python3 scripts/lab-eval/score.py <fleet.jsonl>`

Update this file at the end of every session.

# STATE

Read this file and the active lot file. Do not read the whole repository.

- Baseline 2026-10-30: leak predicted 0/2, FRI > 0 on 0/98 hosts, control false alarms 0.
- Active lot: 1 (engine). See `docs/lots/LOT-1.md`.
- Next action: fix M1, then re-score.
- Score command: `python3 scripts/lab-eval/score.py <fleet.jsonl>`

Update this file at the end of every session.

# Mission P1-saturation

- Component: C4-failure-modes
- Allowed paths: workdir/internal/failuremodes/, workdir/internal/replay/replay_test.go
- Input contract: a stream of `replay.Sample` values, or equivalent usage and capacity with a timestamp, per pod.
- Output contract: `Estimate{Mode, ETF time.Duration, Confidence float64, Usage float64}`. No estimate when capacity is unknown or the trend is not significant.
- Method: usage = memory / limit. Robust slope over a rolling window. ETF = (1 - usage) / slope. Reset the window after a restart.
- Numeric gate: `go test ./internal/replay` shows Predicted 1 of Eligible 1, lead at least 10 minutes, FalseAlarms 0. ETF error at the warning under 20 percent.
- Proof command: `cd workdir && go test ./internal/replay ./internal/failuremodes -v`
- Constraints: Go 1.18, no new dependency, table-driven tests, godoc on exported symbols, English only.
- Stop rule: 2 attempts without score gain, return to the lead.
- Report: score before and after, changed files, blockers, 150 words maximum.

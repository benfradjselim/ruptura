# Mission P1-realistic

- Component: C4-failure-modes
- Allowed paths: workdir/internal/failuremodes/saturation.go, workdir/internal/failuremodes/realistic_test.go
- Input contract: generated usage series at a 30 second interval, deterministic seeds only.
- Output contract: the `Saturation` public API stays unchanged.
- Scenarios, warning means an estimate with ETF at or below 25 minutes:
  - No warning: garbage-collected sawtooth with a flat trend, 5 minute spike that returns to baseline, stable usage at 95 percent, step change to a new flat level after 15 minutes.
  - Warning at least 10 minutes before the limit: linear leak with 5 percent noise, sawtooth whose floor rises until it reaches the limit.
- Numeric gate: all six scenarios pass over 20 seeds each. `go test ./internal/replay` still shows Predicted 1 of 1, FalseAlarms 0.
- Proof command: `cd workdir && GOFLAGS=-p=1 go test ./internal/failuremodes ./internal/replay`
- Constraints: Go 1.18, no new dependency, table-driven tests, godoc on exported symbols, English only.
- Stop rule: 2 attempts without score gain, return to the lead with the failing scenarios.
- Report: scenarios passing before and after, changed files, blockers, 150 words maximum.

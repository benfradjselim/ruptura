# Mission P2-throttle

- Component: C2-catalog and C4-failure-modes
- Allowed paths: workdir/internal/failuremodes/, workdir/internal/sources/, workdir/internal/api/handlers_failuremodes.go, workdir/internal/api/handlers_failuremodes_test.go, workdir/cmd/ruptura/failuremodes.go
- Input contract: catalog signal `cpu_throttling`, mode `condition`. Query: `sum by (namespace,pod) (rate(container_cpu_cfs_throttled_periods_total{container!=""}[5m])) / sum by (namespace,pod) (rate(container_cpu_cfs_periods_total{container!=""}[5m]))`.
- Output contract:
  - A `condition` mode raises an estimate with `Mode: "cpu_throttling"` when the ratio stays at or above 0.25 for 10 minutes, and clears when it stays below for 5 minutes.
  - ETF is zero because the condition is already happening. State becomes `degraded`, never `critical`.
  - Headline: `<name> is CPU throttled <percent> percent of the time.`
  - Memory saturation behavior, headline, and tests stay unchanged.
  - A workload with several active modes reports the most urgent one.
- Numeric gate: table-driven tests for sustained, brief spike, flapping, and clearing. `cd workdir && GOFLAGS=-p=1 go test ./internal/failuremodes ./internal/sources ./internal/api ./internal/replay` passes, with replay still Predicted 1 of 1 and FalseAlarms 0.
- Constraints: Go 1.18, no new dependency, godoc on exported symbols, English only, never touch a cluster.
- Stop rule: 2 attempts without score gain, return to the lead.
- Report: tests before and after, changed files, blockers, 150 words maximum.

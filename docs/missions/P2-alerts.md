# Mission P2-alerts

- Component: C4-failure-modes and C9-api
- Allowed paths: workdir/internal/pipeline/metrics/anomaly_mad.go, workdir/internal/pipeline/metrics/anomaly_mad_test.go, workdir/internal/predictor/anomaly_mad.go, workdir/internal/predictor/anomaly_mad_test.go, workdir/internal/api/handlers_failuremodes.go, workdir/internal/api/handlers_failuremodes_test.go, workdir/internal/api/handlers_ui.go
- Defects:
  1. MAD alerts fire on negligible absolute changes, such as CPU utilization 0.0004 against 0.0002. Require both the MAD score and a minimum absolute deviation: 2 percent of the observed range for bounded 0 to 1 metrics, and 20 percent of the median otherwise.
  2. A workload in crash loop shows `critical` with fused_rupture_index 0. A detected crash loop must report risk 9.
- Numeric gate: table-driven tests prove negligible deviations do not alert, real deviations still alert, and critical crash loops report risk 9. `cd workdir && GOFLAGS=-p=1 go test ./internal/pipeline/metrics ./internal/predictor ./internal/api ./internal/replay` passes.
- Constraints: Go 1.18, no new dependency, godoc on exported symbols, English only, never touch a cluster, never use git stash, rules enforced in production code.
- Stop rule: 2 attempts without score gain, return to the lead.
- Report: tests before and after, changed files, blockers, 150 words maximum.

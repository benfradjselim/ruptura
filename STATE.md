# STATE

Read this file and the active lot file. Do not read the whole repository.

## Handoff

This section is how sessions pass work to each other. Read it first, continue from "Next action".

Rewrite it before any wait over 10 minutes, after each deploy, and at the end of a session. A session can stop without warning, so never leave this for the end. Replace the old content and keep it short. Numbers must come from a real measure.

- Updated: 2026-11-01 11:04 UTC
- Direction from the owner 2026-11-01: reanalyse the formulas, keep only signals proven useful, consume broadly, clear system design. Read `docs/engine/DESIGN.md`, draft awaiting owner review, its phases replace the failure mode list below. Owner also wants user configuration by CRD, like an OpenTelemetry Collector: fixed engine, user declared sources, targets, levels and outputs, see `Product vision` in that file: one central `RupturaInstance`, the operator generates standard Kubernetes objects. Signals, closed failure scope per version, structured output, versions and logs decided, see `Product vision`. Summit: CRD and operator, 5 pod failures, 2 or 3 workload failures, metrics and Kubernetes Events, public benchmark against OpenShift alerts and `predict_linear`. Plan approved, see `Plan to the Summit`, results freeze 2026-11-17. Owner rule: discussion first, execute nothing until the owner gives the go, except reading the pending measure. Full reanalysis requested: `Product position` and `Component review` in that file hold proposed verdicts, nothing removed.
- Plan: fix the engine first. Other components (Helm, operator, UI, CLI, site) start only once the engine passes its gate. Deadline 2026-11-24, Red Hat Summit Paris.
- Work location: `/root/ruptura-fix`, branch `fix/engine-prediction`, pushed. Push with `git -c credential.helper= -c 'credential.helper=!gh auth git-credential' push`, plain push has no credentials.
- Lab: OpenShift, reach it with `/root/lab.sh`, not the Civo kubeconfig. Deploy with `/root/redeploy.sh`, which deletes `/var/tmp/fleet.jsonl`, so copy that file first.
- `gh api` and `gh run` are blocked by policy. Do not bypass this, verify locally.
- LOT-1 passed 2026-11-01 on `78bf127`: 2.33 alerts per minute in the warm hour 01:37 to 02:37 UTC (140 alerts), at least 92 percent below the baseline of 30 or more. `score.py`: 4 of 4 OOM warned, leads 10, 21.1, 21, 20 min, control false alarms 0, gate_pass true, 2 hours of data. One residual: 1 z-score warning on stable-web network.io at 0.23 B/s against 0.003, its state stayed healthy with FRI 0. A minimum absolute deviation would remove it.
- Owner approved all 5 decisions, see `docs/engine/DESIGN.md`.
- Phase 1 code done: absent inputs reported unavailable, weights rescaled over measured inputs, per signal baselines, `pkg/composites` removed. Deployed 03:53 UTC, committed, not pushed. Data of the previous run saved as `/var/tmp/*-20261101-78bf127.*` on the lab.
- Known follow-up: API and CLI still expose value 0 next to `unavailable_signals`, and `ruptura-ctl` colours health 0 red. Calibration is 24 minutes, not 24 hours as the code comments said, comments fixed.
- Risk to check: with few measured signals, stress and fatigue weigh more in health, so false alarms may rise.
- Owner said go on 2026-11-01 and asked not to be asked for approvals: decide, inform, keep this file current. Defaults taken: 7 failures from the plan, README in the last week. Still blocked by policy: `gh run`, `gh api`, do not bypass. Docker Hub account name still unknown.
- Phase 1 closed on `2a2f317`: alerts 2.33 per minute, non demo workloads degraded at least once fell from 78 to 4. score.py gate_pass false only from a first cycle at 9 min after the engine restart. Leads of 30 and 31 min are likely carried from the previous cycle, so `score.py` should reset its warning at each restart. It also counts the same cycle in `predict_linear` readings of 31.
- `097f2e8` structured output and the benchmark faults deployed 11:04 UTC.
- Next action: after 14:04 UTC compare Ruptura and `predict_linear` on step-leak and on sawtooth-batch, which must stay silent. Then week 1 remainder: catalog and normalizer, Kubernetes Events.
- Benchmark first result: on the linear lab leak, tuned `predict_linear` gets 11 of 11 with 26 to 28 min, Ruptura 20 to 21 min. Loss on this scenario, see `Benchmark` in `docs/engine/DESIGN.md`. Do not market linear OOM prediction as the advantage. Next: test scenarios where a single rule struggles.
- Benchmark scenarios ready, not applied: `lab-setup/06-benchmark-faults.yaml`, step-leak must be warned, sawtooth-batch is a healthy control. Apply after the 07:48 measure, record at least 3 hours, then compare with `scripts/lab-eval/benchmark-predict-linear.sh`, which needs its leak pod prefix changed.
- Structured output done, not deployed: `GET /api/v2/failures` and `failure` in `/fleet`, with kind predicted or detected.
- Metric coverage, owner requires it before leaving the engine. Inventory done 2026-11-01, no code changed: the lab Prometheus exposes 2594 metric names, `sources/catalog.yaml` reads 5 of them (0.2 percent), plus kubeletstats over OTLP. Most names are internal and not useful.
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

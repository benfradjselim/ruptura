# Engine contract

Each stage has one implementation, one input, and one observable output.

| Stage | Input | Output | Failure signal |
|---|---|---|---|
| Ingest | OTLP, remote write | Series per workload | `pending_telemetry` |
| Normalize | Raw series, pod limits | Utilization 0-1 against the limit | Unknown limit is reported, never zero |
| Signals | Normalized series | Composite KPIs | Expected signal stays at 0 |
| Forecast | Signal history | Value and time to exhaustion | Horizon error |
| Risk | Signals, forecast | Fused Rupture Index | FRI 0 under a known failure |
| State | Risk, restarts | healthy, degraded, critical | State contradicts risk |
| API | State | JSON | Any 4xx on a valid workload |
| Dashboard | API | Rendered values | Value differs from API |

Rules:
- Zero is a measured value. Missing data is explicit.
- A stage never reads data from a stage after it.
- A change is complete when `scripts/lab-eval/score.py` improves.

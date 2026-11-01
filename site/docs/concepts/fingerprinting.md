# Rupture Fingerprinting & Business Signals

Two intelligence layers that turn raw rupture data into institutional memory and business context.

---

## Rupture Fingerprinting

When a workload enters a confirmed rupture (FusedR ≥ 3.0), Ruptura captures an 11-dimensional KPI signal vector — a **fingerprint** — and stores it against that workload. On every subsequent health query, the current signal state is compared against all historical fingerprints using cosine similarity. A match ≥ 0.85 surfaces as `pattern_match` in the API response.

### Why it matters

The same workload often ruptures for the same reason, in the same pattern. A fingerprint match means:

- You have seen this before
- You probably know how to fix it
- The `resolution_note` field carries what worked last time

Without fingerprinting, every rupture looks novel. With it, Ruptura tells you: *"this looks 94% like the cascade you fixed on April 12 by scaling payment-api ×2."*

### The fingerprint vector

```
[stress, fatigue, 1−mood, pressure, humidity, contagion, 1−resilience, entropy, velocity, throughput_drop, fusedR/10]
```

Inverting `mood` and `resilience` so that all 11 dimensions point in the "bad" direction — a high value in any dimension means more trouble. This makes cosine similarity meaningful: two ruptures that are bad in the same ways score high.

### Debounce

One fingerprint is recorded per workload per hour, even if FusedR stays elevated. This prevents a sustained incident from writing hundreds of near-identical fingerprints and diluting future matches.

### API response

When a match is found, every `GET /api/v2/rupture/{namespace}/{workload}` response includes:

```json
{
  "pattern_match": {
    "matched_rupture_id": "rpt_a3f1b2",
    "similarity": 0.94,
    "matched_at": "2026-04-12T03:22:00Z",
    "resolution_note": "scaled payment-api to 4 replicas — FusedR recovered in 18 minutes"
  }
}
```

`pattern_match` is `null` when no historical fingerprint matches above the 0.85 threshold.

---

## Business Signal Layer

The three business signals in the `business` block of every snapshot connect infrastructure health to business impact. They are always computed — not gated behind calibration.

### `slo_burn_velocity`

```
slo_burn_velocity = current_error_rate / allowed_error_rate

  allowed_error_rate = 1 − (target_percent / 100)
```

| value | Meaning |
|-------|---------|
| < 1.0 | Under budget — consuming error budget slower than allowed |
| 1.0 | Exactly on budget |
| > 1.0 | Burning too fast — at this rate, the monthly budget runs out early |
| > 5.0 | Budget exhausted within hours — escalate now |

Configure SLO contracts in `helm/values.yaml`:

```yaml
slos:
  - workload: payments/Deployment/checkout
    targetPercent: 99.9
    windowDays: 30
    errorBudgetMinutes: 43.2
  - workload: orders/Deployment/api
    targetPercent: 99.5
    windowDays: 30
    errorBudgetMinutes: 216
```

Zero if no SLO is configured for the workload.

### `blast_radius`

The count of unique downstream workloads in the trace topology that have a dependency edge pointing to this workload — i.e., services that would be affected if this workload failed.

```
blast_radius = |{w : w depends on this workload}|
```

A `blast_radius` of 8 means 8 other services call this one. Combined with a rising FusedR, this is your escalation multiplier — a rupture here is not an isolated incident.

Requires OTLP trace ingest. Zero when no topology data is available.

### `recovery_debt`

The count of **near-misses** in the last 7 days: events where FusedR crossed 2.0 (warning threshold) but recovered below 1.0 without ever reaching a confirmed rupture (FusedR ≥ 3.0).

```
recovery_debt = count of near-miss recoveries in [now − 7d, now]
```

A high `recovery_debt` means the workload is repeatedly flirting with rupture and recovering — possibly because operators are manually intervening, or because the issue resolves itself temporarily. It is a leading indicator that a real rupture is coming.

| recovery_debt | Interpretation |
|--------------|----------------|
| 0 | Clean — no recent near-misses |
| 1–3 | Monitor — occasional instability |
| 4–10 | Investigate — repeated near-misses, latent problem |
| > 10 | Escalate — this workload is chronically unstable |

### Full snapshot example

```json
{
  "workload": {"namespace": "payments", "kind": "Deployment", "name": "checkout"},
  "health_score": {"value": 54.2, "state": "fair"},
  "fused_rupture_index": 2.1,
  "business": {
    "slo_burn_velocity": 2.4,
    "blast_radius": 6,
    "recovery_debt": 3
  }
}
```

Reading: SLO budget burning at 2.4× the allowed rate, 6 downstream services at risk, 3 near-misses in the last week. Even though FusedR is 2.1 (warning, not critical), the business context makes this a high-priority investigation.

---

## API

```bash
# Business signals and pattern_match are included in every rupture response
curl -H "Authorization: Bearer $API_KEY" \
  http://localhost:8080/api/v2/rupture/payments/checkout
```

No separate endpoints — they are embedded in the standard rupture snapshot.

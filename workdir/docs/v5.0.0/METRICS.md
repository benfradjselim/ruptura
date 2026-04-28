# OHE v5.0.0 — METRICS.md (Canonical XAI Contract)

**Version:** 5.0.0
**Status:** Authoritative — any formula divergence in code is a bug
**Companion document:** `WHITEPAPER-v5.0.0.md` Section 8

This file documents every KPI computed by OHE. Weights, thresholds, and formulas here are the **source of truth**. If code disagrees with this document, fix the code.

---

## 1. Base Metrics

All raw metrics are normalized to `[0, 1]` where `0` = optimal, `1` = critical.

| Category | Metrics |
|---|---|
| System | CPU, RAM, Disk, Net |
| Application | Req, Err, Lat, Tout |
| Behavioral | Restart, Uptime |

---

## 2. Stress Index (S)

**Formula:**
```
S = 0.3·CPU + 0.2·RAM + 0.2·Latency + 0.2·Errors + 0.1·Timeouts
```

**Weights:**
| Symbol | Input | Weight |
|---|---|---|
| α | CPU | 0.3 |
| β | RAM | 0.2 |
| γ | Latency | 0.2 |
| δ | Errors | 0.2 |
| ε | Timeouts | 0.1 |

**Thresholds:**
| S | State |
|---|---|
| < 0.3 | Calm |
| 0.3 – 0.6 | Nervous |
| 0.6 – 0.8 | Stressed |
| ≥ 0.8 | Panic |

---

## 3. Fatigue (F) — Dissipative v5.0

**Formula:**
```
F_t = max(0, F_{t−1} + (S_t − R_threshold) − λ)
```

**Parameters:**
| Symbol | Meaning | Default |
|---|---|---|
| R_threshold | Rest threshold | 0.3 |
| λ | Recovery coefficient (per interval) | 0.05 |

**Thresholds:**
| F | State | Action |
|---|---|---|
| < 0.3 | Rested | Normal monitoring |
| 0.3 – 0.6 | Tired | Increase observation |
| 0.6 – 0.8 | Exhausted | Plan maintenance |
| ≥ 0.8 | Burnout imminent | Preventive restart |

**Rationale for λ:** Prevents scheduled workloads (backups, batch jobs) from permanently inflating Fatigue. Models natural system recovery during idle periods.

---

## 4. Mood (M)

**Formula:**
```
M = (Uptime × Req) / (Err × Tout × Restart + ε)
```
Where `ε = 1e-9` to avoid division by zero.

**Thresholds:**
| M | Mood |
|---|---|
| > 100 | Happy |
| 50 – 100 | Content |
| 10 – 50 | Neutral |
| 1 – 10 | Sad |
| ≤ 1 | Depressed |

---

## 5. Atmospheric Pressure (P)

**Formula:**
```
P(t) = dS̄/dt + ∫₀ᵗ Ē(τ) dτ
```

**Thresholds:**
| Trend | Prediction |
|---|---|
| P > 0.1 for 10 min | Storm in ~2h |
| P stable | Stable conditions |
| P < 0 | System improving |

---

## 6. Error Humidity (H)

**Formula:**
```
H(t) = (Ē(t) × T̄(t)) / Q̄(t)
```
Where Ē = mean errors, T̄ = mean timeouts, Q̄ = mean throughput.

**Thresholds:**
| H | State | Action |
|---|---|---|
| < 0.1 | Dry | Normal |
| 0.1 – 0.3 | Humid | Watch |
| 0.3 – 0.5 | Very humid | Alert |
| ≥ 0.5 | Storm | Immediate action |

---

## 7. Contagion Index (C)

**Formula:**
```
C(t) = Σ_{i,j} E_{ij}(t) × D_{ij}
```
Where `E_ij` = error rate from service i to j, `D_ij` = dependency weight.

**Thresholds:**
| C | State | Action |
|---|---|---|
| < 0.3 | Low | Normal |
| 0.3 – 0.6 | Moderate | Monitor closely |
| 0.6 – 0.8 | Epidemic | Isolate affected |
| ≥ 0.8 | Pandemic | Global response |

---

## 8. Rupture Index (R) — v5.0 CA-ILR

**Formula:**
```
R = α_burst / α_stable
```
Where `α_burst` is the slope from the 5-minute ILR window and `α_stable` is the slope from the 60-minute ILR window. Guard: if `|α_stable| < 1e-9`, return 0.

**Trigger:** `R > 3` → `Exponential_Failure` alert on the tracked metric (RAM, Latency, Stress).

---

## 9. Prediction Summary

| Prediction | Trigger |
|---|---|
| Storm | ∫ P dτ > θ_p **OR** R > 3 on stress |
| Burnout | F̄ > θ_f (with λ-dissipation applied) |
| Epidemic | C > θ_c |
| Exponential_Failure | R > 3 on RAM or Latency |

Default thresholds: `θ_p = 0.15`, `θ_f = 0.8`, `θ_c = 0.6`.

---

## 10. Change Log

| Version | Date | Change |
|---|---|---|
| 5.0.0 | 2026-04 | Initial canonical METRICS.md. Fatigue now dissipative (λ). Added Rupture Index R. |

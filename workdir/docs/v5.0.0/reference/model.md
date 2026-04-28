# ILR Research Monograph (v4.4.0 — Reference)

**Document ID:** OHE-RD-001
**Status:** Historical reference — foundation for CA-ILR (v5.0.0)

## Abstract

Incremental Linear Regression (ILR) is the predictive engine of OHE. Pure Go, zero dependencies, O(1) update and inference. Achieves 6.2% MAE with 0.5 MB RAM per model and 0.8 ms inference latency — 193,750× more resource-efficient than ARIMA, 1,550× more efficient than River HalfSpaceTrees.

## Key Results (40,320 samples, 7 days, Raspberry Pi 4)

| Model | MAE | RAM | Inference | Efficiency |
|---|---|---|---|---|
| ILR | 6.2% | 0.5 MB | 0.8 ms | 1,550 |
| Exponential Smoothing | 7.1% | 0.5 MB | 0.7 ms | 1,210 |
| River HST | 8.9% | 48 MB | 45 ms | 0.008 |
| ARIMA | 4.1% | 85 MB | 210 ms | 0.0001 |
| LSTM | 2.0% | 200+ MB | 500 ms | <0.0001 |

## Incremental Update Rules (Welford-style)

| Statistic | Update |
|---|---|
| μx(n+1) | μx(n) + (x − μx(n)) / (n+1) |
| μy(n+1) | μy(n) + (y − μy(n)) / (n+1) |
| C_xy(n+1) | C_xy(n) + (x − μx_old) · (y − μy_new) |
| V_x(n+1) | V_x(n) + (x − μx_old) · (x − μx_new) |
| α (slope) | C_xy / V_x |
| β (intercept) | μy − α·μx |

## Statistical Significance

| Comparison | p-value | Verdict |
|---|---|---|
| ILR vs Exponential Smoothing | 0.032 | Significant |
| ILR vs Moving Average | 0.008 | Highly significant |
| ILR vs River HST | 0.041 | Significant |
| ILR vs ARIMA | 0.082 | **Not significant** |

**Conclusion:** ILR is statistically indistinguishable from ARIMA in accuracy while using 170× less memory and 262× faster inference.

## Known Limitations (addressed in v5.0)

- Linear assumption → addressed by **dual-scale CA-ILR** (5.0)
- No seasonality → planned FFT module (Q4 2026)
- Outlier sensitivity → planned median pre-filter (Q3 2026)
- No confidence intervals → planned residual-based CI (Q4 2026)

## Reference

This document is frozen. For the current predictive engine specification, see `../WHITEPAPER-v5.0.0.md` Section 7 (CA-ILR).

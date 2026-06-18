/**
 * displayLabels.js — maps internal API signal keys to SRE-friendly display names.
 *
 * Internal API keys are never changed. This is a pure display-layer mapping.
 * All signal values from the API are in [0,1] range unless noted.
 */

export const SIGNAL_LABELS = {
  health_score:        { display: 'Reliability',     unit: '%',  multiply: 100 },
  fused_rupture_index: { display: 'Risk Score',       unit: '',   multiply: 10  },
  fused_r:             { display: 'Risk Score',       unit: '',   multiply: 10  },
  stress:              { display: 'CPU Pressure',     unit: '%',  multiply: 100 },
  fatigue:             { display: 'Memory Pressure',  unit: '%',  multiply: 100 },
  mood:                { display: 'Trend',            unit: '%',  multiply: 100 },
  contagion:           { display: 'Blast Radius',     unit: '%',  multiply: 100 },
  pressure:            { display: 'Load Index',       unit: '%',  multiply: 100 },
  resilience:          { display: 'Resilience',       unit: '%',  multiply: 100 },
  entropy:             { display: 'Entropy',          unit: '',   multiply: 1   },
  velocity:            { display: 'Velocity',         unit: '%',  multiply: 100 },
  humidity:            { display: 'Saturation',       unit: '%',  multiply: 100 },
  throughput:          { display: 'Throughput',       unit: '%',  multiply: 100 },
  calibration_pct:     { display: 'Calibration',     unit: '%',  multiply: 1   },
}

/**
 * Returns the human-friendly display label for an API signal key.
 * Falls back to the raw key if not mapped.
 */
export function displayLabel(key) {
  return SIGNAL_LABELS[key]?.display ?? key
}

/**
 * Returns the display value for a raw API value.
 * Applies the multiply factor and rounds to 1 decimal.
 */
export function displayValue(key, rawValue) {
  if (rawValue == null || isNaN(rawValue)) return '—'
  const m = SIGNAL_LABELS[key]?.multiply ?? 1
  return (rawValue * m).toFixed(1)
}

/**
 * Returns the unit string for a signal key.
 */
export function displayUnit(key) {
  return SIGNAL_LABELS[key]?.unit ?? ''
}

/**
 * Returns a CSS class based on signal value and key.
 * Used to color-code signals in the UI.
 */
export function signalClass(key, rawValue) {
  if (rawValue == null) return 'muted'
  // Reliability/HealthScore: higher is better
  if (key === 'health_score') {
    if (rawValue >= 0.9) return 'healthy'
    if (rawValue >= 0.7) return 'degraded'
    if (rawValue >= 0.5) return 'at-risk'
    return 'critical'
  }
  // Risk Score: lower is better
  if (key === 'fused_rupture_index' || key === 'fused_r') {
    if (rawValue <= 1.5) return 'healthy'
    if (rawValue <= 3.0) return 'degraded'
    if (rawValue <= 5.0) return 'at-risk'
    return 'critical'
  }
  // All other signals: lower is better
  if (rawValue <= 0.3) return 'healthy'
  if (rawValue <= 0.6) return 'degraded'
  if (rawValue <= 0.8) return 'at-risk'
  return 'critical'
}

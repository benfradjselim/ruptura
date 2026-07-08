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
  const k = resolveAlias(key)
  return SIGNAL_LABELS[k]?.display ?? key
}

/**
 * Returns the display value for a raw API value.
 * Applies the multiply factor and rounds to 1 decimal.
 */
export function displayValue(key, rawValue) {
  if (rawValue == null || isNaN(rawValue)) return '—'
  const m = SIGNAL_LABELS[resolveAlias(key)]?.multiply ?? 1
  return (rawValue * m).toFixed(1)
}

/**
 * Returns the unit string for a signal key.
 */
export function displayUnit(key) {
  return SIGNAL_LABELS[resolveAlias(key)]?.unit ?? ''
}

// FBL-A2-2: SRE-vocabulary aliases. `slo_probability` and `error_budget_burn`
// are not separate API fields — they're SRE-native synonyms for existing
// internal keys (health_score, fused_rupture_index), used where the reader
// benefits from SLO-standard language (e.g. alert rule configuration).
// Internal keys are never renamed; these are display-layer aliases only.
const SRE_ALIASES = {
  slo_probability: 'health_score',
  error_budget_burn: 'fused_rupture_index',
}

/**
 * Resolves an SRE-vocabulary alias (e.g. "slo_probability") to the internal
 * key it stands for. Returns the key unchanged if it isn't an alias.
 */
export function resolveAlias(key) {
  return SRE_ALIASES[key] ?? key
}

/**
 * Returns the alert-context label for a metric key — the same as
 * displayLabel() except fused_rupture_index/fused_r reads as "Error Budget
 * Burn Rate" here, since crossing that threshold *is* burning through the
 * error budget faster than sustainable. Fleet/workload views keep "Risk
 * Score" (displayLabel) — this alias is scoped to alert rules/alerts only,
 * per FBL-A2-2's "error_budget_burn naming in alerts".
 */
export function alertMetricLabel(key) {
  if (key === 'fused_rupture_index' || key === 'fused_r') return 'Error Budget Burn Rate'
  return displayLabel(key)
}

const INCIDENT_PROBABILITY_BANDS = [
  { max: 1.5, label: 'Low incident probability' },
  { max: 3.0, label: 'Elevated incident probability' },
  { max: 5.0, label: 'High incident probability' },
  { max: Infinity, label: 'Critical incident probability' },
]

/**
 * Bands a Fused Rupture Index value into the SRE-native "incident
 * probability" vocabulary REFERENCE.md's B1 reposition calls for, reusing
 * the same thresholds signalClass() already applies for Risk Score coloring.
 */
export function incidentProbabilityBand(fusedR) {
  if (fusedR == null || isNaN(fusedR)) return 'Unknown'
  return INCIDENT_PROBABILITY_BANDS.find((b) => fusedR <= b.max).label
}

/**
 * Returns a CSS class based on signal value and key.
 * Used to color-code signals in the UI.
 */
export function signalClass(key, rawValue) {
  if (rawValue == null) return 'muted'
  key = resolveAlias(key)
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

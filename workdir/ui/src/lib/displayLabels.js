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

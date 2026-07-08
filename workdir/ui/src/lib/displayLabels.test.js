import { describe, it, expect } from 'vitest'
import { signalClass, displayLabel, displayValue, displayUnit } from './displayLabels.js'

// FBL-V4: severity color must derive from the signal's own semantics, not a
// single blanket "lower is better" rule. mood ("Trend") is normalized so
// 1.0 = happy — the opposite of every other composite signal — and a
// missing special case here previously rendered a genuinely happy workload
// (mood=0.9) as "critical" (red) in Fleet.svelte's always-visible signal
// bars. Table-driven across all 10 composite signals named in
// docs/REFERENCE.md, plus health_score/fused_rupture_index.
describe('signalClass', () => {
  const lowerIsBetterSignals = [
    'stress', 'fatigue', 'contagion', 'pressure',
    'resilience', 'entropy', 'velocity', 'humidity', 'throughput',
  ]

  it.each(lowerIsBetterSignals)('%s: low value is healthy, high value is critical', (key) => {
    expect(signalClass(key, 0.1)).toBe('healthy')
    expect(signalClass(key, 0.9)).toBe('critical')
  })

  it.each(lowerIsBetterSignals)('%s: never reads healthy at a high value (the FBL-V4 regression shape)', (key) => {
    expect(signalClass(key, 0.95)).not.toBe('healthy')
  })

  it('mood: high value (happy) is healthy, not critical', () => {
    expect(signalClass('mood', 0.9)).toBe('healthy')
    expect(signalClass('mood', 0.1)).toBe('critical')
  })

  it('mood: boundaries', () => {
    expect(signalClass('mood', 0.8)).toBe('healthy')
    expect(signalClass('mood', 0.5)).toBe('degraded')
    expect(signalClass('mood', 0.3)).toBe('at-risk')
    expect(signalClass('mood', 0.29)).toBe('critical')
  })

  it('health_score: higher is better', () => {
    expect(signalClass('health_score', 0.95)).toBe('healthy')
    expect(signalClass('health_score', 0.1)).toBe('critical')
  })

  it('fused_rupture_index / fused_r: lower is better, aliases agree', () => {
    expect(signalClass('fused_rupture_index', 0.5)).toBe('healthy')
    expect(signalClass('fused_r', 0.5)).toBe('healthy')
    expect(signalClass('fused_rupture_index', 6.0)).toBe('critical')
  })

  it('null/undefined value is muted, never a severity color', () => {
    expect(signalClass('stress', null)).toBe('muted')
    expect(signalClass('mood', undefined)).toBe('muted')
  })
})

describe('displayLabel / displayValue / displayUnit', () => {
  it('mood displays as "Trend", the SRE-facing label', () => {
    expect(displayLabel('mood')).toBe('Trend')
  })

  it('displayValue applies each signal\'s multiply factor', () => {
    expect(displayValue('stress', 0.5)).toBe('50.0')
    expect(displayValue('fused_rupture_index', 0.5)).toBe('5.0')
    expect(displayValue('entropy', 0.5)).toBe('0.5')
  })

  it('displayValue returns an em-dash for missing data, not NaN', () => {
    expect(displayValue('stress', null)).toBe('—')
    expect(displayValue('stress', NaN)).toBe('—')
  })

  it('displayUnit matches each signal\'s configured unit', () => {
    expect(displayUnit('stress')).toBe('%')
    expect(displayUnit('entropy')).toBe('')
  })
})

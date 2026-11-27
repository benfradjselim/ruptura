import { describe, it, expect } from 'vitest'
import { displayLabel, alertMetricLabel } from './displayLabels.js'

describe('displayLabel', () => {
  it('maps an internal key to its display name', () => {
    expect(displayLabel('mood')).toBe('Trend')
  })
  it('falls back to the raw key', () => {
    expect(displayLabel('unknown_key')).toBe('unknown_key')
  })
})

describe('alertMetricLabel', () => {
  it('names the rupture index as an error budget burn in alerts', () => {
    expect(alertMetricLabel('fused_rupture_index')).toBe('Error Budget Burn Rate')
  })
  it('uses the display name otherwise', () => {
    expect(alertMetricLabel('stress')).toBe('CPU Pressure')
  })
})

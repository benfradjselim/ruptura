import { describe, it, expect } from 'vitest'
import { formatEta, sortFailures, missingRoles, recentAnomalies, measuredSignal } from './failures.js'

describe('formatEta', () => {
  it.each([
    [undefined, '—'],
    [0, '—'],
    [45, '< 1 min'],
    [720, '12 min'],
    [5400, '1 h 30 min'],
  ])('formats %s', (seconds, want) => {
    expect(formatEta(seconds)).toBe(want)
  })
})

describe('sortFailures', () => {
  it('puts critical first, then the nearest failure', () => {
    const got = sortFailures([
      { target: 'a', state: 'degraded', etaSeconds: 60 },
      { target: 'b', state: 'critical', etaSeconds: 900 },
      { target: 'c', state: 'critical', etaSeconds: 300 },
      { target: 'd', state: 'critical' },
    ])
    expect(got.map(f => f.target)).toEqual(['c', 'b', 'd', 'a'])
  })

  it('does not change its input', () => {
    const input = [{ target: 'a', state: 'degraded' }, { target: 'b', state: 'critical' }]
    sortFailures(input)
    expect(input[0].target).toBe('a')
  })
})

describe('missingRoles', () => {
  it('lists roles with no data', () => {
    expect(missingRoles([{ role: 'a', available: true }, { role: 'b', available: false }])).toEqual(['b'])
  })
})

describe('recentAnomalies', () => {
  const now = Date.parse('2026-11-02T02:00:00Z')
  const at = (minutes) => new Date(now - minutes * 60000).toISOString()

  it('keeps unacknowledged anomalies of the last 15 minutes', () => {
    const got = recentAnomalies([
      { host: 'a', timestamp: at(5) },
      { host: 'b', timestamp: at(30) },
      { host: 'c', timestamp: at(1), acknowledged: true },
    ], now)
    expect(got.map(a => a.host)).toEqual(['a'])
  })
})

describe('measuredSignal', () => {
  it('turns an unavailable signal into no value', () => {
    expect(measuredSignal({ unavailable_signals: ['mood'] }, 'mood', 0)).toBeNull()
  })

  it('keeps a measured zero', () => {
    expect(measuredSignal({ unavailable_signals: ['mood'] }, 'stress', 0)).toBe(0)
  })

  it('accepts a workload without the list', () => {
    expect(measuredSignal({}, 'stress', 0.4)).toBe(0.4)
  })
})

import { describe, it, expect } from 'vitest'
import { formatEta, sortFailures, missingRoles } from './failures.js'

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

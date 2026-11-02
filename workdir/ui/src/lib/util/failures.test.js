import { describe, it, expect } from 'vitest'
import { formatEta, sortFailures, missingRoles, recentAnomalies, measuredSignal, horizonPlacement, pastPlacement, relatedEvents, cardSummary, horizonClusters } from './failures.js'

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
  it('puts critical first, then what is happening now, then the nearest failure', () => {
    const got = sortFailures([
      { target: 'a', state: 'degraded', etaSeconds: 60 },
      { target: 'b', state: 'critical', etaSeconds: 900 },
      { target: 'c', state: 'critical', etaSeconds: 300 },
      { target: 'd', state: 'critical' },
    ])
    expect(got.map(f => f.target)).toEqual(['d', 'c', 'b', 'a'])
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

describe('horizonPlacement', () => {
  it('places detected failures at now, the middle of the axis', () => {
    expect(horizonPlacement({}, 1800)).toEqual({ position: 50, beyond: false })
  })

  it('places forecasts on the future half', () => {
    expect(horizonPlacement({ etaSeconds: 900 }, 1800)).toEqual({ position: 75, beyond: false })
  })

  it('holds a later failure at the edge', () => {
    expect(horizonPlacement({ etaSeconds: 3600 }, 1800)).toEqual({ position: 100, beyond: true })
  })
})

describe('pastPlacement', () => {
  it('places past events on the first half', () => {
    expect(pastPlacement(0, 30)).toBe(50)
    expect(pastPlacement(15, 30)).toBe(25)
    expect(pastPlacement(30, 30)).toBe(0)
    expect(pastPlacement(45, 30)).toBe(0)
  })
})

describe('relatedEvents', () => {
  it('keeps events of one target, newest first', () => {
    const events = [
      { target: 'a', minutesAgo: 20 },
      { target: 'b', minutesAgo: 5 },
      { target: 'a', minutesAgo: 3 },
    ]
    expect(relatedEvents(events, 'a').map(e => e.minutesAgo)).toEqual([3, 20])
  })
})

describe('cardSummary', () => {
  it('names a forecast failure with its time left', () => {
    expect(cardSummary({ failure: { failureMode: 'saturation', kind: 'predicted', etaSeconds: 720, evidence: ['Memory at 91% of its limit'] } }))
      .toEqual({ title: 'saturation', when: 'in 12 min', facts: ['Memory at 91% of its limit'] })
  })

  it('names a detected failure as happening now', () => {
    expect(cardSummary({ failure: { failureMode: 'crash_loop', kind: 'detected' } }))
      .toEqual({ title: 'crash loop', when: 'now', facts: [] })
  })

  it('says when nothing is known', () => {
    expect(cardSummary({ state: 'healthy' })).toEqual({ title: 'No known failure', when: '', facts: [] })
  })
})

describe('horizonClusters', () => {
  const H = 1800
  const f = (target, etaSeconds, state = 'warning') => ({ target, etaSeconds, state })

  it('groups failures at the same time under one marker', () => {
    const clusters = horizonClusters([f('a', 0), f('b', 0, 'critical'), f('c', 0)], H)
    expect(clusters).toHaveLength(1)
    expect(clusters[0].failures.map(x => x.target)).toEqual(['a', 'b', 'c'])
    expect(clusters[0].state).toBe('critical')
    expect(clusters[0].position).toBe(50)
  })

  it('keeps distant failures apart and alternates their lanes', () => {
    const clusters = horizonClusters([f('a', 0), f('b', 900), f('c', 1000)], H)
    expect(clusters.map(c => c.failures.length)).toEqual([1, 2])
    expect(clusters.map(c => c.lane)).toEqual([0, 1])
  })

  it('anchors labels near the right edge to the left', () => {
    const clusters = horizonClusters([f('a', 0), f('b', 1740), f('c', 2000)], H)
    expect(clusters[1].alignRight).toBe(true)
    expect(clusters[1].beyond).toBe(true)
  })

  it('opens a label to the left when the next one is close', () => {
    expect(horizonClusters([f('a', 0), f('b', 1740)], H)[0].alignRight).toBe(true)
    expect(horizonClusters([f('a', 0)], H)[0].alignRight).toBe(false)
    expect(horizonClusters([f('a', 0), f('b', 900), f('c', 1800)], H).map(c => c.alignRight)).toEqual([true, false, true])
  })
})

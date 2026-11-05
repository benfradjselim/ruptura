import { describe, it, expect } from 'vitest'
import { forecastGeometry, hasForecastChart } from './forecastChart.js'

const MIN = 60 * 1000
const NOW = Date.UTC(2026, 10, 5, 12, 0)
const ramp = (from, to) => Array.from({ length: 11 }, (_, i) => ({ t: NOW - (10 - i) * MIN, usage: from + ((to - from) * i) / 10 }))
const geometry = (pods, failure, extra = {}) =>
  forecastGeometry({ pods, failure, now: NOW, width: 400, height: 200, pad: { left: 0, right: 0, top: 0, bottom: 0 }, ...extra })

describe('hasForecastChart', () => {
  it.each([
    [{ kind: 'predicted', failureMode: 'saturation' }, true],
    [{ kind: 'predicted', failureMode: 'replica_divergence' }, true],
    [{ kind: 'predicted', failureMode: 'recurrent_failure' }, true],
    [{ kind: 'detected', failureMode: 'saturation' }, false],
    [{ kind: 'detected', failureMode: 'crash_loop' }, false],
  ])('%o is %s', (failure, want) => {
    expect(hasForecastChart(failure)).toBe(want)
  })
})

describe('forecastGeometry', () => {
  it('draws the failing pod last and its siblings first', () => {
    const g = geometry(
      [{ pod: 'shop/api-a', points: ramp(0.5, 0.8) }, { pod: 'shop/api-b', points: ramp(0.4, 0.4) }],
      { failureMode: 'replica_divergence', pod: 'shop/api-a', etaSeconds: 1800 },
    )
    expect(g.lines.map(l => [l.pod, l.highlight])).toEqual([['shop/api-b', false], ['shop/api-a', true]])
  })

  it('carries the trend from the last reading to the limit at the failure time', () => {
    const g = geometry([{ pod: 'shop/api-a', points: ramp(0.5, 0.8) }], { failureMode: 'saturation', pod: 'shop/api-a', etaSeconds: 1200 })
    expect(g.projection.x1).toBeCloseTo(g.nowX)
    expect(g.projection.x2).toBeCloseTo(g.eta.x)
    expect(g.projection.y2).toBeCloseTo(g.limitY)
  })

  it('places now at the end of the past 30 minutes and keeps room past the failure', () => {
    const g = geometry([{ pod: 'p', points: ramp(0.5, 0.8) }], { failureMode: 'saturation', pod: 'p', etaSeconds: 1800 })
    expect(g.xTicks.map(t => t.label)).toEqual(['−30 min', 'Now', '+35 min'])
    expect(g.eta.x).toBeLessThan(400)
  })

  it('stops a distant trend at the right edge, below the limit', () => {
    const g = geometry([{ pod: 'p', points: ramp(0.5, 0.6) }], { failureMode: 'saturation', pod: 'p', etaSeconds: 5 * 3600 })
    expect(g.eta).toBeNull()
    expect(g.projection.x2).toBeCloseTo(400)
    expect(g.projection.y2).toBeGreaterThan(g.limitY)
  })

  it('marks the next failure of a rhythm without drawing a trend', () => {
    const g = geometry([{ pod: 'p', points: ramp(0.3, 0.3) }], { failureMode: 'recurrent_failure', etaSeconds: 600 })
    expect(g.projection).toBeNull()
    expect(g.eta).not.toBeNull()
  })

  it('keeps past failures of the window only', () => {
    const g = geometry([{ pod: 'p', points: ramp(0.3, 0.6) }], { failureMode: 'saturation', pod: 'p', etaSeconds: 600 }, { failureTimes: [NOW - 5 * MIN, NOW - 90 * MIN] })
    expect(g.failures).toHaveLength(1)
  })

  it('reports an empty chart when no pod has readings', () => {
    const g = geometry([], { failureMode: 'saturation', pod: 'p', etaSeconds: 600 })
    expect(g.empty).toBe(true)
    expect(g.projection).toBeNull()
  })

  it('always shows the limit inside the chart', () => {
    const g = geometry([{ pod: 'p', points: ramp(0.2, 0.3) }], { failureMode: 'saturation', pod: 'p', etaSeconds: 3600 })
    expect(g.limitY).toBeGreaterThan(0)
    expect(g.limitY).toBeLessThan(200)
    expect(g.yTicks.at(-1).label).toBe('100%')
  })
})

import { describe, it, expect } from 'vitest'
import { warmup, confidenceLabel, etaRange, blindSpotsByNamespace, readableInputs } from './horizonStatus.js'

const NOW = new Date(2026, 10, 6, 10, 0).getTime()

describe('warmup', () => {
  it.each([
    [undefined, null],
    [1800, null],
    [7200, null],
  ])('is none when uptime is %s', (uptime, want) => {
    expect(warmup(uptime, NOW)).toBe(want)
  })

  it('waits for detection in the first 5 minutes', () => {
    expect(warmup(120, NOW)).toEqual({ startedMinutes: 2, detecting: false, forecastsAt: '10:28' })
  })

  it('detects but does not forecast yet after 5 minutes', () => {
    expect(warmup(600, NOW)).toEqual({ startedMinutes: 10, detecting: true, forecastsAt: '10:20' })
  })
})

describe('confidenceLabel', () => {
  it.each([
    [{ kind: 'predicted', failureMode: 'saturation', confidence: 0.92 }, 'trend agreement 92%'],
    [{ kind: 'predicted', failureMode: 'recurrent_failure', confidence: 0.8 }, 'rhythm regularity 80%'],
    [{ kind: 'predicted', failureMode: 'replica_divergence', confidence: 1 }, ''],
    [{ kind: 'detected', failureMode: 'startup_stuck', confidence: 1 }, ''],
    [{ kind: 'predicted', failureMode: 'saturation' }, ''],
  ])('%o reads %s', (failure, want) => {
    expect(confidenceLabel(failure)).toBe(want)
  })
})

describe('etaRange', () => {
  it.each([
    [{}, ''],
    [{ etaEarlySeconds: 660, etaLateSeconds: 1140 }, '11 min to 19 min'],
    [{ etaEarlySeconds: 660 }, 'from 11 min'],
  ])('%o reads %s', (failure, want) => {
    expect(etaRange(failure)).toBe(want)
  })
})

describe('blindSpotsByNamespace', () => {
  it('groups pods by namespace, largest first', () => {
    const got = blindSpotsByNamespace(['b/p2', 'a/p1', 'b/p1', 'c/p1', 'a/p0', 'b/p3'])
    expect(got).toEqual([
      { namespace: 'b', pods: ['p1', 'p2', 'p3'] },
      { namespace: 'a', pods: ['p0', 'p1'] },
      { namespace: 'c', pods: ['p1'] },
    ])
  })

  it('is empty without pods', () => {
    expect(blindSpotsByNamespace(undefined)).toEqual([])
  })
})

describe('readableInputs', () => {
  it('names roles and lists available inputs first, widest first', () => {
    const got = readableInputs([
      { role: 'memory.limit', targets: 36, available: true },
      { role: 'disk.used', targets: 0, available: false },
      { role: 'container.restarts', targets: 137, available: true },
    ])
    expect(got.map(i => [i.label, i.targets])).toEqual([['Restarts', 137], ['Memory limit', 36], ['disk.used', 0]])
  })
})

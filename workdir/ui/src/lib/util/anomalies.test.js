import { describe, it, expect } from 'vitest'
import { metricInfo, formatMeasure, anomalyRows, pressuredNamespaces, groupName } from './anomalies.js'

describe('metricInfo', () => {
  it.each([
    ['k8s.pod.network.io', 'Network traffic'],
    ['k8s.pod.memory_limit_utilization', 'Memory used of limit'],
    ['k8s.pod.cpu.utilization', 'CPU in use'],
    ['something.else', 'something.else'],
  ])('labels %s', (metric, label) => {
    expect(metricInfo(metric).label).toBe(label)
  })
})

describe('formatMeasure', () => {
  it.each([
    ['k8s.pod.network.io', 2048, '2.0 KB/s'],
    ['k8s.pod.network.io', 3.5e6, '3.3 MB/s'],
    ['k8s.pod.memory_limit_utilization', 0.91, '91%'],
    ['k8s.pod.cpu.utilization', 0.25, '0.25 cores'],
    ['other', 12.345, '12.35'],
  ])('formats %s %s', (metric, value, want) => {
    expect(formatMeasure(metric, value)).toBe(want)
  })
})

describe('anomalyRows', () => {
  const now = Date.parse('2026-11-02T09:00:00Z')
  const at = minutes => new Date(now - minutes * 60000).toISOString()

  it('keeps the last hour, newest first, with a comparison to usual', () => {
    const rows = anomalyRows([
      { host: 'n/Deployment/a', metric: 'k8s.pod.network.io', value: 4000, expected: 1000, timestamp: at(10), severity: 'warning' },
      { host: 'n/Deployment/b', metric: 'k8s.pod.network.io', value: 10, expected: 5, timestamp: at(90), severity: 'warning' },
      { host: 'n/Deployment/c', metric: 'k8s.pod.memory_limit_utilization', value: 0.2, expected: 0.4, timestamp: at(2), severity: 'critical' },
    ], now)
    expect(rows.map(r => r.name)).toEqual(['c', 'a'])
    expect(rows[1]).toMatchObject({ scope: 'n', minutesAgo: 10, observed: '3.9 KB/s', usual: '1000 B/s', change: '4× usual' })
    expect(rows[0].change).toBe('half of usual')
  })
})

describe('pressuredNamespaces', () => {
  it('drops namespaces with no pressure', () => {
    const got = pressuredNamespaces({ namespaces: { a: { workloadPressure: 0, propPressure: {} }, b: { workloadPressure: 0.2, propPressure: {} } } })
    expect(Object.keys(got.namespaces)).toEqual(['b'])
  })
})

describe('groupName', () => {
  it('reads group keys with or without prefix', () => {
    expect(groupName('grp.network')).toBe('Network')
    expect(groupName('grp.controlplane')).toBe('Control plane')
    expect(groupName('custom')).toBe('custom')
  })
})

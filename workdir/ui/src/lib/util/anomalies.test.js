import { describe, it, expect } from 'vitest'
import { pressuredNamespaces, groupName } from './anomalies.js'

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

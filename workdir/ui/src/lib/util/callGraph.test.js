import { describe, it, expect } from 'vitest'
import { isPlatform, filterGraph, mergeEdges, layout, blastRadius, hitEdges, NODE_W } from './callGraph.js'

const node = (id, namespace = id.split('/')[0]) => ({ id, namespace, name: id.split('/').pop() })
const shop = {
  nodes: [node('shop/Route/front'), node('shop/Deployment/load'), node('shop/Deployment/api'), node('shop/StatefulSet/db'), node('openshift-x/Deployment/op'), node('openshift-x/Deployment/dep')],
  edges: [
    { from: 'shop/Route/front', to: 'shop/Deployment/api' },
    { from: 'shop/Deployment/load', to: 'shop/Deployment/api' },
    { from: 'shop/Deployment/api', to: 'shop/StatefulSet/db' },
    { from: 'openshift-x/Deployment/op', to: 'openshift-x/Deployment/dep' },
  ],
}

describe('isPlatform', () => {
  it.each([['openshift-monitoring', true], ['kube-system', true], ['demo-shop', false], ['default', false]])('%s is %s', (ns, want) => {
    expect(isPlatform(ns)).toBe(want)
  })
})

describe('filterGraph', () => {
  it('keeps edges inside the selected namespaces and their nodes', () => {
    const got = filterGraph(shop, new Set(['shop']))
    expect(got.edges).toHaveLength(3)
    expect(got.nodes.map(n => n.id)).not.toContain('openshift-x/Deployment/op')
  })
})

describe('layout', () => {
  it('places callers left of what they call', () => {
    const g = layout(filterGraph(shop, new Set(['shop'])))
    const x = id => g.nodes.find(n => n.id === id).x
    expect(x('shop/Route/front')).toBe(x('shop/Deployment/load'))
    expect(x('shop/Deployment/api')).toBeGreaterThan(x('shop/Deployment/load'))
    expect(x('shop/StatefulSet/db')).toBeGreaterThan(x('shop/Deployment/api'))
    expect(g.width).toBeGreaterThan(x('shop/StatefulSet/db') + NODE_W - 1)
    expect(g.edges.every(e => e.d.startsWith('M'))).toBe(true)
  })

  it('survives a cycle', () => {
    const g = layout({ nodes: [node('a/D/x'), node('a/D/y')], edges: [{ from: 'a/D/x', to: 'a/D/y' }, { from: 'a/D/y', to: 'a/D/x' }] })
    expect(g.nodes).toHaveLength(2)
  })

  it('is empty without edges', () => {
    const g = layout({ nodes: [], edges: [] })
    expect(g.nodes).toEqual([])
    expect(g.height).toBeGreaterThan(0)
  })
})

describe('blastRadius', () => {
  it('collects every direct and indirect caller', () => {
    expect([...blastRadius(shop.edges, 'shop/StatefulSet/db')].sort()).toEqual(['shop/Deployment/api', 'shop/Deployment/load', 'shop/Route/front'])
  })

  it('is empty for a node nobody calls', () => {
    expect(blastRadius(shop.edges, 'shop/Deployment/load').size).toBe(0)
  })

  it('marks the edges leading to the failure', () => {
    const radius = blastRadius(shop.edges, 'shop/StatefulSet/db')
    const hit = hitEdges(shop.edges, 'shop/StatefulSet/db', radius)
    expect(hit.has('shop/Deployment/api|shop/StatefulSet/db')).toBe(true)
    expect(hit.has('shop/Deployment/load|shop/Deployment/api')).toBe(true)
    expect(hit.has('openshift-x/Deployment/op|openshift-x/Deployment/dep')).toBe(false)
  })
})

describe('mergeEdges', () => {
  const nodes = [node('td/D/checkout'), node('td/D/payments'), node('td/D/inventory'), node('td/D/fraud')]
  const graph = {
    nodes,
    edges: [
      { from: 'td/D/checkout', to: 'td/D/payments', source: 'env', via: 'PAYMENTS_URL' },
      { from: 'td/D/checkout', to: 'td/D/payments', source: 'traces', via: 'traces_service_graph', callsPerMinute: 114.5, errorRatio: 0.0386 },
      { from: 'td/D/checkout', to: 'td/D/inventory', source: 'traces', via: 'traces_service_graph', callsPerMinute: 109.5, errorRatio: 0 },
      { from: 'td/D/payments', to: 'td/D/fraud', source: 'env', via: 'FRAUD_URL' },
    ],
  }
  it('draws a call declared and observed once, as observed, with both sources', () => {
    const got = mergeEdges(graph).edges
    expect(got).toHaveLength(3)
    const pay = got.find(e => e.to === 'td/D/payments')
    expect(pay.source).toBe('traces')
    expect(pay.observed).toBe(true)
    expect(pay.sources.map(s => s.source)).toEqual(['env', 'traces'])
    expect(pay.callsPerMinute).toBe(114.5)
    expect(pay.errorRatio).toBe(0.0386)
  })
  it('keeps a declared-only call declared and unmeasured', () => {
    const fraud = mergeEdges(graph).edges.find(e => e.to === 'td/D/fraud')
    expect(fraud.source).toBe('env')
    expect(fraud.observed).toBe(false)
    expect(fraud.callsPerMinute).toBeUndefined()
  })
  it('leaves each pair unique for keyed rendering', () => {
    const keys = mergeEdges(graph).edges.map(e => e.from + e.to)
    expect(new Set(keys).size).toBe(keys.length)
  })
})

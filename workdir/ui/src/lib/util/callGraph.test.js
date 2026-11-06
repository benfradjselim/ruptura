import { describe, it, expect } from 'vitest'
import { isPlatform, filterGraph, layout, blastRadius, hitEdges, NODE_W } from './callGraph.js'

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

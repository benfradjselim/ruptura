// Layout of the call graph: columns by call depth, from entry points to the
// deepest dependency, and the blast radius of a failing workload.

export const PLATFORM_PREFIXES = ['openshift-', 'kube-']
export const NODE_W = 190
export const NODE_H = 54
const COL_GAP = 110
const ROW_GAP = 22
const PAD = 24

/** @param {string} namespace */
export function isPlatform(namespace) {
  return namespace === 'default' ? false : PLATFORM_PREFIXES.some(p => namespace.startsWith(p))
}

/**
 * Keeps edges whose two ends are in a selected namespace, and their nodes.
 * @param {{ nodes: any[], edges: any[] }} graph
 * @param {Set<string>} namespaces
 */
export function filterGraph(graph, namespaces) {
  const byId = new Map(graph.nodes.map(n => [n.id, n]))
  const edges = graph.edges.filter(e => namespaces.has(byId.get(e.from)?.namespace) && namespaces.has(byId.get(e.to)?.namespace))
  const ids = new Set(edges.flatMap(e => [e.from, e.to]))
  return { nodes: graph.nodes.filter(n => ids.has(n.id)), edges }
}

/**
 * Longest call depth from a node with no caller. A cycle cannot push a node
 * further than the number of nodes.
 */
function depths(nodes, edges) {
  const depth = new Map(nodes.map(n => [n.id, 0]))
  for (let round = 0; round < nodes.length; round++) {
    let moved = false
    for (const e of edges) {
      const next = (depth.get(e.from) ?? 0) + 1
      if (next > (depth.get(e.to) ?? 0) && next < nodes.length) {
        depth.set(e.to, next)
        moved = true
      }
    }
    if (!moved) break
  }
  return depth
}

/**
 * @param {{ nodes: { id: string, namespace: string, name: string }[], edges: { from: string, to: string }[] }} graph
 */
export function layout(graph) {
  const depth = depths(graph.nodes, graph.edges)
  const columns = []
  for (const node of graph.nodes) {
    const d = depth.get(node.id)
    ;(columns[d] ||= []).push(node)
  }
  const placed = new Map()
  columns.forEach((column = [], c) => {
    column
      .sort((a, b) => a.namespace.localeCompare(b.namespace) || a.name.localeCompare(b.name))
      .forEach((node, r) => placed.set(node.id, { ...node, x: PAD + c * (NODE_W + COL_GAP), y: PAD + r * (NODE_H + ROW_GAP) }))
  })
  const nodes = [...placed.values()]
  const edges = graph.edges.map(e => {
    const a = placed.get(e.from), b = placed.get(e.to)
    const x1 = a.x + NODE_W, y1 = a.y + NODE_H / 2, x2 = b.x, y2 = b.y + NODE_H / 2
    const bend = Math.max(40, (x2 - x1) / 2)
    return { ...e, d: `M${x1},${y1} C${x1 + bend},${y1} ${x2 - bend},${y2} ${x2},${y2}` }
  })
  const rows = Math.max(0, ...columns.map(c => (c ? c.length : 0)))
  return {
    nodes,
    edges,
    width: PAD * 2 + Math.max(1, columns.length) * NODE_W + Math.max(0, columns.length - 1) * COL_GAP,
    height: PAD * 2 + rows * NODE_H + Math.max(0, rows - 1) * ROW_GAP,
  }
}

/**
 * Everything that calls target, directly or not: what its failure can hit.
 * @param {{ from: string, to: string }[]} edges
 * @param {string} target
 * @returns {Set<string>}
 */
export function blastRadius(edges, target) {
  const hit = new Set()
  const queue = [target]
  while (queue.length) {
    const next = queue.shift()
    for (const e of edges) {
      if (e.to === next && !hit.has(e.from) && e.from !== target) {
        hit.add(e.from)
        queue.push(e.from)
      }
    }
  }
  return hit
}

/** Edges on the way from a caller in the blast radius to the failing node. */
export function hitEdges(edges, target, radius) {
  return new Set(edges.filter(e => (e.to === target || radius.has(e.to)) && radius.has(e.from)).map(e => e.from + '|' + e.to))
}

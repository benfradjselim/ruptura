<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import cytoscape from 'cytoscape'
  import { fetchTopology, fetchFleet } from '../lib/api'
  import type { TopologyNode, TopologyEdge } from '../lib/api'

  // ── state ────────────────────────────────────────────────────────────────────
  let container: HTMLDivElement
  let cy: cytoscape.Core | null = null
  let loading = true
  let loadErr = ''
  let edgeType = 'none'
  let nodeCount = 0
  let edgeCount = 0
  let refreshTimer: ReturnType<typeof setInterval>

  // detail panel
  let selId: string | null = null
  let selNode: TopologyNode | null = null
  let selUpstream: string[] = []
  let selDownstream: string[] = []
  let allEdges: TopologyEdge[] = []

  // ── colour helpers ───────────────────────────────────────────────────────────
  function healthColor(score: number, state: string): string {
    if (state === 'pending_telemetry' || state === 'calibrating') return '#2a3550'
    if (score >= 75) return '#00e5a0'
    if (score >= 50) return '#f59e0b'
    if (score >= 25) return '#f97316'
    return '#ef4444'
  }

  function edgeColor(e: TopologyEdge): string {
    if (e.edge_type === 'trace') {
      if (e.error_rate > 0.15) return '#ef4444'
      if (e.error_rate > 0.05) return '#f59e0b'
      return '#3fb950'
    }
    return '#4b7ca8'
  }

  function pct(v: number): string { return (v * 100).toFixed(1) + '%' }
  function bar(v: number): number  { return Math.min(Math.round(v * 100), 100) }

  function shortId(id: string): string {
    const parts = id.split('/')
    return parts[parts.length - 1] || id
  }

  function truncate(s: string, max = 14): string {
    return s.length > max ? s.slice(0, max - 1) + '…' : s
  }

  function stateLabel(s: string): string { return s.replace(/_/g, ' ') }

  // ── build cytoscape elements ─────────────────────────────────────────────────
  function buildElements(nodes: TopologyNode[], edges: TopologyEdge[]): cytoscape.ElementDefinition[] {
    const elems: cytoscape.ElementDefinition[] = []

    for (const n of nodes) {
      const col = healthColor(n.health_score, n.state)
      const label = truncate(n.label || shortId(n.id))
      elems.push({
        data: {
          id: n.id,
          label,
          fullLabel: n.label || shortId(n.id),
          namespace: n.namespace,
          kind: n.kind,
          health_score: n.health_score,
          fused_r: n.fused_r,
          state: n.state,
          stress: n.stress,
          fatigue: n.fatigue,
          contagion: n.contagion,
          mood: n.mood,
          velocity: n.velocity,
          entropy: n.entropy,
          color: col,
          scoreText: n.state === 'pending_telemetry' || n.state === 'calibrating'
            ? '?' : String(Math.round(n.health_score)),
        }
      })
    }

    for (const e of edges) {
      elems.push({
        data: {
          id: e.source + '→' + e.target,
          source: e.source,
          target: e.target,
          edge_type: e.edge_type,
          error_rate: e.error_rate,
          p99_latency_ms: e.p99_latency_ms,
          strength: e.strength,
          color: edgeColor(e),
          dashed: e.edge_type !== 'trace',
        }
      })
    }

    return elems
  }

  // ── load data ────────────────────────────────────────────────────────────────
  async function load() {
    loadErr = ''
    try {
      // Use both fleet (all workloads) and topology (edges + KPIs)
      const [topo, fleet] = await Promise.all([fetchTopology(), fetchFleet()])

      // Merge: topology nodes have KPIs; fleet adds workloads not yet in topology
      const topoById = new Map(topo.nodes.map(n => [n.id, n]))
      const topoNodes: TopologyNode[] = [...topo.nodes]

      for (const h of fleet.hosts) {
        if (!topoById.has(h.host)) {
          topoNodes.push({
            id: h.host,
            label: shortId(h.host),
            namespace: h.host.split('/')[0] ?? '',
            kind: h.host.split('/')[1] ?? 'Deployment',
            health_score: h.health_score,
            fused_r: h.fused_rupture_index,
            state: h.state,
            stress: h.stress,
            fatigue: h.fatigue,
            contagion: h.contagion,
            mood: 0.5,
            velocity: 0.5,
            entropy: 0.5,
          })
        }
      }

      allEdges = topo.edges
      edgeType = topo.edges.length === 0 ? 'none'
        : topo.edges.some(e => e.edge_type === 'trace') ? 'trace' : 'inferred'
      nodeCount = topoNodes.length
      edgeCount = topo.edges.length

      const elems = buildElements(topoNodes, topo.edges)

      if (!cy) {
        initCytoscape(elems)
      } else {
        updateCytoscape(elems)
      }
    } catch (e) {
      loadErr = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }

  // ── init cytoscape ────────────────────────────────────────────────────────────
  function initCytoscape(elems: cytoscape.ElementDefinition[]) {
    cy = cytoscape({
      container,
      elements: elems,
      style: cytoscapeStyle(),
      layout: layoutConfig(elems.filter(e => !e.data?.source).length),
      userZoomingEnabled: true,
      userPanningEnabled: true,
      boxSelectionEnabled: false,
      minZoom: 0.2,
      maxZoom: 3,
    })

    cy.on('tap', 'node', evt => {
      const node = evt.target
      const id = node.id() as string
      if (selId === id) {
        clearSelection()
        return
      }
      selId = id
      selNode = {
        id,
        label: node.data('fullLabel'),
        namespace: node.data('namespace'),
        kind: node.data('kind'),
        health_score: node.data('health_score'),
        fused_r: node.data('fused_r'),
        state: node.data('state'),
        stress: node.data('stress'),
        fatigue: node.data('fatigue'),
        contagion: node.data('contagion'),
        mood: node.data('mood'),
        velocity: node.data('velocity'),
        entropy: node.data('entropy'),
      }
      selUpstream = allEdges.filter(e => e.target === id).map(e => shortId(e.source))
      selDownstream = allEdges.filter(e => e.source === id).map(e => shortId(e.target))

      // dim non-connected elements
      cy!.elements().addClass('dimmed')
      node.removeClass('dimmed')
      node.neighborhood().removeClass('dimmed')
    })

    cy.on('tap', evt => {
      if (evt.target === cy) clearSelection()
    })
  }

  function clearSelection() {
    selId = null
    selNode = null
    selUpstream = []
    selDownstream = []
    cy?.elements().removeClass('dimmed')
  }

  function updateCytoscape(elems: cytoscape.ElementDefinition[]) {
    if (!cy) return
    const prevSelId = selId

    cy.batch(() => {
      // update existing nodes, add new ones
      for (const el of elems) {
        const existing = cy!.getElementById(el.data.id as string)
        if (existing.length > 0) {
          existing.data(el.data)
        } else {
          cy!.add(el)
        }
      }
      // remove nodes/edges no longer in data
      const newIds = new Set(elems.map(e => e.data.id as string))
      cy!.elements().forEach(el => {
        if (!newIds.has(el.id())) el.remove()
      })
    })

    cy.style(cytoscapeStyle())

    // re-run layout only if topology changed significantly
    if (prevSelId) {
      const node = cy.getElementById(prevSelId)
      if (node.length > 0) {
        cy.elements().addClass('dimmed')
        node.removeClass('dimmed')
        node.neighborhood().removeClass('dimmed')
      }
    }
  }

  function layoutConfig(nodeCount: number) {
    return {
      name: 'cose',
      animate: false,
      randomize: true,
      nodeRepulsion: () => nodeCount > 15 ? 10000 : 6000,
      idealEdgeLength: () => 160,
      edgeElasticity: () => 45,
      gravity: 0.25,
      numIter: 1000,
      fit: true,
      padding: 40,
    }
  }

  function cytoscapeStyle(): cytoscape.StylesheetStyle[] {
    return [
      {
        selector: 'node',
        style: {
          'width': 50,
          'height': 50,
          'background-color': 'data(color)',
          'background-opacity': 0.88,
          'border-width': 2,
          'border-color': '#0d1117',
          'label': 'data(label)',
          'text-valign': 'bottom',
          'text-halign': 'center',
          'color': '#cbd5e1',
          'font-size': 11,
          'font-family': '"JetBrains Mono", monospace',
          'text-margin-y': 4,
          'text-background-color': '#0d1117',
          'text-background-opacity': 0.85,
          'text-background-padding': '3px',
          'text-background-shape': 'roundrectangle',
          'text-wrap': 'none',
          'overlay-padding': '6px',
        } as cytoscape.Css.Node,
      },
      {
        selector: 'node[state = "pending_telemetry"]',
        style: { 'background-opacity': 0.4, 'border-style': 'dashed' } as cytoscape.Css.Node,
      },
      {
        selector: 'node[state = "calibrating"]',
        style: { 'background-opacity': 0.55, 'border-style': 'dashed' } as cytoscape.Css.Node,
      },
      {
        selector: 'node:selected',
        style: {
          'border-color': '#a855f7',
          'border-width': 3,
        } as cytoscape.Css.Node,
      },
      {
        selector: '.dimmed',
        style: { 'opacity': 0.15 } as cytoscape.Css.Node,
      },
      {
        selector: 'edge',
        style: {
          'width': 2,
          'line-color': 'data(color)',
          'target-arrow-color': 'data(color)',
          'target-arrow-shape': 'triangle',
          'curve-style': 'bezier',
          'opacity': 0.65,
          'line-style': 'solid',
        } as cytoscape.Css.Edge,
      },
      {
        selector: 'edge[dashed = true]',
        style: { 'line-style': 'dashed', 'line-dash-pattern': [6, 4] } as cytoscape.Css.Edge,
      },
      {
        selector: 'edge.dimmed',
        style: { 'opacity': 0.06 } as cytoscape.Css.Edge,
      },
    ]
  }

  // ── lifecycle ─────────────────────────────────────────────────────────────────
  onMount(() => {
    load()
    refreshTimer = setInterval(load, 15_000)
  })

  onDestroy(() => {
    clearInterval(refreshTimer)
    cy?.destroy()
    cy = null
  })

  function resetView() { cy?.fit(undefined, 40) }
  function reLayout() {
    if (!cy) return
    cy.layout(layoutConfig(cy.nodes().length)).run()
  }
</script>

<!-- ── root ─────────────────────────────────────────────────────────────────── -->
<div class="topo-root">

  <!-- header bar -->
  <div class="bar">
    <span class="bar-title">Workload Topology</span>
    <div class="bar-tags">
      {#if edgeType === 'trace'}
        <span class="tag tag-trace">● Trace-confirmed edges</span>
      {:else if edgeType === 'inferred'}
        <span class="tag tag-infer">◌ Inferred edges (KPI correlation)</span>
      {:else}
        <span class="tag tag-none">No edges — send OTLP traces to see call graph</span>
      {/if}
    </div>
    <div class="bar-actions">
      <button class="btn-sm" on:click={reLayout} title="Re-layout">⊡</button>
      <button class="btn-sm" on:click={resetView} title="Fit to screen">⤢</button>
      <button class="btn-sm" on:click={load} title="Refresh">↺</button>
    </div>
  </div>

  <!-- canvas -->
  <div class="canvas-wrap">
    {#if loading && nodeCount === 0}
      <div class="overlay"><span class="spin">◌</span> Loading…</div>
    {:else if loadErr}
      <div class="overlay err">{loadErr} <button on:click={load}>Retry</button></div>
    {:else if nodeCount === 0}
      <div class="overlay muted">No workloads tracked yet — deploy workloads and configure a Prometheus datasource.</div>
    {/if}
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="cy-container" bind:this={container}></div>
  </div>

  <!-- legend bar -->
  <div class="legend-bar">
    <div class="leg-group">
      <span class="leg-dot" style="background:#00e5a0"></span>Healthy ≥75
      <span class="leg-dot" style="background:#f59e0b"></span>Degraded ≥50
      <span class="leg-dot" style="background:#f97316"></span>Warning ≥25
      <span class="leg-dot" style="background:#ef4444"></span>Critical
      <span class="leg-dot" style="background:#2a3550;border:1px solid #4b6070"></span>No data
    </div>
    <div class="leg-sep"></div>
    <div class="leg-group">
      <span class="leg-line solid"></span>Trace edge
      <span class="leg-line dashed"></span>Inferred edge
    </div>
    <span class="leg-count">{nodeCount} workloads · {edgeCount} edges</span>
  </div>

  <!-- detail panel -->
  {#if selNode}
    {@const n = selNode}
    {@const col = healthColor(n.health_score, n.state)}
    <div class="detail-panel">
      <div class="dp-header" style="border-left:3px solid {col}">
        <div>
          <div class="dp-name">{n.label || shortId(n.id)}</div>
          <div class="dp-meta">{n.namespace}/{n.kind}</div>
        </div>
        <button class="dp-close" on:click={clearSelection}>✕</button>
      </div>

      <div class="dp-block">
        <div class="dp-row">
          <span class="dp-label">State</span>
          <span class="dp-val state-chip" style="background:{col}22;color:{col}">{stateLabel(n.state)}</span>
        </div>
        {#if n.state !== 'pending_telemetry' && n.state !== 'calibrating'}
          <div class="dp-row">
            <span class="dp-label">Health Score</span>
            <span class="dp-val" style="color:{col}">{Math.round(n.health_score)}<span class="dp-unit">/100</span></span>
          </div>
          <div class="dp-bar-track">
            <div class="dp-bar-fill" style="width:{bar(n.health_score/100)}%;background:{col}"></div>
          </div>
          <div class="dp-row">
            <span class="dp-label">Fused-R</span>
            <span class="dp-val" style="color:{n.fused_r > 0.6 ? '#ef4444' : n.fused_r > 0.3 ? '#f59e0b' : '#00e5a0'}">{n.fused_r.toFixed(3)}</span>
          </div>
        {/if}
      </div>

      {#if n.state !== 'pending_telemetry' && n.state !== 'calibrating'}
        <div class="dp-section-title">KPI Signals</div>
        <div class="dp-block dp-kpis">
          {#each [
            { k:'Stress',    v: n.stress,    warn: 0.6 },
            { k:'Fatigue',   v: n.fatigue,   warn: 0.5 },
            { k:'Contagion', v: n.contagion, warn: 0.4 },
            { k:'Mood',      v: n.mood,      warn: -1 },
            { k:'Velocity',  v: n.velocity,  warn: -1 },
            { k:'Entropy',   v: n.entropy,   warn: 0.7 },
          ] as sig}
            <div class="kpi-row">
              <span class="kpi-label">{sig.k}</span>
              <div class="kpi-track">
                <div class="kpi-fill" style="
                  width:{bar(sig.v)}%;
                  background:{sig.warn > 0 && sig.v > sig.warn ? '#ef4444' : '#00e5a0'}"></div>
              </div>
              <span class="kpi-val">{pct(sig.v)}</span>
            </div>
          {/each}
        </div>

        {#if selUpstream.length > 0}
          <div class="dp-section-title">Called by</div>
          <div class="dp-block dp-deps">
            {#each selUpstream as u}
              <span class="dep-chip dep-up">{u}</span>
            {/each}
          </div>
        {/if}

        {#if selDownstream.length > 0}
          <div class="dp-section-title">Calls</div>
          <div class="dp-block dp-deps">
            {#each selDownstream as d}
              <span class="dep-chip dep-dn">{d}</span>
            {/each}
          </div>
        {/if}

        {#if n.contagion > 0.4}
          <div class="dp-alert">
            <span class="dp-alert-icon">◉</span>
            <span>High contagion ({pct(n.contagion)}) — degradation may spread to downstream callers</span>
          </div>
        {/if}
      {/if}
    </div>
  {/if}
</div>

<style>
  .topo-root {
    position: relative;
    display: flex;
    flex-direction: column;
    width: 100%;
    height: calc(100vh - 108px);
    background: var(--bg);
    border-radius: 12px;
    border: 1px solid var(--border);
    overflow: hidden;
  }

  .bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 14px;
    background: var(--surface2);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    flex-wrap: wrap;
    z-index: 4;
  }
  .bar-title { font-weight: 700; font-size: 12px; color: var(--text); white-space: nowrap; }
  .bar-tags  { display: flex; gap: 8px; flex: 1; flex-wrap: wrap; }
  .bar-actions { display: flex; gap: 6px; margin-left: auto; }

  .tag {
    font-size: 10px;
    padding: 2px 8px;
    border-radius: 10px;
    border: 1px solid var(--border);
    white-space: nowrap;
  }
  .tag-trace { color: #3fb950; border-color: #3fb95060; }
  .tag-infer { color: #6b8cba; border-color: #6b8cba60; }
  .tag-none  { color: var(--muted); }

  .btn-sm {
    background: none;
    border: 1px solid var(--border);
    color: var(--muted);
    border-radius: 6px;
    padding: 3px 8px;
    cursor: pointer;
    font-size: 13px;
    transition: color 0.15s;
  }
  .btn-sm:hover { color: var(--text); }

  .canvas-wrap {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  .cy-container {
    width: 100%;
    height: 100%;
    background: var(--bg);
  }

  .overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    background: rgba(5,8,18,0.7);
    color: var(--muted);
    font-size: 14px;
    z-index: 10;
    pointer-events: none;
  }
  .overlay.err { color: var(--red); pointer-events: auto; }
  .overlay.muted { color: var(--muted); }
  .overlay button {
    background: none;
    border: 1px solid var(--border);
    color: var(--text);
    padding: 4px 12px;
    border-radius: 6px;
    cursor: pointer;
    pointer-events: auto;
  }

  .legend-bar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 6px 14px;
    background: var(--surface2);
    border-top: 1px solid var(--border);
    flex-shrink: 0;
    font-size: 10px;
    color: var(--muted);
    flex-wrap: wrap;
    z-index: 4;
  }
  .leg-group  { display: flex; align-items: center; gap: 7px; flex-wrap: wrap; }
  .leg-dot    { width: 8px; height: 8px; border-radius: 50%; display: inline-block; flex-shrink: 0; }
  .leg-sep    { width: 1px; height: 16px; background: var(--border); }
  .leg-count  { margin-left: auto; font-variant-numeric: tabular-nums; white-space: nowrap; }
  .leg-line   { display: inline-block; width: 20px; height: 2px; vertical-align: middle; }
  .leg-line.solid  { background: #3fb950; }
  .leg-line.dashed { background: linear-gradient(90deg,#6b8cba 50%,transparent 50%); background-size: 6px 2px; }

  /* detail panel */
  .detail-panel {
    position: absolute;
    top: 50px;
    right: 14px;
    width: 260px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    z-index: 20;
    display: flex;
    flex-direction: column;
    max-height: calc(100% - 110px);
    overflow-y: auto;
    box-shadow: 0 4px 24px rgba(0,0,0,0.4);
  }
  .dp-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border);
    background: var(--surface2);
    gap: 8px;
  }
  .dp-name { font-weight: 700; font-size: 13px; color: var(--text); }
  .dp-meta { font-size: 9px; color: var(--muted); font-family: monospace; }
  .dp-close {
    background: none; border: none; color: var(--muted);
    cursor: pointer; font-size: 14px; padding: 0; line-height: 1; flex-shrink: 0;
  }
  .dp-close:hover { color: var(--text); }

  .dp-block { padding: 4px 0; }
  .dp-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 12px;
    font-size: 12px;
    border-bottom: 1px solid rgba(30,45,69,0.4);
  }
  .dp-label { color: var(--muted); font-size: 11px; }
  .dp-val   { font-weight: 600; font-variant-numeric: tabular-nums; }
  .dp-unit  { font-size: 9px; font-weight: 400; color: var(--muted); }
  .state-chip { padding: 2px 7px; border-radius: 8px; font-size: 10px; font-weight: 600; text-transform: capitalize; }

  .dp-bar-track {
    height: 3px; background: var(--surface3); margin: 0 12px 6px; border-radius: 2px; overflow: hidden;
  }
  .dp-bar-fill { height: 100%; border-radius: 2px; transition: width 0.4s; }

  .dp-section-title {
    padding: 6px 12px 2px;
    font-size: 9px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
  }

  .dp-kpis { padding: 0 12px 4px; }
  .kpi-row { display: flex; align-items: center; gap: 6px; padding: 3px 0; }
  .kpi-label { font-size: 9px; color: var(--muted); width: 60px; flex-shrink: 0; }
  .kpi-track { flex: 1; height: 4px; background: var(--surface3); border-radius: 2px; overflow: hidden; }
  .kpi-fill  { height: 100%; border-radius: 2px; transition: width 0.4s; }
  .kpi-val   { font-size: 9px; color: var(--text); width: 36px; text-align: right; font-variant-numeric: tabular-nums; }

  .dp-deps { display: flex; flex-wrap: wrap; gap: 5px; padding: 4px 12px 6px; }
  .dep-chip { font-size: 9px; padding: 2px 7px; border-radius: 8px; }
  .dep-up   { background: rgba(168,85,247,0.15); color: #c4b5fd; }
  .dep-dn   { background: rgba(59,186,80,0.12);  color: #3fb950; }

  .dp-alert {
    margin: 6px 10px 10px;
    padding: 7px 9px;
    background: rgba(239,68,68,0.08);
    border: 1px solid rgba(239,68,68,0.3);
    border-radius: 6px;
    font-size: 10px;
    color: var(--muted);
    display: flex;
    gap: 6px;
    line-height: 1.5;
  }
  .dp-alert-icon { color: #ef4444; font-size: 11px; flex-shrink: 0; }

  .spin { display: inline-block; animation: spin 1s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>

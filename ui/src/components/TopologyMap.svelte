<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import cytoscape from 'cytoscape'
  import type { Core, NodeSingular } from 'cytoscape'
  import { fetchTopology } from '../lib/api'
  import type { TopologyNode, TopologyEdge } from '../lib/api'

  export let apiKey = ''

  let container: HTMLDivElement
  let cy: Core | null = null
  let selected: TopologyNode | null = null
  let loadErr = ''
  let loading = true
  let nodeCount = 0
  let edgeCount = 0

  function healthColor(node: TopologyNode): string {
    if (node.state === 'pending_telemetry') return '#484f58'
    if (node.health_score >= 70) return '#3fb950'
    if (node.health_score >= 40) return '#e3b341'
    if (node.health_score >= 20) return '#f0883e'
    return '#e05252'
  }

  function errorColor(rate: number): string {
    if (rate < 0.05) return '#3fb950'
    if (rate < 0.15) return '#e3b341'
    return '#e05252'
  }

  function edgeWidth(callRate: number, maxRate: number): number {
    if (maxRate === 0) return 1
    return 1 + Math.round((callRate / maxRate) * 7)
  }

  function shortId(id: string): string {
    const parts = id.split('/')
    return parts[parts.length - 1] || id
  }

  function isRuptureActive(node: TopologyNode): boolean {
    return node.state !== 'pending_telemetry' && node.fused_r > 1.5 && node.health_score > 60
  }

  async function load() {
    loading = true
    loadErr = ''
    try {
      const graph = await fetchTopology(apiKey)
      nodeCount = graph.nodes.length
      edgeCount = graph.edges.length
      renderGraph(graph.nodes, graph.edges)
    } catch (e) {
      loadErr = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }

  function renderGraph(nodes: TopologyNode[], edges: TopologyEdge[]) {
    if (cy) {
      cy.destroy()
      cy = null
    }

    const maxRate = edges.reduce((m, e) => Math.max(m, e.call_rate), 0)

    const elements = [
      ...nodes.map(n => ({
        data: {
          id: n.id,
          label: shortId(n.id),
          healthScore: n.health_score,
          fusedR: n.fused_r,
          state: n.state,
          rupture: isRuptureActive(n),
          color: healthColor(n),
        },
      })),
      ...edges.map(e => ({
        data: {
          id: `${e.source}→${e.target}`,
          source: e.source,
          target: e.target,
          callRate: e.call_rate,
          errorRate: e.error_rate,
          p99: e.p99_latency_ms,
          width: edgeWidth(e.call_rate, maxRate),
          color: errorColor(e.error_rate),
        },
      })),
    ]

    cy = cytoscape({
      container,
      elements,
      style: [
        {
          selector: 'node',
          style: {
            'background-color': 'data(color)',
            'label': 'data(label)',
            'color': '#c9d1d9',
            'font-size': 10,
            'text-valign': 'bottom',
            'text-margin-y': 4,
            'width': 28,
            'height': 28,
            'border-width': 0,
            'text-background-color': '#0d1117',
            'text-background-opacity': 0.7,
            'text-background-padding': '2px',
          },
        },
        {
          selector: 'node[?rupture]',
          style: {
            'border-width': 2,
            'border-color': '#e05252',
            'border-style': 'dashed',
          },
        },
        {
          selector: 'node:selected',
          style: {
            'border-width': 2,
            'border-color': '#58a6ff',
            'border-style': 'solid',
          },
        },
        {
          selector: 'edge',
          style: {
            'width': 'data(width)',
            'line-color': 'data(color)',
            'target-arrow-color': 'data(color)',
            'target-arrow-shape': 'triangle',
            'curve-style': 'bezier',
            'opacity': 0.7,
          },
        },
        {
          selector: 'edge:selected',
          style: { 'opacity': 1 },
        },
      ],
      layout: {
        name: 'cose',
        animate: false,
        randomize: true,
        nodeRepulsion: () => 8000,
        idealEdgeLength: () => 100,
        edgeElasticity: () => 100,
        gravity: 0.25,
        numIter: 1000,
      },
      userZoomingEnabled: true,
      userPanningEnabled: true,
    })

    cy.on('tap', 'node', (evt) => {
      const n = evt.target as NodeSingular
      const d = n.data()
      selected = {
        id: d.id,
        health_score: d.healthScore,
        fused_r: d.fusedR,
        state: d.state,
      }
    })

    cy.on('tap', (evt) => {
      if (evt.target === cy) selected = null
    })
  }

  onMount(load)

  onDestroy(() => {
    if (cy) cy.destroy()
  })
</script>

<div class="topology-root">
  {#if loading}
    <div class="overlay"><span class="spin">◌</span> Loading topology…</div>
  {:else if loadErr}
    <div class="overlay err">{loadErr} <button on:click={load}>Retry</button></div>
  {:else if nodeCount === 0}
    <div class="overlay muted">
      <div class="empty-icon">⎋</div>
      <strong>No workloads discovered yet.</strong>
      <span>Topology builds from OTLP trace data. Deploy workloads and send traces to populate this view.</span>
    </div>
  {/if}

  <div class="cy-container" bind:this={container}></div>

  <div class="toolbar">
    <span class="count">{nodeCount} nodes · {edgeCount} edges</span>
    <button class="refresh-btn" on:click={load} title="Refresh">↺</button>
    <div class="legend">
      <span class="dot" style="background:#3fb950"></span>healthy
      <span class="dot" style="background:#e3b341"></span>degraded
      <span class="dot" style="background:#f0883e"></span>warning
      <span class="dot" style="background:#e05252"></span>critical
      <span class="dot" style="background:#484f58"></span>pending
    </div>
  </div>

  {#if selected}
    <div class="side-panel">
      <div class="sp-header">
        <span class="sp-title">{shortId(selected.id)}</span>
        <button class="sp-close" on:click={() => (selected = null)}>✕</button>
      </div>
      <div class="sp-id">{selected.id}</div>

      <div class="sp-row">
        <span class="sp-label">State</span>
        <span class="sp-val state-{selected.state}">{selected.state.replace('_', ' ')}</span>
      </div>

      {#if selected.state !== 'pending_telemetry'}
        <div class="sp-row">
          <span class="sp-label">Health Score</span>
          <span class="sp-val">{Math.round(selected.health_score)}</span>
        </div>
        <div class="sp-row">
          <span class="sp-label">FusedR</span>
          <span class="sp-val" class:warn={selected.fused_r > 1.5}>{selected.fused_r.toFixed(2)}</span>
        </div>
        {#if isRuptureActive(selected)}
          <div class="rupture-warn">
            Early rupture signal — FusedR exceeds threshold while HealthScore is still high.
          </div>
        {/if}
      {/if}
    </div>
  {/if}
</div>

<style>
  .topology-root {
    position: relative;
    width: 100%;
    height: calc(100vh - 52px);
    background: #0d1117;
    overflow: hidden;
  }

  .cy-container {
    width: 100%;
    height: 100%;
  }

  .overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    z-index: 10;
    background: rgba(13, 17, 23, 0.85);
    color: var(--muted);
    font-size: 14px;
    text-align: center;
    padding: 32px;
  }

  .overlay.err { color: var(--red); }

  .overlay button {
    background: none;
    border: 1px solid var(--border);
    color: var(--text);
    padding: 4px 12px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
    margin-top: 4px;
  }

  .empty-icon { font-size: 52px; opacity: 0.3; }

  .spin {
    display: inline-block;
    animation: spin 1.2s linear infinite;
  }

  @keyframes spin { to { transform: rotate(360deg); } }

  .toolbar {
    position: absolute;
    bottom: 16px;
    left: 16px;
    display: flex;
    align-items: center;
    gap: 16px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 8px 14px;
    font-size: 12px;
    color: var(--muted);
    z-index: 5;
  }

  .count { font-variant-numeric: tabular-nums; }

  .refresh-btn {
    background: none;
    border: none;
    color: var(--muted);
    cursor: pointer;
    font-size: 16px;
    padding: 0 4px;
    border-radius: 4px;
    line-height: 1;
    transition: color 0.15s;
  }

  .refresh-btn:hover { color: var(--cyan); }

  .legend {
    display: flex;
    align-items: center;
    gap: 6px;
    border-left: 1px solid var(--border);
    padding-left: 14px;
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
  }

  /* side panel */
  .side-panel {
    position: absolute;
    top: 16px;
    right: 16px;
    width: 260px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    z-index: 10;
    overflow: hidden;
  }

  .sp-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 14px;
    border-bottom: 1px solid var(--border);
    background: var(--surface2);
  }

  .sp-title { font-weight: 700; font-size: 14px; }

  .sp-close {
    background: none;
    border: none;
    color: var(--muted);
    cursor: pointer;
    font-size: 14px;
    padding: 2px 5px;
    border-radius: 4px;
  }

  .sp-close:hover { color: var(--text); }

  .sp-id {
    padding: 8px 14px;
    font-size: 10px;
    color: var(--muted);
    font-family: monospace;
    word-break: break-all;
    border-bottom: 1px solid var(--border);
  }

  .sp-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 14px;
    border-bottom: 1px solid rgba(48, 54, 61, 0.5);
    font-size: 13px;
  }

  .sp-row:last-child { border-bottom: none; }

  .sp-label { color: var(--muted); }

  .sp-val { font-variant-numeric: tabular-nums; font-weight: 600; }

  .sp-val.warn { color: var(--red); }

  .state-healthy     { color: #3fb950; }
  .state-degraded    { color: #e3b341; }
  .state-critical    { color: #e05252; }
  .state-pending_telemetry { color: var(--muted); }

  .rupture-warn {
    margin: 8px 14px 12px;
    padding: 8px 10px;
    background: rgba(224, 82, 82, 0.08);
    border: 1px solid rgba(224, 82, 82, 0.3);
    border-radius: 6px;
    font-size: 11px;
    color: var(--red);
    line-height: 1.5;
  }
</style>

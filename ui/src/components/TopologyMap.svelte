<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import cytoscape from 'cytoscape'
  import type { Core, NodeSingular, EdgeSingular } from 'cytoscape'
  import { fetchTopology } from '../lib/api'
  import type { TopologyNode, TopologyEdge } from '../lib/api'

  let container: HTMLDivElement
  let cy: Core | null = null
  let selectedNode: TopologyNode | null = null
  let selectedEdge: TopologyEdge | null = null
  let loadErr = ''
  let loading = true
  let nodeCount = 0
  let edgeCount = 0
  let refreshInterval: ReturnType<typeof setInterval>

  function healthColor(node: TopologyNode): string {
    if (node.state === 'pending_telemetry') return '#2a3550'
    if (node.health_score >= 70) return '#00e5a0'
    if (node.health_score >= 40) return '#f59e0b'
    if (node.health_score >= 20) return '#f97316'
    return '#ef4444'
  }

  function errorColor(rate: number): string {
    if (rate < 0.05) return '#3fb950'
    if (rate < 0.15) return '#e3b341'
    return '#e05252'
  }

  function edgeWidth(callRate: number, maxRate: number): number {
    if (maxRate === 0) return 1
    return 1 + Math.round((callRate / maxRate) * 6)
  }

  function shortId(id: string): string {
    const parts = id.split('/')
    return parts[parts.length - 1] || id
  }

  function isRuptureActive(node: TopologyNode): boolean {
    return node.state !== 'pending_telemetry' && node.fused_r > 1.5 && node.health_score > 60
  }

  function fuseLabel(r: number): string {
    if (r < 1.0) return 'normal'
    if (r < 1.5) return 'elevated'
    if (r < 2.5) return 'warning'
    return 'critical'
  }

  function fmtRate(r: number): string {
    if (r >= 1000) return `${(r / 1000).toFixed(1)}k/s`
    return `${r.toFixed(1)}/s`
  }

  function pct(v: number): string {
    return `${(v * 100).toFixed(1)}%`
  }

  // look up original edge data after click so we can surface it
  let edgesData: TopologyEdge[] = []

  async function load() {
    loading = true
    loadErr = ''
    selectedNode = null
    selectedEdge = null
    try {
      const graph = await fetchTopology()
      nodeCount = graph.nodes.length
      edgeCount = graph.edges.length
      edgesData = graph.edges
      renderGraph(graph.nodes, graph.edges)
    } catch (e) {
      loadErr = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }

  function renderGraph(nodes: TopologyNode[], edges: TopologyEdge[]) {
    if (cy) { cy.destroy(); cy = null }

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
            'color': '#e2e8f0',
            'font-size': 11,
            'font-family': "'JetBrains Mono', monospace",
            'text-valign': 'bottom',
            'text-margin-y': 6,
            'width': 34,
            'height': 34,
            'border-width': 1.5,
            'border-color': '#1e2d45',
            'text-background-color': '#0a0d14',
            'text-background-opacity': 0.8,
            'text-background-padding': '3px',
            'text-background-shape': 'roundrectangle',
          },
        },
        {
          selector: 'node[?rupture]',
          style: {
            'border-width': 3,
            'border-color': '#ef4444',
            'border-style': 'dashed',
          },
        },
        {
          selector: 'node:selected',
          style: {
            'border-width': 3,
            'border-color': '#a855f7',
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
            'opacity': 0.65,
          },
        },
        {
          selector: 'edge:selected',
          style: { 'opacity': 1, 'width': 'data(width)' },
        },
      ],
      layout: {
        name: 'cose',
        animate: false,
        randomize: true,
        nodeRepulsion: () => 9000,
        idealEdgeLength: () => 110,
        edgeElasticity: () => 100,
        gravity: 0.25,
        numIter: 1000,
      },
      userZoomingEnabled: true,
      userPanningEnabled: true,
    })

    cy.on('tap', 'node', (evt) => {
      selectedEdge = null
      const n = evt.target as NodeSingular
      const d = n.data()
      selectedNode = { id: d.id, health_score: d.healthScore, fused_r: d.fusedR, state: d.state }
    })

    cy.on('tap', 'edge', (evt) => {
      selectedNode = null
      const e = evt.target as EdgeSingular
      const d = e.data()
      selectedEdge = edgesData.find(x => x.source === d.source && x.target === d.target) ?? {
        source: d.source, target: d.target,
        call_rate: d.callRate, error_rate: d.errorRate, p99_latency_ms: d.p99,
      }
    })

    cy.on('tap', (evt) => {
      if (evt.target === cy) { selectedNode = null; selectedEdge = null }
    })
  }

  onMount(() => { load(); refreshInterval = setInterval(load, 20_000) })
  onDestroy(() => { clearInterval(refreshInterval); if (cy) cy.destroy() })
</script>

<div class="topology-root">
  <!-- explanation banner -->
  <div class="info-bar">
    <span class="info-title">Service Dependency Graph</span>
    <span class="info-sep">·</span>
    <span class="info-body">
      Nodes = workloads (color = HealthScore). Arrows = service calls observed from OTLP trace spans.
      Edge color: <span class="leg-ok">green (&lt;5% errors)</span> →
      <span class="leg-warn">yellow</span> →
      <span class="leg-crit">red (&gt;15% errors)</span>.
      Edge thickness = call volume. Click a node or edge for details.
    </span>
  </div>

  {#if loading}
    <div class="overlay"><span class="spin">◌</span> Loading topology…</div>
  {:else if loadErr}
    <div class="overlay err">{loadErr} <button on:click={load}>Retry</button></div>
  {:else if nodeCount === 0}
    <div class="overlay muted">
      <div class="empty-icon">⎋</div>
      <strong>No service connections discovered yet</strong>
      <p>
        This graph is populated from <strong>distributed trace spans</strong> sent via OTLP.
        Once your services emit traces that propagate trace-context headers between each other,
        Ruptura's correlator will detect the call graph and display it here.
      </p>
      <p style="font-size:11px;margin-top:4px">
        Tip: instrument services with OpenTelemetry and point them at the OTLP receiver on port 4317.
      </p>
    </div>
  {/if}

  <div class="cy-container" bind:this={container}></div>

  <!-- bottom toolbar -->
  <div class="toolbar">
    <span class="count">{nodeCount} nodes · {edgeCount} edges</span>
    <button class="refresh-btn" on:click={load} title="Refresh now">↺</button>
    <div class="legend">
      <span class="dot" style="background:#00e5a0"></span>healthy (&gt;70)
      <span class="dot" style="background:#f59e0b"></span>degraded (&gt;40)
      <span class="dot" style="background:#f97316"></span>warning (&gt;20)
      <span class="dot" style="background:#ef4444"></span>critical
      <span class="dot" style="background:#2a3550"></span>no telemetry
    </div>
  </div>

  <!-- node side panel -->
  {#if selectedNode}
    {@const n = selectedNode}
    <div class="side-panel">
      <div class="sp-header">
        <span class="sp-title">{shortId(n.id)}</span>
        <button class="sp-close" on:click={() => (selectedNode = null)}>✕</button>
      </div>
      <div class="sp-id">{n.id}</div>

      <div class="sp-section">
        <div class="sp-row">
          <span class="sp-label">State</span>
          <span class="sp-val state-{n.state}">{n.state.replace(/_/g, ' ')}</span>
        </div>
        {#if n.state !== 'pending_telemetry'}
          <div class="sp-row">
            <span class="sp-label">Health Score</span>
            <span class="sp-val" style="color:{healthColor(n)}">{Math.round(n.health_score)}<span class="sp-unit">/100</span></span>
          </div>
          <div class="sp-row">
            <span class="sp-label">FusedR</span>
            <span class="sp-val" style="color:{errorColor(Math.min(n.fused_r / 3, 1))}">{n.fused_r.toFixed(3)} <span class="sp-sub">{fuseLabel(n.fused_r)}</span></span>
          </div>
          <div class="sp-bar-row">
            <div class="sp-bar-track">
              <div class="sp-bar-fill" style="width:{Math.min(n.health_score, 100)}%;background:{healthColor(n)}"></div>
            </div>
          </div>
        {/if}
      </div>

      {#if isRuptureActive(n)}
        <div class="sp-rupture">
          <span class="sp-rupture-title">◉ Early rupture signal</span>
          <span class="sp-rupture-body">FusedR {n.fused_r.toFixed(2)} exceeds threshold while HealthScore is still elevated. Risk of imminent degradation.</span>
        </div>
      {/if}

      <div class="sp-hint">Click background to deselect</div>
    </div>
  {/if}

  <!-- edge side panel -->
  {#if selectedEdge}
    {@const e = selectedEdge}
    <div class="side-panel">
      <div class="sp-header">
        <span class="sp-title">{shortId(e.source)} → {shortId(e.target)}</span>
        <button class="sp-close" on:click={() => (selectedEdge = null)}>✕</button>
      </div>
      <div class="sp-id">{e.source} → {e.target}</div>

      <div class="sp-section">
        <div class="sp-row">
          <span class="sp-label">Call Rate</span>
          <span class="sp-val">{fmtRate(e.call_rate)}</span>
        </div>
        <div class="sp-row">
          <span class="sp-label">Error Rate</span>
          <span class="sp-val" style="color:{errorColor(e.error_rate)}">{pct(e.error_rate)}</span>
        </div>
        <div class="sp-row">
          <span class="sp-label">P99 Latency</span>
          <span class="sp-val">{e.p99_latency_ms.toFixed(1)} ms</span>
        </div>
        <div class="sp-bar-row">
          <span class="sp-sub">Error rate</span>
          <div class="sp-bar-track">
            <div class="sp-bar-fill" style="width:{Math.min(e.error_rate * 100, 100)}%;background:{errorColor(e.error_rate)}"></div>
          </div>
        </div>
      </div>

      {#if e.error_rate > 0.15}
        <div class="sp-rupture">
          <span class="sp-rupture-title">⚠ High error rate on this call path</span>
          <span class="sp-rupture-body">Errors from {shortId(e.source)} may propagate contagion into {shortId(e.target)}.</span>
        </div>
      {/if}

      <div class="sp-hint">Click background to deselect</div>
    </div>
  {/if}
</div>

<style>
  .topology-root {
    position: relative;
    width: 100%;
    height: calc(100vh - 110px);
    background: var(--bg);
    overflow: hidden;
    border-radius: 12px;
    border: 1px solid var(--border);
    display: flex;
    flex-direction: column;
  }

  /* explanation bar */
  .info-bar {
    display: flex;
    align-items: baseline;
    gap: 8px;
    padding: 8px 16px;
    background: var(--surface2);
    border-bottom: 1px solid var(--border);
    font-size: 11px;
    flex-shrink: 0;
    flex-wrap: wrap;
    z-index: 5;
  }

  .info-title {
    font-weight: 700;
    font-size: 12px;
    color: var(--text);
    white-space: nowrap;
  }

  .info-sep { color: var(--border); }

  .info-body { color: var(--muted); line-height: 1.5; }

  .leg-ok   { color: #3fb950; font-weight: 600; }
  .leg-warn { color: #e3b341; font-weight: 600; }
  .leg-crit { color: #e05252; font-weight: 600; }

  .cy-container { flex: 1; min-height: 0; }

  .overlay {
    position: absolute;
    inset: 40px 0 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    z-index: 10;
    background: rgba(13, 17, 23, 0.88);
    color: var(--muted);
    font-size: 14px;
    text-align: center;
    padding: 32px;
  }

  .overlay strong { color: var(--text); font-size: 15px; }
  .overlay p { max-width: 440px; line-height: 1.6; font-size: 13px; }
  .overlay.err { color: var(--red); }

  .overlay button {
    background: none;
    border: 1px solid var(--border);
    color: var(--text);
    padding: 5px 14px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
    margin-top: 4px;
  }

  .empty-icon { font-size: 52px; opacity: 0.25; }

  .spin { display: inline-block; animation: spin 1.2s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }

  /* bottom toolbar */
  .toolbar {
    position: absolute;
    bottom: 14px;
    left: 14px;
    display: flex;
    align-items: center;
    gap: 14px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 7px 14px;
    font-size: 11px;
    color: var(--muted);
    z-index: 5;
    flex-wrap: wrap;
    max-width: calc(100% - 300px);
  }

  .count { font-variant-numeric: tabular-nums; white-space: nowrap; }

  .refresh-btn {
    background: none;
    border: none;
    color: var(--muted);
    cursor: pointer;
    font-size: 16px;
    padding: 0 4px;
    line-height: 1;
    transition: color 0.15s;
  }
  .refresh-btn:hover { color: var(--purple); }

  .legend {
    display: flex;
    align-items: center;
    gap: 6px;
    border-left: 1px solid var(--border);
    padding-left: 14px;
    flex-wrap: wrap;
  }

  .dot {
    width: 8px; height: 8px;
    border-radius: 50%;
    display: inline-block;
    flex-shrink: 0;
  }

  /* side panel (shared for node and edge) */
  .side-panel {
    position: absolute;
    top: 56px;
    right: 14px;
    width: 270px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    z-index: 10;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    max-height: calc(100% - 80px);
    overflow-y: auto;
  }

  .sp-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    background: var(--surface2);
    flex-shrink: 0;
  }

  .sp-title { font-weight: 700; font-size: 13px; color: var(--text); }

  .sp-close {
    background: none; border: none; color: var(--muted);
    cursor: pointer; font-size: 14px; padding: 2px 5px; border-radius: 4px;
  }
  .sp-close:hover { color: var(--text); }

  .sp-id {
    padding: 6px 14px;
    font-size: 9px;
    color: var(--muted);
    font-family: 'JetBrains Mono', monospace;
    word-break: break-all;
    border-bottom: 1px solid var(--border);
  }

  .sp-section { padding: 4px 0; }

  .sp-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 7px 14px;
    font-size: 12px;
    border-bottom: 1px solid rgba(30,45,69,0.4);
  }
  .sp-row:last-child { border-bottom: none; }

  .sp-label { color: var(--muted); font-size: 11px; }

  .sp-val { font-variant-numeric: tabular-nums; font-weight: 600; display: flex; align-items: center; gap: 5px; }
  .sp-unit { font-size: 10px; color: var(--muted); font-weight: 400; }
  .sp-sub  { font-size: 10px; color: var(--muted); font-weight: 400; }

  .sp-bar-row {
    padding: 4px 14px 8px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .sp-bar-track {
    height: 4px;
    background: var(--surface3);
    border-radius: 2px;
    overflow: hidden;
  }

  .sp-bar-fill {
    height: 100%;
    border-radius: 2px;
    transition: width 0.3s;
  }

  .state-healthy           { color: #3fb950; font-weight: 600; }
  .state-degraded          { color: #e3b341; font-weight: 600; }
  .state-critical          { color: #e05252; font-weight: 600; }
  .state-pending_telemetry { color: var(--muted); }
  .state-calibrating       { color: var(--purple); font-weight: 600; }

  .sp-rupture {
    margin: 6px 14px 10px;
    padding: 8px 10px;
    background: rgba(224, 82, 82, 0.07);
    border: 1px solid rgba(224, 82, 82, 0.3);
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .sp-rupture-title { font-size: 10px; font-weight: 700; color: var(--red); text-transform: uppercase; letter-spacing: 0.05em; }
  .sp-rupture-body  { font-size: 11px; color: var(--muted); line-height: 1.5; }

  .sp-hint { font-size: 10px; color: var(--muted); text-align: center; padding: 8px; border-top: 1px solid var(--border); }
</style>

<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { fetchTopology } from '../lib/api'
  import type { TopologyNode, TopologyEdge } from '../lib/api'

  // ── simulation node ─────────────────────────────────────────────────────────
  interface SimNode extends TopologyNode {
    x: number
    y: number
    vx: number
    vy: number
    connected: boolean // has at least one edge
  }

  interface Particle {
    id: string     // stable — edgeKey + index
    edgeKey: string
    t: number      // 0–1 progress along edge
    speed: number
  }

  // ── state ────────────────────────────────────────────────────────────────────
  let svgEl: SVGSVGElement
  let W = 800
  let H = 560

  let simNodes: SimNode[] = []
  let edges: TopologyEdge[] = []
  let particles: Particle[] = []

  let selectedId: string | null = null
  let hoveredId: string | null = null
  let loading = true
  let loadErr = ''
  let edgeType = 'none' // "trace" | "inferred" | "none"

  let rafId: number
  let refreshTimer: ReturnType<typeof setInterval>
  let tick = 0
  let settled = false
  let settleCount = 0

  // ── derived helpers ──────────────────────────────────────────────────────────
  $: connectedNodes = simNodes.filter(n => n.connected)
  $: isolatedNodes  = simNodes.filter(n => !n.connected)
  $: selectedNode   = simNodes.find(n => n.id === selectedId) ?? null

  // edges reachable from hovered node (for impact highlight)
  $: impactEdgeKeys = hoveredId
    ? new Set(edges.filter(e => e.source === hoveredId || e.target === hoveredId).map(edgeKey))
    : new Set<string>()
  $: impactNodeIds = hoveredId
    ? new Set(edges.flatMap(e =>
        e.source === hoveredId ? [e.target] :
        e.target === hoveredId ? [e.source] : []
      ))
    : new Set<string>()

  function edgeKey(e: TopologyEdge) { return e.source + '→' + e.target }

  // ── colour helpers ───────────────────────────────────────────────────────────
  function healthColor(score: number, state: string): string {
    if (state === 'pending_telemetry') return '#2a3550'
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
    // inferred — shade by strength
    const g = Math.round(e.strength * 120)
    return `rgb(80,${g + 60},${g + 100})`
  }

  function pulseClass(n: SimNode): string {
    if (n.state === 'pending_telemetry') return ''
    if (n.health_score < 25 || n.contagion > 0.6) return 'pulse-crit'
    if (n.health_score < 50) return 'pulse-warn'
    return ''
  }

  function stateLabel(s: string): string {
    return s.replace(/_/g, ' ')
  }

  function shortId(id: string): string {
    const parts = id.split('/')
    return parts[parts.length - 1] || id
  }

  function pct(v: number): string { return (v * 100).toFixed(1) + '%' }
  function bar(v: number): number  { return Math.min(Math.round(v * 100), 100) }

  // ── force simulation ─────────────────────────────────────────────────────────
  const REPULSION   = 28000   // strong push-apart
  const ATTRACTION  = 0.03
  const DAMPING     = 0.60    // converge quickly
  const CENTER_G    = 0.005
  const MIN_DIST    = 150     // minimum separation between node centres

  function applyForces() {
    if (settled) return
    const cx = W / 2
    const cy = H * 0.45

    for (const a of simNodes) {
      if (!a.connected) continue
      for (const b of simNodes) {
        if (a === b || !b.connected) continue
        const dx = a.x - b.x
        const dy = a.y - b.y
        const dist = Math.max(Math.sqrt(dx * dx + dy * dy), MIN_DIST)
        const force = REPULSION / (dist * dist)
        a.vx += (dx / dist) * force
        a.vy += (dy / dist) * force
      }
      a.vx += (cx - a.x) * CENTER_G
      a.vy += (cy - a.y) * CENTER_G
    }

    for (const e of edges) {
      const src = simNodes.find(n => n.id === e.source)
      const tgt = simNodes.find(n => n.id === e.target)
      if (!src || !tgt) continue
      const dx = tgt.x - src.x
      const dy = tgt.y - src.y
      const dist = Math.max(Math.sqrt(dx * dx + dy * dy), 1)
      const strength = e.edge_type === 'trace' ? ATTRACTION : ATTRACTION * e.strength * 0.5
      const fx = dx * strength
      const fy = dy * strength
      src.vx += fx; src.vy += fy
      tgt.vx -= fx; tgt.vy -= fy
    }

    let totalKE = 0
    for (const n of simNodes) {
      if (!n.connected) continue
      n.vx *= DAMPING
      n.vy *= DAMPING
      n.x = Math.max(60, Math.min(W - 60, n.x + n.vx))
      n.y = Math.max(60, Math.min(H * 0.85, n.y + n.vy))
      totalKE += n.vx * n.vx + n.vy * n.vy
    }

    // detect convergence — freeze simulation once nodes stop moving
    settleCount = totalKE < 0.4 ? settleCount + 1 : 0
    if (settleCount > 15) settled = true
  }

  // ── particle system ──────────────────────────────────────────────────────────
  function stepParticles() {
    for (const p of particles) {
      p.t += p.speed
      if (p.t > 1) p.t = 0
    }
  }

  function particlePos(p: Particle): { x: number; y: number } | null {
    const e = edges.find(e => edgeKey(e) === p.edgeKey)
    if (!e) return null
    const src = simNodes.find(n => n.id === e.source)
    const tgt = simNodes.find(n => n.id === e.target)
    if (!src || !tgt) return null
    return {
      x: src.x + (tgt.x - src.x) * p.t,
      y: src.y + (tgt.y - src.y) * p.t,
    }
  }

  // ── data loading ─────────────────────────────────────────────────────────────
  async function load() {
    loadErr = ''
    try {
      const graph = await fetchTopology()
      const safeNodes = graph?.nodes ?? []
      const safeEdges = graph?.edges ?? []
      mergeGraph(safeNodes, safeEdges)
      edgeType = safeEdges.length === 0 ? 'none'
        : safeEdges.some(e => e.edge_type === 'trace') ? 'trace' : 'inferred'
    } catch (e) {
      loadErr = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }

  function mergeGraph(newNodes: TopologyNode[], newEdges: TopologyEdge[]) {
    const existingById = new Map(simNodes.map(n => [n.id, n]))
    const connectedIds = new Set(newEdges.flatMap(e => [e.source, e.target]))

    // spread new connected nodes evenly in a ring — avoids the initial cluster
    const connectedList = newNodes.filter(n => connectedIds.has(n.id))
    const ringR = Math.min(W, H) * 0.28
    const cx = W / 2
    const cy = H * 0.44
    let ringIdx = 0

    simNodes = newNodes.map(n => {
      const old = existingById.get(n.id)
      if (old) {
        return { ...n, x: old.x, y: old.y, vx: old.vx, vy: old.vy, connected: connectedIds.has(n.id) }
      }
      if (connectedIds.has(n.id)) {
        const angle = (ringIdx++ / Math.max(connectedList.length, 1)) * 2 * Math.PI - Math.PI / 2
        return {
          ...n,
          x: cx + Math.cos(angle) * ringR + (Math.random() - 0.5) * 30,
          y: cy + Math.sin(angle) * ringR + (Math.random() - 0.5) * 30,
          vx: 0, vy: 0, connected: true,
        }
      }
      return { ...n, x: cx + (Math.random() - 0.5) * 300, y: H * 0.45 + (Math.random() - 0.5) * 200, vx: 0, vy: 0, connected: false }
    })

    edges = newEdges

    // restart physics for new data
    settled = false
    settleCount = 0

    // particles — stable IDs so Svelte doesn't churn the DOM
    const existingById2 = new Map(particles.map(p => [p.id, p]))
    particles = newEdges.flatMap(e => {
      const k = edgeKey(e)
      const count = e.edge_type === 'trace' ? 2 : 1
      return Array.from({ length: count }, (_, i) => {
        const pid = `${k}-${i}`
        const old = existingById2.get(pid)
        return { id: pid, edgeKey: k, t: old?.t ?? i / count, speed: 0.0015 + Math.random() * 0.001 }
      })
    })

    // layout isolated nodes in rows below main graph
    const rowW = Math.floor((W - 40) / 140)
    isolatedNodes.forEach((n, i) => {
      n.x = 30 + (i % rowW) * 140 + 50
      n.y = H * 0.91 + Math.floor(i / rowW) * 70 + 20
    })
  }

  // ── animation loop ───────────────────────────────────────────────────────────
  function frame() {
    tick++
    applyForces()
    stepParticles()
    if (!settled) simNodes = simNodes  // only dirty-check nodes until settled
    particles = particles
    rafId = requestAnimationFrame(frame)
  }

  // ── interaction ───────────────────────────────────────────────────────────────
  function selectNode(id: string) {
    selectedId = selectedId === id ? null : id
  }

  function nodeRadius(n: SimNode): number {
    if (n.state === 'pending_telemetry') return 14
    return 18 + Math.round(n.health_score / 100 * 8)  // 18–26 px
  }

  // ── zoom / pan (simple translate) ────────────────────────────────────────────
  let pan = { x: 0, y: 0 }
  let scale = 1
  let dragging = false
  let lastPt = { x: 0, y: 0 }

  function onWheel(e: WheelEvent) {
    e.preventDefault()
    scale = Math.max(0.4, Math.min(2.5, scale * (1 - e.deltaY * 0.001)))
  }
  function onMousedown(e: MouseEvent) { dragging = true; lastPt = { x: e.clientX, y: e.clientY } }
  function onMousemove(e: MouseEvent) {
    if (!dragging) return
    pan.x += e.clientX - lastPt.x
    pan.y += e.clientY - lastPt.y
    lastPt = { x: e.clientX, y: e.clientY }
  }
  function onMouseup() { dragging = false }

  function resetView() { pan = { x: 0, y: 0 }; scale = 1 }

  // ── lifecycle ─────────────────────────────────────────────────────────────────
  onMount(() => {
    const ro = new ResizeObserver(entries => {
      const r = entries[0].contentRect
      W = r.width || 800
      H = r.height || 560
    })
    if (svgEl?.parentElement) ro.observe(svgEl.parentElement)

    load()
    refreshTimer = setInterval(load, 15_000)
    rafId = requestAnimationFrame(frame)

    return () => { ro.disconnect() }
  })

  onDestroy(() => {
    clearInterval(refreshTimer)
    cancelAnimationFrame(rafId)
  })
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
      <button class="btn-sm" on:click={resetView} title="Reset view">⊡</button>
      <button class="btn-sm" on:click={load} title="Refresh">↺</button>
    </div>
  </div>

  <!-- main canvas -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="canvas-wrap"
    on:wheel={onWheel}
    on:mousedown={onMousedown}
    on:mousemove={onMousemove}
    on:mouseup={onMouseup}
    on:mouseleave={onMouseup}
  >
    {#if loading && simNodes.length === 0}
      <div class="overlay"><span class="spin">◌</span> Loading…</div>
    {:else if loadErr}
      <div class="overlay err">{loadErr} <button on:click={load}>Retry</button></div>
    {/if}

    <svg bind:this={svgEl} class="topo-svg" width={W} height={H}>
      <defs>
        <!-- arrowhead markers per colour -->
        <marker id="arrow-trace-ok"   markerWidth="8" markerHeight="8" refX="7" refY="3" orient="auto"><path d="M0,0 L0,6 L8,3 z" fill="#3fb950"/></marker>
        <marker id="arrow-trace-warn" markerWidth="8" markerHeight="8" refX="7" refY="3" orient="auto"><path d="M0,0 L0,6 L8,3 z" fill="#f59e0b"/></marker>
        <marker id="arrow-trace-crit" markerWidth="8" markerHeight="8" refX="7" refY="3" orient="auto"><path d="M0,0 L0,6 L8,3 z" fill="#ef4444"/></marker>
        <marker id="arrow-infer"      markerWidth="8" markerHeight="8" refX="7" refY="3" orient="auto"><path d="M0,0 L0,6 L8,3 z" fill="#6b8cba"/></marker>
        <!-- glow filter for critical nodes -->
        <filter id="glow-crit">
          <feGaussianBlur stdDeviation="4" result="blur"/>
          <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>
        </filter>
        <filter id="glow-warn">
          <feGaussianBlur stdDeviation="2.5" result="blur"/>
          <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>
        </filter>
      </defs>

      <g transform="translate({pan.x},{pan.y}) scale({scale})">

        <!-- ── EDGES ────────────────────────────────────────────────────────── -->
        {#each edges as e (edgeKey(e))}
          {@const src = simNodes.find(n => n.id === e.source)}
          {@const tgt = simNodes.find(n => n.id === e.target)}
          {@const key = edgeKey(e)}
          {@const isHit = impactEdgeKeys.has(key)}
          {@const isTrace = e.edge_type === 'trace'}
          {@const col = edgeColor(e)}
          {@const markerId = isTrace
            ? (e.error_rate > 0.15 ? 'arrow-trace-crit' : e.error_rate > 0.05 ? 'arrow-trace-warn' : 'arrow-trace-ok')
            : 'arrow-infer'}
          {#if src && tgt}
            <!-- edge path -->
            <line
              x1={src.x} y1={src.y} x2={tgt.x} y2={tgt.y}
              stroke={col}
              stroke-width={isTrace ? Math.max(1.5, Math.min(e.strength * 4, 5)) : 1 + e.strength * 2}
              stroke-dasharray={isTrace ? 'none' : '5 4'}
              opacity={hoveredId ? (isHit ? 0.95 : 0.12) : (isTrace ? 0.7 : 0.45)}
              marker-end="url(#{markerId})"
              class="edge-line"
            />
            <!-- hover hit area -->
            <!-- svelte-ignore a11y-no-static-element-interactions -->
            <line
              x1={src.x} y1={src.y} x2={tgt.x} y2={tgt.y}
              stroke="transparent" stroke-width="12"
              style="cursor:pointer"
            />
            <!-- label for trace edges on hover -->
            {#if isHit && isTrace}
              {@const mx = (src.x + tgt.x) / 2}
              {@const my = (src.y + tgt.y) / 2}
              <rect x={mx - 32} y={my - 10} width="64" height="18" rx="4" fill="#0d1117" opacity="0.85"/>
              <text x={mx} y={my + 3} text-anchor="middle" fill={col} font-size="9" font-family="monospace">
                {pct(e.error_rate)} err · {e.p99_latency_ms.toFixed(0)}ms
              </text>
            {/if}
          {/if}
        {/each}

        <!-- ── PARTICLES ──────────────────────────────────────────────────────── -->
        {#each particles as p (p.id)}
          {@const pos = particlePos(p)}
          {@const e = edges.find(e => edgeKey(e) === p.edgeKey)}
          {#if pos && e}
            <circle
              cx={pos.x} cy={pos.y} r={e.edge_type === 'trace' ? 2.5 : 1.8}
              fill={edgeColor(e)}
              opacity={hoveredId ? (impactEdgeKeys.has(p.edgeKey) ? 0.8 : 0.05) : 0.55}
            />
          {/if}
        {/each}

        <!-- ── CONNECTED NODES ────────────────────────────────────────────────── -->
        {#each connectedNodes as n (n.id)}
          {@const r = nodeRadius(n)}
          {@const col = healthColor(n.health_score, n.state)}
          {@const isSelected = n.id === selectedId}
          {@const isHovered  = n.id === hoveredId}
          {@const isImpact   = impactNodeIds.has(n.id)}
          {@const dimmed     = hoveredId !== null && !isHovered && !isImpact}
          <g
            role="button"
            tabindex="0"
            class="node-g {pulseClass(n)}"
            transform="translate({n.x},{n.y})"
            style="cursor:pointer; opacity:{dimmed ? 0.2 : 1}"
            on:click={() => selectNode(n.id)}
            on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') selectNode(n.id) }}
            on:mouseenter={() => { hoveredId = n.id }}
            on:mouseleave={() => { hoveredId = null }}
          >
            <!-- outer ring (pulse target) -->
            <circle r={r + 6} fill="none" stroke={col} stroke-width="1" opacity="0.25" class="ring"/>
            <!-- contagion ring -->
            {#if n.contagion > 0.2}
              <circle r={r + 11} fill="none" stroke="#ef4444"
                stroke-width={n.contagion * 2}
                stroke-dasharray="4 8"
                opacity={n.contagion * 0.6}
                class="contagion-ring"/>
            {/if}
            <!-- main circle -->
            <circle r={r} fill={col}
              filter={n.health_score < 25 ? 'url(#glow-crit)' : n.health_score < 50 ? 'url(#glow-warn)' : 'none'}
              stroke={isSelected ? '#a855f7' : isImpact ? '#f59e0b' : '#0d1117'}
              stroke-width={isSelected ? 3 : isImpact ? 2.5 : 1.5}
              opacity={n.state === 'pending_telemetry' ? 0.4 : 0.92}
            />
            <!-- health score text -->
            {#if n.state !== 'pending_telemetry'}
              <text text-anchor="middle" dominant-baseline="central"
                fill="#ffffff" font-size={r < 20 ? 9 : 11}
                font-weight="700" font-family="monospace" style="pointer-events:none">
                {Math.round(n.health_score)}
              </text>
            {:else}
              <text text-anchor="middle" dominant-baseline="central"
                fill="#6b7280" font-size="8" font-family="monospace" style="pointer-events:none">?</text>
            {/if}
            <!-- label below -->
            <text y={r + 14} text-anchor="middle"
              fill={dimmed ? '#3a4555' : isSelected ? '#c4b5fd' : '#cbd5e1'}
              font-size="10" font-family="'JetBrains Mono', monospace"
              style="pointer-events:none">
              {n.label || shortId(n.id)}
            </text>
            <!-- namespace chip -->
            {#if n.namespace}
              <text y={r + 26} text-anchor="middle"
                fill="#4b5563" font-size="8" font-family="monospace"
                style="pointer-events:none">
                {n.namespace}
              </text>
            {/if}
          </g>
        {/each}

        <!-- ── ISOLATED NODES (grid row at bottom) ───────────────────────────── -->
        {#if isolatedNodes.length > 0}
          <!-- separator label -->
          <text x="12" y={H * 0.9} fill="#374151" font-size="9" font-family="monospace"
            font-weight="700" letter-spacing="1">
            STANDALONE SERVICES (no observed connections)
          </text>
          {#each isolatedNodes as n (n.id)}
            {@const col = healthColor(n.health_score, n.state)}
            {@const isSelected = n.id === selectedId}
            <g role="button" tabindex="0"
              transform="translate({n.x},{n.y})" style="cursor:pointer"
              on:click={() => selectNode(n.id)}
              on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') selectNode(n.id) }}
              on:mouseenter={() => { hoveredId = n.id }}
              on:mouseleave={() => { hoveredId = null }}>
              <!-- rounded rect background -->
              <rect x="-46" y="-18" width="92" height="36" rx="8"
                fill={col} opacity="0.12"
                stroke={isSelected ? '#a855f7' : col}
                stroke-width={isSelected ? 2 : 1} />
              <!-- health dot -->
              <circle cx="-32" cy="0" r="6" fill={col}
                opacity={n.state === 'pending_telemetry' ? 0.4 : 0.9}/>
              <!-- name + score -->
              <text x="-22" y="-4" fill="#cbd5e1" font-size="10" font-family="monospace" font-weight="600">
                {n.label || shortId(n.id)}
              </text>
              <text x="-22" y="8" fill={col} font-size="9" font-family="monospace">
                {n.state === 'pending_telemetry' ? 'no data' : Math.round(n.health_score) + '/100'}
              </text>
            </g>
          {/each}
        {/if}

      </g>
    </svg>
  </div>

  <!-- ── LEGEND ───────────────────────────────────────────────────────────────── -->
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
      <span class="leg-line solid"></span>Trace edge (confirmed)
      <span class="leg-line dashed"></span>Inferred edge (KPI corr.)
      <span class="leg-ring" style="border-color:#ef4444"></span>Contagion spread
    </div>
    <span class="leg-count">{simNodes.length} workloads · {edges.length} edges</span>
  </div>

  <!-- ── DETAIL PANEL ──────────────────────────────────────────────────────────── -->
  {#if selectedNode}
    {@const n = selectedNode}
    {@const col = healthColor(n.health_score, n.state)}
    {@const upstream   = edges.filter(e => e.target === n.id).map(e => shortId(e.source))}
    {@const downstream = edges.filter(e => e.source === n.id).map(e => shortId(e.target))}
    <div class="detail-panel">
      <div class="dp-header" style="border-left:3px solid {col}">
        <div>
          <div class="dp-name">{n.label || shortId(n.id)}</div>
          <div class="dp-meta">{n.namespace}/{n.kind}</div>
        </div>
        <button class="dp-close" on:click={() => selectedId = null}>✕</button>
      </div>

      <!-- state + score -->
      <div class="dp-block">
        <div class="dp-row">
          <span class="dp-label">State</span>
          <span class="dp-val state-chip" style="background:{col}22;color:{col}">{stateLabel(n.state)}</span>
        </div>
        {#if n.state !== 'pending_telemetry'}
          <div class="dp-row">
            <span class="dp-label">Health Score</span>
            <span class="dp-val" style="color:{col}">{Math.round(n.health_score)}<span class="dp-unit">/100</span></span>
          </div>
          <div class="dp-bar-track">
            <div class="dp-bar-fill" style="width:{bar(n.health_score/100)}%;background:{col}"></div>
          </div>
        {/if}
      </div>

      {#if n.state !== 'pending_telemetry'}
        <!-- KPI signals -->
        <div class="dp-section-title">KPI Signals</div>
        <div class="dp-block dp-kpis">
          {#each [
            { k:'Stress',    v: n.stress },
            { k:'Fatigue',   v: n.fatigue },
            { k:'Contagion', v: n.contagion },
            { k:'Mood',      v: n.mood },
            { k:'Velocity',  v: n.velocity },
            { k:'Entropy',   v: n.entropy },
          ] as sig}
            <div class="kpi-row">
              <span class="kpi-label">{sig.k}</span>
              <div class="kpi-track">
                <div class="kpi-fill" style="
                  width:{bar(sig.v)}%;
                  background:{sig.k === 'Contagion' && sig.v > 0.4 ? '#ef4444' :
                               sig.k === 'Stress'    && sig.v > 0.6 ? '#f97316' :
                               sig.k === 'Mood'      && sig.v < 0.3 ? '#6b7280' :
                               '#00e5a0'}"></div>
              </div>
              <span class="kpi-val">{pct(sig.v)}</span>
            </div>
          {/each}
        </div>

        <!-- dependency relationships -->
        {#if upstream.length > 0}
          <div class="dp-section-title">Called by (upstream)</div>
          <div class="dp-block dp-deps">
            {#each upstream as u}
              <span class="dep-chip dep-up">{u}</span>
            {/each}
          </div>
        {/if}

        {#if downstream.length > 0}
          <div class="dp-section-title">Calls (downstream)</div>
          <div class="dp-block dp-deps">
            {#each downstream as d}
              <span class="dep-chip dep-dn">{d}</span>
            {/each}
          </div>
        {/if}

        <!-- impact warning -->
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

  /* header bar */
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

  /* svg canvas */
  .canvas-wrap {
    position: relative;
    flex: 1;
    overflow: hidden;
    cursor: default;
    min-height: 0;
  }
  .topo-svg { display: block; width: 100%; height: 100%; }

  /* overlay */
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
  }
  .overlay.err { color: var(--red); }
  .overlay button {
    background: none;
    border: 1px solid var(--border);
    color: var(--text);
    padding: 4px 12px;
    border-radius: 6px;
    cursor: pointer;
  }

  /* node animations */
  .ring { animation: none; }
  .pulse-crit .ring {
    animation: pulse-crit 2.2s ease-in-out infinite;
  }
  .pulse-warn .ring {
    animation: pulse-warn 3.5s ease-in-out infinite;
  }
  .contagion-ring {
    animation: spin-slow 20s linear infinite;
    transform-origin: center;
  }
  @keyframes pulse-crit {
    0%, 100% { opacity: 0.12; }
    50%       { opacity: 0.45; }
  }
  @keyframes pulse-warn {
    0%, 100% { opacity: 0.08; }
    50%       { opacity: 0.28; }
  }
  @keyframes spin-slow {
    to { transform: rotate(360deg); }
  }

  .edge-line { transition: opacity 0.2s; }
  .node-g    { transition: opacity 0.2s; }

  /* legend bar */
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
  .leg-ring   { display: inline-block; width: 10px; height: 10px; border-radius: 50%; border: 2px solid; }

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

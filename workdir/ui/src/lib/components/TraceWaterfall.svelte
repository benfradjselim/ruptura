<script>
  export let spans = []

  // Build parent-child tree sorted by startTime
  $: tree = buildTree(spans)

  function buildTree(raw) {
    if (!raw || !raw.length) return []
    const nodes = raw.map(s => {
      const parsed = typeof s === 'string' ? tryParse(s) : s
      return { ...parsed, _children: [] }
    })
    const byId = {}
    for (const n of nodes) {
      const id = n.span_id || n.spanId || n.id
      if (id) byId[id] = n
    }
    const roots = []
    for (const n of nodes) {
      const pid = n.parent_id || n.parentId || n.parentSpanId
      const parent = pid ? byId[pid] : null
      if (parent) parent._children.push(n)
      else roots.push(n)
    }
    // Sort roots and children by start time
    const sortByTime = arr => arr.sort((a, b) => (a.start_time_ms || 0) - (b.start_time_ms || 0))
    sortByTime(roots)
    for (const n of nodes) sortByTime(n._children)
    return roots
  }

  function tryParse(s) {
    try { return JSON.parse(s) } catch { return { name: s } }
  }

  // Flatten tree to rows with indent level
  $: rows = flatten(tree, 0)

  function flatten(nodes, depth) {
    const out = []
    for (const n of nodes) {
      out.push({ node: n, depth })
      if (n._children?.length) out.push(...flatten(n._children, depth + 1))
    }
    return out
  }

  // Timeline scale
  $: minTs = Math.min(...spans.map(s => {
    const p = typeof s === 'string' ? tryParse(s) : s
    return p.start_time_ms || 0
  }).filter(Boolean))

  $: maxTs = Math.max(...spans.map(s => {
    const p = typeof s === 'string' ? tryParse(s) : s
    const dur = p.duration_ms || 0
    return (p.start_time_ms || 0) + dur
  }).filter(Boolean))

  $: totalDur = maxTs - minTs || 1

  function barLeft(n)  { return ((( n.start_time_ms || 0) - minTs) / totalDur * 100).toFixed(1) }
  function barWidth(n) { return (( n.duration_ms || 1) / totalDur * 100).toFixed(1) }

  function spanColor(n) {
    const s = (n.status || '').toLowerCase()
    if (s === 'error') return '#ef4444'
    if (s === 'ok')    return '#22c55e'
    return '#38bdf8'
  }

  let selectedSpan = null
</script>

<div class="wf">
  <div class="wf-header">
    <span class="wf-col name">Span</span>
    <span class="wf-col dur">Duration</span>
    <span class="wf-bar-header">Timeline</span>
  </div>

  {#each rows as { node: n, depth }}
    <div
      class="wf-row"
      class:selected={selectedSpan === n}
      on:click={() => selectedSpan = (selectedSpan === n ? null : n)}
    >
      <span class="wf-col name" style="padding-left: {depth * 16 + 8}px">
        <span class="wf-op">{n.name || n.operation_name || n.operationName || '?'}</span>
        <span class="wf-svc">{n.service || ''}</span>
      </span>
      <span class="wf-col dur">{(n.duration_ms || 0).toFixed(1)}ms</span>
      <div class="wf-bar-cell">
        <div class="wf-bar-track">
          <div
            class="wf-bar"
            style="left: {barLeft(n)}%; width: max(1%, {barWidth(n)}%); background: {spanColor(n)}"
          ></div>
        </div>
      </div>
    </div>
    {#if selectedSpan === n}
      <div class="wf-detail">
        <div class="wf-detail-pairs">
          {#each Object.entries(n).filter(([k]) => !k.startsWith('_') && k !== 'attributes') as [k, v]}
            <div class="dp"><span class="dk">{k}</span><span class="dv">{typeof v === 'object' ? JSON.stringify(v) : v}</span></div>
          {/each}
          {#if n.attributes}
            {#each Object.entries(n.attributes) as [k, v]}
              <div class="dp attr"><span class="dk">{k}</span><span class="dv">{v}</span></div>
            {/each}
          {/if}
        </div>
      </div>
    {/if}
  {/each}
</div>

<style>
  .wf { font-size: 0.8rem; }

  .wf-header {
    display: flex; align-items: center;
    padding: 6px 10px; background: #0f172a; border-radius: 6px 6px 0 0;
    border-bottom: 1px solid #334155; color: #475569; font-size: 0.72rem; text-transform: uppercase; letter-spacing: 0.05em;
  }

  .wf-col.name { flex: 0 0 280px; overflow: hidden; }
  .wf-col.dur  { flex: 0 0 80px; text-align: right; padding-right: 12px; }
  .wf-bar-header { flex: 1; color: #475569; font-size: 0.72rem; }

  .wf-row {
    display: flex; align-items: center;
    padding: 5px 10px; border-bottom: 1px solid #0f172a; cursor: pointer;
  }
  .wf-row:hover    { background: #1e293b; }
  .wf-row.selected { background: #0f3460; }

  .wf-op  { color: #e2e8f0; font-weight: 600; }
  .wf-svc { color: #38bdf8; margin-left: 6px; font-size: 0.72rem; }

  .wf-bar-cell { flex: 1; }
  .wf-bar-track { position: relative; height: 14px; background: #1e293b; border-radius: 2px; }
  .wf-bar {
    position: absolute; height: 100%; border-radius: 2px;
    opacity: 0.85; min-width: 2px; transition: opacity 0.1s;
  }
  .wf-row:hover .wf-bar { opacity: 1; }

  .wf-detail {
    background: #0f172a; border-bottom: 1px solid #334155;
    padding: 10px 20px;
  }
  .wf-detail-pairs { display: flex; flex-wrap: wrap; gap: 8px; }
  .dp { display: flex; gap: 6px; font-size: 0.75rem; }
  .dk { color: #38bdf8; font-weight: 600; }
  .dv { color: #94a3b8; }
  .attr .dk { color: #a78bfa; }
</style>

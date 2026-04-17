<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'
  import { kpiHumanise, humanise, KPI_NAMES } from '../util/format.js'

  export let widget = {}
  export let refreshTick = 0

  let current = 0
  let loading = true
  let error   = ''

  async function load() {
    loading = true; error = ''
    try {
      const host = widget.host || ''
      if (widget.kpi) {
        const res = await api.kpis(host)
        const snap = res?.data || {}
        current = snap[widget.kpi]?.value ?? 0
      } else if (widget.metric) {
        const res = await api.metricRange(widget.metric, host, '-5m')
        const pts = res?.data?.points || []
        current = pts.length ? (pts[pts.length - 1].value ?? 0) : 0
      }
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  onMount(load)
  $: if (refreshTick) load()

  $: isKpi        = KPI_NAMES.has(widget.kpi || '')
  $: isHealthScore = widget.kpi === 'health_score'
  $: hv     = isKpi ? kpiHumanise(widget.kpi, current) : humanise(widget.metric || '', current)
  $: maxVal = widget.max ?? (isHealthScore ? 100 : isKpi ? 1 : 100)
  $: pct    = maxVal ? Math.min(100, Math.max(0, (current / maxVal) * 100)) : 0

  // Arc geometry — half-donut from 210° to 330° (210° sweep)
  const CX = 100, CY = 90, R = 70, SW = 14
  const DEG_START = 210, DEG_SWEEP = 300  // 300° sweep gives a nice arc

  function polar(cx, cy, r, deg) {
    const rad = (deg - 90) * Math.PI / 180
    return [cx + r * Math.cos(rad), cy + r * Math.sin(rad)]
  }

  function arc(pct, r) {
    const startDeg = DEG_START
    const endDeg   = DEG_START + DEG_SWEEP * Math.min(1, Math.max(0, pct / 100))
    const [sx, sy] = polar(CX, CY, r, startDeg)
    const [ex, ey] = polar(CX, CY, r, endDeg)
    const large = (DEG_SWEEP * pct / 100) > 180 ? 1 : 0
    return `M ${sx.toFixed(2)} ${sy.toFixed(2)} A ${r} ${r} 0 ${large} 1 ${ex.toFixed(2)} ${ey.toFixed(2)}`
  }

  $: trackPath = arc(100, R)
  $: valuePath = arc(pct,  R)

  // Color: green→yellow→red based on pct
  // For health_score, higher is BETTER; for stress/fatigue, lower is better
  $: invertColor = !isHealthScore && isKpi
  $: colorPct = invertColor ? (100 - pct) : pct
  $: color = colorPct < 40 ? '#22c55e' : colorPct < 70 ? '#f59e0b' : '#ef4444'

  // Tick marks at 0%, 25%, 50%, 75%, 100%
  $: ticks = [0, 25, 50, 75, 100].map(t => {
    const [x, y] = polar(CX, CY, R + SW * 0.9, DEG_START + (DEG_SWEEP * t / 100))
    return { x: x.toFixed(1), y: y.toFixed(1), label: t === 0 ? '0' : t === 100 ? String(Math.round(maxVal)) : '' }
  })
</script>

{#if loading}
  <div class="g-state">loading…</div>
{:else if error}
  <div class="g-state err">{error}</div>
{:else}
  <div class="g-wrap">
    <svg viewBox="0 0 200 160" class="g-svg" aria-label="{widget.title} gauge: {hv.value}{hv.unit}">
      <!-- track -->
      <path d={trackPath} fill="none" stroke="#1e293b" stroke-width={SW} stroke-linecap="round"/>
      <!-- value arc -->
      <path d={valuePath} fill="none" stroke={color} stroke-width={SW} stroke-linecap="round"
            style="filter: drop-shadow(0 0 4px {color}44)"/>
      <!-- tick marks -->
      {#each ticks as tick}
        {@const [ix, iy] = polar(CX, CY, R - SW * 0.7, DEG_START + (DEG_SWEEP * (ticks.indexOf(tick)) / (ticks.length - 1)))}
        {@const [ox, oy] = polar(CX, CY, R - SW * 0.1, DEG_START + (DEG_SWEEP * (ticks.indexOf(tick)) / (ticks.length - 1)))}
        <line x1={ix.toFixed(1)} y1={iy.toFixed(1)} x2={ox.toFixed(1)} y2={oy.toFixed(1)}
              stroke="#334155" stroke-width="1.5"/>
      {/each}
      <!-- value text -->
      <text x={CX} y={CY - 4} text-anchor="middle" fill="#f1f5f9" font-size="22" font-weight="700" font-family="system-ui">{hv.value}</text>
      <text x={CX} y={CY + 14} text-anchor="middle" fill="#64748b" font-size="11" font-family="system-ui">{hv.unit || widget.kpi || widget.metric || ''}</text>
      <!-- min/max labels -->
      <text x="22" y="148" text-anchor="middle" fill="#475569" font-size="9" font-family="system-ui">0</text>
      <text x="178" y="148" text-anchor="middle" fill="#475569" font-size="9" font-family="system-ui">{Math.round(maxVal)}</text>
      <!-- pct ring label -->
      <text x={CX} y={CY + 34} text-anchor="middle" fill={color} font-size="10" font-family="system-ui" font-weight="600">{pct.toFixed(0)}%</text>
    </svg>
  </div>
{/if}

<style>
  .g-wrap  { display:flex; align-items:center; justify-content:center; width:100%; height:100%; padding:4px; }
  .g-svg   { width:100%; max-width:200px; height:auto; }
  .g-state { display:flex; align-items:center; justify-content:center; height:100%; color:#475569; font-size:0.8rem; }
  .err     { color:#ef4444; }
</style>

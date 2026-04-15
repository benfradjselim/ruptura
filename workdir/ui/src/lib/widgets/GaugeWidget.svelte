<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'
  import { humanise, kpiHumanise, KPI_NAMES } from '../util/format.js'
  import { arcPath, gaugeColor } from '../util/svgChart.js'

  export let widget = {}
  export let refreshTick = 0

  let current = 0
  let loading = true
  let error   = ''

  async function load() {
    loading = true; error = ''
    try {
      const host = widget.host || ''
      const key  = widget.kpi || widget.metric || ''
      if (widget.kpi) {
        const res = await api.kpis(host)
        const snap = res?.data || {}
        current = snap[widget.kpi] ?? 0
      } else if (widget.metric) {
        const res = await api.metricRange(widget.metric, host, '-5m')
        const pts = res?.data || []
        current = pts.length ? pts[pts.length - 1].value ?? 0 : 0
      }
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  onMount(load)
  $: if (refreshTick) load()

  $: isKpi  = KPI_NAMES.has(widget.kpi || '')
  $: hv     = isKpi
    ? kpiHumanise(widget.kpi, current)
    : humanise(widget.metric || '', current)
  $: maxVal = widget.max ?? (isKpi ? 1 : 100)
  $: pct    = maxVal ? Math.min(100, (current / maxVal) * 100) : current * 100
  $: color  = gaugeColor(pct)
</script>

{#if loading}
  <div class="gauge-empty">loading…</div>
{:else if error}
  <div class="gauge-empty err">{error}</div>
{:else}
  <div class="gauge-wrap">
    <svg viewBox="0 0 130 95" class="gauge-svg">
      <path d={arcPath(100, 44, 65, 68)} fill="none" stroke="#1e293b" stroke-width="10" stroke-linecap="round"/>
      {#each [25, 50, 75] as tick}
        {@const ang = -Math.PI * 0.75 + (tick / 100) * Math.PI * 1.5}
        <line
          x1={65 + 36 * Math.cos(ang)} y1={68 + 36 * Math.sin(ang)}
          x2={65 + 44 * Math.cos(ang)} y2={68 + 44 * Math.sin(ang)}
          stroke="#334155" stroke-width="1.2"/>
      {/each}
      {#each [pct] as gPct}
        <path d={arcPath(gPct, 44, 65, 68)} fill="none" stroke={color} stroke-width="10" stroke-linecap="round"/>
      {/each}
      <text x="65" y="68" text-anchor="middle" fill="#e2e8f0" font-size="15" font-weight="700">{hv.value}</text>
      <text x="65" y="80" text-anchor="middle" fill="#64748b" font-size="8">{hv.unit}</text>
      <text x="24" y="90" fill="#475569" font-size="7">0</text>
      <text x="106" y="90" fill="#475569" font-size="7" text-anchor="end">{maxVal}</text>
    </svg>
  </div>
{/if}

<style>
  .gauge-wrap { display: flex; justify-content: center; }
  .gauge-svg  { width: 130px; height: 95px; }
  .gauge-empty { text-align: center; padding: 20px; color: #475569; font-size: 0.8rem; }
  .err { color: #ef4444; }
</style>

<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'
  import { humanise, kpiHumanise, KPI_NAMES, SEVERITY_COLOR } from '../util/format.js'
  import { gaugeColor } from '../util/svgChart.js'

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
        current = res?.data?.[widget.kpi] ?? 0
      } else if (widget.metric) {
        const res = await api.metricRange(widget.metric, host, '-5m')
        const pts = res?.data || []
        current = pts.length ? (pts[pts.length - 1].value ?? 0) : 0
      }
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  onMount(load)
  $: if (refreshTick) load()

  $: isKpi = KPI_NAMES.has(widget.kpi || '')
  $: hv    = isKpi ? kpiHumanise(widget.kpi, current) : humanise(widget.metric || '', current)
  $: pct   = isKpi ? current * 100 : current
  $: color = gaugeColor(pct)
</script>

{#if loading}
  <div class="kpi-loading">loading…</div>
{:else if error}
  <div class="kpi-loading err">{error}</div>
{:else}
  <div class="kpi-wrap">
    <div class="kpi-val" style="color: {color}">{hv.value}</div>
    <div class="kpi-unit">{hv.unit}</div>
  </div>
{/if}

<style>
  .kpi-wrap { display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%; padding: 8px; }
  .kpi-val  { font-size: 2.4rem; font-weight: 800; line-height: 1; }
  .kpi-unit { font-size: 0.8rem; color: #64748b; margin-top: 4px; }
  .kpi-loading { text-align: center; padding: 20px; color: #475569; font-size: 0.8rem; }
  .err { color: #ef4444; }
</style>

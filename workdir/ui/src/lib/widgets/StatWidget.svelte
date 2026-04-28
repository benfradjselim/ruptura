<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'
  import { humanise, kpiHumanise, KPI_NAMES } from '../util/format.js'

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
        current = res?.data?.[widget.kpi]?.value ?? 0
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

  $: isKpi = KPI_NAMES.has(widget.kpi || '')
  $: hv    = isKpi ? kpiHumanise(widget.kpi, current) : humanise(widget.metric || '', current)
</script>

{#if loading}
  <div class="stat-loading">loading…</div>
{:else if error}
  <div class="stat-loading err">{error}</div>
{:else}
  <div class="stat-wrap">
    <div class="stat-val">{hv.value}</div>
    <div class="stat-unit">{hv.unit}</div>
  </div>
{/if}

<style>
  .stat-wrap { display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%; padding: 8px; }
  .stat-val  { font-size: 1.9rem; font-weight: 700; color: #e2e8f0; }
  .stat-unit { font-size: 0.75rem; color: #64748b; margin-top: 2px; }
  .stat-loading { text-align: center; padding: 20px; color: #475569; font-size: 0.8rem; }
  .err { color: #ef4444; }
</style>

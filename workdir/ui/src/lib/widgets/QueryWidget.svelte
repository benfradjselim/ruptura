<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'

  export let widget = {}
  export let refreshTick = 0

  let datasourceId = widget.options?.datasource_id || ''
  let query        = widget.options?.query || ''
  let step         = parseInt(widget.options?.step || '15', 10)
  let timeRange    = widget.options?.range || '1h'

  let result  = null
  let loading = false
  let error   = ''

  // Compute unix timestamps from a relative range string like "1h", "6h", "24h"
  function rangeToUnix(range) {
    const units = { m: 60, h: 3600, d: 86400 }
    const m = range.match(/^(\d+)([mhd])$/)
    const seconds = m ? parseInt(m[1]) * (units[m[2]] || 3600) : 3600
    const now = Math.floor(Date.now() / 1000)
    return { start: now - seconds, end: now }
  }

  async function run() {
    if (!datasourceId || !query) return
    loading = true; error = ''; result = null
    try {
      const { start, end } = rangeToUnix(timeRange)
      const res = await api.datasourceProxy(datasourceId, {
        query,
        start: String(start),
        end:   String(end),
        step,
        type: 'query_range',
      })
      // Prometheus response is nested inside res.data
      result = res?.data?.data || res?.data || null
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  onMount(run)
  $: if (refreshTick) run()

  // Flatten Prometheus matrix results into { metric, values[] } rows
  $: rows = (result?.resultType === 'matrix' ? result.result : []).map(r => ({
    metric: Object.entries(r.metric || {}).map(([k,v]) => `${k}="${v}"`).join(', ') || '{}',
    latest: r.values?.length ? parseFloat(r.values[r.values.length - 1][1]).toFixed(4) : '–',
    points: r.values?.length || 0,
  }))
</script>

{#if loading}
  <div class="qw-state">running…</div>
{:else if error}
  <div class="qw-state err">{error}</div>
{:else if !datasourceId || !query}
  <div class="qw-state muted">configure datasource_id and query in widget options</div>
{:else if rows.length === 0}
  <div class="qw-state muted">no results</div>
{:else}
  <div class="qw-table-wrap">
    <table class="qw-table">
      <thead>
        <tr>
          <th>Series</th>
          <th>Latest value</th>
          <th>Points</th>
        </tr>
      </thead>
      <tbody>
        {#each rows as row}
          <tr>
            <td class="series">{row.metric}</td>
            <td class="val">{row.latest}</td>
            <td class="pts">{row.points}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  .qw-state       { display:flex; align-items:center; justify-content:center; height:100%; color:#64748b; font-size:0.8rem; }
  .err            { color:#ef4444; }
  .muted          { color:#475569; }
  .qw-table-wrap  { overflow:auto; height:100%; padding:4px; }
  .qw-table       { width:100%; border-collapse:collapse; font-size:0.75rem; }
  .qw-table th    { text-align:left; padding:4px 8px; border-bottom:1px solid #334155; color:#94a3b8; font-weight:600; }
  .qw-table td    { padding:4px 8px; border-bottom:1px solid #1e293b; color:#cbd5e1; vertical-align:top; }
  .series         { font-family:monospace; word-break:break-all; }
  .val            { text-align:right; font-variant-numeric:tabular-nums; }
  .pts            { text-align:right; color:#475569; }
</style>

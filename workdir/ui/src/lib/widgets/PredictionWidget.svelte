<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'
  import { toFromParam } from '../stores/timeRange.js'
  import { humanise } from '../util/format.js'
  import { buildPath, buildTimeTicks, buildYLabels } from '../util/svgChart.js'

  export let widget = {}
  export let timeRange = { preset: 60, from: null, to: null }
  export let refreshTick = 0

  const W = 420, H = 110, PX = 12, PY = 10

  let actualPts = []
  let forecastPts = []
  let prediction = null
  let loading = true
  let error   = ''

  async function load() {
    loading = true; error = ''
    try {
      const host    = widget.host    || ''
      const metric  = widget.metric  || widget.kpi || ''
      const horizon = widget.horizon || 60
      const fromParam = toFromParam(timeRange)

      // Load actual data
      let raw = []
      if (widget.kpi) {
        const res = await api.kpi(widget.kpi, host)
        raw = (res?.data?.history || res?.data || []).map(p => ({
          t: new Date(p.timestamp || p.t).getTime(),
          v: p.value ?? p.v ?? 0,
        }))
      } else if (metric) {
        const res = await api.metricRange(metric, host, fromParam)
        raw = (res?.data || []).map(p => ({
          t: new Date(p.timestamp || p.t).getTime(),
          v: p.value ?? p.v ?? 0,
        }))
      }
      actualPts = raw.filter(p => !isNaN(p.t) && !isNaN(p.v)).sort((a, b) => a.t - b.t)

      // Load prediction
      const predRes = await api.predict(host, metric, horizon)
      const preds = predRes?.data?.predictions || []
      prediction = preds.find(p => p.target === metric) || preds[0] || null

      // Build simple forecast line: current value → predicted value over horizon
      if (prediction && actualPts.length) {
        const now   = Date.now()
        const end   = now + horizon * 60_000
        forecastPts = [
          { t: actualPts[actualPts.length - 1].t, v: prediction.current },
          { t: end, v: prediction.predicted },
        ]
      }
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  onMount(load)
  $: if (refreshTick, timeRange) { load() }

  $: allPts    = [...actualPts, ...forecastPts]
  $: ticks     = buildTimeTicks(allPts, W, PX)
  $: yLabel    = buildYLabels(allPts)
  $: actualStr  = buildPath(actualPts, W, H, PX, PY)
  $: forecastStr= buildPath(forecastPts, W, H, PX, PY)

  function hv(v) { return humanise(widget.metric || widget.kpi || '', v) }

  const TREND_COLOR = { rising: '#ef4444', falling: '#38bdf8', stable: '#94a3b8' }
</script>

<div class="pred-wrap">
  {#if loading}
    <div class="pred-msg">loading…</div>
  {:else if error}
    <div class="pred-msg err">{error}</div>
  {:else if actualPts.length < 2}
    <div class="pred-msg">Collecting data…</div>
  {:else}
    <svg viewBox="0 0 {W} {H}" class="pred-svg" preserveAspectRatio="none">
      <!-- Actual line -->
      <polyline points={actualStr} fill="none" stroke="#38bdf8" stroke-width="1.8" />
      <!-- Forecast dashed line -->
      {#if forecastPts.length >= 2}
        <polyline points={forecastStr} fill="none"
          stroke={TREND_COLOR[prediction?.trend] ?? '#94a3b8'}
          stroke-width="1.5" stroke-dasharray="6,4" />
      {/if}
      <!-- X ticks -->
      {#each ticks as tk}
        <text x={tk.x} y={H - 1} fill="#475569" font-size="7" text-anchor="middle">{tk.label}</text>
      {/each}
      <!-- Y labels -->
      <text x={PX - 2} y={PY + 6}     fill="#475569" font-size="7" text-anchor="end">{yLabel.max}</text>
      <text x={PX - 2} y={H - PY - 1} fill="#475569" font-size="7" text-anchor="end">{yLabel.min}</text>
    </svg>

    {#if prediction}
      <div class="pred-footer">
        <span class="trend" style="color: {TREND_COLOR[prediction.trend] ?? '#94a3b8'}">
          {prediction.trend === 'rising' ? '↑' : prediction.trend === 'falling' ? '↓' : '→'}
          {prediction.trend}
        </span>
        <span class="pred-val">
          in {prediction.horizon_minutes}m: <strong>{hv(prediction.predicted).value} {hv(prediction.predicted).unit}</strong>
        </span>
      </div>
    {/if}
  {/if}
</div>

<style>
  .pred-wrap { position: relative; width: 100%; }
  .pred-svg  { width: 100%; height: 110px; display: block; }
  .pred-msg  { text-align: center; padding: 20px; color: #475569; font-size: 0.8rem; }
  .err       { color: #ef4444; }
  .pred-footer { display: flex; justify-content: space-between; align-items: center; padding: 4px 8px 0; font-size: 0.78rem; }
  .trend { font-weight: 600; }
  .pred-val { color: #94a3b8; }
  .pred-val strong { color: #e2e8f0; }
</style>

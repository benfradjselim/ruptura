<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'
  import { toFromParam } from '../stores/timeRange.js'
  import { humanise } from '../util/format.js'
  import { buildPath, buildArea, buildTimeTicks, buildForecastTicks, buildYLabels, buildHGridLines, buildVGridLines } from '../util/svgChart.js'

  export let widget = {}
  export let timeRange = { preset: 60, from: null, to: null, future: false }
  export let refreshTick = 0

  const W = 420, H = 120, PX = 38, PY = 10

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
        raw = (res?.data?.points || []).map(p => ({
          t: new Date(p.timestamp || p.t).getTime(),
          v: p.value ?? p.v ?? 0,
        }))
      } else if (metric) {
        const res = await api.metricRange(metric, host, fromParam)
        raw = (res?.data?.points || []).map(p => ({
          t: new Date(p.timestamp || p.t).getTime(),
          v: p.value ?? p.v ?? 0,
        }))
      }
      actualPts = raw.filter(p => !isNaN(p.t) && !isNaN(p.v)).sort((a, b) => a.t - b.t)

      // Load prediction
      const predRes = await api.predict(host, metric, horizon)
      const preds = predRes?.data?.predictions || []
      prediction = preds.find(p => p.target === metric) || preds[0] || null

      // Build forecast line: from now → now+horizon
      if (prediction && actualPts.length) {
        const now = Date.now()
        forecastPts = [
          { t: now, v: prediction.current },
          { t: now + horizon * 60_000, v: prediction.predicted },
        ]
      }
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  onMount(load)
  $: if (refreshTick, timeRange) { load() }

  const metricKey = widget.kpi || widget.metric || ''

  $: allPts      = [...actualPts, ...forecastPts]
  // In future mode: ticks span only the forecast window (labels show future times)
  // but are positioned in the full allPts coordinate space so they align with the chart.
  $: ticks       = timeRange.future && forecastPts.length >= 2
    ? buildForecastTicks(allPts, forecastPts, W, PX)
    : buildTimeTicks(allPts, W, PX)
  $: yLabel      = buildYLabels(allPts, metricKey)
  $: hLines      = buildHGridLines(allPts, H, PY, 4)
  $: vLines      = buildVGridLines(allPts, W, PX)
  $: actualStr   = buildPath(actualPts, W, H, PX, PY)
  $: areaStr     = buildArea(actualPts, W, H, PX, PY)
  $: forecastStr = buildPath(forecastPts, W, H, PX, PY)

  function hv(v) { return humanise(metricKey, v) }

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
      <defs>
        <linearGradient id="predGrad{metricKey}" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%"   stop-color="#38bdf8" stop-opacity="0.25"/>
          <stop offset="100%" stop-color="#38bdf8" stop-opacity="0"/>
        </linearGradient>
      </defs>

      <!-- Horizontal grid lines -->
      {#each hLines as gl}
        <line x1={PX} y1={gl.y} x2={W - 4} y2={gl.y}
              stroke="#1e3a5f" stroke-width="0.6" stroke-dasharray="3,3"/>
      {/each}

      <!-- Vertical grid lines -->
      {#each vLines as vx}
        <line x1={vx} y1={PY} x2={vx} y2={H - PY}
              stroke="#1e3a5f" stroke-width="0.6" stroke-dasharray="3,3"/>
      {/each}

      <!-- Area fill (actual data) -->
      <polygon points={areaStr} fill="url(#predGrad{metricKey})" />

      <!-- Actual line -->
      <polyline points={actualStr} fill="none" stroke="#38bdf8" stroke-width="1.8" />

      <!-- Forecast dashed line -->
      {#if forecastPts.length >= 2}
        <polyline points={forecastStr} fill="none"
          stroke={TREND_COLOR[prediction?.trend] ?? '#94a3b8'}
          stroke-width="1.5" stroke-dasharray="6,4" />
      {/if}

      <!-- Y-axis line -->
      <line x1={PX} y1={PY} x2={PX} y2={H - PY} stroke="#334155" stroke-width="0.5"/>

      <!-- Y labels -->
      <text x={PX - 3} y={PY + 5}    fill="#64748b" font-size="7" text-anchor="end">{yLabel.max}</text>
      <text x={PX - 3} y={H / 2 + 3} fill="#64748b" font-size="7" text-anchor="end">{yLabel.mid}</text>
      <text x={PX - 3} y={H - PY}    fill="#64748b" font-size="7" text-anchor="end">{yLabel.min}</text>

      <!-- X-axis line -->
      <line x1={PX} y1={H - PY} x2={W - 4} y2={H - PY} stroke="#334155" stroke-width="0.5"/>

      <!-- X-axis time ticks -->
      {#each ticks as tk}
        <text x={tk.x} y={H - 1} fill="#475569" font-size="6.5" text-anchor="middle">{tk.label}</text>
      {/each}
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
  .pred-svg  { width: 100%; height: 120px; display: block; }
  .pred-msg  { text-align: center; padding: 20px; color: #475569; font-size: 0.8rem; }
  .err       { color: #ef4444; }
  .pred-footer { display: flex; justify-content: space-between; align-items: center; padding: 4px 8px 0; font-size: 0.78rem; }
  .trend { font-weight: 600; }
  .pred-val { color: #94a3b8; }
  .pred-val strong { color: #e2e8f0; }
</style>

<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../api.js'
  import { toFromParam } from '../stores/timeRange.js'
  import { humanise } from '../util/format.js'
  import { buildPath, buildArea, buildTimeTicks, buildYLabels, valToY } from '../util/svgChart.js'

  export let widget   = {}   // {title, metric, kpi, host, threshold, thresholdSeverity}
  export let timeRange = { preset: 60, from: null, to: null }
  export let refreshTick = 0

  const W = 420, H = 110, PX = 12, PY = 10

  let points = []
  let loading = true
  let error = ''
  let thresholds = []   // [{value, severity, label}]

  async function load() {
    loading = true; error = ''
    try {
      const fromParam = toFromParam(timeRange)
      const host = widget.host || ''
      let raw = []

      if (widget.kpi) {
        // KPI timeseries
        const res = await api.kpi(widget.kpi, host)
        raw = (res?.data?.history || res?.data || []).map(p => ({
          t: new Date(p.timestamp || p.t).getTime(),
          v: p.value ?? p.v ?? 0,
        }))
      } else if (widget.metric) {
        const res = await api.metricRange(widget.metric, host, fromParam)
        raw = (res?.data || []).map(p => ({
          t: new Date(p.timestamp || p.t).getTime(),
          v: p.value ?? p.v ?? 0,
        }))
      }
      points = raw.filter(p => !isNaN(p.t) && !isNaN(p.v))
                  .sort((a, b) => a.t - b.t)
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  // Load alert rules matching this metric for threshold lines
  async function loadThresholds() {
    if (!widget.metric && !widget.kpi) return
    const target = widget.kpi || widget.metric
    try {
      const res = await api.alertRules()
      const rules = res?.data || []
      thresholds = rules
        .filter(r => r.metric === target || r.Metric === target)
        .map(r => ({ value: r.threshold ?? r.Threshold, severity: r.severity ?? r.Severity, label: r.name ?? r.Name }))
    } catch { /* non-fatal */ }
  }

  onMount(() => { load(); loadThresholds() })
  $: if (refreshTick, timeRange) { load() }

  $: ticks  = buildTimeTicks(points, W, PX)
  $: yLabel = buildYLabels(points)
  $: pathStr = buildPath(points, W, H, PX, PY)
  $: areaStr = buildArea(points, W, H, PX, PY)

  const metricKey = widget.kpi || widget.metric || ''
  function hv(v) { return humanise(metricKey, v) }
</script>

<div class="ts-wrap">
  {#if loading}
    <div class="ts-loading">loading…</div>
  {:else if error}
    <div class="ts-error">{error}</div>
  {:else if points.length < 2}
    <div class="ts-empty">No data in range</div>
  {:else}
    <svg viewBox="0 0 {W} {H}" class="ts-svg" preserveAspectRatio="none">
      <!-- Area fill -->
      <polygon points={areaStr} fill="url(#tsGrad)" opacity="0.25" />
      <!-- Threshold lines -->
      {#each thresholds as th}
        {#each [valToY(th.value, points, H, PY)] as yTh}
          <line x1={PX} y1={yTh} x2={W - PX} y2={yTh}
                stroke={th.severity === 'critical' ? '#ef4444' : th.severity === 'warning' ? '#fbbf24' : '#38bdf8'}
                stroke-width="1" stroke-dasharray="4,3" opacity="0.7" />
          <text x={W - PX - 2} y={yTh - 3} fill="#94a3b8" font-size="7" text-anchor="end">{th.label}</text>
        {/each}
      {/each}
      <!-- Sparkline -->
      <polyline points={pathStr} fill="none" stroke="#38bdf8" stroke-width="1.5" />
      <!-- X-axis ticks -->
      {#each ticks as tk}
        <text x={tk.x} y={H - 1} fill="#475569" font-size="7" text-anchor="middle">{tk.label}</text>
      {/each}
      <!-- Y labels -->
      <text x={PX - 2} y={PY + 6}      fill="#475569" font-size="7" text-anchor="end">{yLabel.max}</text>
      <text x={PX - 2} y={H/2 + 3}     fill="#475569" font-size="7" text-anchor="end">{yLabel.mid}</text>
      <text x={PX - 2} y={H - PY - 1}  fill="#475569" font-size="7" text-anchor="end">{yLabel.min}</text>
      <defs>
        <linearGradient id="tsGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%"   stop-color="#38bdf8" />
          <stop offset="100%" stop-color="#38bdf8" stop-opacity="0" />
        </linearGradient>
      </defs>
    </svg>
    <!-- Latest value label -->
    {#if points.length}
      {@const last = points[points.length - 1]}
      {@const h = hv(last.v)}
      <div class="ts-current">{h.value}<span class="ts-unit">{h.unit}</span></div>
    {/if}
  {/if}
</div>

<style>
  .ts-wrap { position: relative; width: 100%; }
  .ts-svg  { width: 100%; height: 110px; display: block; }
  .ts-loading, .ts-error, .ts-empty {
    text-align: center; padding: 20px; color: #475569; font-size: 0.8rem;
  }
  .ts-error { color: #ef4444; }
  .ts-current {
    position: absolute; top: 4px; right: 8px;
    font-size: 1.1rem; font-weight: 700; color: #e2e8f0;
  }
  .ts-unit { font-size: 0.7rem; color: #64748b; margin-left: 2px; }
</style>

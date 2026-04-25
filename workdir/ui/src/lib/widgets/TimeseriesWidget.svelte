<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'
  import { toFromParam } from '../stores/timeRange.js'
  import { humanise } from '../util/format.js'
  import { buildPath, buildArea, buildTimeTicks, buildYLabels, buildHGridLines, buildVGridLines, valToY } from '../util/svgChart.js'

  export let widget    = {}
  export let timeRange = { preset: 60, from: null, to: null }
  export let refreshTick = 0

  const W = 420, H = 120, PX = 38, PY = 10

  let points = []
  let loading = true
  let error = ''
  let thresholds = []

  async function load() {
    loading = true; error = ''
    try {
      const fromParam = toFromParam(timeRange)
      const host = widget.host || ''
      let raw = []
      if (widget.kpi) {
        const res = await api.kpi(widget.kpi, host)
        raw = (res?.data?.points || []).map(p => ({
          t: new Date(p.timestamp || p.t).getTime(),
          v: p.value ?? p.v ?? 0,
        }))
      } else if (widget.metric) {
        const res = await api.metricRange(widget.metric, host, fromParam)
        raw = (res?.data?.points || []).map(p => ({
          t: new Date(p.timestamp || p.t).getTime(),
          v: p.value ?? p.v ?? 0,
        }))
      }
      points = raw.filter(p => !isNaN(p.t) && !isNaN(p.v)).sort((a, b) => a.t - b.t)
    } catch (e) { error = e.message }
    finally { loading = false }
  }

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

  const metricKey = widget.kpi || widget.metric || ''

  $: ticks    = buildTimeTicks(points, W, PX)
  $: yLabel   = buildYLabels(points, metricKey)
  $: hLines   = buildHGridLines(points, H, PY, 4)
  $: vLines   = buildVGridLines(points, W, PX)
  $: pathStr  = buildPath(points, W, H, PX, PY)
  $: areaStr  = buildArea(points, W, H, PX, PY)

  function hv(v) { return humanise(metricKey, v) }
</script>

<div class="ts-wrap">
  {#if loading}
    <div class="ts-msg">loading…</div>
  {:else if error}
    <div class="ts-msg err">{error}</div>
  {:else if points.length < 2}
    <div class="ts-msg muted">No data in range</div>
  {:else}
    <!-- Latest value badge -->
    {@const last = points[points.length - 1]}
    {@const h = hv(last.v)}
    <div class="ts-current">{h.value}<span class="ts-unit"> {h.unit}</span></div>

    <svg viewBox="0 0 {W} {H}" class="ts-svg" preserveAspectRatio="none">
      <defs>
        <linearGradient id="tsGrad{metricKey}" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%"   stop-color="#38bdf8" stop-opacity="0.35"/>
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

      <!-- Area fill -->
      <polygon points={areaStr} fill="url(#tsGrad{metricKey})" />

      <!-- Threshold lines -->
      {#each thresholds as th}
        {#each [valToY(th.value, points, H, PY)] as yTh}
          <line x1={PX} y1={yTh} x2={W - 4} y2={yTh}
                stroke={th.severity === 'critical' ? '#ef4444' : th.severity === 'warning' ? '#fbbf24' : '#38bdf8'}
                stroke-width="1" stroke-dasharray="4,3" opacity="0.8"/>
          <text x={W - 6} y={yTh - 2} fill="#94a3b8" font-size="6.5" text-anchor="end">{th.label}</text>
        {/each}
      {/each}

      <!-- Sparkline -->
      <polyline points={pathStr} fill="none" stroke="#38bdf8" stroke-width="1.6"/>

      <!-- Y-axis line -->
      <line x1={PX} y1={PY} x2={PX} y2={H - PY} stroke="#334155" stroke-width="0.5"/>

      <!-- Y labels -->
      <text x={PX - 3} y={PY + 5}       fill="#64748b" font-size="7" text-anchor="end">{yLabel.max}</text>
      <text x={PX - 3} y={H / 2 + 3}    fill="#64748b" font-size="7" text-anchor="end">{yLabel.mid}</text>
      <text x={PX - 3} y={H - PY}       fill="#64748b" font-size="7" text-anchor="end">{yLabel.min}</text>

      <!-- X-axis line -->
      <line x1={PX} y1={H - PY} x2={W - 4} y2={H - PY} stroke="#334155" stroke-width="0.5"/>

      <!-- X-axis time ticks -->
      {#each ticks as tk}
        <text x={tk.x} y={H - 1} fill="#475569" font-size="6.5" text-anchor="middle">{tk.label}</text>
      {/each}
    </svg>
  {/if}
</div>

<style>
  .ts-wrap { position: relative; width: 100%; }
  .ts-svg  { width: 100%; height: 120px; display: block; }
  .ts-msg  { text-align: center; padding: 20px; font-size: 0.8rem; color: #475569; }
  .ts-msg.err   { color: #ef4444; }
  .ts-msg.muted { color: #334155; }
  .ts-current {
    position: absolute; top: 3px; right: 6px;
    font-size: 1rem; font-weight: 700; color: #e2e8f0; line-height: 1;
  }
  .ts-unit { font-size: 0.65rem; color: #64748b; }
</style>

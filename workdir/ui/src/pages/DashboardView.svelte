<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'

  export let dashboardId
  export let onBack

  let dashboard = null
  let widgetData = {}   // widgetIndex -> { points, current, unit, max }
  let loading = true
  let error = ''
  let host = ''
  let refreshTimer = null

  const PCT_METRICS = new Set(['cpu_percent','memory_percent','disk_percent'])
  const KPI_METRICS = new Set(['stress','fatigue','mood','pressure','humidity','contagion'])
  const NET_METRICS  = new Set(['net_rx_bps','net_tx_bps'])

  function metricUnit(name) {
    if (PCT_METRICS.has(name) || KPI_METRICS.has(name)) return '%'
    if (NET_METRICS.has(name)) return 'bps'
    if (name === 'uptime_seconds') return 's'
    if (name.endsWith('_gb')) return 'GB'
    if (name.endsWith('_mb')) return 'MB'
    return ''
  }

  function scaleValue(name, v) {
    if (KPI_METRICS.has(name)) return v * 100
    return v
  }

  async function detectHost() {
    try { const r = await api.health(); host = r.data?.host || '' } catch {}
  }

  async function loadWidget(idx, widget) {
    if (!widget.metric) return
    try {
      let points = []
      let current = 0

      if (widget.type === 'timeseries') {
        const r = await api.metricRange(widget.metric, host, '-1h')
        const raw = r.data?.points || []
        points = raw.map(p => ({
          t: new Date(p.timestamp).getTime(),
          v: Math.max(0, scaleValue(widget.metric, p.value))
        })).filter(p => !isNaN(p.t) && p.t > 1000000)
        current = points.length ? points[points.length - 1].v : 0
      } else {
        // gauge / kpi / stat — just get latest value from metrics
        const r = await api.metrics(host)
        const raw = r.data?.metrics || {}
        current = Math.max(0, scaleValue(widget.metric, raw[widget.metric] ?? 0))
      }

      const max = PCT_METRICS.has(widget.metric) || KPI_METRICS.has(widget.metric) ? 100 : undefined

      widgetData = {
        ...widgetData,
        [idx]: { points, current, unit: metricUnit(widget.metric), max }
      }
    } catch {}
  }

  async function loadAll() {
    if (!dashboard) return
    await Promise.all(dashboard.widgets.map((w, i) => loadWidget(i, w)))
  }

  onMount(async () => {
    loading = true
    try {
      await detectHost()
      const r = await api.dashboardGet(dashboardId)
      dashboard = r.data
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
    await loadAll()
    if (dashboard?.refresh_seconds > 0) {
      refreshTimer = setInterval(loadAll, dashboard.refresh_seconds * 1000)
    }
  })

  onDestroy(() => clearInterval(refreshTimer))

  // --- SVG line chart helper ---
  function buildPath(points, width, height, pad) {
    if (!points || points.length < 2) return ''
    const xs = points.map(p => p.t)
    const ys = points.map(p => p.v)
    const xMin = Math.min(...xs), xMax = Math.max(...xs)
    const yMin = 0, yMax = Math.max(...ys, 1)
    const sx = (v) => pad + (v - xMin) / (xMax - xMin || 1) * (width - pad * 2)
    const sy = (v) => height - pad - (v - yMin) / (yMax - yMin || 1) * (height - pad * 2)
    return points.map((p, i) => (i === 0 ? 'M' : 'L') + sx(p.t).toFixed(1) + ',' + sy(p.v).toFixed(1)).join(' ')
  }

  function buildArea(points, width, height, pad) {
    const path = buildPath(points, width, height, pad)
    if (!path) return ''
    const xs = points.map(p => p.t)
    const xMin = Math.min(...xs), xMax = Math.max(...xs)
    const sx = (v) => pad + (v - xMin) / (xMax - xMin || 1) * (width - pad * 2)
    const bY = height - pad
    const lastX = sx(xs[xs.length - 1]).toFixed(1)
    const firstX = sx(xs[0]).toFixed(1)
    return path + ` L${lastX},${bY} L${firstX},${bY} Z`
  }

  // --- Arc gauge helper ---
  function arcPath(pct, r, cx, cy) {
    const clamped = Math.min(Math.max(pct, 0), 100)
    const startAngle = -Math.PI * 0.75
    const endAngle = startAngle + (clamped / 100) * Math.PI * 1.5
    const x1 = cx + r * Math.cos(startAngle)
    const y1 = cy + r * Math.sin(startAngle)
    const x2 = cx + r * Math.cos(endAngle)
    const y2 = cy + r * Math.sin(endAngle)
    const large = endAngle - startAngle > Math.PI ? 1 : 0
    return `M ${x1.toFixed(1)} ${y1.toFixed(1)} A ${r} ${r} 0 ${large} 1 ${x2.toFixed(1)} ${y2.toFixed(1)}`
  }

  function gaugeColor(pct) {
    if (pct >= 80) return '#ef4444'
    if (pct >= 60) return '#f97316'
    if (pct >= 40) return '#eab308'
    return '#22c55e'
  }
</script>

<div class="view">
  <div class="view-header">
    <button class="back-btn" on:click={onBack}>← Boards</button>
    {#if dashboard}
      <h1>{dashboard.name}</h1>
      <span class="refresh-badge">↻ {dashboard.refresh_seconds}s</span>
    {/if}
  </div>

  {#if loading}
    <p class="muted">Loading dashboard…</p>
  {:else if error}
    <p class="err">{error}</p>
  {:else if dashboard}
    <div class="widget-grid">
      {#each dashboard.widgets as widget, idx}
        {@const wd = widgetData[idx]}
        <div class="widget card type-{widget.type}">
          <div class="widget-title">{widget.title}</div>

          {#if widget.type === 'timeseries'}
            <div class="chart-wrap">
              {#if wd?.points?.length > 1}
                <svg viewBox="0 0 320 100" class="chart">
                  <!-- Grid lines -->
                  <line x1="20" y1="10" x2="20" y2="85" stroke="#334155" stroke-width="0.5"/>
                  <line x1="20" y1="85" x2="310" y2="85" stroke="#334155" stroke-width="0.5"/>
                  <!-- Area fill -->
                  <path d={buildArea(wd.points, 320, 100, 20)} fill="#38bdf820" />
                  <!-- Line -->
                  <path d={buildPath(wd.points, 320, 100, 20)} fill="none" stroke="#38bdf8" stroke-width="1.5"/>
                  <!-- Latest value label -->
                  <text x="308" y="12" fill="#94a3b8" font-size="9" text-anchor="end">
                    {wd.current.toFixed(1)}{wd.unit}
                  </text>
                </svg>
              {:else}
                <div class="no-data">No data yet</div>
              {/if}
            </div>

          {:else if widget.type === 'gauge'}
            <div class="gauge-wrap">
              <svg viewBox="0 0 120 80" class="gauge-svg">
                <!-- Background arc -->
                <path d={arcPath(100, 40, 60, 60)} fill="none" stroke="#334155" stroke-width="8" stroke-linecap="round"/>
                <!-- Value arc -->
                <path d={arcPath(wd?.max ? (wd.current / wd.max) * 100 : wd?.current ?? 0, 40, 60, 60)}
                      fill="none"
                      stroke={gaugeColor(wd?.max ? (wd.current / wd.max) * 100 : wd?.current ?? 0)}
                      stroke-width="8" stroke-linecap="round"/>
                <!-- Center label -->
                <text x="60" y="62" text-anchor="middle" fill="#e2e8f0" font-size="14" font-weight="bold">
                  {(wd?.current ?? 0).toFixed(1)}{wd?.unit ?? ''}
                </text>
              </svg>
            </div>

          {:else if widget.type === 'kpi'}
            <div class="kpi-val" style="color: {gaugeColor(wd?.current ?? 0)}">
              {(wd?.current ?? 0).toFixed(1)}{wd?.unit ?? ''}
            </div>

          {:else}
            <!-- stat / alerts / unknown -->
            <div class="stat-val">
              {(wd?.current ?? 0).toFixed(2)}{wd?.unit ?? ''}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .view { padding: 0; }
  .view-header { display: flex; align-items: center; gap: 1rem; margin-bottom: 1.25rem; }
  .back-btn { background: #334155; border: none; color: #94a3b8; padding: 0.3rem 0.7rem; border-radius: 5px; cursor: pointer; font-size: 0.8rem; }
  .back-btn:hover { color: #e2e8f0; }
  h1 { margin: 0; font-size: 1.1rem; color: #e2e8f0; flex: 1; }
  .refresh-badge { font-size: 0.7rem; color: #475569; }

  .widget-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 1rem;
  }

  .card { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 0.75rem 1rem; }

  .widget-title { font-size: 0.75rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.04em; margin-bottom: 0.5rem; }

  /* Timeseries */
  .type-timeseries { grid-column: span 2; }
  .chart-wrap { width: 100%; }
  .chart { width: 100%; height: auto; display: block; }
  .no-data { color: #475569; font-size: 0.8rem; padding: 1rem 0; text-align: center; }

  /* Gauge */
  .gauge-wrap { display: flex; justify-content: center; }
  .gauge-svg { width: 120px; height: 80px; }

  /* KPI / Stat */
  .kpi-val { font-size: 2rem; font-weight: 700; text-align: center; padding: 0.5rem 0; }
  .stat-val { font-size: 1.6rem; font-weight: 600; color: #e2e8f0; text-align: center; padding: 0.5rem 0; }

  .muted { color: #64748b; }
  .err { color: #f87171; }
</style>

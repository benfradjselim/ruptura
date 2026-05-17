<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte'
  import { Chart, registerables } from 'chart.js'
  import {
    fetchFleet, fetchRuptures, fetchHistory, fetchActions,
    fetchWorkloadK8s, approveAction, rejectAction,
    fetchExplain, fetchPredictions, fetchLogs, emergencyStop, fetchForecast,
  } from '../lib/api'
  import type {
    FleetHost, RuptureSnapshot, HistoryPoint, Action,
    WorkloadK8sMeta, PredictionEntry, LogEntry, ForecastResult, ModelContribution,
  } from '../lib/api'
  import WorkloadCard from '../components/WorkloadCard.svelte'
  import SuppressionModal from '../components/SuppressionModal.svelte'
  import WeightsModal from '../components/WeightsModal.svelte'

  Chart.register(...registerables)

  // ── state ─────────────────────────────────────────────────────────────────
  let hosts: FleetHost[] = []
  let ruptureMap: Record<string, RuptureSnapshot> = {}
  let selected: FleetHost | null = null
  let history: HistoryPoint[] = []
  let actions: Action[] = []
  let k8sMeta: WorkloadK8sMeta | null = null
  let k8sError = ''
  let events: Array<{ type: string; workload: string; fused_r?: number; ts: string }> = []
  let predictions: PredictionEntry[] = []
  let logs: LogEntry[] = []
  let predHorizon = 120

  let tab = 'signals'
  let loading = true
  let histLoading = false
  let actLoading = false
  let predLoading = false
  let logsLoading = false
  let explainLoading = false
  let explainText = ''
  let explainMode = 'narrative'
  let search = ''
  let error = ''
  let showSuppression = false
  let showWeights = false
  let suppressionDefaultWorkload = ''
  let emergencyStopping = false

  // chart refs
  let tsCanvas: HTMLCanvasElement
  let fcastCanvas: HTMLCanvasElement
  let tsChart: Chart | null = null
  let fcastChart: Chart | null = null

  // forecast state
  let fcastResult: ForecastResult | null = null
  let fcastMetric = 'health_score'
  let fcastHorizon = 1440  // default 24h
  let fcastLoading = false

  // SSE
  let sse: EventSource | null = null

  // ── signal config ─────────────────────────────────────────────────────────
  const SIG_COLORS: Record<string, string> = {
    health_score: '#00e5a0',
    stress:       '#ef4444',
    fatigue:      '#f97316',
    mood:         '#06b6d4',
    pressure:     '#f59e0b',
    humidity:     '#3b82f6',
    contagion:    '#ec4899',
    resilience:   '#00e5a0',
    entropy:      '#a855f7',
    velocity:     '#fb7185',
    throughput:   '#818cf8',
    fused_r:      '#ef4444',
  }

  const TS_SIGS = [
    { key: 'health_score', label: 'Health',     on: true  },
    { key: 'stress',       label: 'Stress',     on: true  },
    { key: 'fatigue',      label: 'Fatigue',    on: false },
    { key: 'mood',         label: 'Mood',       on: false },
    { key: 'pressure',     label: 'Pressure',   on: false },
    { key: 'humidity',     label: 'Humidity',   on: false },
    { key: 'contagion',    label: 'Contagion',  on: false },
    { key: 'resilience',   label: 'Resilience', on: false },
    { key: 'entropy',      label: 'Entropy',    on: false },
    { key: 'velocity',     label: 'Velocity',   on: false },
    { key: 'throughput',   label: 'Throughput', on: false },
    { key: 'fused_r',      label: 'FusedR',     on: false },
  ]

  let tsSigs = TS_SIGS.map(s => ({ ...s }))

  const CHART_DEFAULTS = {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    animation: { duration: 300 },
    plugins: {
      legend: { display: false },
      tooltip: { bodyFont: { family: "'JetBrains Mono', monospace", size: 11 } },
    },
    scales: {
      x: { grid: { display: false }, ticks: { color: '#556080', maxTicksLimit: 8, font: { size: 10 } } },
      y: { grid: { color: 'rgba(30,45,69,0.8)' }, ticks: { color: '#556080', font: { size: 10 } } },
    },
    elements: { point: { radius: 0, hoverRadius: 4 }, line: { tension: 0.4 } },
  }

  // ── chart helpers ─────────────────────────────────────────────────────────
  function destroyCharts() {
    tsChart?.destroy(); tsChart = null
    fcastChart?.destroy(); fcastChart = null
  }

  async function drawTSChart() {
    if (!tsCanvas || !history.length) return
    tsChart?.destroy()
    const labels = history.map(p => {
      const d = new Date(p.ts)
      return `${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}`
    })
    const datasets = tsSigs.filter(s => s.on).map(s => ({
      label: s.label,
      data: history.map(p => {
        const v = snapField(p, s.key)
        return s.key === 'health_score' ? Math.round(v) : +v.toFixed(3)
      }),
      borderColor: SIG_COLORS[s.key] ?? '#888',
      backgroundColor: (SIG_COLORS[s.key] ?? '#888') + '14',
      borderWidth: 1.5,
      fill: true,
    }))
    tsChart = new Chart(tsCanvas, {
      type: 'line',
      data: { labels, datasets },
      options: {
        ...CHART_DEFAULTS,
        plugins: {
          ...CHART_DEFAULTS.plugins,
          legend: { display: datasets.length > 1, labels: { boxWidth: 8, padding: 8, color: '#556080', font: { size: 10 } } },
        },
      },
    })
  }

  async function drawForecastChart() {
    const snap = currentSnap
    if (!snap) return
    const host = snap.workload?.namespace
      ? `${snap.workload.namespace}/${snap.workload.kind}/${snap.workload.name}`
      : snap.host
    if (!host) return

    fcastLoading = true
    fcastChart?.destroy(); fcastChart = null
    try {
      fcastResult = await fetchForecast(host, fcastMetric, fcastHorizon)
    } catch { fcastResult = null }
    fcastLoading = false

    // Wait for Svelte to render the canvas (it's conditionally shown based on fcastResult)
    await tick()

    if (!fcastResult || !fcastCanvas) return
    const pts = fcastResult.points ?? []
    if (pts.length === 0) return

    const now = fcastResult.current
    const labels = ['Now', ...pts.map(p => {
      const m = p.offset_minutes
      if (m >= 1440) return `+${Math.round(m/1440)}d`
      if (m >= 60)   return `+${Math.round(m/60)}h`
      return `+${m}m`
    })]
    const means  = [now,  ...pts.map(p => p.mean)]
    const lo80   = [now,  ...pts.map(p => p.lower_80)]
    const hi80   = [now,  ...pts.map(p => p.upper_80)]

    fcastChart = new Chart(fcastCanvas, {
      type: 'line',
      data: {
        labels,
        datasets: [
          {
            label: fcastMetric,
            data: means,
            borderColor: '#06b6d4',
            backgroundColor: 'rgba(6,182,212,0.07)',
            borderWidth: 2,
            fill: false,
            pointRadius: 3,
            pointHoverRadius: 6,
            tension: 0.3,
          },
          {
            label: '80% CI low',
            data: lo80,
            borderColor: 'rgba(6,182,212,0.2)',
            borderDash: [4, 4],
            borderWidth: 1,
            fill: false,
            pointRadius: 0,
            tension: 0.3,
          },
          {
            label: '80% CI high',
            data: hi80,
            borderColor: 'rgba(6,182,212,0.2)',
            borderDash: [4, 4],
            borderWidth: 1,
            fill: '-1',
            backgroundColor: 'rgba(6,182,212,0.06)',
            pointRadius: 0,
            tension: 0.3,
          },
        ],
      },
      options: {
        ...CHART_DEFAULTS,
        plugins: {
          ...CHART_DEFAULTS.plugins,
          legend: { display: false },
        },
        scales: {
          ...CHART_DEFAULTS.scales,
          y: { ...CHART_DEFAULTS.scales.y, min: 0, max: 100 },
        },
      },
    })
  }

  async function changeFcastMetric(m: string) {
    fcastMetric = m
    await tick()
    drawForecastChart()
  }

  async function changeFcastHorizon(h: number) {
    fcastHorizon = h
    await tick()
    drawForecastChart()
  }

  // ── derived snap (reactive, no extra network call) ────────────────────────
  $: currentSnap = selected ? (ruptureMap[selected.host] ?? null) : null

  // ── data loading ──────────────────────────────────────────────────────────
  let refreshTimer: ReturnType<typeof setInterval>

  async function loadFleet() {
    try {
      const [fleetData, snapshots] = await Promise.all([fetchFleet(), fetchRuptures()])
      const newMap: Record<string, RuptureSnapshot> = {}
      for (const s of (snapshots ?? [])) {
        const key = s.workload?.namespace
          ? `${s.workload.namespace}/${s.workload.kind}/${s.workload.name}`
          : s.host
        newMap[key] = s
      }
      ruptureMap = newMap

      const merged = ((fleetData?.hosts) ?? []).map(h => {
        const snap = newMap[h.host]
        if (!snap) return h
        const cal = snap.status === 'calibrating' || snap.status === 'pending_telemetry'
        return {
          ...h,
          state: cal ? 'calibrating' as const : h.state,
          calibration_progress: snap.calibration_progress ?? 100,
        }
      })
      hosts = merged

      if (selected) {
        const updated = hosts.find(h => h.host === selected!.host)
        if (updated) selected = updated
      }
      error = ''
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }

  async function loadHistory(h: FleetHost) {
    histLoading = true
    history = []
    try { history = await fetchHistory(h.host) } catch { history = [] }
    finally {
      histLoading = false
      await tick()
      drawTSChart()
    }
  }

  async function loadActions() {
    actLoading = true
    try { actions = await fetchActions() } catch { actions = [] }
    finally { actLoading = false }
  }

  async function loadK8s(h: FleetHost) {
    k8sMeta = null; k8sError = ''
    const parts = h.host.split('/')
    if (parts.length !== 3) return
    const [ns, kind, name] = parts
    try { k8sMeta = await fetchWorkloadK8s(ns, kind, name) }
    catch (e) { k8sError = e instanceof Error ? e.message : String(e) }
  }

  async function loadPredictions(h: FleetHost) {
    predLoading = true; predictions = []
    try {
      const r = await fetchPredictions(h.host, predHorizon)
      predictions = r.predictions ?? []
    } catch { predictions = [] }
    finally { predLoading = false }
  }

  function changePredHorizon(h: number) {
    predHorizon = h
    if (selected) loadPredictions(selected)
  }

  async function loadLogs(h: FleetHost) {
    logsLoading = true; logs = []
    const service = h.host.split('/').pop() ?? h.host
    try { logs = await fetchLogs(service) } catch { logs = [] }
    finally { logsLoading = false }
  }

  async function loadExplain() {
    const snap = currentSnap
    if (!snap) return
    const id = snap.rupture_events?.[snap.rupture_events.length - 1]?.id ?? selected?.host ?? ''
    if (!id) return
    explainLoading = true; explainText = ''
    try {
      const r = await fetchExplain(id, explainMode as 'narrative' | 'formula' | 'pipeline')
      explainText = r.narrative ?? r.formula ?? JSON.stringify(r, null, 2)
    } catch (e) { explainText = `Error: ${e instanceof Error ? e.message : e}` }
    finally { explainLoading = false }
  }

  async function select(h: FleetHost) {
    selected = h
    history = []; k8sMeta = null; k8sError = ''; explainText = ''
    predictions = []; logs = []
    tab = 'signals'
    destroyCharts()
    await loadK8s(h)
  }

  function switchTab(t: string) {
    tab = t
    if (!selected) return
    if (t === 'history') loadHistory(selected)
    else if (t === 'forecast') { tick().then(drawForecastChart) }
    else if (t === 'actions') loadActions()
    else if (t === 'predictions') loadPredictions(selected)
    else if (t === 'logs') loadLogs(selected)
  }

  async function handleApprove(id: string) {
    try { await approveAction(id); await loadActions() } catch {}
  }

  async function handleReject(id: string) {
    try { await rejectAction(id); await loadActions() } catch {}
  }

  async function handleEmergencyStop() {
    if (!confirm('Trigger emergency stop? This will halt all pending actions.')) return
    emergencyStopping = true
    try { await emergencyStop() } catch {}
    finally { emergencyStopping = false }
  }

  // ── SSE ───────────────────────────────────────────────────────────────────
  function startSSE() {
    sse = new EventSource('/api/v2/events', { withCredentials: false })
    sse.onmessage = (e: MessageEvent) => {
      try {
        const evt = JSON.parse(e.data)
        if (evt.type === 'heartbeat' || evt.type === 'connected') return
        events = [evt, ...events].slice(0, 200)
      } catch {}
    }
    sse.onerror = () => {
      sse?.close()
      setTimeout(startSSE, 15_000)
    }
  }

  onMount(() => {
    loadFleet()
    refreshTimer = setInterval(loadFleet, 10_000)
    startSSE()
  })

  onDestroy(() => {
    clearInterval(refreshTimer)
    sse?.close()
    destroyCharts()
  })

  // ── helpers ───────────────────────────────────────────────────────────────
  function hsColor(v: number) {
    if (v >= 70) return 'var(--green)'
    if (v >= 40) return 'var(--yellow)'
    return 'var(--red)'
  }

  function fuseColor(v: number) {
    if (v < 1.0) return 'var(--green)'
    if (v < 1.5) return 'var(--yellow)'
    if (v < 2.5) return 'var(--orange)'
    return 'var(--red)'
  }

  function evtTypeColor(t: string) {
    return t === 'rupture' ? 'var(--red)' : t === 'recovery' ? 'var(--green)' : 'var(--muted)'
  }

  function fmtTs(ts: string) { return new Date(ts).toLocaleTimeString() }
  function fmtTsFull(ts: string) { return new Date(ts).toLocaleString() }

  function snapField(p: HistoryPoint, key: string): number {
    return (p as unknown as Record<string, number>)[key] ?? 0
  }

  function snapKpi(key: string) {
    const snap = currentSnap
    if (!snap) return null
    return (snap as unknown as Record<string, { value: number; state: string; trend: string }>)[key] ?? null
  }

  function predTrendColor(t: string): string {
    if (t === 'improving' || t === 'falling') return 'var(--green)'
    if (t === 'degrading' || t === 'rising') return 'var(--red)'
    return 'var(--muted)'
  }

  function logSevColor(s: string): string {
    const sev = s.toLowerCase()
    if (sev === 'error' || sev === 'fatal') return 'var(--red)'
    if (sev === 'warn' || sev === 'warning') return 'var(--yellow)'
    if (sev === 'debug') return 'var(--muted)'
    return 'var(--cyan)'
  }

  // ── reactive counts ───────────────────────────────────────────────────────
  $: healthy  = hosts.filter(h => h.state === 'healthy').length
  $: degraded = hosts.filter(h => h.state === 'degraded').length
  $: critical = hosts.filter(h => h.state === 'critical').length
  $: calib    = hosts.filter(h => h.state === 'calibrating').length
  $: pending  = hosts.filter(h => h.state === 'pending_telemetry').length
  $: liveCount = events.filter(e => e.type === 'rupture').length

  $: filtered = hosts.filter(h =>
    !search || h.host.toLowerCase().includes(search.toLowerCase())
  )

  const KPI_ORDER = ['stress','fatigue','mood','pressure','humidity','contagion','resilience','entropy','velocity','throughput']
  const KPI_COLORS: Record<string, string> = {
    stress:'#ef4444', fatigue:'#f97316', mood:'#06b6d4', pressure:'#f59e0b',
    humidity:'#3b82f6', contagion:'#ec4899', resilience:'#00e5a0',
    entropy:'#a855f7', velocity:'#fb7185', throughput:'#818cf8',
  }
</script>

<div class="fleet">
  <!-- ── summary bar ── -->
  <div class="summary">
    <div class="stat"><span class="slabel">Total</span><span class="sval">{hosts.length}</span></div>
    <div class="stat ok"><span class="slabel">Healthy</span><span class="sval">{healthy}</span></div>
    <div class="stat warn"><span class="slabel">Degraded</span><span class="sval">{degraded}</span></div>
    <div class="stat crit"><span class="slabel">Critical</span><span class="sval">{critical}</span></div>
    {#if calib > 0}
      <div class="stat cal"><span class="slabel">Calibrating</span><span class="sval">{calib}</span></div>
    {/if}
    {#if pending > 0}
      <div class="stat pend"><span class="slabel">Pending</span><span class="sval">{pending}</span></div>
    {/if}
    {#if liveCount > 0}
      <div class="live-badge">◉ {liveCount} live rupture{liveCount > 1 ? 's' : ''}</div>
    {/if}
    <div class="toolbar">
      <input class="search" bind:value={search} placeholder="Filter…" />
      <button class="tool-btn" on:click={() => { suppressionDefaultWorkload = ''; showSuppression = true }}>🔕 Silence</button>
      <button class="tool-btn" on:click={() => showWeights = true}>⚖ Weights</button>
      <button class="tool-btn danger" on:click={handleEmergencyStop} disabled={emergencyStopping}>
        {emergencyStopping ? '…' : '⚠ E-Stop'}
      </button>
      <button class="refresh-btn" on:click={loadFleet} title="Refresh">↻</button>
    </div>
  </div>

  <div class="layout">
    <!-- ── workload list ── -->
    <div class="list">
      {#if loading}
        <div class="empty">Loading…</div>
      {:else if error}
        <div class="error-box">{error}</div>
      {:else if filtered.length === 0}
        <div class="empty">No workloads match "{search}"</div>
      {:else}
        {#each filtered as host (host.host)}
          <WorkloadCard
            {host}
            snap={ruptureMap[host.host] ?? null}
            selected={selected?.host === host.host}
            on:click={() => select(host)}
          />
        {/each}
      {/if}
    </div>

    <!-- ── detail panel ── -->
    {#if selected}
      <div class="detail">
        <div class="detail-header">
          <div>
            <div class="detail-name">{selected.host.split('/').pop()}</div>
            <div class="detail-meta">{selected.host}</div>
          </div>
          <div class="hdr-actions">
            <button class="icon-btn" on:click={() => { suppressionDefaultWorkload = selected?.host ?? ''; showSuppression = true }} title="Silence">🔕</button>
            <button class="icon-btn close" on:click={() => { selected = null; destroyCharts() }}>✕</button>
          </div>
        </div>

        <!-- tabs -->
        <div class="tabs">
          {#each [
            ['signals','Signals'],
            ['history','History'],
            ['forecast','Forecast'],
            ['predictions','Predict'],
            ['events', 'Events' + (liveCount > 0 ? ` (${liveCount})` : '')],
            ['logs','Logs'],
            ['actions','Actions'],
            ['k8s','Kubernetes'],
          ] as [tid, tlabel]}
            <button class="tab" class:active={tab === tid} on:click={() => switchTab(tid)}>
              {tlabel}
            </button>
          {/each}
        </div>

        <!-- ── SIGNALS ── -->
        {#if tab === 'signals'}
          {#if !currentSnap}
            <div class="notice">
              <div class="notice-icon">◌</div>
              <strong>No snapshot data yet</strong>
              <p>Waiting for the engine to process telemetry for this workload.</p>
            </div>
          {:else}
            {@const snap = currentSnap}

            <!-- calibrating notice (non-blocking — show signals anyway) -->
            {#if snap.status === 'calibrating'}
              <div class="calib-banner">
                <span class="spin">⊙</span>
                Calibrating baseline — {snap.calibration_progress ?? 0}% complete
                {#if snap.calibration_eta_minutes > 0}
                  · ETA {snap.calibration_eta_minutes}m
                {/if}
              </div>
            {/if}

            <!-- pattern warning -->
            {#if snap.pattern_match}
              {@const pm = snap.pattern_match}
              <div class="pattern-warn">
                <div class="pw-title">◈ Pattern match — {Math.round(pm.similarity * 100)}% similar to past rupture</div>
                <div class="pw-meta">
                  Matched: {pm.matched_rupture_id}
                  {#if pm.resolution} · Resolution: {pm.resolution}{/if}
                </div>
              </div>
            {/if}

            <!-- headline -->
            <div class="headline">
              <div class="hs-block">
                <span class="hs-num" style="color:{hsColor(snap.health_score?.value ?? 0)}">
                  {Math.round(snap.health_score?.value ?? 0)}
                </span>
                <span class="hs-label">HealthScore</span>
                <span class="hs-trend {snap.health_score?.trend}">{snap.health_score?.trend ?? ''}</span>
              </div>
              <div class="fused-block">
                <span class="fused-num" style="color:{fuseColor(snap.fused_rupture_index)}">
                  {snap.fused_rupture_index.toFixed(3)}
                </span>
                <span class="fused-label">FusedR</span>
                <span class="fused-sub">{snap.fused_rupture_index < 1 ? 'normal' : snap.fused_rupture_index < 1.5 ? 'elevated' : snap.fused_rupture_index < 2.5 ? 'warning' : 'critical'}</span>
              </div>
              {#if snap.business}
                {@const biz = snap.business}
                <div class="biz-block">
                  <div class="biz-row">
                    <span class="slabel">Blast radius</span>
                    <span class="biz-val" style="color:{biz.blast_radius > 3 ? 'var(--red)' : biz.blast_radius > 0 ? 'var(--yellow)' : 'var(--muted)'}">
                      {biz.blast_radius} downstream
                    </span>
                  </div>
                  <div class="biz-row">
                    <span class="slabel">Recovery debt</span>
                    <span class="biz-val" style="color:{biz.recovery_debt > 2 ? 'var(--orange)' : 'var(--muted)'}">
                      {biz.recovery_debt} near-miss{biz.recovery_debt !== 1 ? 'es' : ''}
                    </span>
                  </div>
                  {#if biz.slo_burn_velocity > 0}
                    <div class="biz-row">
                      <span class="slabel">SLO burn</span>
                      <span class="biz-val" style="color:{biz.slo_burn_velocity > 2 ? 'var(--red)' : biz.slo_burn_velocity > 1 ? 'var(--yellow)' : 'var(--green)'}">
                        {biz.slo_burn_velocity.toFixed(2)}× budget
                      </span>
                    </div>
                  {/if}
                </div>
              {/if}
            </div>

            <!-- 10 KPI cells -->
            <div class="kpi-grid">
              {#each KPI_ORDER as key}
                {@const kpi = snapKpi(key)}
                {#if kpi}
                  <div class="kpi-cell" style="--accent:{KPI_COLORS[key] ?? '#888'}">
                    <div class="kpi-bar-track">
                      <div class="kpi-bar-fill" style="width:{Math.min(kpi.value, 100)}%;background:{KPI_COLORS[key] ?? '#888'}"></div>
                    </div>
                    <div class="kpi-row">
                      <span class="kpi-name">{key}</span>
                      <span class="kpi-val" class:ok={kpi.state==='ok'} class:warn={kpi.state==='warning'} class:crit={kpi.state==='critical'}>
                        {kpi.value.toFixed(2)}
                      </span>
                    </div>
                    <div class="kpi-trend {kpi.trend}">{kpi.trend}</div>
                  </div>
                {/if}
              {/each}
            </div>

            <!-- explain -->
            <div class="explain-section">
              <div class="explain-bar">
                <span class="slabel">Explain</span>
                <button class="tab-sm" class:active={explainMode==='narrative'} on:click={() => { explainMode='narrative'; loadExplain() }}>Narrative</button>
                <button class="tab-sm" class:active={explainMode==='formula'}   on:click={() => { explainMode='formula';   loadExplain() }}>Formula</button>
                {#if !explainText && !explainLoading}
                  <button class="tool-btn-sm" on:click={loadExplain}>Load</button>
                {/if}
              </div>
              {#if explainLoading}
                <div class="loading">Loading…</div>
              {:else if explainText}
                <pre class="explain-pre">{explainText}</pre>
              {/if}
            </div>
          {/if}

        <!-- ── HISTORY ── -->
        {:else if tab === 'history'}
          <div class="sig-toggles">
            {#each tsSigs as sig}
              <button
                class="sig-toggle"
                class:active={sig.on}
                style="--sc:{SIG_COLORS[sig.key] ?? '#888'}"
                on:click={() => { sig.on = !sig.on; tsSigs = tsSigs; drawTSChart() }}
              >{sig.label}</button>
            {/each}
          </div>
          {#if histLoading}
            <div class="loading">Loading history…</div>
          {:else if history.length === 0}
            <div class="notice"><p>No history data yet. History accumulates as the engine polls workloads.</p></div>
          {:else}
            <div class="chart-wrap"><canvas bind:this={tsCanvas}></canvas></div>
          {/if}

        <!-- ── FORECAST ── -->
        {:else if tab === 'forecast'}
          <div class="fcast-controls">
            <div class="fcast-ctrl-row">
              <span class="slabel">Signal:</span>
              {#each ['health_score','stress','fatigue','mood','velocity','entropy','contagion'] as sig}
                <button class="hz-btn" class:active={fcastMetric === sig} on:click={() => changeFcastMetric(sig)}>{sig.replace('_',' ')}</button>
              {/each}
            </div>
            <div class="fcast-ctrl-row">
              <span class="slabel">Horizon:</span>
              {#each [{m:60,l:'1h'},{m:120,l:'2h'},{m:360,l:'6h'},{m:720,l:'12h'},{m:1440,l:'24h'},{m:2880,l:'48h'}] as opt}
                <button class="hz-btn" class:active={fcastHorizon === opt.m} on:click={() => changeFcastHorizon(opt.m)}>{opt.l}</button>
              {/each}
            </div>
          </div>
          {#if currentSnap?.health_forecast}
            {@const f = currentSnap.health_forecast}
            <div class="forecast-meta">
              <div class="fcast-stat"><span class="slabel">Trend</span><span class="fcast-val" class:good={f.trend==='improving'} class:bad={f.trend==='degrading'}>{f.trend}</span></div>
              <div class="fcast-stat"><span class="slabel">In 15 min</span><span class="fcast-val" style="color:{hsColor(f.in_15min ?? 0)}">{Math.round(f.in_15min ?? 0)}</span></div>
              <div class="fcast-stat"><span class="slabel">In 30 min</span><span class="fcast-val" style="color:{hsColor(f.in_30min ?? 0)}">{Math.round(f.in_30min ?? 0)}</span></div>
              {#if fcastResult?.confidence !== undefined}
                <div class="fcast-stat"><span class="slabel">Conf.</span><span class="fcast-val">{Math.round((fcastResult.confidence ?? 0) * 100)}%</span></div>
              {/if}
              {#if f.critical_eta_minutes > 0}
                <div class="fcast-stat"><span class="slabel">Critical ETA</span><span class="fcast-val" style="color:var(--red)">{f.critical_eta_minutes}m</span></div>
              {/if}
            </div>
            {#if (f.confidence_window ?? 0) < 60}
              <div class="low-conf-banner">⚠ Low confidence — fewer than 60 observations.</div>
            {/if}
          {/if}
          {#if fcastLoading}
            <div class="loading">Loading forecast…</div>
          {:else if fcastResult?.warming_up}
            <div class="notice"><p>Accumulating observations — forecast available after ~15 minutes of data.</p></div>
          {:else if !fcastResult || (fcastResult.points ?? []).length === 0}
            <div class="notice"><p>No forecast available — workload needs more signal history.</p></div>
          {:else}
            <div class="chart-wrap"><canvas bind:this={fcastCanvas}></canvas></div>
            {#if (fcastResult?.models ?? []).length > 0}
              <div class="model-contrib">
                {#each (fcastResult.models ?? []) as m (m.name)}
                  <div class="model-chip" title="{m.name}: weight {(m.weight*100).toFixed(0)}%, mean {m.mean.toFixed(2)}">
                    <span class="model-name">{m.name}</span>
                    <span class="model-w">{(m.weight * 100).toFixed(0)}%</span>
                  </div>
                {/each}
              </div>
            {/if}
          {/if}

        <!-- ── PREDICTIONS ── -->
        {:else if tab === 'predictions'}
          <div class="pred-horizon-bar">
            <span class="slabel">Horizon:</span>
            {#each [{m:60,l:'1h'},{m:120,l:'2h'},{m:360,l:'6h'},{m:720,l:'12h'},{m:1440,l:'24h'},{m:2880,l:'48h'}] as opt}
              <button class="hz-btn" class:active={predHorizon === opt.m} on:click={() => changePredHorizon(opt.m)}>{opt.l}</button>
            {/each}
          </div>
          {#if predLoading}
            <div class="loading">Loading predictions…</div>
          {:else if predictions.length === 0}
            <div class="notice"><p>No predictions available. The predictor needs calibrated signal history.</p></div>
          {:else}
            <div class="pred-grid">
              {#each predictions as p}
                <div class="pred-card">
                  <div class="pred-name">{p.target}</div>
                  <div class="pred-vals">
                    <div class="pred-now">
                      <span class="slabel">Now</span>
                      <span class="pred-num">{p.current.toFixed(2)}</span>
                    </div>
                    <div class="pred-arrow" style="color:{predTrendColor(p.trend)}">→</div>
                    <div class="pred-then">
                      <span class="slabel">+{p.horizon_minutes}m</span>
                      <span class="pred-num" style="color:{predTrendColor(p.trend)}">{p.predicted.toFixed(2)}</span>
                    </div>
                  </div>
                  <div class="pred-trend" style="color:{predTrendColor(p.trend)}">
                    {p.trend}
                  </div>
                </div>
              {/each}
            </div>
          {/if}

        <!-- ── EVENTS ── -->
        {:else if tab === 'events'}
          <div class="event-list">
            {#if events.length === 0}
              <div class="notice"><p>Waiting for events… (SSE stream active)</p></div>
            {:else}
              {#each events as evt}
                <div class="event-row" style="--etc:{evtTypeColor(evt.type)}">
                  <span class="evt-type">{evt.type}</span>
                  <span class="evt-wl">{(evt.workload ?? '').split('/').pop() ?? evt.workload}</span>
                  {#if evt.fused_r != null}
                    <span class="evt-fused" style="color:{fuseColor(evt.fused_r ?? 0)}">FusedR {(evt.fused_r ?? 0).toFixed(2)}</span>
                  {/if}
                  <span class="evt-ts">{fmtTs(evt.ts)}</span>
                </div>
              {/each}
            {/if}
          </div>

        <!-- ── LOGS ── -->
        {:else if tab === 'logs'}
          {#if logsLoading}
            <div class="loading">Loading logs…</div>
          {:else if logs.length === 0}
            <div class="notice"><p>No logs found for this workload. Logs appear when OTLP log data is ingested.</p></div>
          {:else}
            <div class="log-list">
              {#each logs as log}
                <div class="log-row">
                  <span class="log-sev" style="color:{logSevColor(log.severity)}">{(log.severity || 'INFO').toUpperCase().slice(0,4)}</span>
                  <span class="log-ts">{fmtTs(log.timestamp)}</span>
                  <span class="log-body">{log.body}</span>
                </div>
              {/each}
            </div>
          {/if}

        <!-- ── ACTIONS ── -->
        {:else if tab === 'actions'}
          {#if actLoading}
            <div class="loading">Loading actions…</div>
          {:else if actions.length === 0}
            <div class="notice"><p>No pending actions.</p></div>
          {:else}
            {#each actions as action}
              <div class="action-card" class:tier1={action.tier === 1}>
                <div class="action-hdr">
                  <span class="action-tier">Tier {action.tier}</span>
                  <span class="action-kind">{action.kind}</span>
                  <span class="action-state" class:pending={action.state==='pending'}>{action.state}</span>
                </div>
                <div class="action-desc">{action.description}</div>
                <div class="action-host">{action.host}</div>
                {#if action.state === 'pending'}
                  <div class="action-btns">
                    <button class="btn-approve" on:click={() => handleApprove(action.id)}>✓ Approve</button>
                    <button class="btn-reject"  on:click={() => handleReject(action.id)}>✕ Reject</button>
                  </div>
                {/if}
              </div>
            {/each}
          {/if}

        <!-- ── KUBERNETES ── -->
        {:else if tab === 'k8s'}
          {#if k8sError && !k8sMeta}
            <div class="notice">
              <div class="notice-icon">☸</div>
              <strong>{k8sError.includes('503') ? 'Not running in-cluster' : 'Metadata unavailable'}</strong>
              <p>{k8sError}</p>
            </div>
          {:else if k8sMeta}
            <div class="k8s-section">
              <div class="k8s-field"><span class="slabel">Image</span><span class="k8s-img">{k8sMeta.image || '—'}</span></div>
              <div class="k8s-field"><span class="slabel">Last Deploy</span><span>{k8sMeta.last_deploy ? fmtTsFull(k8sMeta.last_deploy) : '—'}</span></div>
            </div>
            <div class="k8s-section">
              <div class="slabel">Replicas</div>
              <div class="replica-gauge">
                <div class="replica-track">
                  <div class="replica-fill"
                    class:replica-ok={k8sMeta.replicas.ready === k8sMeta.replicas.desired}
                    class:replica-warn={k8sMeta.replicas.ready < k8sMeta.replicas.desired}
                    style="width:{k8sMeta.replicas.desired > 0 ? (k8sMeta.replicas.ready/k8sMeta.replicas.desired)*100 : 0}%"
                  ></div>
                </div>
                <span class="replica-txt">{k8sMeta.replicas.ready} / {k8sMeta.replicas.desired} ready</span>
              </div>
            </div>
            {#if k8sMeta.pods?.length}
              <div class="slabel" style="margin-bottom:6px">Pods ({k8sMeta.pods.length})</div>
              <table class="pod-table">
                <thead><tr><th>Name</th><th>Status</th><th>Restarts</th></tr></thead>
                <tbody>
                  {#each k8sMeta.pods as pod}
                    <tr>
                      <td class="pod-name">{pod.name}</td>
                      <td><span class="pod-status" class:pod-running={pod.status==='Running'} class:pod-failed={pod.status==='Failed'}>{pod.status}</span></td>
                      <td class="pod-restarts" class:warn={pod.restarts > 0}>{pod.restarts}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}
          {:else}
            <div class="loading">Loading Kubernetes metadata…</div>
          {/if}
        {/if}
      </div>
    {/if}
  </div>
</div>

{#if showSuppression}
  <SuppressionModal defaultWorkload={suppressionDefaultWorkload} on:close={() => showSuppression = false} />
{/if}
{#if showWeights}
  <WeightsModal on:close={() => showWeights = false} />
{/if}

<style>
  .fleet { display: flex; flex-direction: column; gap: 16px; }

  .summary {
    display: flex; align-items: center; gap: 20px;
    background: var(--surface); border: 1px solid var(--border);
    border-radius: 12px; padding: 12px 18px; flex-wrap: wrap;
  }

  .stat { display: flex; flex-direction: column; gap: 1px; }
  .slabel { font-size: 9px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.07em; }
  .sval { font-size: 20px; font-weight: 700; font-variant-numeric: tabular-nums; line-height: 1; }
  .stat.ok .sval   { color: var(--green); }
  .stat.warn .sval { color: var(--yellow); }
  .stat.crit .sval { color: var(--red); }
  .stat.cal .sval  { color: var(--purple); }
  .stat.pend .sval { color: var(--muted); }

  .live-badge {
    font-size: 11px; font-weight: 700; color: var(--red);
    background: rgba(239,68,68,0.1); border: 1px solid rgba(239,68,68,0.3);
    padding: 3px 10px; border-radius: 20px;
    animation: live-pulse 2s ease-in-out infinite;
  }
  @keyframes live-pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.6; } }

  .toolbar { display: flex; align-items: center; gap: 8px; margin-left: auto; }

  .search {
    background: var(--surface2); border: 1px solid var(--border);
    color: var(--text); padding: 5px 10px; border-radius: 6px;
    font-size: 12px; outline: none; width: 120px;
  }
  .search:focus { border-color: var(--purple); }

  .tool-btn {
    background: var(--surface2); border: 1px solid var(--border);
    color: var(--muted); padding: 5px 11px; border-radius: 6px;
    cursor: pointer; font-size: 12px; font-weight: 500;
    transition: color 0.15s, border-color 0.15s;
  }
  .tool-btn:hover { color: var(--purple); border-color: var(--purple); }
  .tool-btn.danger:hover { color: var(--red); border-color: var(--red); }
  .tool-btn.danger:disabled { opacity: 0.5; cursor: default; }

  .refresh-btn {
    background: none; border: 1px solid var(--border); color: var(--muted);
    padding: 5px 10px; border-radius: 6px; cursor: pointer; font-size: 16px;
    transition: color 0.15s;
  }
  .refresh-btn:hover { color: var(--purple); }

  .layout { display: grid; grid-template-columns: 280px 1fr; gap: 16px; align-items: start; }
  @media (max-width: 720px) { .layout { grid-template-columns: 1fr; } }

  .list { display: flex; flex-direction: column; gap: 8px; }

  .empty { text-align: center; color: var(--muted); padding: 32px 16px; background: var(--surface); border-radius: 12px; border: 1px solid var(--border); }
  .error-box { text-align: center; color: var(--red); padding: 32px 16px; background: var(--surface); border-radius: 12px; border: 1px solid rgba(239,68,68,0.3); }

  .detail {
    background: var(--surface); border: 1px solid var(--border);
    border-radius: 12px; padding: 18px;
    position: sticky; top: 68px;
    max-height: calc(100vh - 90px); overflow-y: auto;
  }

  .detail-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 14px; }
  .detail-name { font-weight: 700; font-size: 17px; }
  .detail-meta { font-size: 10px; color: var(--muted); margin-top: 2px; font-family: monospace; }

  .hdr-actions { display: flex; align-items: center; gap: 4px; }

  .icon-btn {
    background: none; border: 1px solid var(--border); color: var(--muted);
    cursor: pointer; font-size: 13px; padding: 3px 8px; border-radius: 5px;
    transition: color 0.15s, border-color 0.15s;
  }
  .icon-btn:hover { color: var(--yellow); border-color: var(--yellow); }
  .icon-btn.close:hover { color: var(--text); border-color: var(--border); }

  .tabs {
    display: flex; margin-bottom: 16px;
    border-bottom: 1px solid var(--border);
    overflow-x: auto; scrollbar-width: none;
  }
  .tabs::-webkit-scrollbar { display: none; }
  .tab {
    background: none; border: none; color: var(--muted);
    padding: 7px 10px; cursor: pointer; font-size: 11px; font-weight: 500;
    border-bottom: 2px solid transparent; margin-bottom: -1px;
    transition: color 0.15s, border-color 0.15s; white-space: nowrap;
  }
  .tab:hover { color: var(--text); }
  .tab.active { color: var(--purple); border-bottom-color: var(--purple); }

  /* calibrating banner */
  .calib-banner {
    display: flex; align-items: center; gap: 8px;
    background: rgba(168,85,247,0.06); border: 1px solid rgba(168,85,247,0.2);
    border-radius: 8px; padding: 8px 12px; margin-bottom: 12px;
    font-size: 12px; color: var(--purple);
  }

  /* pattern warning */
  .pattern-warn {
    background: rgba(245,158,11,0.07); border: 1px solid rgba(245,158,11,0.25);
    border-radius: 8px; padding: 10px 12px; margin-bottom: 12px;
  }
  .pw-title { font-size: 12px; font-weight: 700; color: var(--yellow); margin-bottom: 4px; }
  .pw-meta { font-size: 11px; color: var(--muted); font-family: 'JetBrains Mono', monospace; }

  /* headline */
  .headline {
    display: flex; gap: 20px; align-items: flex-start;
    margin-bottom: 16px; padding-bottom: 16px;
    border-bottom: 1px solid var(--border); flex-wrap: wrap;
  }
  .hs-block, .fused-block { display: flex; flex-direction: column; gap: 2px; }
  .hs-num { font-size: 52px; font-weight: 700; font-variant-numeric: tabular-nums; line-height: 1; }
  .hs-label, .fused-label { font-size: 10px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.06em; }
  .hs-trend, .fused-sub { font-size: 10px; color: var(--muted); }
  .hs-trend.rising { color: var(--red); }
  .hs-trend.falling { color: var(--green); }
  .fused-num { font-size: 26px; font-weight: 700; font-variant-numeric: tabular-nums; font-family: 'JetBrains Mono', monospace; line-height: 1; }

  .biz-block { display: flex; flex-direction: column; gap: 6px; margin-left: auto; }
  .biz-row { display: flex; flex-direction: column; gap: 1px; }
  .biz-val { font-size: 12px; font-weight: 600; font-family: 'JetBrains Mono', monospace; }

  /* KPI grid */
  .kpi-grid {
    display: grid; grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
    gap: 6px; margin-bottom: 16px;
  }
  .kpi-cell {
    background: var(--surface2); border: 1px solid var(--border);
    border-radius: 8px; padding: 8px 10px;
    border-left: 2px solid var(--accent);
  }
  .kpi-bar-track { height: 3px; background: var(--surface3); border-radius: 2px; margin-bottom: 6px; overflow: hidden; }
  .kpi-bar-fill { height: 100%; border-radius: 2px; transition: width 0.5s; }
  .kpi-row { display: flex; justify-content: space-between; align-items: baseline; }
  .kpi-name { font-size: 9px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
  .kpi-val { font-size: 12px; font-weight: 700; font-family: 'JetBrains Mono', monospace; color: var(--text); }
  .kpi-val.ok { color: var(--green); }
  .kpi-val.warn { color: var(--yellow); }
  .kpi-val.crit { color: var(--red); }
  .kpi-trend { font-size: 9px; color: var(--muted); margin-top: 2px; }
  .kpi-trend.rising { color: var(--red); }
  .kpi-trend.falling { color: var(--green); }

  /* explain */
  .explain-section { margin-top: 4px; }
  .explain-bar { display: flex; align-items: center; gap: 6px; margin-bottom: 8px; }
  .tab-sm {
    background: none; border: 1px solid var(--border); color: var(--muted);
    padding: 3px 8px; border-radius: 4px; cursor: pointer; font-size: 10px;
    transition: color 0.15s, border-color 0.15s;
  }
  .tab-sm.active { color: var(--purple); border-color: var(--purple); }
  .tool-btn-sm {
    background: var(--surface2); border: 1px solid var(--border); color: var(--muted);
    padding: 3px 8px; border-radius: 4px; cursor: pointer; font-size: 10px;
  }
  .explain-pre {
    background: var(--surface2); border: 1px solid var(--border);
    border-radius: 8px; padding: 12px; font-size: 11px; line-height: 1.6;
    overflow-x: auto; white-space: pre-wrap; color: var(--text);
    font-family: 'JetBrains Mono', monospace; max-height: 280px;
  }

  /* sig toggles */
  .sig-toggles { display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 10px; }
  .sig-toggle {
    background: var(--surface2); border: 1px solid var(--border); color: var(--muted);
    padding: 3px 8px; border-radius: 4px; cursor: pointer; font-size: 10px; font-weight: 500;
    transition: color 0.15s, border-color 0.15s, background 0.15s;
  }
  .sig-toggle.active { background: color-mix(in srgb, var(--sc) 15%, transparent); color: var(--sc); border-color: var(--sc); }

  /* chart */
  .chart-wrap { height: 220px; position: relative; }

  /* forecast */
  .fcast-controls { display: flex; flex-direction: column; gap: 6px; margin-bottom: 12px; }
  .fcast-ctrl-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
  .forecast-meta { display: flex; gap: 16px; flex-wrap: wrap; margin-bottom: 12px; }
  .fcast-stat { display: flex; flex-direction: column; gap: 2px; }
  .fcast-val { font-size: 16px; font-weight: 700; font-variant-numeric: tabular-nums; }
  .fcast-val.good { color: var(--green); }
  .fcast-val.bad  { color: var(--red); }
  .fcast-val.warn { color: var(--yellow); }
  .low-conf-banner {
    font-size: 11px; color: var(--yellow);
    background: rgba(245,158,11,0.07); border: 1px solid rgba(245,158,11,0.2);
    border-radius: 6px; padding: 6px 10px; margin-bottom: 10px;
  }

  /* predictions */
  .pred-horizon-bar { display: flex; align-items: center; gap: 6px; margin-bottom: 12px; flex-wrap: wrap; }
  .hz-btn {
    padding: 3px 10px; border-radius: 12px; border: 1px solid var(--border);
    background: var(--surface2); color: var(--muted); font-size: 11px; cursor: pointer;
    transition: all 0.15s;
  }
  .hz-btn:hover { border-color: var(--accent); color: var(--accent); }
  .hz-btn.active { background: var(--accent); color: #000; border-color: var(--accent); font-weight: 700; }
  .model-contrib { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 8px; }
  .model-chip {
    display: flex; align-items: center; gap: 4px;
    background: var(--surface2); border: 1px solid var(--border);
    border-radius: 20px; padding: 2px 8px; font-size: 10px;
  }
  .model-name { color: var(--muted); font-family: 'JetBrains Mono', monospace; }
  .model-w { color: var(--purple); font-weight: 700; font-family: 'JetBrains Mono', monospace; }
  .pred-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 8px; }
  .pred-card {
    background: var(--surface2); border: 1px solid var(--border);
    border-radius: 8px; padding: 10px 12px;
  }
  .pred-name { font-size: 10px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.06em; margin-bottom: 8px; }
  .pred-vals { display: flex; align-items: center; gap: 6px; }
  .pred-now, .pred-then { display: flex; flex-direction: column; gap: 1px; }
  .pred-num { font-size: 14px; font-weight: 700; font-family: 'JetBrains Mono', monospace; }
  .pred-arrow { font-size: 16px; }
  .pred-trend { font-size: 10px; margin-top: 6px; }

  /* events */
  .event-list { display: flex; flex-direction: column; gap: 3px; }
  .event-row {
    display: flex; align-items: center; gap: 10px;
    padding: 6px 8px; border-radius: 6px;
    background: var(--surface2); border-left: 2px solid var(--etc);
    font-size: 11px;
  }
  .evt-type { font-weight: 700; color: var(--etc); font-size: 10px; text-transform: uppercase; letter-spacing: 0.05em; min-width: 60px; }
  .evt-wl { font-family: 'JetBrains Mono', monospace; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .evt-fused { font-family: 'JetBrains Mono', monospace; font-size: 10px; }
  .evt-ts { font-size: 10px; color: var(--muted); flex-shrink: 0; }

  /* logs */
  .log-list { display: flex; flex-direction: column; gap: 2px; max-height: 360px; overflow-y: auto; }
  .log-row {
    display: flex; align-items: baseline; gap: 8px;
    padding: 4px 6px; border-radius: 4px; font-size: 11px;
    background: var(--surface2);
  }
  .log-sev { font-size: 9px; font-weight: 800; font-family: 'JetBrains Mono', monospace; flex-shrink: 0; min-width: 32px; }
  .log-ts { font-size: 9px; color: var(--muted); flex-shrink: 0; font-family: 'JetBrains Mono', monospace; }
  .log-body { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); }

  /* actions */
  .action-card {
    background: var(--surface2); border: 1px solid var(--border);
    border-radius: 8px; padding: 12px 14px; margin-bottom: 8px;
  }
  .action-card.tier1 { border-color: rgba(168,85,247,0.4); }
  .action-hdr { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
  .action-tier { font-size: 10px; font-weight: 700; color: var(--purple); background: rgba(168,85,247,0.1); padding: 2px 7px; border-radius: 4px; }
  .action-kind { font-size: 12px; font-weight: 600; }
  .action-state { font-size: 10px; color: var(--muted); margin-left: auto; }
  .action-state.pending { color: var(--yellow); }
  .action-desc { font-size: 13px; margin-bottom: 4px; }
  .action-host { font-size: 10px; color: var(--muted); font-family: monospace; margin-bottom: 8px; }
  .action-btns { display: flex; gap: 8px; }
  .btn-approve {
    background: rgba(0,229,160,0.1); border: 1px solid rgba(0,229,160,0.3);
    color: var(--green); padding: 5px 14px; border-radius: 6px; cursor: pointer; font-size: 12px; font-weight: 600;
  }
  .btn-approve:hover { background: rgba(0,229,160,0.18); }
  .btn-reject {
    background: rgba(239,68,68,0.08); border: 1px solid rgba(239,68,68,0.25);
    color: var(--red); padding: 5px 14px; border-radius: 6px; cursor: pointer; font-size: 12px; font-weight: 600;
  }
  .btn-reject:hover { background: rgba(239,68,68,0.14); }

  /* k8s */
  .k8s-section { margin-bottom: 14px; }
  .k8s-field { display: flex; flex-direction: column; gap: 2px; margin-bottom: 8px; }
  .k8s-img { font-size: 11px; font-family: 'JetBrains Mono', monospace; color: var(--cyan); word-break: break-all; }
  .replica-gauge { display: flex; align-items: center; gap: 10px; margin-top: 6px; }
  .replica-track { flex: 1; height: 6px; background: var(--surface3); border-radius: 3px; overflow: hidden; }
  .replica-fill { height: 100%; border-radius: 3px; }
  .replica-fill.replica-ok { background: var(--green); }
  .replica-fill.replica-warn { background: var(--yellow); }
  .replica-txt { font-size: 11px; font-family: 'JetBrains Mono', monospace; color: var(--muted); flex-shrink: 0; }
  .pod-table { width: 100%; border-collapse: collapse; font-size: 11px; margin-top: 8px; }
  .pod-table th { color: var(--muted); font-size: 9px; text-transform: uppercase; letter-spacing: 0.06em; padding: 0 0 6px; text-align: left; border-bottom: 1px solid var(--border); }
  .pod-table td { padding: 5px 0; border-bottom: 1px solid rgba(30,45,69,0.5); }
  .pod-name { font-family: 'JetBrains Mono', monospace; font-size: 10px; }
  .pod-status { font-size: 10px; font-weight: 600; }
  .pod-status.pod-running { color: var(--green); }
  .pod-status.pod-failed  { color: var(--red); }
  .pod-restarts.warn { color: var(--yellow); }

  /* notices */
  .notice {
    text-align: center; padding: 32px 16px; color: var(--muted);
    display: flex; flex-direction: column; align-items: center; gap: 8px; line-height: 1.6;
  }
  .notice-icon { font-size: 24px; opacity: 0.4; }
  .loading { text-align: center; color: var(--muted); padding: 24px; font-size: 13px; }

  /* spin */
  @keyframes spin { to { transform: rotate(360deg); } }
  .spin { display: inline-block; animation: spin 2s linear infinite; }
</style>

<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte'
  import { Chart, registerables } from 'chart.js'
  import {
    fetchFleet, fetchRupture, fetchHistory, fetchActions,
    fetchWorkloadK8s, approveAction, rejectAction, fetchExplain,
  } from '../lib/api'
  import type { FleetHost, RuptureSnapshot, HistoryPoint, Action, WorkloadK8sMeta } from '../lib/api'
  import WorkloadCard from '../components/WorkloadCard.svelte'
  import SuppressionModal from '../components/SuppressionModal.svelte'
  import WeightsModal from '../components/WeightsModal.svelte'

  Chart.register(...registerables)

  // ── state ─────────────────────────────────────────────────────────────────
  let hosts: FleetHost[] = []
  let selected: FleetHost | null = null
  let snap: RuptureSnapshot | null = null
  let history: HistoryPoint[] = []
  let actions: Action[] = []
  let k8sMeta: WorkloadK8sMeta | null = null
  let k8sError = ''
  let events: Array<{ type: string; workload: string; fused_r?: number; ts: string }> = []

  let tab: 'signals' | 'history' | 'forecast' | 'events' | 'actions' | 'k8s' = 'signals'
  let loading = true
  let snapLoading = false
  let histLoading = false
  let actLoading = false
  let explainLoading = false
  let explainText = ''
  let explainMode: 'narrative' | 'formula' = 'narrative'
  let search = ''
  let error = ''
  let showSuppression = false
  let showWeights = false
  let suppressionDefaultWorkload = ''

  // chart refs
  let tsCanvas: HTMLCanvasElement
  let fcastCanvas: HTMLCanvasElement
  let tsChart: Chart | null = null
  let fcastChart: Chart | null = null

  // SSE
  let sse: EventSource | null = null

  const SIG_COLORS: Record<string, string> = {
    health_score: '#00e5a0',
    stress:      '#ef4444',
    fatigue:     '#f97316',
    mood:        '#06b6d4',
    pressure:    '#f59e0b',
    humidity:    '#3b82f6',
    contagion:   '#ec4899',
    resilience:  '#00e5a0',
    entropy:     '#a855f7',
    velocity:    '#fb7185',
    fused_r:     '#ef4444',
  }

  const TS_SIGS = [
    { key: 'health_score', label: 'HealthScore', on: true },
    { key: 'stress',       label: 'Stress',      on: true },
    { key: 'fatigue',      label: 'Fatigue',     on: false },
    { key: 'mood',         label: 'Mood',        on: false },
    { key: 'pressure',     label: 'Pressure',    on: false },
    { key: 'humidity',     label: 'Humidity',    on: false },
    { key: 'contagion',    label: 'Contagion',   on: false },
    { key: 'resilience',   label: 'Resilience',  on: false },
    { key: 'entropy',      label: 'Entropy',     on: false },
    { key: 'velocity',     label: 'Velocity',    on: false },
    { key: 'fused_r',      label: 'FusedR',      on: false },
  ]

  let tsSigs = TS_SIGS.map(s => ({ ...s }))

  // ── chart helpers ─────────────────────────────────────────────────────────
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
      x: {
        grid: { display: false },
        ticks: { color: '#556080', maxTicksLimit: 8, font: { size: 10 } },
      },
      y: {
        grid: { color: 'rgba(30,45,69,0.8)' },
        ticks: { color: '#556080', font: { size: 10 } },
      },
    },
    elements: { point: { radius: 0, hoverRadius: 4 }, line: { tension: 0.4 } },
  }

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
    const datasets = tsSigs
      .filter(s => s.on)
      .map(s => ({
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
      options: { ...CHART_DEFAULTS, plugins: { ...CHART_DEFAULTS.plugins, legend: { display: datasets.length > 1, labels: { boxWidth: 8, padding: 8, color: '#556080', font: { size: 10 } } } } },
    })
  }

  async function drawForecastChart() {
    if (!fcastCanvas || !snap?.health_forecast) return
    fcastChart?.destroy()
    const f = snap.health_forecast
    const now = Math.round(snap.health_score?.value ?? 0)
    fcastChart = new Chart(fcastCanvas, {
      type: 'line',
      data: {
        labels: ['Now', '+15 min', '+30 min'],
        datasets: [{
          data: [now, Math.round(f.in_15min ?? 0), Math.round(f.in_30min ?? 0)],
          borderColor: '#06b6d4',
          backgroundColor: 'rgba(6,182,212,0.07)',
          borderWidth: 2,
          fill: true,
          pointBackgroundColor: '#06b6d4',
          pointRadius: 5,
          pointHoverRadius: 7,
        }],
      },
      options: {
        ...CHART_DEFAULTS,
        scales: {
          ...CHART_DEFAULTS.scales,
          y: { ...CHART_DEFAULTS.scales.y, min: 0, max: 100 },
        },
      },
    })
  }

  // ── data loading ──────────────────────────────────────────────────────────
  let refreshTimer: ReturnType<typeof setInterval>

  async function loadFleet() {
    try {
      const data = await fetchFleet()
      hosts = data.hosts ?? []
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

  async function loadSnap(h: FleetHost) {
    if (h.state === 'pending_telemetry' || h.state === 'calibrating') { snap = null; return }
    snapLoading = true
    try {
      snap = await fetchRupture(h.host)
    } catch { snap = null }
    finally { snapLoading = false }
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

  async function loadExplain() {
    if (!snap) return
    const id = snap.rupture_events?.[snap.rupture_events.length - 1]?.id ?? selected?.host ?? ''
    if (!id) return
    explainLoading = true; explainText = ''
    try {
      const r = await fetchExplain(id, explainMode)
      explainText = r.narrative ?? r.formula ?? JSON.stringify(r, null, 2)
    } catch (e) { explainText = `Error: ${e instanceof Error ? e.message : e}` }
    finally { explainLoading = false }
  }

  async function select(h: FleetHost) {
    selected = h
    snap = null; history = []; k8sMeta = null; k8sError = ''; explainText = ''
    tab = 'signals'
    destroyCharts()
    await Promise.all([loadSnap(h), loadK8s(h)])
  }

  async function switchTab(t: string) {
    tab = t as typeof tab
    if (!selected) return
    if (t === 'history') {
      await loadHistory(selected)
    } else if (t === 'forecast') {
      await tick()
      drawForecastChart()
    } else if (t === 'actions') {
      await loadActions()
    }
  }

  async function handleApprove(id: string) {
    try { await approveAction(id); await loadActions() } catch { /* show inline? */ }
  }

  async function handleReject(id: string) {
    try { await rejectAction(id); await loadActions() } catch {}
  }

  // ── SSE ───────────────────────────────────────────────────────────────────
  function startSSE() {
    sse = new EventSource('/api/v2/events')
    sse.onmessage = (e: MessageEvent) => {
      try {
        const evt = JSON.parse(e.data)
        if (evt.type === 'heartbeat') return
        events = [evt, ...events].slice(0, 120)
      } catch {}
    }
    sse.onerror = () => {
      sse?.close()
      setTimeout(startSSE, 10_000)
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

  // ── reactive ──────────────────────────────────────────────────────────────
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

  $: healthy   = hosts.filter(h => h.state === 'healthy').length
  $: degraded  = hosts.filter(h => h.state === 'degraded').length
  $: critical  = hosts.filter(h => h.state === 'critical').length
  $: calib     = hosts.filter(h => h.state === 'calibrating').length
  $: pending   = hosts.filter(h => h.state === 'pending_telemetry').length
  $: liveCount = events.filter(e => e.type === 'rupture').length

  $: filtered = hosts.filter(h =>
    !search || h.host.toLowerCase().includes(search.toLowerCase())
  )

  const KPI_ORDER = ['stress','fatigue','mood','pressure','humidity','contagion','resilience','entropy','velocity']

  function snapKpi(key: string): { value: number; state: string; trend: string } | null {
    if (!snap) return null
    return (snap as Record<string, unknown>)[key] as { value: number; state: string; trend: string } | null
  }

  function snapField(p: HistoryPoint, key: string): number {
    return (p as unknown as Record<string, number>)[key] ?? 0
  }
  const KPI_COLORS: Record<string, string> = {
    stress:'#ef4444', fatigue:'#f97316', mood:'#06b6d4', pressure:'#f59e0b',
    humidity:'#3b82f6', contagion:'#ec4899', resilience:'#00e5a0',
    entropy:'#a855f7', velocity:'#fb7185',
  }

  function evtTypeColor(t: string) {
    return t === 'rupture' ? 'var(--red)' : t === 'recovery' ? 'var(--green)' : 'var(--muted)'
  }

  function fmtTs(ts: string) {
    return new Date(ts).toLocaleTimeString()
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
      <button class="refresh-btn" on:click={loadFleet} title="Refresh">↻</button>
    </div>
  </div>

  <div class="layout">
    <!-- ── workload list ── -->
    <div class="list">
      {#if loading}
        <div class="empty">Loading…</div>
      {:else if error}
        <div class="error">{error}</div>
      {:else if filtered.length === 0}
        <div class="empty">No workloads match "{search}"</div>
      {:else}
        {#each filtered as host (host.host)}
          <WorkloadCard
            {host}
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
            ['events','Events' + (liveCount > 0 ? ` (${liveCount})` : '')],
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
          {#if selected.state === 'pending_telemetry'}
            <div class="notice">
              <div class="notice-icon">◌</div>
              <strong>Awaiting telemetry</strong>
              <p>Configure this workload to push OTLP metrics to <code>otlp-service:4317</code>.</p>
            </div>
          {:else if selected.state === 'calibrating'}
            <div class="notice calib-notice">
              <div class="notice-icon spin">⊙</div>
              <strong>Calibrating baseline</strong>
              <p>Ruptura received metrics and is building the adaptive baseline. Detection will activate once enough signal history is collected.</p>
            </div>
          {:else if snapLoading}
            <div class="loading">Loading signals…</div>
          {:else if snap}
            <!-- health + fusedR headline -->
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
            </div>

            <!-- 9 KPI cells -->
            <div class="kpi-grid">
              {#each KPI_ORDER as key}
                {@const kpi = snapKpi(key)}
                {#if kpi}
                  <div class="kpi-cell" style="--accent:{KPI_COLORS[key]}">
                    <div class="kpi-bar-track">
                      <div class="kpi-bar-fill" style="width:{Math.min(kpi.value, 100)}%;background:{KPI_COLORS[key]}"></div>
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

            <!-- explain section -->
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
                <div class="loading">Loading explanation…</div>
              {:else if explainText}
                <pre class="explain-pre">{explainText}</pre>
              {/if}
            </div>
          {:else}
            <div class="loading">No signal data available.</div>
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
            <div class="notice"><p>No history data yet for this workload.</p></div>
          {:else}
            <div class="chart-wrap"><canvas bind:this={tsCanvas}></canvas></div>
          {/if}

        <!-- ── FORECAST ── -->
        {:else if tab === 'forecast'}
          {#if !snap?.health_forecast}
            <div class="notice"><p>No forecast available yet — workload needs more signal history.</p></div>
          {:else}
            {@const f = snap.health_forecast}
            <div class="forecast-meta">
              <div class="fcast-stat">
                <span class="slabel">Trend</span>
                <span class="fcast-val" class:good={f.trend==='improving'} class:bad={f.trend==='degrading'}>{f.trend}</span>
              </div>
              <div class="fcast-stat">
                <span class="slabel">In 15 min</span>
                <span class="fcast-val" style="color:{hsColor(f.in_15min ?? 0)}">{Math.round(f.in_15min ?? 0)}</span>
              </div>
              <div class="fcast-stat">
                <span class="slabel">In 30 min</span>
                <span class="fcast-val" style="color:{hsColor(f.in_30min ?? 0)}">{Math.round(f.in_30min ?? 0)}</span>
              </div>
              <div class="fcast-stat">
                <span class="slabel">Confidence</span>
                <span class="fcast-val" class:warn={(f.confidence_window ?? 0) < 60}>{f.confidence_window ?? 0} obs.</span>
              </div>
              {#if f.critical_eta_minutes > 0}
                <div class="fcast-stat">
                  <span class="slabel">Critical ETA</span>
                  <span class="fcast-val" style="color:var(--red)">{f.critical_eta_minutes}m</span>
                </div>
              {/if}
            </div>
            {#if (f.confidence_window ?? 0) < 60}
              <div class="low-conf-banner">⚠ Low confidence — fewer than 60 observations. ETAs beyond 30 min are suppressed.</div>
            {/if}
            <div class="chart-wrap"><canvas bind:this={fcastCanvas}></canvas></div>
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
                  <span class="evt-wl">{evt.workload?.split('/').pop() ?? evt.workload}</span>
                  {#if evt.fused_r != null}
                    <span class="evt-fused" style="color:{fuseColor(evt.fused_r ?? 0)}">FusedR {(evt.fused_r ?? 0).toFixed(2)}</span>
                  {/if}
                  <span class="evt-ts">{fmtTs(evt.ts)}</span>
                </div>
              {/each}
            {/if}
          </div>

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
              <div class="k8s-field"><span class="slabel">Last Deploy</span><span>{k8sMeta.last_deploy ? new Date(k8sMeta.last_deploy).toLocaleString() : '—'}</span></div>
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
  /* ── layout ── */
  .fleet { display: flex; flex-direction: column; gap: 16px; }

  .summary {
    display: flex;
    align-items: center;
    gap: 20px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 12px 18px;
    flex-wrap: wrap;
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
    font-size: 11px;
    font-weight: 700;
    color: var(--red);
    background: rgba(239,68,68,0.1);
    border: 1px solid rgba(239,68,68,0.3);
    padding: 3px 10px;
    border-radius: 20px;
    animation: live-pulse 2s ease-in-out infinite;
  }
  @keyframes live-pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.6; } }

  .toolbar { display: flex; align-items: center; gap: 8px; margin-left: auto; }

  .search {
    background: var(--surface2);
    border: 1px solid var(--border);
    color: var(--text);
    padding: 5px 10px;
    border-radius: 6px;
    font-size: 12px;
    outline: none;
    width: 120px;
  }
  .search:focus { border-color: var(--purple); }

  .tool-btn {
    background: var(--surface2);
    border: 1px solid var(--border);
    color: var(--muted);
    padding: 5px 11px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 500;
    transition: color 0.15s, border-color 0.15s;
  }
  .tool-btn:hover { color: var(--purple); border-color: var(--purple); }

  .refresh-btn {
    background: none;
    border: 1px solid var(--border);
    color: var(--muted);
    padding: 5px 10px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 16px;
    transition: color 0.15s;
  }
  .refresh-btn:hover { color: var(--purple); }

  /* ── grid ── */
  .layout {
    display: grid;
    grid-template-columns: 280px 1fr;
    gap: 16px;
    align-items: start;
  }
  @media (max-width: 720px) { .layout { grid-template-columns: 1fr; } }

  .list { display: flex; flex-direction: column; gap: 8px; }

  .empty, .error {
    text-align: center; color: var(--muted); padding: 32px 16px;
    background: var(--surface); border-radius: 12px;
    border: 1px solid var(--border); line-height: 1.8;
  }
  .error { color: var(--red); border-color: var(--red); }

  /* ── detail panel ── */
  .detail {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 18px;
    position: sticky;
    top: 68px;
    max-height: calc(100vh - 90px);
    overflow-y: auto;
  }

  .detail-header {
    display: flex; justify-content: space-between; align-items: flex-start;
    margin-bottom: 14px;
  }
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

  /* tabs */
  .tabs {
    display: flex; gap: 0; margin-bottom: 16px;
    border-bottom: 1px solid var(--border);
    overflow-x: auto; scrollbar-width: none;
  }
  .tabs::-webkit-scrollbar { display: none; }
  .tab {
    background: none; border: none; color: var(--muted);
    padding: 7px 12px; cursor: pointer; font-size: 11px; font-weight: 500;
    border-bottom: 2px solid transparent; margin-bottom: -1px;
    transition: color 0.15s, border-color 0.15s; white-space: nowrap;
  }
  .tab:hover { color: var(--text); }
  .tab.active { color: var(--purple); border-bottom-color: var(--purple); }

  /* ── signals tab ── */
  .headline {
    display: flex; gap: 24px; align-items: flex-end;
    margin-bottom: 16px; padding-bottom: 16px;
    border-bottom: 1px solid var(--border);
  }
  .hs-block, .fused-block { display: flex; flex-direction: column; gap: 2px; }
  .hs-num {
    font-size: 52px; font-weight: 700; font-variant-numeric: tabular-nums;
    line-height: 1;
  }
  .hs-label { font-size: 10px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.06em; }
  .hs-trend, .fused-sub { font-size: 10px; color: var(--muted); }
  .hs-trend.rising { color: var(--red); }
  .hs-trend.falling { color: var(--green); }

  .fused-num {
    font-size: 28px; font-weight: 700; font-variant-numeric: tabular-nums;
    line-height: 1; font-family: 'JetBrains Mono', monospace;
  }
  .fused-label { font-size: 10px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.06em; }

  .kpi-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; margin-bottom: 14px; }

  .kpi-cell {
    background: var(--surface2); border-radius: 8px;
    padding: 8px 10px; border: 1px solid var(--border);
    transition: border-color 0.15s;
  }
  .kpi-cell:hover { border-color: var(--accent); }

  .kpi-bar-track {
    height: 3px; background: var(--surface3); border-radius: 2px;
    overflow: hidden; margin-bottom: 6px;
  }
  .kpi-bar-fill { height: 100%; border-radius: 2px; transition: width 0.4s; min-width: 2px; }

  .kpi-row { display: flex; justify-content: space-between; align-items: baseline; }
  .kpi-name { font-size: 9px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
  .kpi-val { font-size: 13px; font-weight: 700; font-variant-numeric: tabular-nums; font-family: 'JetBrains Mono', monospace; }
  .kpi-val.ok   { color: var(--green); }
  .kpi-val.warn { color: var(--yellow); }
  .kpi-val.crit { color: var(--red); }

  .kpi-trend { font-size: 9px; color: var(--muted); margin-top: 1px; }
  .kpi-trend.rising  { color: var(--red); }
  .kpi-trend.falling { color: var(--green); }

  /* explain */
  .explain-section { margin-top: 12px; }
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
    transition: color 0.15s;
  }
  .tool-btn-sm:hover { color: var(--purple); }
  .explain-pre {
    background: var(--surface2); border: 1px solid var(--border);
    border-radius: 8px; padding: 12px; font-size: 11px; font-family: 'JetBrains Mono', monospace;
    color: var(--text); line-height: 1.7; white-space: pre-wrap; word-break: break-word;
    max-height: 200px; overflow-y: auto;
  }

  /* ── history tab ── */
  .sig-toggles { display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 10px; }
  .sig-toggle {
    background: var(--surface2); border: 1px solid var(--border); color: var(--muted);
    padding: 3px 8px; border-radius: 20px; cursor: pointer; font-size: 10px;
    transition: color 0.15s, border-color 0.15s, background 0.15s;
  }
  .sig-toggle.active { color: var(--sc); border-color: var(--sc); background: color-mix(in srgb, var(--sc) 10%, transparent); }

  .chart-wrap { height: 240px; position: relative; }

  /* ── forecast tab ── */
  .forecast-meta { display: flex; gap: 16px; flex-wrap: wrap; margin-bottom: 14px; }
  .fcast-stat { display: flex; flex-direction: column; gap: 2px; }
  .fcast-val { font-size: 18px; font-weight: 700; font-variant-numeric: tabular-nums; }
  .fcast-val.good { color: var(--green); }
  .fcast-val.bad  { color: var(--red); }
  .fcast-val.warn { color: var(--yellow); }

  .low-conf-banner {
    background: rgba(245,158,11,0.08); border: 1px solid rgba(245,158,11,0.3);
    border-radius: 6px; padding: 7px 10px; font-size: 11px; color: var(--yellow);
    margin-bottom: 10px;
  }

  /* ── events tab ── */
  .event-list { display: flex; flex-direction: column; gap: 4px; max-height: 420px; overflow-y: auto; }
  .event-row {
    display: flex; align-items: center; gap: 8px; padding: 6px 10px;
    background: var(--surface2); border-radius: 6px; font-size: 11px;
    border-left: 3px solid var(--etc);
  }
  .evt-type { font-weight: 700; color: var(--etc); text-transform: uppercase; font-size: 9px; width: 56px; flex-shrink: 0; }
  .evt-wl { flex: 1; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: monospace; }
  .evt-fused { font-family: 'JetBrains Mono', monospace; font-size: 10px; flex-shrink: 0; }
  .evt-ts { font-size: 10px; color: var(--muted); flex-shrink: 0; font-family: monospace; }

  /* ── actions tab ── */
  .action-card {
    background: var(--surface2); border: 1px solid var(--border);
    border-radius: 8px; padding: 12px; margin-bottom: 8px;
  }
  .action-card.tier1 { border-color: rgba(239,68,68,0.3); }
  .action-hdr { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
  .action-tier {
    font-size: 9px; font-weight: 700; padding: 2px 6px; border-radius: 3px;
    background: rgba(239,68,68,0.1); color: var(--red); text-transform: uppercase;
  }
  .action-kind { font-size: 11px; font-weight: 600; flex: 1; }
  .action-state { font-size: 9px; color: var(--muted); text-transform: uppercase; }
  .action-state.pending { color: var(--yellow); }
  .action-desc { font-size: 12px; color: var(--text); margin-bottom: 4px; line-height: 1.4; }
  .action-host { font-size: 10px; color: var(--muted); font-family: monospace; margin-bottom: 8px; }
  .action-btns { display: flex; gap: 6px; }
  .btn-approve {
    background: rgba(0,229,160,0.1); border: 1px solid rgba(0,229,160,0.3);
    color: var(--green); padding: 4px 12px; border-radius: 5px;
    cursor: pointer; font-size: 11px; font-weight: 600;
    transition: background 0.15s;
  }
  .btn-approve:hover { background: rgba(0,229,160,0.2); }
  .btn-reject {
    background: rgba(239,68,68,0.08); border: 1px solid rgba(239,68,68,0.25);
    color: var(--red); padding: 4px 12px; border-radius: 5px;
    cursor: pointer; font-size: 11px; font-weight: 600;
    transition: background 0.15s;
  }
  .btn-reject:hover { background: rgba(239,68,68,0.15); }

  /* ── kubernetes tab ── */
  .k8s-section { margin-bottom: 14px; }
  .k8s-field { display: flex; flex-direction: column; gap: 3px; margin-bottom: 8px; }
  .k8s-img { font-family: monospace; font-size: 11px; color: var(--cyan); word-break: break-all; }

  .replica-gauge { display: flex; align-items: center; gap: 10px; margin-top: 5px; }
  .replica-track { flex: 1; height: 7px; background: var(--surface2); border-radius: 4px; overflow: hidden; }
  .replica-fill { height: 100%; border-radius: 4px; background: var(--muted); transition: width 0.3s; }
  .replica-fill.replica-ok { background: var(--green); }
  .replica-fill.replica-warn { background: var(--yellow); }
  .replica-txt { font-size: 11px; color: var(--muted); white-space: nowrap; font-variant-numeric: tabular-nums; }

  .pod-table { width: 100%; border-collapse: collapse; font-size: 11px; }
  .pod-table th { text-align: left; color: var(--muted); font-weight: 500; padding: 4px 6px; border-bottom: 1px solid var(--border); text-transform: uppercase; letter-spacing: 0.04em; font-size: 9px; }
  .pod-table td { padding: 5px 6px; border-bottom: 1px solid var(--border); color: var(--text); }
  .pod-table tr:last-child td { border-bottom: none; }
  .pod-name { font-family: monospace; font-size: 10px; max-width: 130px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .pod-status { display: inline-block; padding: 1px 6px; border-radius: 3px; font-size: 10px; font-weight: 500; background: var(--surface2); color: var(--muted); }
  .pod-status.pod-running { background: rgba(0,229,160,0.12); color: var(--green); }
  .pod-status.pod-failed  { background: rgba(239,68,68,0.12); color: var(--red); }
  .pod-restarts { font-variant-numeric: tabular-nums; }
  .pod-restarts.warn { color: var(--yellow); font-weight: 600; }

  /* shared */
  .notice {
    display: flex; flex-direction: column; align-items: center; gap: 10px;
    padding: 28px 12px; text-align: center; color: var(--muted); font-size: 12px;
  }
  .notice strong { color: var(--text); font-size: 14px; }
  .notice p { line-height: 1.7; max-width: 280px; }
  .notice code { font-family: monospace; background: var(--surface2); padding: 1px 5px; border-radius: 3px; color: var(--cyan); }
  .notice-icon { font-size: 36px; opacity: 0.4; }
  .calib-notice .notice-icon { color: var(--purple); opacity: 1; }
  .spin { animation: spin 2s linear infinite; display: inline-block; }
  @keyframes spin { to { transform: rotate(360deg); } }

  .loading { text-align: center; color: var(--muted); font-size: 12px; padding: 20px; }
</style>

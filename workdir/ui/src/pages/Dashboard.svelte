<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'
  import { token, kpis, alerts, wsConnected } from '../lib/store.js'
  import KpiGauge from '../lib/KpiGauge.svelte'
  import Sparkline from '../lib/Sparkline.svelte'

  let host = ''
  let wsClient = null
  let liveKpis = {}
  let metricHistory = {}
  let recentAlerts = []
  let predictions = []
  let pollTimer = null
  let dataflow = { metrics: 0, logs: 0, traces: 0 }

  const KPI_NAMES = ['stress','fatigue','mood','pressure','humidity','contagion']
  const ETF_KPI_NAMES = ['resilience','entropy','velocity','health_score']
  const PCT_METRICS = new Set(['cpu_percent','memory_percent','disk_percent'])
  const KPI_METRICS = new Set(['stress','fatigue','mood','pressure','humidity','contagion','resilience','entropy','velocity'])
  const PRED_SHOW = new Set([...PCT_METRICS, ...KPI_METRICS, 'load_avg_1'])

  const KPI_LABELS = {
    stress:'CPU Pressure', fatigue:'Memory Pressure', mood:'Trend',
    pressure:'Load Index', humidity:'Saturation', contagion:'Blast Radius',
    resilience:'Resilience', entropy:'Entropy', velocity:'Velocity', health_score:'Reliability'
  }

  function fmtPred(p) {
    const isKpi = KPI_METRICS.has(p.target)
    const isPct = PCT_METRICS.has(p.target)
    const scale = isKpi ? 100 : 1
    const unit = (isKpi || isPct) ? '%' : ''
    return {
      cur: (Math.max(0, p.current * scale)).toFixed(1) + unit,
      pred: (Math.max(0, p.predicted * scale)).toFixed(1) + unit
    }
  }

  function kpiVal(name) {
    const v = liveKpis[name]?.value ?? 0
    return name === 'health_score' ? v : v * 100
  }

  function kpiColor(name, val) {
    const v = val / 100
    if (name === 'mood' || name === 'resilience') {
      return v >= 0.7 ? 'var(--green)' : v >= 0.4 ? 'var(--amber)' : 'var(--red)'
    }
    return v <= 0.3 ? 'var(--green)' : v <= 0.6 ? 'var(--amber)' : 'var(--red)'
  }

  async function detectHost() {
    try { const r = await api.health(); host = r.data?.host || '' } catch {}
  }
  async function loadKPIs() {
    try { const r = await api.kpis(host); liveKpis = r.data || {} } catch {}
  }
  async function loadPredictions() {
    try {
      const r = await api.predict(host, '', 120)
      const all = r.data?.predictions || []
      predictions = (Array.isArray(all) ? all : Object.values(all)).filter(p => PRED_SHOW.has(p.target))
    } catch {}
  }
  async function loadAlerts() {
    try { const r = await api.alerts(); recentAlerts = (r.data || []).slice(0, 8) } catch {}
  }

  function connectWS() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    wsClient = new WebSocket(`${proto}://${location.host}/api/v1/ws?token=${encodeURIComponent($token || '')}`)
    wsClient.onopen  = () => wsConnected.set(true)
    wsClient.onclose = () => { wsConnected.set(false); setTimeout(connectWS, 3000) }
    wsClient.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data)
        if (msg.type === 'kpi' && msg.payload)    liveKpis = { ...liveKpis, ...msg.payload }
        if (msg.type === 'alert' && msg.payload)   recentAlerts = [msg.payload, ...recentAlerts].slice(0, 8)
        if (msg.type === 'metric' && msg.payload) {
          const { name, value } = msg.payload
          metricHistory = { ...metricHistory, [name]: [...(metricHistory[name] || []), value].slice(-20) }
        }
      } catch {}
    }
  }

  onMount(async () => {
    await detectHost()
    loadKPIs(); loadAlerts(); loadPredictions()
    connectWS()
    api.dataflow().then(r => { if (r.data) dataflow = r.data }).catch(() => {})
    pollTimer = setInterval(() => {
      loadKPIs(); loadAlerts(); loadPredictions()
      api.dataflow().then(r => { if (r.data) dataflow = r.data }).catch(() => {})
    }, 15000)
  })

  onDestroy(() => { if (wsClient) wsClient.close(); clearInterval(pollTimer) })

  async function ackAlert(id) {
    await api.alertAck(id).catch(() => {})
    loadAlerts()
  }

  $: healthVal = kpiVal('health_score')
  $: healthColor = healthVal >= 70 ? 'var(--green)' : healthVal >= 40 ? 'var(--amber)' : 'var(--red)'
</script>

<!-- Müller-Brockmann: main content area is the grid zone -->
<div class="dash-spread">

  <!-- Grid overlay (same container — §2.2 compliance) -->
  <div class="guides" aria-hidden="true">
    <div class="cols"></div>
    <div class="rows"></div>
    <div class="mline l"></div>
    <div class="mline r"></div>
  </div>

  <!-- Hero band: large FRI numeral (Swiss big numeral move) + health score -->
  <div class="band hero-band">
    <div class="hero-num-col" style="grid-column: 1 / 5">
      <div class="kicker">Health Score</div>
      <div class="hero-numeral" style="color:{healthColor}">{healthVal.toFixed(0)}</div>
      <div class="hero-unit">/ 100</div>
      <div class="live-dot" class:live={$wsConnected} title={$wsConnected ? 'Live' : 'Polling'}></div>
    </div>

    <!-- Dataflow counters -->
    <div class="flow-col" style="grid-column: 5 / 9">
      <div class="kicker">Data ingest</div>
      <div class="flow-row">
        <div class="flow-item">
          <span class="flow-val num">{dataflow.metrics.toLocaleString()}</span>
          <span class="flow-lbl">Metrics</span>
        </div>
        <div class="flow-sep"></div>
        <div class="flow-item">
          <span class="flow-val num">{dataflow.logs.toLocaleString()}</span>
          <span class="flow-lbl">Logs</span>
        </div>
        <div class="flow-sep"></div>
        <div class="flow-item">
          <span class="flow-val num">{dataflow.traces.toLocaleString()}</span>
          <span class="flow-lbl">Traces</span>
        </div>
      </div>
    </div>

    <!-- Active alerts count -->
    <div class="alert-count-col" style="grid-column: 9 / 13">
      <div class="kicker">Active alerts</div>
      <div class="hero-numeral-sm num" style="color:{recentAlerts.filter(a=>!a.acknowledged).length > 0 ? 'var(--red)' : 'var(--green)'}">
        {recentAlerts.filter(a=>!a.acknowledged).length}
      </div>
    </div>
  </div>

  <!-- Divider -->
  <div class="band rule-band">
    <div class="rule" style="grid-column: 1 / -1"></div>
  </div>

  <!-- KPI signals band -->
  <div class="band signals-band">
    <div class="section-head" style="grid-column: 1 / -1">
      <span class="kicker">KPI Signals</span>
    </div>
    {#each KPI_NAMES as name}
      {@const val = kpiVal(name)}
      {@const color = kpiColor(name, val)}
      <div class="signal-card" style="grid-column: span 2">
        <div class="signal-label">{KPI_LABELS[name] || name}</div>
        <div class="signal-val num" style="color:{color}">{val.toFixed(0)}<span class="unit">%</span></div>
        <div class="signal-bar-track">
          <div class="signal-bar-fill" style="width:{Math.min(val,100)}%; background:{color}"></div>
        </div>
      </div>
    {/each}
  </div>

  <!-- ETF composed KPIs band -->
  <div class="band etf-band">
    <div class="section-head" style="grid-column: 1 / -1">
      <span class="kicker">Composed KPIs</span>
      <span class="kicker-badge">ensemble</span>
    </div>
    {#each ETF_KPI_NAMES as name}
      {@const val = kpiVal(name)}
      {@const color = kpiColor(name, val)}
      <div class="signal-card etf" style="grid-column: span 3">
        <div class="signal-label">{KPI_LABELS[name] || name}</div>
        <div class="signal-val lg num" style="color:{color}">{val.toFixed(0)}</div>
        <div class="signal-bar-track">
          <div class="signal-bar-fill" style="width:{Math.min(val,100)}%; background:{color}"></div>
        </div>
      </div>
    {/each}
  </div>

  <div class="band rule-band">
    <div class="rule" style="grid-column: 1 / -1"></div>
  </div>

  <!-- Predictions + Alerts band -->
  <div class="band bottom-band">
    <!-- Predictions table -->
    {#if predictions.length > 0}
      <div style="grid-column: 1 / 7">
        <div class="section-head"><span class="kicker">Predictions</span><span class="kicker-badge violet">2h horizon</span></div>
        <table class="data-table">
          <thead>
            <tr>
              <th>Signal</th>
              <th class="num-col">Now</th>
              <th class="num-col">→ 2h</th>
              <th>Trend</th>
            </tr>
          </thead>
          <tbody>
            {#each predictions as p}
              {@const f = fmtPred(p)}
              <tr>
                <td>{p.target}</td>
                <td class="num-col num">{f.cur}</td>
                <td class="num-col num">{f.pred}</td>
                <td class="trend trend-{p.trend}">{p.trend}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

    <!-- Alerts table -->
    <div style="grid-column: {predictions.length > 0 ? '7 / 13' : '1 / -1'}">
      <div class="section-head">
        <span class="kicker">Recent alerts</span>
        <span class="kicker-badge {recentAlerts.filter(a=>!a.acknowledged).length > 0 ? 'red' : ''}">{recentAlerts.length}</span>
      </div>
      {#if recentAlerts.length === 0}
        <div class="empty-state">
          <span class="empty-icon">✓</span>
          <span>No active alerts</span>
        </div>
      {:else}
        <table class="data-table">
          <thead>
            <tr><th>Workload</th><th>Rule</th><th>Severity</th><th></th></tr>
          </thead>
          <tbody>
            {#each recentAlerts as a}
              <tr class:crit={a.severity === 'critical'} class:warn={a.severity === 'warning'}>
                <td>{a.host}</td>
                <td class="rule-cell">{a.rule_id}</td>
                <td>
                  <span class="sev-badge sev-{a.severity || 'info'}">{a.severity || 'info'}</span>
                </td>
                <td>
                  {#if !a.acknowledged}
                    <button class="ack-btn" on:click={() => ackAlert(a.id)}>Ack</button>
                  {:else}
                    <span class="acked">✓</span>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  </div>

  <!-- Sparklines band -->
  {#if Object.keys(metricHistory).length > 0}
    <div class="band spark-band">
      <div class="section-head" style="grid-column: 1 / -1"><span class="kicker">Live metrics</span></div>
      {#each Object.entries(metricHistory) as [name, vals]}
        <div class="spark-card" style="grid-column: span 3">
          <span class="spark-name">{name}</span>
          <Sparkline data={vals} />
          <span class="spark-val num">{vals[vals.length-1]?.toFixed(1)}%</span>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  /* ── Grid scaffold — ONE source of truth ── */
  .dash-spread {
    position: relative;
    padding: 32px 24px;
    display: grid;
    grid-template-columns: repeat(12, 1fr);
    column-gap: 20px;
    row-gap: 0;
    overflow-y: auto;
    height: 100%;
  }

  /* ── Grid overlay (SAME container as content — §2.2) ── */
  .guides {
    position: absolute;
    inset: 0;
    pointer-events: none;
    z-index: 60;
    opacity: 0;
    transition: opacity 0.26s;
  }
  :global(body.grid-on) .guides { opacity: 1; }
  .guides .cols {
    position: absolute; top: 0; bottom: 0; left: 24px; right: 24px;
    display: grid; grid-template-columns: repeat(12, 1fr); column-gap: 20px;
  }
  .guides .rows {
    position: absolute; left: 24px; right: 24px; top: 0; bottom: 0;
    background-image:
      repeating-linear-gradient(to bottom, rgba(34,211,238,0.08) 0 1px, transparent 1px 24px),
      repeating-linear-gradient(to bottom, rgba(34,211,238,0.04) 0 1px, transparent 1px 8px);
  }
  .guides .mline { position: absolute; top: 0; bottom: 0; width: 1px; background: rgba(34,211,238,0.25); }
  .guides .mline.l { left: 24px; } .guides .mline.r { right: 24px; }

  /* ── Bands — children placed by column line ── */
  .band {
    grid-column: 1 / -1;
    display: grid;
    grid-template-columns: subgrid;
    column-gap: 20px;
    align-items: start;
    padding-top: 24px;
    padding-bottom: 8px;
  }
  @supports not (grid-template-columns: subgrid) {
    .band { grid-template-columns: repeat(12, 1fr); }
  }

  .rule-band { padding: 0; }
  .rule { height: 1px; background: var(--border, rgba(148,163,184,0.10)); margin: 8px 0; }

  /* ── Hero band — Swiss big numerals ── */
  .hero-band { align-items: end; padding-bottom: 24px; }

  .kicker {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--text-3, #3F4D5C);
    margin-bottom: 8px;
    display: block;
  }

  /* THE signature move: large mono numeral flush-left */
  .hero-numeral {
    font-family: "DM Mono", "Fira Code", monospace;
    font-size: clamp(56px, 6vw, 80px);
    font-weight: 500;
    line-height: 1;
    letter-spacing: -0.02em;
    font-variant-numeric: tabular-nums;
    margin-left: -3px; /* optical ink alignment */
  }
  .hero-unit {
    font-family: "DM Mono", monospace;
    font-size: 16px;
    color: var(--text-3, #3F4D5C);
    margin-top: 4px;
    font-variant-numeric: tabular-nums;
  }

  .hero-numeral-sm {
    font-family: "DM Mono", "Fira Code", monospace;
    font-size: 48px;
    font-weight: 500;
    line-height: 1;
    font-variant-numeric: tabular-nums;
    margin-top: 8px;
    margin-left: -2px;
  }

  .live-dot {
    width: 7px; height: 7px; border-radius: 50%;
    background: var(--text-3, #3F4D5C);
    margin-top: 12px;
    display: inline-block;
  }
  .live-dot.live { background: var(--green, #22C55E); box-shadow: 0 0 6px var(--green, #22C55E); }

  .flow-col { display: flex; flex-direction: column; }
  .flow-row { display: flex; align-items: center; gap: 24px; margin-top: 8px; }
  .flow-item { display: flex; flex-direction: column; gap: 4px; }
  .flow-val {
    font-family: "DM Mono", monospace;
    font-size: 28px;
    font-weight: 500;
    line-height: 1;
    color: var(--text, #E2E8F0);
    font-variant-numeric: tabular-nums;
  }
  .flow-lbl {
    font-size: 10px; font-weight: 600; letter-spacing: 0.1em;
    text-transform: uppercase; color: var(--text-3, #3F4D5C);
  }
  .flow-sep { width: 1px; height: 32px; background: var(--border, rgba(148,163,184,0.10)); }

  /* ── Signal cards ── */
  .section-head {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
  }
  .kicker-badge {
    font-size: 9px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    padding: 2px 6px;
    border-radius: 3px;
    background: var(--surface-2, #1E2535);
    color: var(--text-3, #3F4D5C);
    border: 1px solid var(--border, rgba(148,163,184,0.10));
  }
  .kicker-badge.violet { background: var(--violet-dim, rgba(139,92,246,0.10)); color: var(--violet, #8B5CF6); }
  .kicker-badge.red { background: var(--red-dim, rgba(239,68,68,0.10)); color: var(--red, #EF4444); }

  .signal-card {
    background: var(--surface, #1E293B);
    border: 1px solid var(--border, rgba(148,163,184,0.10));
    border-radius: 4px;
    padding: 12px;
    margin-bottom: 8px;
  }
  .signal-card.etf { border-color: var(--accent-dim, rgba(56,189,248,0.15)); }
  .signal-label { font-size: 10px; font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; color: var(--text-3, #3F4D5C); margin-bottom: 6px; }
  .signal-val {
    font-family: "DM Mono", monospace;
    font-size: 24px;
    font-weight: 500;
    line-height: 1;
    font-variant-numeric: tabular-nums;
    margin-bottom: 8px;
  }
  .signal-val.lg { font-size: 32px; }
  .unit { font-size: 14px; color: var(--text-3, #3F4D5C); margin-left: 1px; }
  .signal-bar-track {
    height: 3px;
    background: var(--surface-3, #1C2540);
    border-radius: 2px;
    overflow: hidden;
  }
  .signal-bar-fill { height: 100%; border-radius: 2px; transition: width 0.4s ease; }

  /* ── Data tables ── */
  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
    margin-top: 8px;
  }
  th {
    text-align: left;
    padding: 6px 8px;
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-3, #3F4D5C);
    border-bottom: 1px solid var(--border, rgba(148,163,184,0.10));
  }
  td {
    padding: 7px 8px;
    border-bottom: 1px solid var(--border, rgba(148,163,184,0.08));
    color: var(--text-2, #94A3B8);
    vertical-align: middle;
  }
  .num-col { text-align: right; font-family: "DM Mono", monospace; font-variant-numeric: tabular-nums; }
  .rule-cell { color: var(--text-3, #3F4D5C); font-size: 11px; max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  tr.crit td { background: var(--red-dim, rgba(239,68,68,0.05)); }
  tr.warn td { background: var(--amber-dim, rgba(245,158,11,0.05)); }

  .trend { font-weight: 600; }
  .trend-rising  { color: var(--red, #EF4444); }
  .trend-falling { color: var(--green, #22C55E); }
  .trend-stable  { color: var(--text-3, #3F4D5C); }

  .sev-badge {
    font-size: 9px; font-weight: 700; letter-spacing: 0.06em;
    text-transform: uppercase; padding: 2px 6px; border-radius: 3px;
  }
  .sev-critical { background: var(--red-dim, rgba(239,68,68,0.12)); color: var(--red, #EF4444); }
  .sev-warning  { background: var(--amber-dim, rgba(245,158,11,0.12)); color: var(--amber, #F59E0B); }
  .sev-info     { background: var(--surface-3, #1C2540); color: var(--text-3, #3F4D5C); }

  .ack-btn {
    background: none; border: 1px solid var(--border-2, rgba(148,163,184,0.18));
    color: var(--text-3, #3F4D5C); padding: 2px 8px; border-radius: 3px;
    font-size: 10px; cursor: pointer; font-family: inherit;
  }
  .ack-btn:hover { border-color: var(--accent, #38BDF8); color: var(--accent, #38BDF8); }
  .acked { color: var(--green, #22C55E); font-size: 12px; }

  .empty-state {
    display: flex; align-items: center; gap: 8px;
    padding: 16px 8px; font-size: 12px; color: var(--text-3, #3F4D5C);
  }
  .empty-icon { color: var(--green, #22C55E); }

  /* ── Sparklines band ── */
  .spark-band { padding-top: 16px; }
  .spark-card {
    display: flex; align-items: center; gap: 8px;
    background: var(--surface, #1E293B);
    border: 1px solid var(--border, rgba(148,163,184,0.10));
    border-radius: 4px;
    padding: 8px 12px;
    margin-bottom: 8px;
  }
  .spark-name { font-size: 11px; color: var(--text-2, #94A3B8); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .spark-val { font-family: "DM Mono", monospace; font-size: 12px; font-weight: 500; color: var(--text, #E2E8F0); font-variant-numeric: tabular-nums; }

  /* Grid toggle script */
  :global(.grid-toggle) {
    position: fixed;
    bottom: 24px; right: 24px; z-index: 200;
    display: flex; align-items: center; gap: 6px;
    background: var(--surface-2); border: 1px solid var(--border-2);
    color: var(--text-3); font-size: 10px; letter-spacing: 0.1em;
    text-transform: uppercase; padding: 6px 10px; border-radius: 4px;
    cursor: pointer; font-family: "DM Mono", monospace;
  }
  :global(body.grid-on .grid-toggle) { background: var(--accent); color: #000; border-color: transparent; }
</style>

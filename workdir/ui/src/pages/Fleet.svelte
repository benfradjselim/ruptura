<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'
  import { displayLabel, displayValue, displayUnit, signalClass, incidentProbabilityBand } from '../lib/displayLabels.js'

  let fleet = null
  let error = null
  let loading = true
  let interval
  let expandedWorkload = null
  let activeTab = {}   // { workloadKey: 'signals'|'history'|'forecast' }
  let historyData = {} // { workloadKey: [] }
  let forecastData = {} // { workloadKey: {} }
  let summaryData = {} // { workloadKey: { headline, confidence, recommended_action, warming_up } }
  let loadingDetail = {}

  // Infra context: keyed by namespace; matched on namespace only (v8.1 key normalization gap)
  let infraByNs = {} // { [namespace]: [{ group, health, spread, objectCount }] }

  const SIGNAL_KEYS = ['stress', 'fatigue', 'mood', 'contagion', 'pressure', 'resilience']

  async function load() {
    try {
      const [fleetRes, infraRes] = await Promise.allSettled([
        api.fleet(),
        api.infraGroups(),
      ])
      if (fleetRes.status === 'fulfilled') fleet = fleetRes.value
      if (infraRes.status === 'fulfilled') {
        const ns = {}
        for (const g of (infraRes.value.groups || [])) {
          const key = g.namespace || ''
          if (!ns[key]) ns[key] = []
          ns[key].push(g)
        }
        infraByNs = ns
      }
      error = null
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  function hostKey(workload) {
    return workload.host || workload.key || workload.name || ''
  }

  function parseHost(workload) {
    const raw = hostKey(workload)
    const parts = raw.split('/')
    if (parts.length >= 3) return { ns: parts[0], kind: parts[1], name: parts.slice(2).join('/') }
    return { ns: '', kind: 'host', name: raw }
  }

  async function expand(workload) {
    const key = hostKey(workload)
    if (expandedWorkload === key) {
      expandedWorkload = null
      return
    }
    expandedWorkload = key
    if (!activeTab[key]) activeTab[key] = 'signals'
    loadTab(workload, activeTab[key])
    loadSummary(workload)
  }

  // FBL-A2-1: forecast-as-hero — the summary sentence is the first thing
  // shown on expand, before the chart/signal tabs.
  async function loadSummary(workload) {
    const key = hostKey(workload)
    if (summaryData[key]) return
    const { ns, kind, name } = parseHost(workload)
    try {
      summaryData[key] = await api.workloadSummary(ns, kind, name)
    } catch {
      summaryData[key] = null
    }
    summaryData = { ...summaryData }
  }

  async function loadTab(workload, tab) {
    const key = hostKey(workload)
    activeTab[key] = tab
    if (tab === 'history' && !historyData[key]) {
      loadingDetail[key] = true
      try {
        const from = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString()
        const res = await api.history(key, from)
        historyData[key] = Array.isArray(res) ? res : (res.snapshots || [])
      } catch { historyData[key] = [] }
      loadingDetail[key] = false
    }
    if (tab === 'forecast' && !forecastData[key]) {
      loadingDetail[key] = true
      try {
        const res = await api.forecastWorkload('health_score', key)
        forecastData[key] = res
      } catch { forecastData[key] = { error: true } }
      loadingDetail[key] = false
    }
    activeTab = { ...activeTab }
  }

  function stateClass(state) {
    if (!state) return 'unknown'
    const s = state.toLowerCase()
    if (s === 'healthy' || s === 'excellent' || s === 'good') return 'healthy'
    if (s === 'degraded' || s === 'fair') return 'degraded'
    if (s === 'at-risk' || s === 'warning' || s === 'poor') return 'at-risk'
    if (s === 'critical' || s === 'emergency' || s === 'rupture') return 'critical'
    if (s === 'calibrating' || s === 'warming') return 'calibrating'
    return 'unknown'
  }

  function healthPct(w) {
    const v = w.health_score ?? w.kpis?.health_score ?? null
    if (v == null) return null
    return (v * 100).toFixed(0)
  }

  function riskScore(w) {
    const r = w.fused_rupture_index ?? w.fused_r ?? null
    if (r == null) return null
    return (r * 10).toFixed(1)
  }

  function workloadLabel(w) {
    const { ns, name } = parseHost(w)
    if (ns) return `${ns}/${name}`
    return w.host || 'unknown'
  }

  function getSignal(w, key) {
    if (w.signals) return w.signals[key]
    if (w.kpis) return w.kpis[key]
    return w[key]
  }

  function infraGroups(w) {
    const ns = parseHost(w).ns || ''
    const nsGroups = infraByNs[ns] || []
    const clusterGroups = infraByNs[''] || []
    return [...nsGroups, ...clusterGroups]
  }

  function infraHealthColor(h) {
    if (h >= 0.9) return 'var(--green)'
    if (h >= 0.6) return 'var(--amber)'
    return 'var(--red)'
  }

  const INFRA_GROUP_LABELS = {
    node: 'Node', network: 'Net', storage: 'Store',
    admission: 'Admission', tenancy: 'Tenancy',
    operators: 'Ops', co: 'CO', mcp: 'MCP',
  }

  onMount(() => {
    load()
    interval = setInterval(load, 15000)
  })
  onDestroy(() => clearInterval(interval))
</script>

<div class="fleet-page">
  <div class="fleet-header">
    <h1>Fleet</h1>
    {#if fleet}
      <span class="fleet-count">{fleet.hosts?.length ?? 0} workloads</span>
    {/if}
  </div>

  {#if loading}
    <div class="fleet-loading">
      <div class="spinner"></div>
      <p>Loading fleet…</p>
    </div>

  {:else if error}
    <div class="fleet-error">
      <p>⚠ Could not load fleet: {error}</p>
      <button on:click={load}>Retry</button>
    </div>

  {:else if !fleet?.hosts?.length}
    <!-- ITEM-017: Empty state -->
    <div class="empty-state">
      <div class="empty-icon">⏱</div>
      <h2>Calibrating baselines</h2>
      <p>
        Ruptura is learning your workloads' normal behavior.
        This typically takes <strong>5–15 minutes</strong> once telemetry is flowing.
      </p>
      <p class="empty-hint">
        Send metrics via OTLP on port 4317 or Prometheus remote-write on <code>/api/v2/write</code>.
      </p>
      <div class="empty-actions">
        <a href="https://benfradjselim.github.io/ruptura/getting-started/ingest/" target="_blank" rel="noopener">
          Setup guide →
        </a>
        <button on:click={load}>Check again</button>
      </div>
    </div>

  {:else}
    <div class="workload-grid">
      {#each fleet.hosts as w (w.host)}
        {@const key = w.host}
        {@const pct = healthPct(w)}
        {@const risk = riskScore(w)}
        {@const sc = stateClass(w.state)}
        {@const expanded = expandedWorkload === key}

        <div class="workload-card {sc}" class:expanded>
          <!-- Card header -->
          <button class="card-header" on:click={() => expand(w)}>
            <div class="card-identity">
              <span class="state-dot {sc}"></span>
              <span class="workload-name">{workloadLabel(w)}</span>
              {#if w.state}
                <span class="state-badge {sc}">{w.state}</span>
              {/if}
            </div>
            <div class="card-scores">
              {#if pct != null}
                <div class="score-block">
                  <span class="score-label">Reliability</span>
                  <!-- ITEM-010: null guard on health_score -->
                  <span class="score-value {signalClass('health_score', w.health_score ?? w.kpis?.health_score)}">{pct}%</span>
                </div>
              {:else}
                <div class="score-block calibrating-pill">
                  <span class="score-label">Calibrating</span>
                  {#if (w.calibration_progress ?? 0) > 0}
                    <div class="calib-bar-track" title="Calibration {w.calibration_progress}%">
                      <div class="calib-bar-fill" style="width: {w.calibration_progress}%"></div>
                    </div>
                    <span class="calib-pct">{w.calibration_progress}%</span>
                  {/if}
                </div>
              {/if}
              {#if risk != null}
                <div class="score-block" title={incidentProbabilityBand(w.fused_rupture_index ?? w.fused_r)}>
                  <span class="score-label">Risk</span>
                  <span class="score-value {signalClass('fused_r', w.fused_rupture_index ?? w.fused_r)}">{risk}</span>
                </div>
              {/if}
              <span class="expand-icon">{expanded ? '▲' : '▼'}</span>
            </div>
          </button>

          <!-- Signal mini-bars (always visible) -->
          <div class="signal-bars">
            {#each SIGNAL_KEYS as sk}
              {@const raw = getSignal(w, sk)}
              {#if raw != null}
                <div class="signal-bar-item" title="{displayLabel(sk)}: {displayValue(sk, raw)}{displayUnit(sk)}">
                  <span class="signal-bar-label">{displayLabel(sk)}</span>
                  <div class="signal-bar-track">
                    <div class="signal-bar-fill {signalClass(sk, raw)}" style="width: {Math.min(raw * 100, 100)}%"></div>
                  </div>
                  <span class="signal-bar-val">{displayValue(sk, raw)}{displayUnit(sk)}</span>
                </div>
              {/if}
            {/each}
          </div>

          <!-- Infra context bar — matched on namespace only (v8.1) -->
          {#if expanded}
            {@const ig = infraGroups(w)}
            {#if ig.length > 0}
              <div class="infra-bar">
                <span class="infra-bar-label">Infra</span>
                {#each ig as g}
                  <div class="infra-chip"
                    style="border-color:{infraHealthColor(g.health)}"
                    title="{INFRA_GROUP_LABELS[g.group] || g.group}: {Math.round(g.health * 100)}% healthy, {g.objectCount} objects">
                    <span class="infra-dot" style="background:{infraHealthColor(g.health)}"></span>
                    <span class="infra-chip-name">{INFRA_GROUP_LABELS[g.group] || g.group}</span>
                    <span class="infra-chip-val" style="color:{infraHealthColor(g.health)}">{Math.round(g.health * 100)}%</span>
                  </div>
                {/each}
              </div>
            {/if}
          {/if}

          <!-- Expanded detail tabs -->
          {#if expanded}
            <div class="card-detail">
              <!-- FBL-A2-1: forecast-as-hero — the sentence is the first thing
                   shown, before the chart (forecast tab) or raw signals. -->
              {#if summaryData[key]}
                <div class="summary-hero" class:warming={summaryData[key].warming_up}>
                  <p class="summary-headline">{summaryData[key].headline}</p>
                  {#if !summaryData[key].warming_up}
                    <p class="summary-action">{summaryData[key].recommended_action}</p>
                  {/if}
                </div>
              {/if}

              <div class="tab-bar">
                {#each ['signals', 'history', 'forecast'] as t}
                  <button
                    class="tab-btn"
                    class:active={activeTab[key] === t}
                    on:click={() => loadTab(w, t)}
                  >{t.charAt(0).toUpperCase() + t.slice(1)}</button>
                {/each}
              </div>

              {#if activeTab[key] === 'signals'}
                <div class="signals-detail">
                  {#each Object.entries(w.signals || w.kpis || { stress: w.stress, fatigue: w.fatigue, contagion: w.contagion, health_score: w.health_score }).filter(([,v]) => v != null) as [k, v]}
                    {#if typeof v === 'number'}
                      <div class="signal-row">
                        <span class="signal-name">{displayLabel(k)}</span>
                        <div class="signal-bar-track wide">
                          <div class="signal-bar-fill {signalClass(k, v)}" style="width: {Math.min(v * 100, 100)}%"></div>
                        </div>
                        <span class="signal-val {signalClass(k, v)}">{displayValue(k, v)}{displayUnit(k)}</span>
                      </div>
                    {:else if v != null && typeof v === 'object' && v.value != null}
                      <div class="signal-row">
                        <span class="signal-name">{displayLabel(k)}</span>
                        <div class="signal-bar-track wide">
                          <div class="signal-bar-fill {signalClass(k, v.value)}" style="width: {Math.min(v.value * 100, 100)}%"></div>
                        </div>
                        <span class="signal-val {signalClass(k, v.value)}">{displayValue(k, v.value)}{displayUnit(k)}</span>
                      </div>
                    {/if}
                  {/each}
                </div>

              {:else if activeTab[key] === 'history'}
                {#if loadingDetail[key]}
                  <div class="detail-loading"><div class="spinner sm"></div> Loading history…</div>
                {:else if !historyData[key]?.length}
                  <!-- ITEM-017: History empty state -->
                  <div class="detail-empty">
                    <p>No history yet — snapshots accumulate over time.</p>
                    <p class="hint">Check back after a few hours of telemetry.</p>
                  </div>
                {:else}
                  <div class="history-list">
                    {#each historyData[key].slice(0, 20) as snap}
                      <div class="history-row">
                        <span class="history-time">{new Date(snap.timestamp || snap.ts).toLocaleTimeString()}</span>
                        <span class="history-score {signalClass('health_score', snap.health_score)}">
                          {snap.health_score != null ? (snap.health_score * 100).toFixed(0) + '%' : '—'}
                        </span>
                        <span class="history-fri {signalClass('fused_r', snap.fused_rupture_index ?? snap.fused_r)}">
                          R={((snap.fused_rupture_index ?? snap.fused_r ?? 0) * 10).toFixed(1)}
                        </span>
                      </div>
                    {/each}
                  </div>
                {/if}

              {:else if activeTab[key] === 'forecast'}
                {#if loadingDetail[key]}
                  <div class="detail-loading"><div class="spinner sm"></div> Loading forecast…</div>
                {:else if forecastData[key]?.note}
                  <!-- ITEM-017: Forecast calibrating empty state -->
                  <div class="detail-calibrating">
                    <div class="spinner sm"></div>
                    <p>{forecastData[key].note}</p>
                    <p class="hint">Forecasts become available after baseline calibration (~60 min).</p>
                  </div>
                {:else if forecastData[key]?.error}
                  <div class="detail-empty"><p>Forecast not available for this workload yet.</p></div>
                {:else if forecastData[key]?.points?.length}
                  <div class="forecast-list">
                    {#each forecastData[key].points as pt}
                      <div class="forecast-row">
                        <span class="forecast-offset">+{pt.offset_minutes}m</span>
                        <div class="signal-bar-track wide">
                          <div class="signal-bar-fill {signalClass('health_score', pt.mean)}"
                               style="width: {Math.min((pt.mean ?? 0) * 100, 100)}%"></div>
                        </div>
                        <span class="forecast-val">{pt.mean != null ? (pt.mean * 100).toFixed(0) + '%' : '—'}</span>
                      </div>
                    {/each}
                  </div>
                {:else}
                  <div class="detail-empty"><p>No forecast data available.</p></div>
                {/if}
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
.fleet-page { padding: 1.5rem; max-width: 1200px; margin: 0 auto; }
.fleet-header { display: flex; align-items: baseline; gap: 1rem; margin-bottom: 1.5rem; }
.fleet-header h1 { margin: 0; font-size: 1.5rem; font-weight: 700; }
.fleet-count { color: var(--text-secondary, var(--text-2)); font-size: 0.875rem; }

/* Loading / error */
.fleet-loading, .fleet-error { display: flex; flex-direction: column; align-items: center; gap: 1rem; padding: 4rem; color: var(--text-secondary, var(--text-2)); }
.spinner { width: 24px; height: 24px; border: 2px solid currentColor; border-top-color: transparent; border-radius: 50%; animation: spin 0.8s linear infinite; }
.spinner.sm { width: 16px; height: 16px; }
@keyframes spin { to { transform: rotate(360deg); } }

/* Empty state */
.empty-state { text-align: center; padding: 4rem 2rem; max-width: 480px; margin: 0 auto; }
.empty-icon { font-size: 3rem; margin-bottom: 1rem; }
.empty-state h2 { font-size: 1.25rem; font-weight: 600; margin: 0 0 0.75rem; }
.empty-state p { color: var(--text-secondary, var(--text-2)); margin: 0 0 0.5rem; line-height: 1.6; }
.empty-hint { font-size: 0.875rem; }
.empty-hint code { background: var(--surface, #1e2535); padding: 0.15em 0.4em; border-radius: 4px; font-size: 0.8em; }
.empty-actions { display: flex; gap: 1rem; justify-content: center; margin-top: 1.5rem; flex-wrap: wrap; }
.empty-actions a, .empty-actions button { padding: 0.5rem 1rem; border-radius: 6px; font-size: 0.875rem; cursor: pointer; text-decoration: none; }
.empty-actions a { background: var(--accent-cyan, #06b6d4); color: #000; font-weight: 600; }
.empty-actions button { background: var(--surface, #1e2535); color: var(--text-primary, #f1f5f9); border: 1px solid var(--border, rgba(255,255,255,0.06)); }

/* Grid */
.workload-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 1rem; }

/* Card */
.workload-card { background: var(--surface, #141824); border: 1px solid var(--border, rgba(255,255,255,0.06)); border-radius: 8px; overflow: hidden; transition: border-color 0.15s; }
.workload-card.healthy { border-left: 3px solid var(--green); }
.workload-card.degraded { border-left: 3px solid var(--amber); }
.workload-card.at-risk { border-left: 3px solid #f97316; }
.workload-card.critical { border-left: 3px solid var(--red); }
.workload-card.calibrating { border-left: 3px solid #6366f1; }
.workload-card.expanded { grid-column: 1 / -1; }

.card-header { display: flex; align-items: center; justify-content: space-between; width: 100%; padding: 0.875rem 1rem; background: none; border: none; cursor: pointer; text-align: left; color: inherit; gap: 0.5rem; }
.card-identity { display: flex; align-items: center; gap: 0.5rem; min-width: 0; flex: 1; }
.state-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.state-dot.healthy { background: var(--green); }
.state-dot.degraded { background: var(--amber); }
.state-dot.at-risk { background: #f97316; }
.state-dot.critical { background: var(--red); animation: pulse-red 1.5s ease-in-out infinite; }
.state-dot.calibrating { background: #6366f1; animation: pulse-indigo 2s ease-in-out infinite; }
@keyframes pulse-red { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }
@keyframes pulse-indigo { 0%,100% { opacity: 1; } 50% { opacity: 0.5; } }
.workload-name { font-weight: 600; font-size: 0.875rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.state-badge { font-size: 0.7rem; padding: 0.15em 0.5em; border-radius: 999px; font-weight: 500; }
.state-badge.healthy { background: rgba(34,197,94,0.15); color: var(--green); }
.state-badge.degraded { background: rgba(245,158,11,0.15); color: var(--amber); }
.state-badge.at-risk { background: rgba(249,115,22,0.15); color: #f97316; }
.state-badge.critical { background: rgba(239,68,68,0.15); color: var(--red); }
.state-badge.calibrating { background: rgba(99,102,241,0.15); color: #818cf8; }

.card-scores { display: flex; align-items: center; gap: 0.75rem; flex-shrink: 0; }
.score-block { display: flex; flex-direction: column; align-items: flex-end; }
.score-label { font-size: 0.65rem; color: var(--text-secondary, var(--text-2)); text-transform: uppercase; letter-spacing: 0.05em; }
.score-value { font-size: 1rem; font-weight: 700; font-variant-numeric: tabular-nums; }
.score-value.healthy { color: var(--green); }
.score-value.degraded { color: var(--amber); }
.score-value.at-risk { color: #f97316; }
.score-value.critical { color: var(--red); }
.calibrating-pill { font-size: 0.75rem; color: #818cf8; gap: 0.25rem; }
.calib-bar-track { width: 60px; height: 4px; background: rgba(99,102,241,0.2); border-radius: 2px; overflow: hidden; }
.calib-bar-fill { height: 100%; background: #818cf8; border-radius: 2px; transition: width 0.5s ease; }
.calib-pct { font-size: 0.65rem; color: #818cf8; font-variant-numeric: tabular-nums; }
.expand-icon { color: var(--text-secondary, var(--text-2)); font-size: 0.7rem; }

/* Signal bars */
.signal-bars { display: flex; flex-direction: column; gap: 0.3rem; padding: 0 1rem 0.75rem; }
.signal-bar-item { display: grid; grid-template-columns: 90px 1fr 42px; align-items: center; gap: 0.5rem; }
.signal-bar-label { font-size: 0.7rem; color: var(--text-secondary, var(--text-2)); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.signal-bar-track { height: 4px; background: var(--surface-high, #1e2535); border-radius: 2px; overflow: hidden; }
.signal-bar-track.wide { height: 6px; }
.signal-bar-fill { height: 100%; border-radius: 2px; transition: width 0.4s ease; }
.signal-bar-fill.healthy { background: var(--green); }
.signal-bar-fill.degraded { background: var(--amber); }
.signal-bar-fill.at-risk { background: #f97316; }
.signal-bar-fill.critical { background: var(--red); }
.signal-bar-fill.muted { background: var(--text-muted, #475569); }
.signal-bar-val { font-size: 0.7rem; font-variant-numeric: tabular-nums; text-align: right; color: var(--text-secondary, var(--text-2)); }

/* Expanded detail */
.card-detail { border-top: 1px solid var(--border, rgba(255,255,255,0.06)); padding: 1rem; }

/* FBL-A2-1: forecast-as-hero — the one sentence an SRE reads first */
.summary-hero { margin-bottom: 1rem; padding: 0.75rem 1rem; border-radius: 8px; background: var(--bg-2, rgba(255,255,255,0.03)); border-left: 3px solid var(--accent-cyan, #06b6d4); }
.summary-hero.warming { border-left-color: var(--amber); }
.summary-headline { margin: 0; font-size: 0.95rem; font-weight: 600; line-height: 1.4; }
.summary-action { margin: 0.35rem 0 0; font-size: 0.8rem; color: var(--text-secondary, var(--text-2)); }
.tab-bar { display: flex; gap: 0.25rem; margin-bottom: 1rem; }
.tab-btn { padding: 0.35rem 0.75rem; border-radius: 6px; border: 1px solid var(--border, rgba(255,255,255,0.06)); background: none; color: var(--text-secondary, var(--text-2)); font-size: 0.8rem; cursor: pointer; transition: all 0.15s; }
.tab-btn.active { background: var(--accent-cyan, #06b6d4); color: #000; border-color: transparent; font-weight: 600; }

.signals-detail { display: flex; flex-direction: column; gap: 0.5rem; }
.signal-row { display: grid; grid-template-columns: 120px 1fr 60px; align-items: center; gap: 0.75rem; }
.signal-name { font-size: 0.8rem; color: var(--text-secondary, var(--text-2)); }
.signal-val { font-size: 0.8rem; font-weight: 600; font-variant-numeric: tabular-nums; text-align: right; }
.signal-val.healthy { color: var(--green); }
.signal-val.degraded { color: var(--amber); }
.signal-val.at-risk { color: #f97316; }
.signal-val.critical { color: var(--red); }

.detail-loading { display: flex; align-items: center; gap: 0.75rem; padding: 1.5rem; color: var(--text-secondary, var(--text-2)); font-size: 0.875rem; }
.detail-empty { padding: 1.5rem; color: var(--text-secondary, var(--text-2)); font-size: 0.875rem; }
.detail-empty p { margin: 0 0 0.25rem; }
.detail-empty .hint { font-size: 0.8rem; color: var(--text-muted, #475569); }
.detail-calibrating { display: flex; flex-direction: column; align-items: center; gap: 0.5rem; padding: 2rem; text-align: center; color: var(--text-secondary, var(--text-2)); }
.detail-calibrating p { margin: 0; font-size: 0.875rem; }
.detail-calibrating .hint { font-size: 0.8rem; color: var(--text-muted, #475569); }

.history-list, .forecast-list { display: flex; flex-direction: column; gap: 0.35rem; }
.history-row, .forecast-row { display: grid; align-items: center; gap: 0.75rem; font-size: 0.8rem; font-variant-numeric: tabular-nums; }
.history-row { grid-template-columns: 70px 50px 60px; }
.history-time { color: var(--text-secondary, var(--text-2)); }
.history-score, .history-fri { font-weight: 600; }
.history-score.healthy { color: var(--green); }
.history-score.degraded { color: var(--amber); }
.history-score.critical { color: var(--red); }
.forecast-row { grid-template-columns: 50px 1fr 50px; }
.forecast-offset { color: var(--text-secondary, var(--text-2)); }
.forecast-val { font-weight: 600; text-align: right; }

/* Infra context bar */
.infra-bar {
  display: flex; align-items: center; flex-wrap: wrap; gap: 6px;
  padding: 8px 12px;
  border-top: 1px solid var(--border);
  background: var(--surface-3, #1C2540);
}
.infra-bar-label {
  font-size: 9px; font-weight: 700; text-transform: uppercase;
  letter-spacing: 0.12em; color: var(--text-3); margin-right: 4px;
}
.infra-chip {
  display: flex; align-items: center; gap: 4px;
  padding: 2px 7px; border-radius: 3px; border: 1px solid;
  background: var(--surface-2);
}
.infra-dot     { width: 5px; height: 5px; border-radius: 50%; flex-shrink: 0; }
.infra-chip-name { font-size: 9px; color: var(--text-2); }
.infra-chip-val  { font-family: var(--font-mono); font-size: 9px; font-variant-numeric: tabular-nums; }
</style>

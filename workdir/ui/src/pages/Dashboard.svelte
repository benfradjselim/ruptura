<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'

  let health = null
  let dataflow = null
  let fleet = null
  let infraGroups = []
  let alerts = []
  let loading = true
  let error = null
  let interval

  async function load() {
    try {
      const [hRes, dfRes, flRes, igRes, alRes] = await Promise.allSettled([
        api.health(),
        api.dataflow(),
        api.fleet(),
        api.infraGroups(),
        api.alerts(),
      ])
      if (hRes.status === 'fulfilled')  health      = hRes.value
      if (dfRes.status === 'fulfilled') dataflow    = dfRes.value
      if (flRes.status === 'fulfilled') fleet       = flRes.value
      if (igRes.status === 'fulfilled') infraGroups = igRes.value.groups || []
      if (alRes.status === 'fulfilled') alerts      = Array.isArray(alRes.value) ? alRes.value : []
      error = null
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  onMount(() => {
    load()
    interval = setInterval(load, 30_000)
  })

  onDestroy(() => clearInterval(interval))

  function fmtUptime(s) {
    if (s == null) return '—'
    const h = Math.floor(s / 3600)
    const m = Math.floor((s % 3600) / 60)
    if (h > 0) return `${h}h ${m}m`
    return `${m}m`
  }

  function fmtNum(n) {
    if (n == null) return '—'
    if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
    if (n >= 1_000)     return (n / 1_000).toFixed(1) + 'K'
    return String(n)
  }

  function healthStatusClass(s) {
    if (s === 'ready' || s === 'online') return 'ok'
    if (s === 'starting')                return 'warn'
    return 'err'
  }

  function hostStateClass(s) {
    if (s === 'healthy')   return 'ok'
    if (s === 'degraded')  return 'warn'
    if (s === 'critical')  return 'err'
    return 'dim'
  }

  function sevClass(sev) {
    if (!sev) return ''
    const s = sev.toLowerCase()
    if (s === 'critical' || s === 'emergency') return 'err'
    if (s === 'warning' || s === 'warn')       return 'warn'
    return 'ok'
  }

  $: activeAlerts     = alerts.filter(a => !a.acknowledged)
  $: namespaceCount   = new Set(infraGroups.map(g => g.namespace).filter(Boolean)).size
  $: agitatedGroups   = infraGroups.filter(g => g.agitated).length
  $: notableHosts     = (fleet?.hosts || []).filter(h => h.state === 'critical' || h.state === 'degraded').slice(0, 6)
</script>

<div class="page-wrap">

  <!-- ── Header ─────────────────────────────────────────────────────────────── -->
  <div class="band page-header">
    <div style="grid-column:1/9">
      <p class="kicker">Ruptura</p>
      <h1 class="page-title">Overview</h1>
    </div>
    <div style="grid-column:9/13; display:flex; align-items:center; justify-content:flex-end; gap:8px">
      {#if health}
        <span class="badge {healthStatusClass(health.status)}">{health.status}</span>
        <span class="badge dim">{health.edition || 'community'} · v{health.version || '—'}</span>
      {/if}
    </div>
  </div>

  {#if loading && !health}
    <div class="band"><p class="muted">Loading…</p></div>
  {:else if error}
    <div class="band"><p class="error-msg">{error}</p></div>
  {:else}

  <!-- ── Section 1: Engine health ──────────────────────────────────────────── -->
  <div class="band section-band">
    <div class="section-label" style="grid-column:1/13">
      <span class="kicker">Engine</span>
    </div>

    <!-- Status -->
    <div class="stat-card" style="grid-column:1/4">
      <p class="stat-label">Status</p>
      <p class="stat-value">
        <span class="dot {healthStatusClass(health?.status)}"></span>
        {health?.status || '—'}
      </p>
      <p class="stat-sub">{health?.message || ''}</p>
    </div>

    <!-- Uptime -->
    <div class="stat-card" style="grid-column:4/7">
      <p class="stat-label">Uptime</p>
      <p class="stat-value mono">{fmtUptime(health?.uptime_seconds)}</p>
      <p class="stat-sub">since last restart</p>
    </div>

    <!-- Rupture detection -->
    <div class="stat-card" style="grid-column:7/10">
      <p class="stat-label">Rupture detection</p>
      <p class="stat-value">
        <span class="dot {health?.rupture_detection === 'active' ? 'ok' : 'warn'}"></span>
        {health?.rupture_detection || '—'}
      </p>
      <p class="stat-sub">5-model ensemble</p>
    </div>

    <!-- Tracker count -->
    <div class="stat-card" style="grid-column:10/13">
      <p class="stat-label">Signal trackers</p>
      <p class="stat-value mono">{Object.keys(health?.trackers || {}).length}</p>
      <p class="stat-sub">active trackers</p>
    </div>
  </div>

  <!-- ── Section 2: Data ingest ─────────────────────────────────────────────── -->
  <div class="band section-band">
    <div class="section-label" style="grid-column:1/13">
      <span class="kicker">Data ingest</span>
    </div>

    <div class="stat-card" style="grid-column:1/5">
      <p class="stat-label">Metrics ingested</p>
      <p class="stat-value mono accent">{fmtNum(dataflow?.metrics)}</p>
      <p class="stat-sub">OTLP datapoints</p>
    </div>

    <div class="stat-card" style="grid-column:5/9">
      <p class="stat-label">Logs ingested</p>
      <p class="stat-value mono">{fmtNum(dataflow?.logs)}</p>
      <p class="stat-sub">log entries</p>
    </div>

    <div class="stat-card" style="grid-column:9/13">
      <p class="stat-label">Traces ingested</p>
      <p class="stat-value mono">{fmtNum(dataflow?.traces)}</p>
      <p class="stat-sub">spans</p>
    </div>
  </div>

  <!-- ── Section 3: Infra summary ──────────────────────────────────────────── -->
  <div class="band section-band">
    <div class="section-label" style="grid-column:1/13">
      <span class="kicker">Infrastructure</span>
    </div>

    <div class="stat-card" style="grid-column:1/4">
      <p class="stat-label">Namespaces</p>
      <p class="stat-value mono">{namespaceCount}</p>
      <p class="stat-sub">observed</p>
    </div>

    <div class="stat-card" style="grid-column:4/7">
      <p class="stat-label">Infra groups</p>
      <p class="stat-value mono">{infraGroups.length}</p>
      <p class="stat-sub">object groups</p>
    </div>

    <div class="stat-card" style="grid-column:7/10">
      <p class="stat-label">Agitated groups</p>
      <p class="stat-value mono {agitatedGroups > 0 ? 'text-warn' : ''}">{agitatedGroups}</p>
      <p class="stat-sub">above GNI threshold</p>
    </div>

    {#if infraGroups.length > 0}
    <div class="group-list" style="grid-column:1/13; margin-top:4px">
      {#each infraGroups.slice(0, 8) as g}
        <div class="group-row">
          <span class="group-name">{g.namespace}/{g.group}</span>
          <div class="health-bar-bg">
            <div class="health-bar-fill" style="width:{(g.health * 100).toFixed(0)}%; background:{g.health > 0.7 ? 'var(--green,#22C55E)' : g.health > 0.4 ? 'var(--amber,#F59E0B)' : 'var(--red,#EF4444)'}"></div>
          </div>
          <span class="group-pct mono">{(g.health * 100).toFixed(0)}%</span>
          {#if g.agitated}<span class="badge warn" style="font-size:9px;padding:1px 5px">agitated</span>{/if}
        </div>
      {/each}
    </div>
    {/if}
  </div>

  <!-- ── Section 4: Fleet summary ──────────────────────────────────────────── -->
  <div class="band section-band">
    <div class="section-label" style="grid-column:1/13">
      <span class="kicker">Fleet</span>
    </div>

    <div class="stat-card" style="grid-column:1/4">
      <p class="stat-label">Total workloads</p>
      <p class="stat-value mono">{fleet?.total_hosts ?? '—'}</p>
      <p class="stat-sub">monitored</p>
    </div>

    <div class="stat-card" style="grid-column:4/6">
      <p class="stat-label">Healthy</p>
      <p class="stat-value mono text-ok">{fleet?.healthy_hosts ?? '—'}</p>
      <p class="stat-sub">≥ 70 HS</p>
    </div>

    <div class="stat-card" style="grid-column:6/8">
      <p class="stat-label">Degraded</p>
      <p class="stat-value mono text-warn">{fleet?.degraded_hosts ?? '—'}</p>
      <p class="stat-sub">40–70 HS</p>
    </div>

    <div class="stat-card" style="grid-column:8/10">
      <p class="stat-label">Critical</p>
      <p class="stat-value mono text-err">{fleet?.critical_hosts ?? '—'}</p>
      <p class="stat-sub">&lt; 40 HS</p>
    </div>

    {#if notableHosts.length > 0}
    <div class="host-list" style="grid-column:1/13; margin-top:4px">
      <p class="list-hdr">Critical &amp; degraded workloads</p>
      {#each notableHosts as h}
        <div class="host-row">
          <span class="dot {hostStateClass(h.state)}"></span>
          <span class="host-name">{h.host}</span>
          <span class="host-hs mono">{h.health_score.toFixed(0)}</span>
          <span class="host-fri mono text-2">{h.fused_rupture_index?.toFixed(2) ?? '—'}</span>
          {#if h.calibration_progress < 100}
            <span class="badge dim" style="font-size:9px;padding:1px 5px">cal {h.calibration_progress}%</span>
          {/if}
        </div>
      {/each}
    </div>
    {:else if fleet && fleet.total_hosts > 0}
    <div style="grid-column:1/13; margin-top:4px">
      <p class="muted">All workloads healthy.</p>
    </div>
    {:else if fleet && fleet.total_hosts === 0}
    <div style="grid-column:1/13; margin-top:4px">
      <p class="muted">No workloads detected yet. Waiting for telemetry.</p>
    </div>
    {/if}
  </div>

  <!-- ── Section 5: Active alerts ──────────────────────────────────────────── -->
  <div class="band section-band">
    <div class="section-label" style="grid-column:1/13">
      <span class="kicker">Active alerts</span>
      <span class="badge {activeAlerts.length > 0 ? 'err' : 'dim'}" style="margin-left:8px">{activeAlerts.length}</span>
    </div>

    {#if activeAlerts.length === 0}
      <div style="grid-column:1/13">
        <p class="muted">No active alerts.</p>
      </div>
    {:else}
      <div class="alert-list" style="grid-column:1/13">
        {#each activeAlerts.slice(0, 10) as a}
          <div class="alert-row">
            <span class="badge {sevClass(a.severity)}">{a.severity || 'info'}</span>
            <span class="alert-host mono">{a.host}</span>
            <span class="alert-metric text-2">{a.metric}</span>
            <span class="alert-score mono text-2">{a.score?.toFixed(2) ?? '—'}</span>
          </div>
        {/each}
        {#if activeAlerts.length > 10}
          <p class="muted" style="margin-top:8px">+{activeAlerts.length - 10} more — see Alerts page</p>
        {/if}
      </div>
    {/if}
  </div>

  {/if}
</div>

<style>
  .page-wrap {
    padding-bottom: 48px;
  }

  /* ── Band / 12-col grid ── */
  .band {
    display: grid;
    grid-template-columns: repeat(12, 1fr);
    gap: 0 16px;
    padding: 20px 32px;
    border-bottom: 1px solid var(--border, rgba(148,163,184,0.10));
  }
  .page-header { padding: 24px 32px 20px; }

  .section-band { padding-top: 20px; }

  .section-label {
    display: flex;
    align-items: center;
    margin-bottom: 16px;
  }

  /* ── Stat cards ── */
  .stat-card {
    background: var(--surface, #1E293B);
    border: 1px solid var(--border, rgba(148,163,184,0.10));
    border-radius: 6px;
    padding: 16px;
    margin-bottom: 0;
  }

  .stat-label {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.10em;
    text-transform: uppercase;
    color: var(--text-3, #3F4D5C);
    margin-bottom: 8px;
  }

  .stat-value {
    font-size: 28px;
    font-weight: 700;
    color: var(--text, #E2E8F0);
    line-height: 1;
    margin-bottom: 6px;
    display: flex;
    align-items: center;
    gap: 8px;
    font-variant-numeric: tabular-nums;
  }

  .stat-sub {
    font-size: 11px;
    color: var(--text-3, #3F4D5C);
  }

  /* ── Badges ── */
  .badge {
    display: inline-flex;
    align-items: center;
    padding: 2px 8px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .badge.ok   { background: rgba(34,197,94,0.15);  color: var(--green,  #22C55E); }
  .badge.warn { background: rgba(245,158,11,0.15); color: var(--amber,  #F59E0B); }
  .badge.err  { background: rgba(239,68,68,0.15);  color: var(--red,    #EF4444); }
  .badge.dim  { background: rgba(148,163,184,0.10); color: var(--text-2, #94A3B8); }

  /* ── Status dots ── */
  .dot {
    width: 8px; height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
    display: inline-block;
  }
  .dot.ok   { background: var(--green, #22C55E); }
  .dot.warn { background: var(--amber, #F59E0B); }
  .dot.err  { background: var(--red,   #EF4444); }
  .dot.dim  { background: var(--text-3, #3F4D5C); }

  /* ── Typography utilities ── */
  .mono { font-family: "DM Mono", "Fira Code", monospace; font-variant-numeric: tabular-nums; }
  .muted { font-size: 13px; color: var(--text-3, #3F4D5C); }
  .text-ok   { color: var(--green, #22C55E); }
  .text-warn { color: var(--amber, #F59E0B); }
  .text-err  { color: var(--red,   #EF4444); }
  .text-2    { color: var(--text-2, #94A3B8); }
  .accent    { color: var(--accent, #38BDF8); }

  .error-msg {
    font-size: 13px;
    color: var(--red, #EF4444);
  }

  /* ── Infra group rows ── */
  .group-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .group-row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 12px;
    background: var(--surface, #1E293B);
    border: 1px solid var(--border, rgba(148,163,184,0.10));
    border-radius: 4px;
  }
  .group-name {
    flex: 1;
    font-size: 12px;
    font-family: "DM Mono", "Fira Code", monospace;
    color: var(--text, #E2E8F0);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .health-bar-bg {
    width: 120px;
    height: 4px;
    background: rgba(148,163,184,0.12);
    border-radius: 2px;
    flex-shrink: 0;
    overflow: hidden;
  }
  .health-bar-fill {
    height: 100%;
    border-radius: 2px;
    transition: width 0.3s ease;
  }
  .group-pct {
    font-size: 12px;
    color: var(--text-2, #94A3B8);
    width: 36px;
    text-align: right;
    flex-shrink: 0;
  }

  /* ── Host list ── */
  .list-hdr {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-3, #3F4D5C);
    margin-bottom: 8px;
  }
  .host-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .host-row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    background: var(--surface, #1E293B);
    border: 1px solid var(--border, rgba(148,163,184,0.10));
    border-radius: 4px;
  }
  .host-name {
    flex: 1;
    font-size: 12px;
    font-family: "DM Mono", "Fira Code", monospace;
    color: var(--text, #E2E8F0);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .host-hs {
    font-size: 13px;
    font-weight: 700;
    color: var(--text, #E2E8F0);
    width: 32px;
    text-align: right;
    flex-shrink: 0;
  }
  .host-fri {
    font-size: 11px;
    width: 40px;
    text-align: right;
    flex-shrink: 0;
  }

  /* ── Alert list ── */
  .alert-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .alert-row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    background: var(--surface, #1E293B);
    border: 1px solid var(--border, rgba(148,163,184,0.10));
    border-radius: 4px;
  }
  .alert-host {
    flex: 1;
    font-size: 12px;
    color: var(--text, #E2E8F0);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .alert-metric {
    font-size: 11px;
    color: var(--text-2, #94A3B8);
    white-space: nowrap;
    flex-shrink: 0;
  }
  .alert-score {
    font-size: 11px;
    width: 40px;
    text-align: right;
    flex-shrink: 0;
  }

  .kicker {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--text-3, #3F4D5C);
    margin-bottom: 4px;
  }

  .page-title {
    font-size: 22px;
    font-weight: 700;
    color: var(--text, #E2E8F0);
    letter-spacing: -0.01em;
  }
</style>

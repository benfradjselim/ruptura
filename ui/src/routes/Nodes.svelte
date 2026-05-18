<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { fetchNodes, fetchFleet } from '../lib/api'
  import type { ClusterNode, FleetHost } from '../lib/api'

  let nodes: ClusterNode[] = []
  let hosts: FleetHost[] = []
  let fleetTotal = 0
  let fleetHealthy = 0
  let fleetDegraded = 0
  let fleetCritical = 0

  let error = ''
  let loading = true
  let refreshTimer: ReturnType<typeof setInterval>
  let lastRefresh = ''

  // sort / filter state
  let sortField: 'fused_rupture_index' | 'health_score' | 'stress' | 'fatigue' | 'contagion' = 'fused_rupture_index'
  let sortDir: 1 | -1 = -1  // -1 = desc
  let filterState: 'all' | 'degraded' | 'critical' = 'all'

  async function load() {
    try {
      const [nodeList, fleet] = await Promise.all([fetchNodes(), fetchFleet()])
      nodes = nodeList
      hosts = fleet.hosts
      fleetTotal = fleet.total_hosts
      fleetHealthy = fleet.healthy_hosts
      fleetDegraded = fleet.degraded_hosts
      fleetCritical = fleet.critical_hosts
      error = ''
      lastRefresh = new Date().toLocaleTimeString()
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }

  onMount(() => {
    load()
    refreshTimer = setInterval(load, 15_000)
  })
  onDestroy(() => clearInterval(refreshTimer))

  // ── derived ───────────────────────────────────────────────────────────────────
  $: sortedHosts = [...hosts]
    .filter(h => {
      if (filterState === 'critical') return h.state === 'critical'
      if (filterState === 'degraded') return h.state === 'degraded' || h.state === 'critical'
      return true
    })
    .sort((a, b) => {
      const av = a[sortField] as number
      const bv = b[sortField] as number
      return (av - bv) * sortDir
    })

  $: degradingCount = hosts.filter(h =>
    h.health_forecast?.trend === 'degrading' && h.state !== 'critical'
  ).length

  $: criticalEtaHosts = hosts.filter(h =>
    h.health_forecast?.critical_eta_minutes != null &&
    h.health_forecast.critical_eta_minutes > 0 &&
    h.health_forecast.critical_eta_minutes < 60
  ).sort((a, b) =>
    (a.health_forecast!.critical_eta_minutes) - (b.health_forecast!.critical_eta_minutes)
  )

  // ── helpers ───────────────────────────────────────────────────────────────────
  function healthColor(score: number, state: string): string {
    if (state === 'pending_telemetry' || state === 'calibrating') return 'var(--muted)'
    if (score >= 75) return 'var(--green)'
    if (score >= 50) return 'var(--yellow)'
    if (score >= 25) return 'var(--orange)'
    return 'var(--red)'
  }

  function frColor(v: number): string {
    if (v >= 0.7) return 'var(--red)'
    if (v >= 0.4) return 'var(--orange)'
    if (v >= 0.2) return 'var(--yellow)'
    return 'var(--green)'
  }

  function pctColor(v: number): string {
    if (v >= 85) return 'var(--red)'
    if (v >= 65) return 'var(--yellow)'
    return 'var(--text)'
  }

  function workloadName(host: string): string {
    const p = host.split('/')
    return p[p.length - 1] || host
  }

  function trendIcon(trend?: string): string {
    if (trend === 'degrading') return '↓'
    if (trend === 'improving') return '↑'
    return '→'
  }

  function trendColor(trend?: string): string {
    if (trend === 'degrading') return 'var(--red)'
    if (trend === 'improving') return 'var(--green)'
    return 'var(--muted)'
  }

  function stateClass(s: string): string {
    if (s === 'critical') return 'st-crit'
    if (s === 'degraded') return 'st-deg'
    if (s === 'healthy') return 'st-ok'
    if (s === 'calibrating') return 'st-cal'
    return 'st-pend'
  }

  function cycleSort(field: typeof sortField) {
    if (sortField === field) { sortDir = sortDir === -1 ? 1 : -1 }
    else { sortField = field; sortDir = -1 }
  }

  function sortArrow(field: typeof sortField): string {
    if (sortField !== field) return ''
    return sortDir === -1 ? ' ↓' : ' ↑'
  }

  function etaLabel(min: number): string {
    if (min < 5) return `< 5 min`
    if (min < 60) return `~${Math.round(min)} min`
    return `> 1h`
  }
</script>

<div class="cluster-page">

  <!-- ── PREDICTION BANNER ─────────────────────────────────────────────────────── -->
  {#if criticalEtaHosts.length > 0}
    <div class="prediction-banner">
      <span class="pred-icon">⚠</span>
      <span class="pred-label">Ruptura Forecast</span>
      <div class="pred-items">
        {#each criticalEtaHosts.slice(0, 4) as h}
          <span class="pred-item">
            <b>{workloadName(h.host)}</b>
            → critical in <b style="color:var(--red)">{etaLabel(h.health_forecast?.critical_eta_minutes ?? 0)}</b>
          </span>
        {/each}
        {#if criticalEtaHosts.length > 4}
          <span class="pred-more">+{criticalEtaHosts.length - 4} more</span>
        {/if}
      </div>
    </div>
  {/if}

  {#if degradingCount > 0}
    <div class="degrade-banner">
      <span class="dg-icon">↓</span>
      <b>{degradingCount} workload{degradingCount > 1 ? 's' : ''}</b> trending degraded —
      health forecast shows decline over the next 30 min.
    </div>
  {/if}

  <!-- ── CLUSTER SUMMARY ───────────────────────────────────────────────────────── -->
  {#if !loading}
    <div class="summary-row">
      <div class="sum-card">
        <div class="sum-val">{fleetTotal}</div>
        <div class="sum-label">Total workloads</div>
      </div>
      <div class="sum-card ok">
        <div class="sum-val" style="color:var(--green)">{fleetHealthy}</div>
        <div class="sum-label">Healthy</div>
      </div>
      <div class="sum-card warn">
        <div class="sum-val" style="color:var(--yellow)">{fleetDegraded}</div>
        <div class="sum-label">Degraded</div>
      </div>
      <div class="sum-card crit">
        <div class="sum-val" style="color:var(--red)">{fleetCritical}</div>
        <div class="sum-label">Critical</div>
      </div>
      <div class="sum-card">
        <div class="sum-val" style="color:var(--orange)">{degradingCount}</div>
        <div class="sum-label">Forecast: degrading</div>
      </div>
      <div class="sum-card">
        <div class="sum-val">{nodes.length}</div>
        <div class="sum-label">Cluster nodes</div>
      </div>
    </div>
  {/if}

  <!-- ── NODE HEALTH GRID ──────────────────────────────────────────────────────── -->
  {#if nodes.length > 0}
    <div class="section-header">
      <span class="section-title">Node Health</span>
    </div>
    <div class="node-grid">
      {#each nodes as n}
        {@const worstColor = frColor(n.worst_fused_r)}
        <div class="node-card" style="border-color:{n.disk_pressure ? 'var(--red)' : 'var(--border)'}">
          <div class="node-name">{n.name}</div>
          <div class="node-stats">
            <div class="ns-item">
              <span class="ns-label">CPU</span>
              <span class="ns-val" style="color:{pctColor(n.cpu_pct)}">{n.cpu_pct.toFixed(1)}%</span>
              <div class="ns-bar"><div class="ns-fill" style="width:{Math.min(n.cpu_pct,100)}%;background:{pctColor(n.cpu_pct)}"></div></div>
            </div>
            <div class="ns-item">
              <span class="ns-label">Memory</span>
              <span class="ns-val" style="color:{pctColor(n.memory_pct)}">{n.memory_pct.toFixed(1)}%</span>
              <div class="ns-bar"><div class="ns-fill" style="width:{Math.min(n.memory_pct,100)}%;background:{pctColor(n.memory_pct)}"></div></div>
            </div>
          </div>
          <div class="node-footer">
            <span class="node-wl">{n.workload_count} workloads</span>
            <span class="node-fr" style="color:{worstColor}">Fused-R {n.worst_fused_r.toFixed(3)}</span>
            {#if n.disk_pressure}<span class="disk-badge">⚠ disk</span>{/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <!-- ── WORKLOAD RISK TABLE ───────────────────────────────────────────────────── -->
  <div class="section-header">
    <span class="section-title">Workload Risk — Fused Signal Analysis</span>
    <div class="filter-pills">
      <button class="pill" class:active={filterState==='all'}     on:click={() => filterState='all'}>All</button>
      <button class="pill" class:active={filterState==='degraded'} on:click={() => filterState='degraded'}>Degraded+</button>
      <button class="pill" class:active={filterState==='critical'} on:click={() => filterState='critical'}>Critical</button>
    </div>
    {#if lastRefresh}
      <span class="refresh-ts">Updated {lastRefresh}</span>
    {/if}
  </div>

  {#if loading}
    <div class="loading">Loading…</div>
  {:else if error}
    <div class="err-banner">{error}</div>
  {:else if sortedHosts.length === 0}
    <div class="placeholder">
      {#if filterState !== 'all'}
        <p>No workloads in this state. <button class="link-btn" on:click={() => filterState='all'}>Show all</button></p>
      {:else}
        <div class="icon">◫</div>
        <div class="title">No workloads tracked yet</div>
        <p>Deploy workloads and configure a Prometheus datasource. Ruptura will begin tracking KPIs automatically.</p>
      {/if}
    </div>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th class="col-name">Workload</th>
            <th class="col-state">State</th>
            <th class="col-hs" on:click={() => cycleSort('health_score')} role="button" tabindex="0" on:keydown={e => e.key==='Enter' && cycleSort('health_score')}>
              Health{sortArrow('health_score')}
            </th>
            <th on:click={() => cycleSort('fused_rupture_index')} role="button" tabindex="0" on:keydown={e => e.key==='Enter' && cycleSort('fused_rupture_index')}>
              Fused-R{sortArrow('fused_rupture_index')}
            </th>
            <th on:click={() => cycleSort('stress')} role="button" tabindex="0" on:keydown={e => e.key==='Enter' && cycleSort('stress')}>
              Stress{sortArrow('stress')}
            </th>
            <th on:click={() => cycleSort('fatigue')} role="button" tabindex="0" on:keydown={e => e.key==='Enter' && cycleSort('fatigue')}>
              Fatigue{sortArrow('fatigue')}
            </th>
            <th on:click={() => cycleSort('contagion')} role="button" tabindex="0" on:keydown={e => e.key==='Enter' && cycleSort('contagion')}>
              Contagion{sortArrow('contagion')}
            </th>
            <th class="col-forecast">Forecast (15m / 30m)</th>
            <th class="col-alerts">Alerts</th>
          </tr>
        </thead>
        <tbody>
          {#each sortedHosts as h}
            {@const hcol = healthColor(h.health_score, h.state)}
            {@const forecast = h.health_forecast}
            {@const f15 = forecast?.in_15min ?? h.health_score}
            {@const f30 = forecast?.in_30min ?? h.health_score}
            {@const trend = forecast?.trend}
            <tr class="wl-row" class:row-crit={h.state==='critical'} class:row-deg={h.state==='degraded'}>
              <td class="col-name">
                <div class="wl-name">{workloadName(h.host)}</div>
                <div class="wl-host">{h.host.split('/').slice(0,2).join('/')}</div>
              </td>
              <td>
                <span class="state-chip {stateClass(h.state)}">{h.state.replace(/_/g,' ')}</span>
              </td>
              <td>
                <div class="hs-cell">
                  <span style="color:{hcol};font-weight:700">{Math.round(h.health_score)}</span>
                  <div class="hs-bar"><div class="hs-fill" style="width:{h.health_score}%;background:{hcol}"></div></div>
                </div>
              </td>
              <td style="color:{frColor(h.fused_rupture_index)};font-weight:600;font-variant-numeric:tabular-nums">
                {h.fused_rupture_index.toFixed(3)}
              </td>
              <td>
                <div class="sig-bar"><div class="sig-fill" style="width:{h.stress*100}%;background:{h.stress>0.6?'var(--red)':h.stress>0.3?'var(--yellow)':'var(--green)'}"></div></div>
                <span class="sig-val">{(h.stress*100).toFixed(0)}%</span>
              </td>
              <td>
                <div class="sig-bar"><div class="sig-fill" style="width:{h.fatigue*100}%;background:{h.fatigue>0.5?'var(--red)':h.fatigue>0.25?'var(--yellow)':'var(--green)'}"></div></div>
                <span class="sig-val">{(h.fatigue*100).toFixed(0)}%</span>
              </td>
              <td>
                <div class="sig-bar"><div class="sig-fill" style="width:{h.contagion*100}%;background:{h.contagion>0.4?'var(--red)':h.contagion>0.2?'var(--yellow)':'var(--green)'}"></div></div>
                <span class="sig-val">{(h.contagion*100).toFixed(0)}%</span>
              </td>
              <td class="col-forecast">
                {#if forecast && h.state !== 'pending_telemetry' && h.state !== 'calibrating'}
                  <div class="forecast-cell">
                    <span class="trend-icon" style="color:{trendColor(trend)}">{trendIcon(trend)}</span>
                    <span class="f15" style="color:{healthColor(f15,'active')}" title="in 15 min">{Math.round(f15)}</span>
                    <span class="f-sep">/</span>
                    <span class="f30" style="color:{healthColor(f30,'active')}" title="in 30 min">{Math.round(f30)}</span>
                    {#if forecast.critical_eta_minutes > 0 && forecast.critical_eta_minutes < 60}
                      <span class="eta-badge">⚠ {etaLabel(forecast.critical_eta_minutes)}</span>
                    {/if}
                  </div>
                {:else}
                  <span class="muted-text">—</span>
                {/if}
              </td>
              <td class="col-alerts">
                {#if h.active_alerts > 0}
                  <span class="alert-badge">{h.active_alerts}</span>
                {:else}
                  <span class="muted-text">—</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

</div>

<style>
  .cluster-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  /* ── banners ── */
  .prediction-banner {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 10px 14px;
    background: rgba(239,68,68,0.08);
    border: 1px solid rgba(239,68,68,0.35);
    border-radius: 8px;
    font-size: 12px;
    color: var(--text);
    flex-wrap: wrap;
  }
  .pred-icon { color: var(--red); font-size: 14px; flex-shrink: 0; }
  .pred-label { font-weight: 700; color: var(--red); white-space: nowrap; flex-shrink: 0; }
  .pred-items { display: flex; gap: 14px; flex-wrap: wrap; flex: 1; }
  .pred-item { font-size: 11px; color: var(--muted); }
  .pred-item b { color: var(--text); }
  .pred-more { font-size: 11px; color: var(--muted); }

  .degrade-banner {
    padding: 8px 14px;
    background: rgba(249,115,22,0.07);
    border: 1px solid rgba(249,115,22,0.3);
    border-radius: 8px;
    font-size: 12px;
    color: var(--muted);
  }
  .dg-icon { color: var(--orange); margin-right: 6px; }
  .degrade-banner b { color: var(--orange); }

  /* ── summary cards ── */
  .summary-row {
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
  }
  .sum-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px 18px;
    min-width: 100px;
    flex: 1;
  }
  .sum-card.ok   { border-color: rgba(0,229,160,0.2); }
  .sum-card.warn { border-color: rgba(245,158,11,0.2); }
  .sum-card.crit { border-color: rgba(239,68,68,0.2); }
  .sum-val { font-size: 24px; font-weight: 700; font-variant-numeric: tabular-nums; line-height: 1.2; }
  .sum-label { font-size: 10px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.06em; margin-top: 2px; }

  /* ── section header ── */
  .section-header {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }
  .section-title { font-size: 13px; font-weight: 700; color: var(--text); flex: 1; }
  .refresh-ts { font-size: 10px; color: var(--muted); margin-left: auto; }

  .filter-pills { display: flex; gap: 5px; }
  .pill {
    background: none;
    border: 1px solid var(--border);
    color: var(--muted);
    font-size: 10px;
    padding: 2px 10px;
    border-radius: 10px;
    cursor: pointer;
    transition: all 0.15s;
  }
  .pill.active { background: var(--surface2); color: var(--text); border-color: var(--accent); }
  .pill:hover:not(.active) { color: var(--text); }

  /* ── node grid ── */
  .node-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 10px;
  }
  .node-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .node-name { font-size: 12px; font-weight: 700; font-family: monospace; color: var(--text); }
  .node-stats { display: flex; flex-direction: column; gap: 4px; }
  .ns-item { display: flex; align-items: center; gap: 6px; }
  .ns-label { font-size: 9px; color: var(--muted); width: 40px; text-transform: uppercase; flex-shrink: 0; }
  .ns-val { font-size: 11px; font-variant-numeric: tabular-nums; font-weight: 600; width: 38px; text-align: right; flex-shrink: 0; }
  .ns-bar { flex: 1; height: 3px; background: var(--surface3); border-radius: 2px; overflow: hidden; }
  .ns-fill { height: 100%; border-radius: 2px; transition: width 0.4s; }
  .node-footer {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 10px;
    flex-wrap: wrap;
    border-top: 1px solid var(--border);
    padding-top: 6px;
  }
  .node-wl { color: var(--muted); }
  .node-fr { font-variant-numeric: tabular-nums; font-weight: 600; margin-left: auto; }
  .disk-badge { color: var(--red); font-weight: 700; }

  /* ── table ── */
  .table-wrap {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow-x: auto;
  }

  table { width: 100%; border-collapse: collapse; font-size: 12px; }

  th {
    text-align: left;
    padding: 10px 12px;
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    border-bottom: 1px solid var(--border);
    background: var(--surface2);
    white-space: nowrap;
    user-select: none;
  }

  th[role="button"] { cursor: pointer; }
  th[role="button"]:hover { color: var(--text); }

  td {
    padding: 8px 12px;
    border-bottom: 1px solid rgba(30,45,69,0.5);
    vertical-align: middle;
  }

  tr:last-child td { border-bottom: none; }

  .wl-row:hover td { background: rgba(255,255,255,0.02); }
  .row-crit td:first-child { border-left: 2px solid var(--red); }
  .row-deg  td:first-child { border-left: 2px solid var(--yellow); }

  .col-name { min-width: 160px; }
  .col-forecast { min-width: 160px; }
  .col-alerts { text-align: center; }
  .col-hs { min-width: 100px; }

  .wl-name { font-weight: 600; color: var(--text); }
  .wl-host { font-size: 9px; color: var(--muted); font-family: monospace; margin-top: 1px; }

  /* state chips */
  .state-chip {
    display: inline-block;
    padding: 2px 7px;
    border-radius: 8px;
    font-size: 9px;
    font-weight: 700;
    text-transform: capitalize;
    letter-spacing: 0.02em;
    white-space: nowrap;
  }
  .st-crit { background: rgba(239,68,68,0.15);  color: var(--red); }
  .st-deg  { background: rgba(245,158,11,0.15); color: var(--yellow); }
  .st-ok   { background: rgba(0,229,160,0.12);  color: var(--green); }
  .st-cal  { background: rgba(85,96,128,0.2);   color: var(--muted); }
  .st-pend { background: rgba(85,96,128,0.12);  color: var(--muted); }

  /* health score bar */
  .hs-cell { display: flex; flex-direction: column; gap: 3px; min-width: 80px; }
  .hs-bar { height: 3px; background: var(--surface3); border-radius: 2px; overflow: hidden; }
  .hs-fill { height: 100%; border-radius: 2px; transition: width 0.4s; }

  /* signal bars */
  .sig-bar { display: inline-block; width: 48px; height: 3px; background: var(--surface3); border-radius: 2px; overflow: hidden; vertical-align: middle; margin-right: 4px; }
  .sig-fill { height: 100%; border-radius: 2px; transition: width 0.4s; }
  .sig-val { font-size: 10px; font-variant-numeric: tabular-nums; color: var(--muted); }

  /* forecast */
  .forecast-cell { display: flex; align-items: center; gap: 5px; flex-wrap: wrap; }
  .trend-icon { font-size: 14px; font-weight: 700; }
  .f15, .f30 { font-size: 12px; font-weight: 700; font-variant-numeric: tabular-nums; }
  .f-sep { color: var(--muted); font-size: 10px; }
  .eta-badge {
    font-size: 9px;
    padding: 1px 5px;
    border-radius: 4px;
    background: rgba(239,68,68,0.12);
    color: var(--red);
    font-weight: 700;
    white-space: nowrap;
  }

  .alert-badge {
    display: inline-block;
    background: rgba(239,68,68,0.15);
    color: var(--red);
    font-size: 11px;
    font-weight: 700;
    padding: 2px 8px;
    border-radius: 8px;
  }

  .muted-text { color: var(--muted); font-size: 10px; }

  /* misc */
  .loading { color: var(--muted); text-align: center; padding: 40px; }
  .err-banner {
    padding: 10px 14px;
    border-radius: 6px;
    font-size: 12px;
    color: var(--red);
    background: rgba(224,82,82,0.08);
    border: 1px solid var(--red);
  }

  .placeholder {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    min-height: 30vh;
    text-align: center;
    color: var(--muted);
    justify-content: center;
  }
  .icon { font-size: 48px; opacity: 0.3; }
  .title { font-size: 18px; font-weight: 700; color: var(--text); }
  p { max-width: 380px; line-height: 1.8; font-size: 13px; }
  .link-btn {
    background: none;
    border: none;
    color: var(--accent);
    cursor: pointer;
    font-size: 13px;
    text-decoration: underline;
  }
</style>

<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'

  let fleet = null
  let error = null
  let loading = true
  let interval

  async function load() {
    try {
      const res = await api.fleet()
      fleet = res.data
      error = null
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  onMount(() => {
    load()
    interval = setInterval(load, 15000)
  })
  onDestroy(() => clearInterval(interval))

  function stateClass(state) {
    if (state === 'healthy') return 'healthy'
    if (state === 'degraded') return 'degraded'
    return 'critical'
  }

  function scoreColor(score) {
    if (score >= 80) return '#22c55e'
    if (score >= 60) return '#eab308'
    if (score >= 40) return '#f97316'
    return '#ef4444'
  }

  function bar(value, max = 100) {
    return Math.min(100, (value / max) * 100).toFixed(1)
  }
</script>

<div class="page">
  <div class="page-header">
    <h1>Fleet Overview</h1>
    <p class="subtitle">All monitored hosts — live health matrix</p>
  </div>

  {#if loading}
    <div class="loading">Loading fleet data...</div>
  {:else if error}
    <div class="error">{error}</div>
  {:else if fleet}
    <!-- Summary Cards -->
    <div class="summary-row">
      <div class="summary-card total">
        <div class="s-num">{fleet.total_hosts}</div>
        <div class="s-label">Total Hosts</div>
      </div>
      <div class="summary-card healthy">
        <div class="s-num">{fleet.healthy_hosts}</div>
        <div class="s-label">Healthy</div>
      </div>
      <div class="summary-card degraded">
        <div class="s-num">{fleet.degraded_hosts}</div>
        <div class="s-label">Degraded</div>
      </div>
      <div class="summary-card critical">
        <div class="s-num">{fleet.critical_hosts}</div>
        <div class="s-label">Critical</div>
      </div>
    </div>

    <!-- Host Grid -->
    {#if fleet.hosts && fleet.hosts.length > 0}
      <div class="host-grid">
        {#each fleet.hosts as host}
          <div class="host-card {stateClass(host.state)}">
            <div class="host-header">
              <span class="host-name">{host.host}</span>
              <span class="state-badge {stateClass(host.state)}">{host.state}</span>
            </div>

            <div class="health-score-row">
              <span class="hs-label">Health Score</span>
              <span class="hs-value" style="color:{scoreColor(host.health_score)}">{host.health_score.toFixed(1)}</span>
            </div>
            <div class="score-bar">
              <div class="score-fill" style="width:{bar(host.health_score)}%;background:{scoreColor(host.health_score)}"></div>
            </div>

            <div class="kpi-mini-row">
              <div class="kpi-mini">
                <span class="kpi-mini-label">Stress</span>
                <span class="kpi-mini-val" style="color:{scoreColor(100 - host.stress*100)}">{(host.stress*100).toFixed(0)}%</span>
              </div>
              <div class="kpi-mini">
                <span class="kpi-mini-label">Fatigue</span>
                <span class="kpi-mini-val" style="color:{scoreColor(100 - host.fatigue*100)}">{(host.fatigue*100).toFixed(0)}%</span>
              </div>
              <div class="kpi-mini">
                <span class="kpi-mini-label">Contagion</span>
                <span class="kpi-mini-val" style="color:{scoreColor(100 - host.contagion*100)}">{(host.contagion*100).toFixed(0)}%</span>
              </div>
              <div class="kpi-mini">
                <span class="kpi-mini-label">Alerts</span>
                <span class="kpi-mini-val" class:alert-count={host.active_alerts > 0}>{host.active_alerts}</span>
              </div>
            </div>

            <div class="last-seen">
              Last seen: {new Date(host.last_seen).toLocaleTimeString()}
            </div>
          </div>
        {/each}
      </div>
    {:else}
      <div class="empty">No hosts reporting yet. Deploy OHE agents to monitored nodes.</div>
    {/if}
  {/if}
</div>

<style>
  .page { max-width: 1400px; }
  .page-header { margin-bottom: 1.5rem; }
  h1 { font-size: 1.5rem; font-weight: 700; color: #f1f5f9; margin-bottom: 0.25rem; }
  .subtitle { color: #64748b; font-size: 0.9rem; }

  .loading, .error, .empty { padding: 2rem; text-align: center; color: #64748b; }
  .error { color: #ef4444; }

  .summary-row {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1rem;
    margin-bottom: 1.5rem;
  }
  .summary-card {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 8px;
    padding: 1.25rem;
    text-align: center;
  }
  .summary-card.healthy { border-color: #22c55e40; }
  .summary-card.degraded { border-color: #eab30840; }
  .summary-card.critical { border-color: #ef444440; }
  .s-num { font-size: 2rem; font-weight: 800; color: #f1f5f9; }
  .summary-card.healthy .s-num { color: #22c55e; }
  .summary-card.degraded .s-num { color: #eab308; }
  .summary-card.critical .s-num { color: #ef4444; }
  .s-label { font-size: 0.8rem; color: #64748b; margin-top: 0.25rem; text-transform: uppercase; letter-spacing: 0.05em; }

  .host-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 1rem;
  }
  .host-card {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 10px;
    padding: 1.25rem;
    transition: border-color 0.15s;
  }
  .host-card.healthy { border-left: 3px solid #22c55e; }
  .host-card.degraded { border-left: 3px solid #eab308; }
  .host-card.critical { border-left: 3px solid #ef4444; }

  .host-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
  }
  .host-name { font-weight: 700; color: #f1f5f9; font-size: 1rem; }
  .state-badge {
    font-size: 0.7rem;
    font-weight: 600;
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .state-badge.healthy { background: #22c55e20; color: #22c55e; }
  .state-badge.degraded { background: #eab30820; color: #eab308; }
  .state-badge.critical { background: #ef444420; color: #ef4444; }

  .health-score-row {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 0.3rem;
  }
  .hs-label { font-size: 0.75rem; color: #94a3b8; }
  .hs-value { font-size: 1.4rem; font-weight: 700; }

  .score-bar {
    height: 6px;
    background: #334155;
    border-radius: 3px;
    margin-bottom: 0.85rem;
    overflow: hidden;
  }
  .score-fill { height: 100%; border-radius: 3px; transition: width 0.5s ease; }

  .kpi-mini-row {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 0.5rem;
    margin-bottom: 0.75rem;
  }
  .kpi-mini { text-align: center; }
  .kpi-mini-label { display: block; font-size: 0.65rem; color: #475569; text-transform: uppercase; letter-spacing: 0.04em; margin-bottom: 2px; }
  .kpi-mini-val { font-size: 0.9rem; font-weight: 700; color: #94a3b8; }
  .kpi-mini-val.alert-count { color: #ef4444; }

  .last-seen { font-size: 0.7rem; color: #475569; }
</style>

<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'
  import { token, kpis, alerts, wsConnected } from '../lib/store.js'
  import KpiGauge from '../lib/KpiGauge.svelte'
  import Sparkline from '../lib/Sparkline.svelte'

  let host = ''
  let wsClient = null
  let liveKpis = {}
  let metricHistory = {}   // metric -> array of values (last 20)
  let recentAlerts = []
  let pollTimer = null

  const KPI_NAMES = ['stress', 'fatigue', 'mood', 'pressure', 'humidity', 'contagion']

  async function loadKPIs() {
    try {
      const r = await api.kpis(host)
      liveKpis = r.data || {}
    } catch {}
  }

  async function loadAlerts() {
    try {
      const r = await api.alerts()
      recentAlerts = (r.data || []).slice(0, 10)
    } catch {}
  }

  function connectWS() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const tok = $token || ''
    wsClient = new WebSocket(`${proto}://${location.host}/api/v1/ws?token=${encodeURIComponent(tok)}`)
    wsClient.onopen = () => wsConnected.set(true)
    wsClient.onclose = () => {
      wsConnected.set(false)
      setTimeout(connectWS, 3000)
    }
    wsClient.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data)
        if (msg.type === 'kpi' && msg.payload) {
          liveKpis = { ...liveKpis, ...msg.payload }
        }
        if (msg.type === 'alert' && msg.payload) {
          recentAlerts = [msg.payload, ...recentAlerts].slice(0, 10)
        }
        if (msg.type === 'metric' && msg.payload) {
          const { name, value } = msg.payload
          const hist = metricHistory[name] || []
          metricHistory = { ...metricHistory, [name]: [...hist, value].slice(-20) }
        }
      } catch {}
    }
  }

  onMount(() => {
    loadKPIs()
    loadAlerts()
    connectWS()
    pollTimer = setInterval(() => { loadKPIs(); loadAlerts() }, 15000)
  })

  onDestroy(() => {
    if (wsClient) wsClient.close()
    clearInterval(pollTimer)
  })

  async function ackAlert(id) {
    await api.alertAck(id).catch(() => {})
    loadAlerts()
  }
</script>

<div class="dashboard">
  <!-- KPI strip -->
  <section class="kpi-strip card">
    <h2>Holistic KPIs
      <span class="ws-dot" class:connected={$wsConnected} title={$wsConnected ? 'Live' : 'Polling'}></span>
    </h2>
    <div class="gauges">
      {#each KPI_NAMES as name}
        {@const kpi = liveKpis[name] || {}}
        <KpiGauge {name} value={kpi.value ?? 0} state={kpi.state ?? ''} />
      {/each}
    </div>
  </section>

  <!-- Metric sparklines -->
  {#if Object.keys(metricHistory).length > 0}
    <section class="sparklines card">
      <h2>Live Metrics</h2>
      <div class="sparks">
        {#each Object.entries(metricHistory) as [name, vals]}
          <div class="spark-item">
            <span class="spark-name">{name}</span>
            <Sparkline data={vals} />
            <span class="spark-val">{vals[vals.length-1]?.toFixed(1)}%</span>
          </div>
        {/each}
      </div>
    </section>
  {/if}

  <!-- Recent alerts -->
  <section class="alerts-section card">
    <h2>Recent Alerts <span class="badge">{recentAlerts.length}</span></h2>
    {#if recentAlerts.length === 0}
      <p class="empty">No active alerts</p>
    {:else}
      <table>
        <thead>
          <tr><th>Host</th><th>Rule</th><th>Severity</th><th>Time</th><th></th></tr>
        </thead>
        <tbody>
          {#each recentAlerts as a}
            <tr class="sev-{a.severity}">
              <td>{a.host}</td>
              <td>{a.rule_id}</td>
              <td><span class="badge-sev">{a.severity || 'info'}</span></td>
              <td>{new Date(a.fired_at).toLocaleTimeString()}</td>
              <td>
                {#if !a.acknowledged}
                  <button class="btn-sm" on:click={() => ackAlert(a.id)}>Ack</button>
                {:else}
                  <span class="acked">✓</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </section>
</div>

<style>
  .dashboard { display: flex; flex-direction: column; gap: 1rem; }
  .card { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 1rem; }
  h2 { margin: 0 0 1rem; font-size: 0.95rem; color: #94a3b8; text-transform: uppercase; letter-spacing: 0.05em; display: flex; align-items: center; gap: 0.5rem; }
  .gauges { display: flex; flex-wrap: wrap; gap: 1.5rem; }
  .ws-dot { width: 8px; height: 8px; border-radius: 50%; background: #64748b; }
  .ws-dot.connected { background: #22c55e; box-shadow: 0 0 6px #22c55e; }
  .sparks { display: flex; flex-wrap: wrap; gap: 1rem; }
  .spark-item { display: flex; align-items: center; gap: 0.5rem; font-size: 0.8rem; color: #94a3b8; }
  .spark-name { width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .spark-val { width: 40px; text-align: right; color: #e2e8f0; font-weight: 600; }
  table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
  th { text-align: left; padding: 0.4rem 0.5rem; color: #64748b; font-weight: 500; border-bottom: 1px solid #334155; }
  td { padding: 0.4rem 0.5rem; border-bottom: 1px solid #1e293b; color: #cbd5e1; }
  .badge { background: #334155; border-radius: 10px; padding: 0 6px; font-size: 0.75rem; font-weight: 600; }
  .badge-sev { font-size: 0.7rem; padding: 1px 6px; border-radius: 10px; background: #334155; }
  .sev-critical td { background: rgba(239,68,68,0.05); }
  .sev-warning td { background: rgba(234,179,8,0.05); }
  .empty { color: #475569; font-size: 0.85rem; }
  .btn-sm { background: #0284c7; border: none; color: #fff; padding: 2px 8px; border-radius: 4px; cursor: pointer; font-size: 0.75rem; }
  .acked { color: #22c55e; font-size: 0.8rem; }
</style>

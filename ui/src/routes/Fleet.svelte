<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { fetchFleet, fetchKPIs } from '../lib/api'
  import type { FleetHost, KPIMap } from '../lib/api'
  import WorkloadCard from '../components/WorkloadCard.svelte'

  let hosts: FleetHost[] = []
  let selected: FleetHost | null = null
  let kpis: KPIMap | null = null
  let loading = true
  let error = ''
  let refreshTimer: ReturnType<typeof setInterval>

  async function load() {
    try {
      const data = await fetchFleet()
      hosts = data.hosts ?? []
      error = ''
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }

  async function select(h: FleetHost) {
    selected = h
    kpis = null
    if (h.state === 'pending_telemetry') return
    try {
      kpis = await fetchKPIs(h.host)
    } catch {
      kpis = null
    }
  }

  onMount(() => {
    load()
    refreshTimer = setInterval(load, 10_000)
  })

  onDestroy(() => clearInterval(refreshTimer))

  function hsColor(v: number) {
    if (v >= 70) return 'var(--green)'
    if (v >= 40) return 'var(--yellow)'
    return 'var(--red)'
  }

  $: healthy = hosts.filter(h => h.state === 'healthy').length
  $: degraded = hosts.filter(h => h.state === 'degraded').length
  $: critical = hosts.filter(h => h.state === 'critical').length
  $: pending = hosts.filter(h => h.state === 'pending_telemetry').length
</script>

<div class="fleet">
  <!-- summary bar -->
  <div class="summary">
    <div class="stat">
      <span class="label">Total</span>
      <span class="val">{hosts.length}</span>
    </div>
    <div class="stat ok">
      <span class="label">Healthy</span>
      <span class="val">{healthy}</span>
    </div>
    <div class="stat warn">
      <span class="label">Degraded</span>
      <span class="val">{degraded}</span>
    </div>
    <div class="stat crit">
      <span class="label">Critical</span>
      <span class="val">{critical}</span>
    </div>
    {#if pending > 0}
      <div class="stat pend">
        <span class="label">Pending</span>
        <span class="val">{pending}</span>
      </div>
    {/if}
    <button class="refresh" on:click={load} title="Refresh">↻</button>
  </div>

  <div class="layout">
    <!-- workload list -->
    <div class="list">
      {#if loading}
        <div class="empty">Loading…</div>
      {:else if error}
        <div class="error">{error}</div>
      {:else if hosts.length === 0}
        <div class="empty">No workloads discovered yet.<br>Send OTLP telemetry to register workloads.</div>
      {:else}
        {#each hosts as host (host.host)}
          <WorkloadCard
            {host}
            selected={selected?.host === host.host}
            on:click={() => select(host)}
          />
        {/each}
      {/if}
    </div>

    <!-- detail panel -->
    {#if selected}
      <div class="detail">
        <div class="detail-header">
          <div>
            <div class="detail-name">{selected.host.split('/').pop()}</div>
            <div class="detail-meta">{selected.host}</div>
          </div>
          <button class="close" on:click={() => { selected = null; kpis = null }}>✕</button>
        </div>

        {#if selected.state === 'pending_telemetry'}
          <div class="pending-detail">
            <div class="pending-icon">◌</div>
            <div class="pending-title">Awaiting telemetry</div>
            <p>
              Ruptura discovered this workload from the Kubernetes API but hasn't
              received any OTLP metrics yet. Configure your workload to push metrics
              to <code>otlp-service:4317</code>.
            </p>
          </div>
        {:else}
          <div class="hs-big" style="color: {hsColor(selected.health_score)}">
            {Math.round(selected.health_score)}
            <span class="hs-label">HealthScore</span>
          </div>

          {#if kpis}
            <div class="kpi-grid">
              {#each Object.entries(kpis) as [key, kpi]}
                <div class="kpi-cell">
                  <div class="kpi-name">{key.replace('_', ' ')}</div>
                  <div
                    class="kpi-val"
                    class:ok={kpi.state === 'ok'}
                    class:warn={kpi.state === 'warning'}
                    class:crit={kpi.state === 'critical'}
                  >
                    {kpi.value.toFixed(1)}
                  </div>
                  <div class="kpi-trend">{kpi.trend}</div>
                </div>
              {/each}
            </div>
          {:else}
            <div class="loading-kpi">Loading KPIs…</div>
          {/if}
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .fleet {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .summary {
    display: flex;
    align-items: center;
    gap: 24px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 14px 20px;
  }

  .stat {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .stat .label {
    font-size: 10px;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .stat .val {
    font-size: 22px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    line-height: 1;
  }

  .stat.ok .val { color: var(--green); }
  .stat.warn .val { color: var(--yellow); }
  .stat.crit .val { color: var(--red); }
  .stat.pend .val { color: var(--blue); }

  .refresh {
    margin-left: auto;
    background: none;
    border: 1px solid var(--border);
    color: var(--muted);
    padding: 6px 10px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 16px;
    transition: color 0.15s;
  }

  .refresh:hover { color: var(--cyan); border-color: var(--cyan); }

  .layout {
    display: grid;
    grid-template-columns: 300px 1fr;
    gap: 16px;
    align-items: start;
  }

  @media (max-width: 700px) {
    .layout { grid-template-columns: 1fr; }
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .empty, .error {
    text-align: center;
    color: var(--muted);
    padding: 32px 16px;
    background: var(--surface);
    border-radius: 10px;
    border: 1px solid var(--border);
    line-height: 1.8;
  }

  .error { color: var(--red); border-color: var(--red); }

  .detail {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 20px;
    position: sticky;
    top: 72px;
  }

  .detail-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 16px;
  }

  .detail-name {
    font-weight: 700;
    font-size: 17px;
  }

  .detail-meta {
    font-size: 11px;
    color: var(--muted);
    margin-top: 2px;
  }

  .close {
    background: none;
    border: none;
    color: var(--muted);
    cursor: pointer;
    font-size: 16px;
    padding: 2px 6px;
    border-radius: 4px;
  }

  .close:hover { color: var(--text); }

  .hs-big {
    font-size: 56px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    line-height: 1;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
    margin-bottom: 20px;
  }

  .hs-label {
    font-size: 11px;
    font-weight: 400;
    color: var(--muted);
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .kpi-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  .kpi-cell {
    background: var(--surface2);
    border-radius: 8px;
    padding: 10px 12px;
  }

  .kpi-name {
    font-size: 10px;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    margin-bottom: 4px;
  }

  .kpi-val {
    font-size: 18px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    line-height: 1;
    color: var(--text);
  }

  .kpi-val.ok { color: var(--green); }
  .kpi-val.warn { color: var(--yellow); }
  .kpi-val.crit { color: var(--red); }

  .kpi-trend {
    font-size: 10px;
    color: var(--muted);
    margin-top: 2px;
  }

  .loading-kpi {
    color: var(--muted);
    font-size: 12px;
    text-align: center;
    padding: 20px;
  }

  .pending-detail {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 24px 8px;
    text-align: center;
  }

  .pending-icon {
    font-size: 40px;
    color: var(--blue);
    opacity: 0.6;
  }

  .pending-title {
    font-weight: 600;
    font-size: 16px;
    color: var(--blue);
  }

  .pending-detail p {
    font-size: 12px;
    color: var(--muted);
    line-height: 1.7;
  }

  .pending-detail code {
    font-family: monospace;
    background: var(--surface2);
    padding: 1px 5px;
    border-radius: 3px;
    color: var(--cyan);
  }
</style>

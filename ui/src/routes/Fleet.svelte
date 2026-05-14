<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { fetchFleet, fetchKPIs, fetchWorkloadK8s } from '../lib/api'
  import type { FleetHost, KPIMap, WorkloadK8sMeta } from '../lib/api'
  import WorkloadCard from '../components/WorkloadCard.svelte'
  import SuppressionModal from '../components/SuppressionModal.svelte'
  import WeightsModal from '../components/WeightsModal.svelte'

  let hosts: FleetHost[] = []
  let selected: FleetHost | null = null
  let kpis: KPIMap | null = null
  let k8sMeta: WorkloadK8sMeta | null = null
  let k8sError = ''
  let activeTab: 'signals' | 'kubernetes' = 'signals'
  let loading = true
  let error = ''
  let refreshTimer: ReturnType<typeof setInterval>

  let showSuppression = false
  let showWeights = false
  let suppressionDefaultWorkload = ''

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
    k8sMeta = null
    k8sError = ''
    activeTab = 'signals'
    if (h.state !== 'pending_telemetry') {
      fetchKPIs(h.host).then(v => { kpis = v }).catch(() => { kpis = null })
    }
    loadK8sMeta(h)
  }

  async function loadK8sMeta(h: FleetHost) {
    // host key format: namespace/kind/name
    const parts = h.host.split('/')
    if (parts.length !== 3) return
    const [ns, kind, name] = parts
    try {
      k8sMeta = await fetchWorkloadK8s(ns, kind, name)
      k8sError = ''
    } catch (e) {
      k8sError = e instanceof Error ? e.message : String(e)
    }
  }

  function switchTab(tab: 'signals' | 'kubernetes') {
    activeTab = tab
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

  function silenceWorkload(h: FleetHost) {
    suppressionDefaultWorkload = h.host
    showSuppression = true
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
    <div class="toolbar">
      <button class="tool-btn" on:click={() => { suppressionDefaultWorkload = ''; showSuppression = true }} title="Maintenance windows">
        🔕 Silence
      </button>
      <button class="tool-btn" on:click={() => { showWeights = true }} title="Signal weight overrides">
        ⚖ Weights
      </button>
      <button class="refresh" on:click={load} title="Refresh">↻</button>
    </div>
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
          <div class="detail-actions">
          <button class="detail-action-btn" on:click={() => selected && silenceWorkload(selected)} title="Add maintenance window for this workload">
            🔕
          </button>
          <button class="close" on:click={() => { selected = null; kpis = null }}>✕</button>
        </div>
        </div>

        <!-- tabs -->
        <div class="tabs">
          <button class="tab" class:active={activeTab === 'signals'} on:click={() => switchTab('signals')}>Signals</button>
          <button class="tab" class:active={activeTab === 'kubernetes'} on:click={() => switchTab('kubernetes')}>Kubernetes</button>
        </div>

        {#if activeTab === 'signals'}
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
        {:else}
          <!-- Kubernetes tab -->
          {#if k8sError && !k8sMeta}
            <div class="k8s-unavail">
              {#if k8sError.includes('503') || k8sError.includes('not available')}
                <div class="k8s-icon">☸</div>
                <div class="k8s-msg">Not running in-cluster</div>
                <p>Kubernetes metadata is only available when Ruptura runs inside a Kubernetes pod.</p>
              {:else if k8sError.includes('404') || k8sError.includes('not found')}
                <div class="k8s-icon">☸</div>
                <div class="k8s-msg">Workload not discovered yet</div>
                <p>This workload hasn't appeared in the discovery cache. It may still be starting up.</p>
              {:else}
                <div class="k8s-icon">!</div>
                <div class="k8s-msg">Metadata unavailable</div>
                <p>{k8sError}</p>
              {/if}
            </div>
          {:else if k8sMeta}
            <!-- image + replicas row -->
            <div class="k8s-section">
              <div class="k8s-field">
                <span class="k8s-label">Image</span>
                <span class="k8s-value k8s-image">{k8sMeta.image || '—'}</span>
              </div>
              <div class="k8s-field">
                <span class="k8s-label">Last Deploy</span>
                <span class="k8s-value">{k8sMeta.last_deploy ? new Date(k8sMeta.last_deploy).toLocaleString() : '—'}</span>
              </div>
            </div>

            <!-- replica gauge -->
            <div class="k8s-section">
              <div class="k8s-label">Replicas</div>
              <div class="replica-gauge">
                <div class="replica-bar-wrap">
                  <div
                    class="replica-bar"
                    class:replica-ok={k8sMeta.replicas.ready === k8sMeta.replicas.desired}
                    class:replica-warn={k8sMeta.replicas.ready < k8sMeta.replicas.desired}
                    style="width: {k8sMeta.replicas.desired > 0 ? (k8sMeta.replicas.ready / k8sMeta.replicas.desired) * 100 : 0}%"
                  ></div>
                </div>
                <span class="replica-text">{k8sMeta.replicas.ready} / {k8sMeta.replicas.desired} ready</span>
              </div>
            </div>

            <!-- resources -->
            {#if k8sMeta.resources.requests.cpu || k8sMeta.resources.limits.memory}
              <div class="k8s-section">
                <div class="k8s-label">Resources</div>
                <div class="res-grid">
                  <div class="res-row">
                    <span class="res-name">CPU request</span>
                    <span class="res-val">{k8sMeta.resources.requests.cpu || '—'}</span>
                  </div>
                  <div class="res-row">
                    <span class="res-name">CPU limit</span>
                    <span class="res-val">{k8sMeta.resources.limits.cpu || '—'}</span>
                  </div>
                  <div class="res-row">
                    <span class="res-name">Mem request</span>
                    <span class="res-val">{k8sMeta.resources.requests.memory || '—'}</span>
                  </div>
                  <div class="res-row">
                    <span class="res-name">Mem limit</span>
                    <span class="res-val">{k8sMeta.resources.limits.memory || '—'}</span>
                  </div>
                </div>
              </div>
            {/if}

            <!-- pod table -->
            {#if k8sMeta.pods && k8sMeta.pods.length > 0}
              <div class="k8s-section">
                <div class="k8s-label">Pods ({k8sMeta.pods.length})</div>
                <table class="pod-table">
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Node</th>
                      <th>Status</th>
                      <th>Restarts</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each k8sMeta.pods as pod}
                      <tr>
                        <td class="pod-name">{pod.name}</td>
                        <td class="pod-node">{pod.node}</td>
                        <td>
                          <span
                            class="pod-status"
                            class:pod-running={pod.status === 'Running'}
                            class:pod-pending={pod.status === 'Pending'}
                            class:pod-failed={pod.status === 'Failed' || pod.status === 'CrashLoopBackOff'}
                          >{pod.status}</span>
                        </td>
                        <td class="pod-restarts" class:pod-restarts-warn={pod.restarts > 0}>{pod.restarts}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {:else}
              <div class="loading-kpi">No pods in cache yet</div>
            {/if}
          {:else}
            <div class="loading-kpi">Loading Kubernetes metadata…</div>
          {/if}
        {/if}
      </div>
    {/if}
  </div>
</div>

{#if showSuppression}
  <SuppressionModal
    defaultWorkload={suppressionDefaultWorkload}
    on:close={() => { showSuppression = false }}
  />
{/if}

{#if showWeights}
  <WeightsModal on:close={() => { showWeights = false }} />
{/if}

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

  .toolbar {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-left: auto;
  }

  .tool-btn {
    background: var(--surface2);
    border: 1px solid var(--border);
    color: var(--muted);
    padding: 6px 12px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 500;
    transition: color 0.15s, border-color 0.15s;
  }

  .tool-btn:hover { color: var(--cyan); border-color: var(--cyan); }

  .refresh {
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

  .detail-actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .detail-action-btn {
    background: none;
    border: 1px solid var(--border);
    color: var(--muted);
    cursor: pointer;
    font-size: 14px;
    padding: 3px 8px;
    border-radius: 5px;
    transition: color 0.15s, border-color 0.15s;
  }

  .detail-action-btn:hover { color: var(--yellow); border-color: var(--yellow); }

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

  /* ── tabs ── */
  .tabs {
    display: flex;
    gap: 2px;
    margin-bottom: 16px;
    border-bottom: 1px solid var(--border);
  }

  .tab {
    background: none;
    border: none;
    color: var(--muted);
    padding: 7px 14px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 500;
    border-bottom: 2px solid transparent;
    margin-bottom: -1px;
    transition: color 0.15s, border-color 0.15s;
  }

  .tab:hover { color: var(--text); }
  .tab.active { color: var(--cyan); border-bottom-color: var(--cyan); }

  /* ── kubernetes tab ── */
  .k8s-section {
    margin-bottom: 14px;
  }

  .k8s-label {
    font-size: 10px;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    margin-bottom: 5px;
  }

  .k8s-field {
    display: flex;
    flex-direction: column;
    gap: 3px;
    margin-bottom: 10px;
  }

  .k8s-value {
    font-size: 12px;
    color: var(--text);
  }

  .k8s-image {
    font-family: monospace;
    font-size: 11px;
    color: var(--cyan);
    word-break: break-all;
  }

  .replica-gauge {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .replica-bar-wrap {
    flex: 1;
    height: 8px;
    background: var(--surface2);
    border-radius: 4px;
    overflow: hidden;
  }

  .replica-bar {
    height: 100%;
    border-radius: 4px;
    background: var(--muted);
    transition: width 0.3s;
  }

  .replica-bar.replica-ok { background: var(--green); }
  .replica-bar.replica-warn { background: var(--yellow); }

  .replica-text {
    font-size: 11px;
    color: var(--muted);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }

  .res-grid {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .res-row {
    display: flex;
    justify-content: space-between;
    font-size: 11px;
  }

  .res-name { color: var(--muted); }
  .res-val { font-family: monospace; color: var(--text); }

  .pod-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 11px;
  }

  .pod-table th {
    text-align: left;
    color: var(--muted);
    font-weight: 500;
    padding: 4px 6px;
    border-bottom: 1px solid var(--border);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    font-size: 10px;
  }

  .pod-table td {
    padding: 5px 6px;
    border-bottom: 1px solid var(--border);
    color: var(--text);
    vertical-align: middle;
  }

  .pod-table tr:last-child td { border-bottom: none; }

  .pod-name { font-family: monospace; font-size: 10px; max-width: 110px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .pod-node { color: var(--muted); font-size: 10px; }

  .pod-status {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 500;
    background: var(--surface2);
    color: var(--muted);
  }

  .pod-status.pod-running { background: color-mix(in srgb, var(--green) 15%, transparent); color: var(--green); }
  .pod-status.pod-pending { background: color-mix(in srgb, var(--yellow) 15%, transparent); color: var(--yellow); }
  .pod-status.pod-failed  { background: color-mix(in srgb, var(--red) 15%, transparent); color: var(--red); }

  .pod-restarts { font-variant-numeric: tabular-nums; }
  .pod-restarts.pod-restarts-warn { color: var(--yellow); font-weight: 600; }

  .k8s-unavail {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    padding: 24px 8px;
    text-align: center;
  }

  .k8s-icon {
    font-size: 32px;
    color: var(--muted);
    opacity: 0.5;
  }

  .k8s-msg {
    font-weight: 600;
    font-size: 14px;
    color: var(--muted);
  }

  .k8s-unavail p {
    font-size: 11px;
    color: var(--muted);
    line-height: 1.6;
    max-width: 260px;
  }
</style>

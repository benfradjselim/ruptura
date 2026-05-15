<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { fetchNodes, fetchNodeDetail } from '../lib/api'
  import type { ClusterNode, NodeDetail } from '../lib/api'

  let nodes: ClusterNode[] = []
  let selected: NodeDetail | null = null
  let selectedName = ''
  let error = ''
  let detailError = ''
  let loading = true
  let detailLoading = false
  let refreshTimer: ReturnType<typeof setInterval>

  async function load() {
    try {
      nodes = await fetchNodes()
      error = ''
      if (selectedName) refreshDetail(selectedName)
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }

  async function refreshDetail(name: string) {
    try {
      selected = await fetchNodeDetail(name)
      detailError = ''
    } catch (e) {
      detailError = e instanceof Error ? e.message : String(e)
    }
  }

  async function selectNode(name: string) {
    if (selectedName === name) {
      selectedName = ''
      selected = null
      return
    }
    selectedName = name
    selected = null
    detailError = ''
    detailLoading = true
    await refreshDetail(name)
    detailLoading = false
  }

  onMount(() => {
    load()
    refreshTimer = setInterval(load, 15_000)
  })

  onDestroy(() => clearInterval(refreshTimer))

  function frColor(v: number) {
    if (v >= 5) return 'var(--red)'
    if (v >= 1.5) return 'var(--orange)'
    if (v >= 1.0) return 'var(--yellow)'
    return 'var(--green)'
  }

  function pctColor(v: number) {
    if (v >= 85) return 'var(--red)'
    if (v >= 65) return 'var(--yellow)'
    return 'var(--text)'
  }

  function statusBadgeClass(s: string) {
    if (s === 'active') return 'badge-active'
    if (s === 'calibrating') return 'badge-calibrating'
    if (s === 'pending_telemetry') return 'badge-pending'
    return 'badge-removed'
  }
</script>

<div class="nodes-page">
  <h1 class="page-title">Cluster Nodes</h1>

  {#if loading}
    <div class="loading">Loading…</div>
  {:else if error}
    <div class="err-banner">{error}</div>
  {:else if nodes.length === 0}
    <div class="placeholder">
      <div class="icon">◫</div>
      <div class="title">No Nodes Found</div>
      <p>
        Node health data is aggregated from workload KPI signals. It becomes available once workloads
        start reporting metrics. Run Ruptura in-cluster with <code>autodiscovery.enabled=true</code> to
        populate this view automatically.
      </p>
    </div>
  {:else}
    <div class="layout" class:has-detail={!!selectedName}>
      <!-- Node list -->
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Node</th>
              <th>CPU</th>
              <th>Memory</th>
              <th>Disk Pressure</th>
              <th>Workloads</th>
              <th>Worst FusedR</th>
            </tr>
          </thead>
          <tbody>
            {#each nodes as node}
              <tr
                class:selected={node.name === selectedName}
                on:click={() => selectNode(node.name)}
                role="button"
                tabindex="0"
                on:keydown={e => e.key === 'Enter' && selectNode(node.name)}
              >
                <td class="node-name">{node.name}</td>
                <td style="color:{pctColor(node.cpu_pct)}">{node.cpu_pct.toFixed(1)}%</td>
                <td style="color:{pctColor(node.memory_pct)}">{node.memory_pct.toFixed(1)}%</td>
                <td class:pressure={node.disk_pressure}>{node.disk_pressure ? 'Yes' : 'No'}</td>
                <td>{node.workload_count}</td>
                <td style="color:{frColor(node.worst_fused_r)}">{node.worst_fused_r.toFixed(2)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <!-- Detail panel -->
      {#if selectedName}
        <div class="detail-panel">
          <div class="detail-header">
            <span class="detail-title">{selectedName}</span>
            <button class="close-btn" on:click={() => { selectedName = ''; selected = null }}>✕</button>
          </div>

          {#if detailLoading}
            <div class="detail-loading">Loading…</div>
          {:else if detailError}
            <div class="detail-err">{detailError}</div>
          {:else if selected}
            <!-- Stats row -->
            <div class="stat-row">
              <div class="stat">
                <div class="stat-label">CPU</div>
                <div class="stat-val" style="color:{pctColor(selected.cpu_pct)}">{selected.cpu_pct.toFixed(1)}%</div>
              </div>
              <div class="stat">
                <div class="stat-label">Memory</div>
                <div class="stat-val" style="color:{pctColor(selected.memory_pct)}">{selected.memory_pct.toFixed(1)}%</div>
              </div>
              <div class="stat">
                <div class="stat-label">Disk Pressure</div>
                <div class="stat-val" class:pressure={selected.disk_pressure}>{selected.disk_pressure ? 'Yes' : 'No'}</div>
              </div>
              <div class="stat">
                <div class="stat-label">Worst FusedR</div>
                <div class="stat-val" style="color:{frColor(selected.worst_fused_r)}">{selected.worst_fused_r.toFixed(2)}</div>
              </div>
            </div>

            <!-- Workloads on this node -->
            <div class="workloads-label">
              Workloads ({selected.workloads?.length ?? 0})
            </div>

            {#if selected.workloads && selected.workloads.length > 0}
              <div class="workload-list">
                {#each selected.workloads as wl}
                  <div class="workload-row">
                    <div class="wl-ref">{wl.ref}</div>
                    <div class="wl-right">
                      <span class="wl-hs" style="color:{pctColor(100 - wl.health_score)}">
                        HS {Math.round(wl.health_score)}
                      </span>
                      <span class="wl-fused" style="color:{frColor(wl.fused_r)}">
                        R {wl.fused_r.toFixed(2)}
                      </span>
                      <span class="badge {statusBadgeClass(wl.status)}">{wl.status}</span>
                    </div>
                  </div>
                {/each}
              </div>
            {:else}
              <div class="no-workloads">No workloads reporting from this node yet.</div>
            {/if}
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .nodes-page { display: flex; flex-direction: column; gap: 20px; }

  .page-title { font-size: 18px; font-weight: 700; }

  .loading { color: var(--muted); text-align: center; padding: 40px; }

  .err-banner {
    padding: 10px 14px;
    border-radius: 6px;
    font-size: 12px;
    color: var(--red);
    background: rgba(224, 82, 82, 0.08);
    border: 1px solid var(--red);
  }

  .placeholder {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    min-height: 50vh;
    text-align: center;
    color: var(--muted);
    justify-content: center;
  }

  .icon { font-size: 48px; opacity: 0.3; }
  .title { font-size: 20px; font-weight: 700; color: var(--text); }
  p { max-width: 380px; line-height: 1.8; font-size: 13px; }
  code { font-family: monospace; font-size: 11px; background: var(--surface2); padding: 1px 4px; border-radius: 3px; }

  .layout {
    display: grid;
    grid-template-columns: 1fr;
    gap: 16px;
  }

  .layout.has-detail {
    grid-template-columns: 1fr 320px;
  }

  .table-wrap {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow: hidden;
  }

  table { width: 100%; border-collapse: collapse; font-size: 13px; }

  th {
    text-align: left;
    padding: 12px 16px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    border-bottom: 1px solid var(--border);
    background: var(--surface2);
  }

  td {
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
    font-variant-numeric: tabular-nums;
  }

  tr:last-child td { border-bottom: none; }

  tr[role="button"] { cursor: pointer; }
  tr[role="button"]:hover td { background: var(--surface2); }
  tr.selected td { background: color-mix(in srgb, var(--accent) 8%, transparent); }

  .node-name { font-weight: 600; color: var(--text); }
  .pressure { color: var(--red); font-weight: 600; }

  /* ── Detail panel ── */
  .detail-panel {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    gap: 0;
    overflow: hidden;
    align-self: start;
  }

  .detail-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
    background: var(--surface2);
  }

  .detail-title { font-weight: 700; font-size: 13px; font-family: monospace; }

  .close-btn {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--muted);
    font-size: 14px;
    padding: 2px 6px;
    border-radius: 4px;
  }

  .close-btn:hover { background: var(--surface); color: var(--text); }

  .detail-loading, .detail-err {
    padding: 20px;
    text-align: center;
    font-size: 12px;
    color: var(--muted);
  }

  .detail-err { color: var(--red); }

  .stat-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0;
    border-bottom: 1px solid var(--border);
  }

  .stat {
    padding: 12px 16px;
    border-right: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
  }

  .stat:nth-child(2n) { border-right: none; }
  .stat:nth-last-child(-n+2) { border-bottom: none; }

  .stat-label { font-size: 10px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 4px; }
  .stat-val { font-size: 18px; font-weight: 700; font-variant-numeric: tabular-nums; }

  .workloads-label {
    padding: 10px 16px 6px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
    border-bottom: 1px solid var(--border);
  }

  .workload-list { overflow-y: auto; max-height: 400px; }

  .workload-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 16px;
    border-bottom: 1px solid var(--border);
    gap: 8px;
  }

  .workload-row:last-child { border-bottom: none; }

  .wl-ref {
    font-family: monospace;
    font-size: 10px;
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }

  .wl-right {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }

  .wl-hs, .wl-fused { font-size: 11px; font-variant-numeric: tabular-nums; font-weight: 600; }

  .badge {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 9px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }

  .badge-active { background: color-mix(in srgb, var(--green) 15%, transparent); color: var(--green); }
  .badge-calibrating { background: color-mix(in srgb, var(--yellow) 15%, transparent); color: var(--yellow); }
  .badge-pending { background: color-mix(in srgb, var(--muted) 15%, transparent); color: var(--muted); }
  .badge-removed { background: color-mix(in srgb, var(--red) 15%, transparent); color: var(--red); }

  .no-workloads { padding: 20px 16px; text-align: center; font-size: 12px; color: var(--muted); }
</style>

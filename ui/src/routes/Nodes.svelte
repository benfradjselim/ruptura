<script lang="ts">
  import { onMount } from 'svelte'
  import { fetchNodes } from '../lib/api'
  import type { ClusterNode } from '../lib/api'

  let nodes: ClusterNode[] = []
  let error = ''
  let loading = true

  onMount(async () => {
    try {
      nodes = await fetchNodes()
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  })

  function frColor(v: number) {
    if (v >= 5) return 'var(--red)'
    if (v >= 1.5) return 'var(--orange)'
    if (v >= 1.0) return 'var(--yellow)'
    return 'var(--green)'
  }
</script>

<div class="nodes-page">
  <h1 class="page-title">Cluster Nodes</h1>

  {#if loading}
    <div class="loading">Loading…</div>
  {:else if error || nodes.length === 0}
    <div class="placeholder">
      <div class="icon">◫</div>
      <div class="title">Node Health View</div>
      <p>
        Per-node CPU, memory, disk pressure, and worst FusedR across workloads.<br>
        <strong>Full implementation ships in S3-2 (GAP-V7-03).</strong>
      </p>
      {#if error}
        <div class="err">{error}</div>
      {/if}
    </div>
  {:else}
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
            <tr>
              <td class="node-name">{node.name}</td>
              <td>{node.cpu_pct.toFixed(1)}%</td>
              <td>{node.memory_pct.toFixed(1)}%</td>
              <td class:pressure={node.disk_pressure}>{node.disk_pressure ? 'Yes' : 'No'}</td>
              <td>{node.workload_count}</td>
              <td style="color:{frColor(node.worst_fused_r)}">{node.worst_fused_r.toFixed(2)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .nodes-page { display: flex; flex-direction: column; gap: 20px; }

  .page-title { font-size: 18px; font-weight: 700; }

  .loading { color: var(--muted); text-align: center; padding: 40px; }

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

  .err {
    font-size: 11px;
    color: var(--red);
    background: rgba(224, 82, 82, 0.08);
    border: 1px solid var(--red);
    border-radius: 6px;
    padding: 6px 12px;
  }

  .table-wrap {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow: hidden;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

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

  tr:hover td { background: var(--surface2); }

  .node-name { font-weight: 600; color: var(--text); }

  .pressure { color: var(--red); font-weight: 600; }
</style>

<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let traces = [], loading = true, error = '', selected = null
  let filterService = ''

  async function load() {
    loading = true
    try {
      const r = await api.traces({ limit: 50, service: filterService || undefined })
      traces = r.data || []
    } catch(e) { error = e.message }
    finally { loading = false }
  }

  onMount(load)

  function durationMs(t) {
    if (!t.duration_ms) return '—'
    return t.duration_ms.toFixed(1) + 'ms'
  }
  function statusClass(t) {
    if (t.error || t.status_code >= 500) return 'err'
    if (t.status_code >= 400) return 'warn'
    return 'ok'
  }
</script>

<div class="page-wrap">
  <div class="band page-header">
    <div style="grid-column:1/7"><h1 class="page-title">Traces</h1></div>
    <div style="grid-column:7/13; display:flex; gap:8px; align-items:center; justify-content:flex-end">
      <input class="input-sm" placeholder="Filter service…" bind:value={filterService} on:input={load} />
      <button class="btn btn-ghost btn-sm" on:click={load}>Refresh</button>
    </div>
  </div>

  {#if loading}
    <div class="loading band"><div class="spinner"></div></div>
  {:else if error}
    <div class="band"><div style="grid-column:1/-1;color:var(--red);font-size:13px">{error}</div></div>
  {:else if traces.length === 0}
    <div class="band"><div style="grid-column:1/-1;color:var(--text-3);font-size:13px;padding:32px 0">No traces</div></div>
  {:else}
    <div class="band">
      <!-- Trace list -->
      <div style="grid-column:1/8">
        <table class="data-table">
          <thead><tr><th>Trace ID</th><th>Service</th><th>Duration</th><th>Status</th><th>Time</th></tr></thead>
          <tbody>
            {#each traces as t}
              <tr class:selected={selected?.trace_id === t.trace_id} on:click={() => selected = selected?.trace_id === t.trace_id ? null : t} style="cursor:pointer">
                <td class="mono trace-id">{t.trace_id?.substring(0,12)}…</td>
                <td>{t.service || '—'}</td>
                <td class="mono {statusClass(t) === 'err' ? 'red' : ''}">{durationMs(t)}</td>
                <td>
                  <span class="status-dot {statusClass(t)}"></span>
                  <span class="status-text">{t.status_code || '—'}</span>
                </td>
                <td class="time-cell">{new Date(t.start_time).toLocaleTimeString()}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <!-- Trace detail -->
      <div style="grid-column:8/13">
        {#if selected}
          <div class="detail-card">
            <div class="detail-label">Trace detail</div>
            <div class="detail-row"><span class="detail-key">Trace ID</span><span class="mono detail-val">{selected.trace_id}</span></div>
            <div class="detail-row"><span class="detail-key">Service</span><span class="detail-val">{selected.service}</span></div>
            <div class="detail-row"><span class="detail-key">Duration</span><span class="mono detail-val">{durationMs(selected)}</span></div>
            <div class="detail-row"><span class="detail-key">Status</span><span class="detail-val">{selected.status_code || '—'}</span></div>
            {#if selected.spans}
              <div class="detail-label" style="margin-top:16px">Spans ({selected.spans.length})</div>
              {#each selected.spans as sp}
                <div class="span-row">
                  <span class="span-name">{sp.operation_name || sp.name}</span>
                  <span class="mono span-dur">{sp.duration_ms?.toFixed(1)}ms</span>
                </div>
              {/each}
            {/if}
          </div>
        {:else}
          <div class="detail-empty">Click a trace to inspect</div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .page-wrap { padding:32px 24px; overflow-y:auto; height:100%; display:grid; grid-template-columns:repeat(12,1fr); column-gap:20px; row-gap:0; align-content:start; }
  .band { grid-column:1/-1; display:grid; grid-template-columns:subgrid; column-gap:20px; margin-bottom:24px; align-items:start; }
  @supports not (grid-template-columns:subgrid) { .band { grid-template-columns:repeat(12,1fr); } }
  .page-title { font-size:22px; font-weight:700; letter-spacing:-0.02em; color:var(--text); }
  .input-sm { background:var(--surface-2); border:1px solid var(--border-2); border-radius:4px; color:var(--text); padding:5px 10px; font-size:12px; font-family:inherit; }
  .input-sm:focus { outline:none; border-color:var(--accent); }
  .btn { display:inline-flex; align-items:center; padding:5px 12px; border-radius:4px; border:1px solid var(--border-2); font-size:12px; cursor:pointer; font-family:inherit; }
  .btn-ghost { background:transparent; color:var(--text-2); }
  .btn-ghost:hover { background:var(--surface-2); }
  .btn-sm { padding:4px 10px; font-size:11px; }
  .loading { padding:64px; display:flex; justify-content:center; }
  .spinner { width:20px; height:20px; border-radius:50%; border:2px solid var(--border-2); border-top-color:var(--accent); animation:spin .7s linear infinite; }
  @keyframes spin { to{transform:rotate(360deg)} }
  .data-table { width:100%; border-collapse:collapse; font-size:12px; }
  th { text-align:left; padding:6px 8px; font-size:10px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; color:var(--text-3); border-bottom:1px solid var(--border); }
  td { padding:7px 8px; border-bottom:1px solid var(--border); color:var(--text-2); vertical-align:middle; }
  tr:hover td { background:var(--surface-2); }
  tr.selected td { background:var(--accent-dim); }
  .mono { font-family:"DM Mono",monospace; font-variant-numeric:tabular-nums; font-size:11px; }
  .trace-id { color:var(--text-3); }
  .red { color:var(--red); }
  .status-dot { display:inline-block; width:6px; height:6px; border-radius:50%; margin-right:6px; vertical-align:middle; }
  .status-dot.ok { background:var(--green); }
  .status-dot.warn { background:var(--amber); }
  .status-dot.err { background:var(--red); }
  .status-text { font-size:11px; }
  .time-cell { white-space:nowrap; font-size:11px; color:var(--text-3); }
  .detail-card { background:var(--surface); border:1px solid var(--border); border-radius:4px; padding:16px; }
  .detail-label { font-size:10px; font-weight:700; letter-spacing:0.10em; text-transform:uppercase; color:var(--text-3); margin-bottom:12px; }
  .detail-row { display:flex; gap:12px; align-items:baseline; padding:4px 0; border-bottom:1px solid var(--border); }
  .detail-key { font-size:11px; color:var(--text-3); min-width:80px; }
  .detail-val { font-size:12px; color:var(--text-2); }
  .span-row { display:flex; justify-content:space-between; padding:4px 0; border-bottom:1px solid var(--border); font-size:12px; }
  .span-name { color:var(--text-2); }
  .span-dur { color:var(--text-3); }
  .detail-empty { color:var(--text-3); font-size:12px; padding:24px; text-align:center; background:var(--surface); border:1px solid var(--border); border-radius:4px; border-style:dashed; }
</style>

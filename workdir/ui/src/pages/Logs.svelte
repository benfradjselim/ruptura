<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let logs = [], loading = true, error = '', limit = 100
  let filterLevel = 'all', filterService = ''

  async function load() {
    loading = true
    try {
      const r = await api.logs({ limit, level: filterLevel !== 'all' ? filterLevel : undefined, service: filterService || undefined })
      logs = r.data || []
    } catch(e) { error = e.message }
    finally { loading = false }
  }

  onMount(load)

  const LEVEL_CLASS = { error:'err', warn:'warn', warning:'warn', info:'info', debug:'debug' }
</script>

<div class="page-wrap">
  <div class="band page-header">
    <div style="grid-column:1/7">
      <h1 class="page-title">Logs</h1>
    </div>
    <div style="grid-column:7/13; display:flex; gap:8px; align-items:center; justify-content:flex-end">
      <select class="input-sm" bind:value={filterLevel} on:change={load}>
        <option value="all">All levels</option>
        <option value="error">Error</option>
        <option value="warn">Warning</option>
        <option value="info">Info</option>
        <option value="debug">Debug</option>
      </select>
      <input class="input-sm" placeholder="Filter service…" bind:value={filterService} on:input={load} />
      <button class="btn btn-ghost btn-sm" on:click={load}>Refresh</button>
    </div>
  </div>

  {#if loading}
    <div class="loading band"><div class="spinner" style="grid-column:1/-1; margin:auto"></div></div>
  {:else if error}
    <div class="band"><div style="grid-column:1/-1; color:var(--red); font-size:13px">{error}</div></div>
  {:else if logs.length === 0}
    <div class="band"><div style="grid-column:1/-1; color:var(--text-3); font-size:13px; padding:32px 0">No logs matching filter</div></div>
  {:else}
    <div class="band log-band">
      <div class="log-list" style="grid-column:1/-1">
        {#each logs as log}
          <div class="log-row {LEVEL_CLASS[log.level] || 'info'}">
            <span class="log-time mono">{new Date(log.timestamp).toLocaleTimeString()}</span>
            <span class="log-level">{log.level || 'info'}</span>
            <span class="log-service mono">{log.service || '—'}</span>
            <span class="log-msg">{log.message}</span>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .page-wrap { padding:32px 24px; overflow-y:auto; height:100%; display:grid; grid-template-columns:repeat(12,1fr); column-gap:20px; row-gap:0; align-content:start; }
  .band { grid-column:1/-1; display:grid; grid-template-columns:subgrid; column-gap:20px; margin-bottom:24px; align-items:center; }
  @supports not (grid-template-columns:subgrid) { .band { grid-template-columns:repeat(12,1fr); } }
  .page-title { font-size:22px; font-weight:700; letter-spacing:-0.02em; color:var(--text); }
  .input-sm { background:var(--surface-2); border:1px solid var(--border-2); border-radius:4px; color:var(--text); padding:5px 10px; font-size:12px; font-family:inherit; }
  .input-sm:focus { outline:none; border-color:var(--accent); }
  .btn { display:inline-flex; align-items:center; padding:5px 12px; border-radius:4px; border:1px solid var(--border-2); font-size:12px; font-weight:500; cursor:pointer; font-family:inherit; }
  .btn-ghost { background:transparent; color:var(--text-2); }
  .btn-ghost:hover { background:var(--surface-2); }
  .btn-sm { padding:4px 10px; font-size:11px; }
  .loading { padding:64px 0; }
  .spinner { width:20px; height:20px; border-radius:50%; border:2px solid var(--border-2); border-top-color:var(--accent); animation:spin .7s linear infinite; }
  @keyframes spin { to{transform:rotate(360deg)} }
  .log-list { display:flex; flex-direction:column; gap:2px; }
  .log-row { display:grid; grid-template-columns:80px 56px 140px 1fr; gap:12px; align-items:baseline; padding:5px 8px; border-radius:3px; font-size:12px; line-height:1.5; }
  .log-row.err  { background:var(--red-dim); }
  .log-row.warn { background:var(--amber-dim); }
  .log-row.info { }
  .log-row.debug { opacity:.6; }
  .log-time { color:var(--text-3); font-size:11px; }
  .log-level { font-size:9px; font-weight:700; text-transform:uppercase; letter-spacing:.06em; }
  .log-row.err  .log-level { color:var(--red); }
  .log-row.warn .log-level { color:var(--amber); }
  .log-row.info .log-level { color:var(--accent); }
  .log-row.debug .log-level { color:var(--text-3); }
  .log-service { color:var(--text-3); font-size:11px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .log-msg { color:var(--text-2); word-break:break-all; }
  .mono { font-family:"DM Mono",monospace; font-variant-numeric:tabular-nums; }
</style>

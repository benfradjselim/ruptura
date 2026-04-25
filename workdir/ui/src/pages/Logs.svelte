<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'
  import { timeRange, absRange } from '../lib/stores/timeRange.js'
  import TimeRangePicker from '../lib/components/TimeRangePicker.svelte'
  import LogDetailPanel  from '../lib/components/LogDetailPanel.svelte'
  import { SEVERITY_COLOR } from '../lib/util/format.js'

  const SEVERITIES = ['', 'debug', 'info', 'warn', 'warning', 'error', 'critical']
  const MAX_LINES  = 5000

  let logs     = []
  let loading  = false
  let error    = ''
  let selected = null   // detail panel entry

  let filterService  = ''
  let filterSeverity = ''
  let filterQ        = ''

  let liveMode = false
  let liveSource = null

  async function queryLogs() {
    loading = true; error = ''
    try {
      const range = $absRange
      const res = await api.logs({
        service:  filterService,
        severity: filterSeverity,
        q:        filterQ,
        from:     range.from.toISOString(),
        to:       range.to.toISOString(),
        limit:    MAX_LINES,
      })
      logs = (res?.data || []).map(parse).filter(Boolean)
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  function parse(raw) {
    if (!raw) return null
    if (typeof raw === 'object') return raw
    try { return JSON.parse(raw) }
    catch { return { message: String(raw) } }
  }

  function startLive() {
    if (liveSource) { liveSource.close(); liveSource = null }
    liveMode = true
    liveSource = api.logStream({ service: filterService, severity: filterSeverity, q: filterQ })
    liveSource.addEventListener('message', (e) => {
      const entry = parse(e.data)
      if (entry) {
        logs = [entry, ...logs].slice(0, MAX_LINES)
      }
    })
    liveSource.onerror = () => { liveMode = false }
  }

  function stopLive() {
    if (liveSource) { liveSource.close(); liveSource = null }
    liveMode = false
  }

  onMount(queryLogs)
  onDestroy(stopLive)

  function severityColor(entry) {
    const sev = (entry?.level || entry?.severity || entry?.labels?.level || '').toLowerCase()
    if (sev === 'error' || sev === 'critical') return '#ef4444'
    if (sev === 'warn' || sev === 'warning')   return '#fbbf24'
    if (sev === 'debug')                       return '#64748b'
    return '#94a3b8'
  }

  function entryMsg(entry) {
    return entry?.message || entry?.body || entry?.log || '(no message)'
  }

  function entryTime(entry) {
    const ts = entry?.timestamp || entry?.time || entry?.ts
    if (!ts) return ''
    return new Date(ts).toLocaleTimeString()
  }

  function entryService(entry) {
    return entry?.service || entry?.labels?.service || entry?.host || ''
  }
</script>

<div class="logs-page">
  <!-- Toolbar -->
  <div class="toolbar">
    <h1>Logs Explorer</h1>
    <div class="filters">
      <input class="filter-inp" type="text" bind:value={filterService}  placeholder="Service / host" />
      <select class="filter-sel" bind:value={filterSeverity}>
        <option value="">All severities</option>
        {#each SEVERITIES.filter(s => s) as s}
          <option value={s}>{s}</option>
        {/each}
      </select>
      <input class="filter-inp wide" type="text" bind:value={filterQ} placeholder="Search…" />
      <TimeRangePicker />
      <button class="btn-query" on:click={queryLogs} disabled={loading}>
        {loading ? '…' : 'Query'}
      </button>
      {#if !liveMode}
        <button class="btn-live" on:click={startLive}>⬤ Live</button>
      {:else}
        <button class="btn-live-stop" on:click={stopLive}>■ Stop</button>
      {/if}
    </div>
  </div>

  {#if error}<div class="err-bar">{error}</div>{/if}

  <div class="log-main">
    <!-- Log stream -->
    <div class="log-list" class:with-detail={!!selected}>
      {#if loading}
        <div class="log-state">Querying logs…</div>
      {:else if !logs.length}
        <div class="log-state muted">No logs found in range</div>
      {:else}
        <div class="log-count">{logs.length} lines {liveMode ? '(live)' : ''}</div>
        {#each logs as entry, i}
          <div
            class="log-line"
            class:selected={selected === entry}
            on:click={() => selected = (selected === entry ? null : entry)}
          >
            <span class="log-time">{entryTime(entry)}</span>
            <span class="log-svc">{entryService(entry)}</span>
            <span class="log-sev" style="color: {severityColor(entry)}">
              {(entry?.level || entry?.severity || '').toUpperCase() || 'LOG'}
            </span>
            <span class="log-msg">{entryMsg(entry)}</span>
          </div>
        {/each}
      {/if}
    </div>

    <!-- Detail panel -->
    {#if selected}
      <LogDetailPanel entry={selected} onClose={() => selected = null} />
    {/if}
  </div>
</div>

<style>
  .logs-page { display: flex; flex-direction: column; height: calc(100vh - 80px); gap: 0; }

  .toolbar {
    display: flex; align-items: flex-start; flex-direction: column; gap: 8px;
    padding-bottom: 10px; border-bottom: 1px solid #334155;
    flex-shrink: 0;
  }
  h1 { margin: 0; font-size: 1.1rem; color: #e2e8f0; }

  .filters { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
  .filter-inp {
    background: #1e293b; border: 1px solid #334155; border-radius: 6px;
    color: #e2e8f0; padding: 6px 10px; font-size: 0.83rem; width: 140px;
  }
  .filter-inp.wide { flex: 1; min-width: 180px; }
  .filter-sel {
    background: #1e293b; border: 1px solid #334155; border-radius: 6px;
    color: #e2e8f0; padding: 6px 8px; font-size: 0.83rem;
  }
  .filter-inp:focus, .filter-sel:focus { border-color: #38bdf8; outline: none; }

  .btn-query {
    background: #0284c7; border: none; color: #fff; padding: 6px 14px; border-radius: 6px;
    cursor: pointer; font-size: 0.83rem; font-weight: 600;
  }
  .btn-query:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn-live      { background: #14532d; border: 1px solid #22c55e; color: #86efac; padding: 6px 12px; border-radius: 6px; cursor: pointer; font-size: 0.8rem; }
  .btn-live-stop { background: #7f1d1d; border: 1px solid #ef4444; color: #fca5a5; padding: 6px 12px; border-radius: 6px; cursor: pointer; font-size: 0.8rem; }

  .err-bar { background: #7f1d1d; border: 1px solid #ef4444; color: #fca5a5; padding: 8px 14px; border-radius: 6px; font-size: 0.82rem; margin: 6px 0; flex-shrink: 0; }

  .log-main { display: flex; flex: 1; overflow: hidden; gap: 0; border: 1px solid #334155; border-radius: 8px; margin-top: 8px; }

  .log-list { flex: 1; overflow-y: auto; font-family: 'Courier New', monospace; }
  .log-list.with-detail { flex: 1; }

  .log-state { text-align: center; padding: 2rem; color: #475569; font-size: 0.85rem; }
  .log-count { padding: 4px 10px; font-size: 0.72rem; color: #475569; background: #0f172a; border-bottom: 1px solid #1e293b; }

  .log-line {
    display: flex; align-items: baseline; gap: 10px;
    padding: 4px 10px; font-size: 0.78rem; cursor: pointer;
    border-bottom: 1px solid #0f172a;
  }
  .log-line:hover { background: #1e293b; }
  .log-line.selected { background: #0f3460; }

  .log-time { color: #475569; white-space: nowrap; flex-shrink: 0; width: 80px; }
  .log-svc  { color: #38bdf8; white-space: nowrap; width: 100px; overflow: hidden; text-overflow: ellipsis; flex-shrink: 0; }
  .log-sev  { font-weight: 700; white-space: nowrap; flex-shrink: 0; width: 65px; font-size: 0.72rem; }
  .log-msg  { color: #e2e8f0; flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>

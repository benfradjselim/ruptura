<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import TraceWaterfall from '../lib/components/TraceWaterfall.svelte'

  let traces  = []
  let loading = false
  let error   = ''
  let selectedTrace = null   // full trace spans
  let selectedId    = null

  let filterService = ''
  let filterTraceId = ''

  async function search() {
    loading = true; error = ''; selectedTrace = null; selectedId = null
    try {
      const res = await api.traceSearch({ service: filterService, limit: 50 })
      traces = res?.data?.traces || []
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  async function openTrace(traceId) {
    selectedId = traceId
    try {
      const res = await api.traceGet(traceId)
      selectedTrace = res?.data || []
    } catch (e) { error = e.message }
  }

  async function openById() {
    if (!filterTraceId.trim()) return
    await openTrace(filterTraceId.trim())
  }

  onMount(search)

  function fmtDur(ms) {
    if (!ms) return '—'
    if (ms >= 1000) return (ms/1000).toFixed(2) + 's'
    return ms.toFixed(1) + 'ms'
  }

  function fmtTs(ms) {
    if (!ms) return '—'
    return new Date(ms).toLocaleTimeString()
  }
</script>

<div class="traces-page">
  <div class="toolbar">
    <h1>Trace Explorer</h1>
    <div class="filters">
      <input class="filter-inp" type="text" bind:value={filterService}  placeholder="Service name" />
      <input class="filter-inp wide" type="text" bind:value={filterTraceId} placeholder="Trace ID (exact)" />
      <button class="btn-query" on:click={search}>Search</button>
      <button class="btn-ghost" on:click={openById} disabled={!filterTraceId.trim()}>Open Trace</button>
    </div>
  </div>

  {#if error}<div class="err-bar">{error}</div>{/if}

  <div class="traces-main">
    <!-- List panel -->
    {#if !selectedTrace}
      <div class="traces-list">
        {#if loading}
          <div class="ts-state">Searching…</div>
        {:else if !traces.length}
          <div class="ts-state muted">No traces found. Send OTLP traces to /otlp/v1/traces to start tracing.</div>
        {:else}
          <div class="t-table">
            <div class="t-head">
              <span>Trace ID</span>
              <span>Root Service</span>
              <span>Operation</span>
              <span>Start</span>
              <span>Duration</span>
              <span>Spans</span>
              <span>Status</span>
            </div>
            {#each traces as t}
              <div class="t-row" on:click={() => openTrace(t.traceId)}>
                <code class="t-id">{t.traceId.slice(0, 12)}…</code>
                <span class="t-svc">{t.rootService || '—'}</span>
                <span class="t-op">{t.rootOp || '—'}</span>
                <span class="t-time">{fmtTs(t.startTimeMs)}</span>
                <span class="t-dur">{fmtDur(t.durationMs)}</span>
                <span class="t-spans">{t.spanCount}</span>
                <span class="t-status" class:err={t.hasError}>
                  {t.hasError ? '✕ Error' : '✓ OK'}
                </span>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {:else}
      <!-- Trace detail -->
      <div class="trace-detail">
        <div class="trace-detail-header">
          <button class="back-btn" on:click={() => { selectedTrace = null; selectedId = null }}>← Back</button>
          <span class="trace-id-label">Trace: <code>{selectedId}</code></span>
          <span class="span-count">{selectedTrace.length} spans</span>
        </div>
        <TraceWaterfall spans={selectedTrace} />
      </div>
    {/if}
  </div>
</div>

<style>
  .traces-page { display: flex; flex-direction: column; gap: 0.75rem; }
  .toolbar { display: flex; flex-direction: column; gap: 8px; padding-bottom: 10px; border-bottom: 1px solid #334155; }
  h1 { margin: 0; font-size: 1.1rem; color: #e2e8f0; }
  .filters { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
  .filter-inp {
    background: #1e293b; border: 1px solid #334155; border-radius: 6px;
    color: #e2e8f0; padding: 6px 10px; font-size: 0.83rem; width: 150px;
  }
  .filter-inp.wide { flex: 1; min-width: 200px; }
  .filter-inp:focus { border-color: #38bdf8; outline: none; }
  .btn-query { background: #0284c7; border: none; color: #fff; padding: 6px 14px; border-radius: 6px; cursor: pointer; font-size: 0.83rem; font-weight: 600; }
  .btn-ghost { background: transparent; border: 1px solid #334155; color: #94a3b8; padding: 6px 12px; border-radius: 6px; cursor: pointer; font-size: 0.83rem; }
  .btn-ghost:disabled { opacity: 0.4; cursor: not-allowed; }

  .err-bar { background: #7f1d1d; border: 1px solid #ef4444; color: #fca5a5; padding: 8px 14px; border-radius: 6px; font-size: 0.82rem; }

  .traces-main { flex: 1; }

  .t-table { border: 1px solid #334155; border-radius: 8px; overflow: hidden; }
  .t-head  { display: grid; grid-template-columns: 1.5fr 1fr 1.5fr 0.8fr 0.8fr 0.5fr 0.7fr; padding: 8px 14px; background: #0f172a; font-size: 0.72rem; color: #475569; text-transform: uppercase; letter-spacing: 0.05em; gap: 8px; }
  .t-row   { display: grid; grid-template-columns: 1.5fr 1fr 1.5fr 0.8fr 0.8fr 0.5fr 0.7fr; padding: 9px 14px; border-top: 1px solid #1e293b; align-items: center; gap: 8px; font-size: 0.8rem; cursor: pointer; }
  .t-row:hover { background: #1e293b; }

  .t-id    { color: #94a3b8; font-size: 0.75rem; }
  .t-svc   { color: #38bdf8; font-weight: 600; }
  .t-op    { color: #e2e8f0; }
  .t-time  { color: #64748b; }
  .t-dur   { color: #94a3b8; }
  .t-spans { color: #64748b; text-align: center; }
  .t-status { font-size: 0.75rem; font-weight: 600; color: #22c55e; }
  .t-status.err { color: #ef4444; }

  .ts-state { text-align: center; padding: 2rem; color: #475569; font-size: 0.85rem; }

  .trace-detail { display: flex; flex-direction: column; gap: 10px; }
  .trace-detail-header { display: flex; align-items: center; gap: 10px; padding: 8px 0; }
  .back-btn { background: transparent; border: 1px solid #334155; color: #64748b; padding: 4px 10px; border-radius: 6px; cursor: pointer; font-size: 0.8rem; }
  .back-btn:hover { border-color: #38bdf8; color: #38bdf8; }
  .trace-id-label { font-size: 0.82rem; color: #64748b; }
  .trace-id-label code { color: #38bdf8; }
  .span-count { font-size: 0.75rem; color: #475569; background: #1e293b; padding: 2px 8px; border-radius: 4px; }
</style>

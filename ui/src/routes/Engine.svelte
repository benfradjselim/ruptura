<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { fetchEngineStatus, fetchEngineStorage } from '../lib/api'
  import type { EngineStatus, EngineStorage } from '../lib/api'

  let status: EngineStatus | null = null
  let storage: EngineStorage | null = null
  let statusErr = ''
  let storageErr = ''
  let loading = true
  let apiLatencyMs = 0
  let interval: ReturnType<typeof setInterval>

  async function load() {
    const t0 = Date.now()
    try {
      ;[status, storage] = await Promise.all([
        fetchEngineStatus(),
        fetchEngineStorage(),
      ])
      apiLatencyMs = Date.now() - t0
      statusErr = ''
      storageErr = ''
    } catch (e) {
      statusErr = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }

  function formatUptime(secs: number): string {
    if (secs < 60) return `${secs}s`
    if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`
    const h = Math.floor(secs / 3600)
    const m = Math.floor((secs % 3600) / 60)
    return `${h}h ${m}m`
  }

  function formatBytes(b: number): string {
    if (b === 0) return '0 B'
    if (b < 1024) return `${b} B`
    if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`
    if (b < 1024 * 1024 * 1024) return `${(b / 1024 / 1024).toFixed(1)} MB`
    return `${(b / 1024 / 1024 / 1024).toFixed(2)} GB`
  }

  function ingestBarWidth(val: number, max: number): string {
    if (max === 0) return '0%'
    return `${Math.min(100, (val / max) * 100).toFixed(1)}%`
  }

  $: ingestMax = status
    ? Math.max(status.ingest.metrics_per_sec, status.ingest.logs_per_sec, status.ingest.traces_per_sec, 1)
    : 1

  onMount(() => {
    load()
    interval = setInterval(load, 15_000)
  })

  onDestroy(() => clearInterval(interval))
</script>

<div class="engine-page">
  <div class="page-header">
    <h1 class="page-title">Engine Status</h1>
    <button class="refresh-btn" on:click={load} title="Refresh">↺</button>
  </div>

  {#if loading}
    <div class="loading">Loading…</div>
  {:else if statusErr}
    <div class="err-banner">{statusErr} <button on:click={load}>Retry</button></div>
  {:else if status}
    <!-- Row 1: Runtime identity -->
    <div class="section-label">Runtime</div>
    <div class="grid grid-4">
      <div class="card">
        <div class="card-title">Version</div>
        <div class="big-val">{status.version || '—'}</div>
      </div>
      <div class="card">
        <div class="card-title">Edition</div>
        <div class="big-val edition">{status.edition || 'community'}</div>
      </div>
      <div class="card">
        <div class="card-title">Uptime</div>
        <div class="big-val">{formatUptime(status.uptime_seconds)}</div>
      </div>
      <div class="card">
        <div class="card-title">API Latency</div>
        <div class="big-val" class:warn={apiLatencyMs > 500}>{apiLatencyMs} ms</div>
      </div>
    </div>

    <!-- Row 2: Analyzer -->
    <div class="section-label">Analyzer</div>
    <div class="grid grid-4">
      <div class="card">
        <div class="card-title">Active</div>
        <div class="big-val green">{status.analyzer.active_workloads}</div>
        <div class="sub">workloads</div>
      </div>
      <div class="card">
        <div class="card-title">Calibrating</div>
        <div class="big-val yellow">{status.analyzer.calibrating_workloads}</div>
        <div class="sub">workloads</div>
      </div>
      <div class="card">
        <div class="card-title">Pending</div>
        <div class="big-val muted">{status.analyzer.pending_workloads ?? 0}</div>
        <div class="sub">no telemetry yet</div>
      </div>
      <div class="card">
        <div class="card-title">Tick Interval</div>
        <div class="big-val">{status.analyzer.tick_interval_ms / 1000}s</div>
      </div>
    </div>

    <!-- Row 3: Ingest rate bars -->
    <div class="section-label">Ingest Rates</div>
    <div class="card ingest-card">
      {#each [
        { label: 'Metrics', val: status.ingest.metrics_per_sec, color: 'var(--cyan)' },
        { label: 'Logs',    val: status.ingest.logs_per_sec,    color: 'var(--blue)' },
        { label: 'Traces',  val: status.ingest.traces_per_sec,  color: '#bc8cff' },
      ] as row}
        <div class="ingest-row">
          <span class="ingest-label">{row.label}</span>
          <div class="bar-track">
            <div class="bar-fill" style="width:{ingestBarWidth(row.val, ingestMax)};background:{row.color}"></div>
          </div>
          <span class="ingest-val">{row.val.toFixed(1)}/s</span>
        </div>
      {/each}
    </div>

    <!-- Row 4: Actions queue -->
    <div class="section-label">Action Engine</div>
    <div class="grid grid-3">
      <div class="card">
        <div class="card-title">Pending Tier-1</div>
        <div class="big-val" class:red={status.actions.pending_tier1 > 0}>{status.actions.pending_tier1}</div>
        <div class="sub">auto-execute queue</div>
      </div>
      <div class="card">
        <div class="card-title">Pending Tier-2</div>
        <div class="big-val" class:yellow={status.actions.pending_tier2 > 0}>{status.actions.pending_tier2}</div>
        <div class="sub">suggested actions</div>
      </div>
      <div class="card">
        <div class="card-title">Executed (last hour)</div>
        <div class="big-val">{status.actions.executed_last_hour}</div>
      </div>
    </div>

    <!-- Row 5: Storage -->
    {#if storage}
      <div class="section-label">Storage (BadgerDB)</div>
      <div class="grid grid-4">
        <div class="card">
          <div class="card-title">LSM Disk</div>
          <div class="big-val">{formatBytes(storage.badger.disk_bytes)}</div>
        </div>
        <div class="card">
          <div class="card-title">Value Log</div>
          <div class="big-val">{formatBytes(storage.badger.vlog_size_bytes)}</div>
        </div>
        <div class="card">
          <div class="card-title">Tables</div>
          <div class="big-val">{storage.badger.num_tables}</div>
        </div>
        <div class="card">
          <div class="card-title">Keys (approx)</div>
          <div class="big-val">{storage.badger.keys.toLocaleString()}</div>
        </div>
      </div>
    {:else if storageErr}
      <div class="err-banner small">{storageErr}</div>
    {/if}
  {/if}

  <!-- Footer -->
  <div class="footer">
    ruptura-ui v1.0.0
    {#if status}· connected to ruptura v{status.version}{/if}
    {#if apiLatencyMs > 0}· API latency {apiLatencyMs}ms{/if}
  </div>
</div>

<style>
  .engine-page {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding-bottom: 48px;
  }

  .page-header {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .page-title { font-size: 18px; font-weight: 700; }

  .refresh-btn {
    background: none;
    border: 1px solid var(--border);
    color: var(--muted);
    cursor: pointer;
    font-size: 16px;
    padding: 3px 9px;
    border-radius: 6px;
    transition: color 0.15s;
  }

  .refresh-btn:hover { color: var(--cyan); }

  .loading { color: var(--muted); text-align: center; padding: 40px; }

  .err-banner {
    background: rgba(224, 82, 82, 0.08);
    border: 1px solid rgba(224, 82, 82, 0.3);
    border-radius: 8px;
    padding: 10px 14px;
    font-size: 13px;
    color: var(--red);
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .err-banner.small { font-size: 11px; padding: 6px 12px; }

  .err-banner button {
    background: none;
    border: 1px solid var(--red);
    color: var(--red);
    padding: 2px 8px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
  }

  .section-label {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
    margin-top: 4px;
  }

  .grid {
    display: grid;
    gap: 12px;
  }

  .grid-4 { grid-template-columns: repeat(4, 1fr); }
  .grid-3 { grid-template-columns: repeat(3, 1fr); }

  @media (max-width: 800px) {
    .grid-4, .grid-3 { grid-template-columns: repeat(2, 1fr); }
  }

  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .card-title {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }

  .big-val {
    font-size: 22px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    color: var(--text);
    line-height: 1.2;
  }

  .big-val.green   { color: #3fb950; }
  .big-val.yellow  { color: #e3b341; }
  .big-val.red     { color: #e05252; }
  .big-val.muted   { color: var(--muted); }
  .big-val.edition { text-transform: capitalize; font-size: 18px; }
  .big-val.warn    { color: #e3b341; }

  .sub { font-size: 10px; color: var(--muted); }

  /* Ingest rate bars */
  .ingest-card { gap: 10px; }

  .ingest-row {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 12px;
  }

  .ingest-label {
    width: 52px;
    color: var(--muted);
    flex-shrink: 0;
  }

  .bar-track {
    flex: 1;
    height: 6px;
    background: var(--surface2);
    border-radius: 3px;
    overflow: hidden;
  }

  .bar-fill {
    height: 100%;
    border-radius: 3px;
    transition: width 0.4s ease;
    min-width: 2px;
  }

  .ingest-val {
    width: 60px;
    text-align: right;
    font-variant-numeric: tabular-nums;
    color: var(--text);
    flex-shrink: 0;
  }

  /* Footer */
  .footer {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 6px 24px;
    background: var(--surface);
    border-top: 1px solid var(--border);
    font-size: 10px;
    color: var(--muted);
    z-index: 10;
  }
</style>

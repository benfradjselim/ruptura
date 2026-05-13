<script lang="ts">
  import { onMount } from 'svelte'
  import { fetchEngineStatus } from '../lib/api'
  import type { EngineStatus } from '../lib/api'

  let status: EngineStatus | null = null
  let error = ''
  let loading = true

  onMount(async () => {
    try {
      status = await fetchEngineStatus()
    } catch (e) {
      // Endpoint ships in S2-2 — show graceful placeholder until then.
      error = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  })
</script>

<div class="engine-page">
  <h1 class="page-title">Engine Status</h1>

  {#if loading}
    <div class="loading">Loading…</div>
  {:else if error || !status}
    <div class="placeholder">
      <div class="icon">⚙</div>
      <div class="title">Engine Self-Health</div>
      <p>
        Fusion state, ingest rates, active workloads, and storage stats.<br>
        <strong>Full implementation ships in S2-2 (MISSING-08).</strong>
      </p>
      {#if error}
        <div class="err">{error}</div>
      {/if}
    </div>
  {:else}
    <div class="grid">
      <div class="card">
        <div class="card-title">Analyzer</div>
        <div class="row"><span>Active workloads</span><strong>{status.analyzer.active_workloads}</strong></div>
        <div class="row"><span>Calibrating</span><strong>{status.analyzer.calibrating_workloads}</strong></div>
        <div class="row"><span>Tick interval</span><strong>{status.analyzer.tick_interval_ms} ms</strong></div>
        <div class="row"><span>Last tick</span><strong>{status.analyzer.last_tick_ago_ms} ms ago</strong></div>
      </div>

      <div class="card">
        <div class="card-title">Ingest</div>
        <div class="row"><span>Metrics/s</span><strong>{status.ingest.metrics_per_sec.toFixed(1)}</strong></div>
        <div class="row"><span>Logs/s</span><strong>{status.ingest.logs_per_sec.toFixed(1)}</strong></div>
        <div class="row"><span>Traces/s</span><strong>{status.ingest.traces_per_sec.toFixed(1)}</strong></div>
      </div>

      <div class="card">
        <div class="card-title">Actions</div>
        <div class="row"><span>Pending Tier-1</span><strong>{status.actions.pending_tier1}</strong></div>
        <div class="row"><span>Pending Tier-2</span><strong>{status.actions.pending_tier2}</strong></div>
        <div class="row"><span>Executed last hour</span><strong>{status.actions.executed_last_hour}</strong></div>
      </div>

      <div class="card">
        <div class="card-title">Runtime</div>
        <div class="row"><span>Version</span><strong>{status.version}</strong></div>
        <div class="row"><span>Edition</span><strong>{status.edition}</strong></div>
        <div class="row"><span>Uptime</span><strong>{Math.round(status.uptime_seconds / 60)} min</strong></div>
      </div>
    </div>
  {/if}
</div>

<style>
  .engine-page { display: flex; flex-direction: column; gap: 20px; }

  .page-title {
    font-size: 18px;
    font-weight: 700;
  }

  .loading {
    color: var(--muted);
    text-align: center;
    padding: 40px;
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

  .title {
    font-size: 20px;
    font-weight: 700;
    color: var(--text);
  }

  p { max-width: 380px; line-height: 1.8; font-size: 13px; }

  .err {
    font-size: 11px;
    color: var(--red);
    background: rgba(224, 82, 82, 0.08);
    border: 1px solid var(--red);
    border-radius: 6px;
    padding: 6px 12px;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 16px;
  }

  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .card-title {
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--muted);
    padding-bottom: 8px;
    border-bottom: 1px solid var(--border);
  }

  .row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 13px;
    color: var(--muted);
  }

  .row strong {
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }
</style>

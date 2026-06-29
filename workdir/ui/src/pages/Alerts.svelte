<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let alertList = [], loading = true, error = ''

  async function load() {
    loading = true
    try { const r = await api.alerts(); alertList = r || [] }
    catch (e) { error = e.message }
    finally { loading = false }
  }

  async function ack(id) {
    await api.alertAck(id).catch(() => {})
    load()
  }

  onMount(load)

  const SEV_CLASS = { critical: 'crit', emergency: 'crit', warning: 'warn', info: 'info' }
</script>

<div class="page-wrap">
  <div class="page-header band">
    <div style="grid-column:1/9">
      <h1 class="page-title">Alerts</h1>
    </div>
    <div style="grid-column:9/13; display:flex; align-items:center; justify-content:flex-end; gap:8px">
      <span class="kicker">{alertList.filter(a=>!a.acknowledged).length} active</span>
      <button class="btn btn-ghost btn-sm" on:click={load}>Refresh</button>
    </div>
  </div>

  {#if loading}
    <div class="loading"><div class="spinner"></div></div>
  {:else if error}
    <div class="error-state">{error}</div>
  {:else if alertList.length === 0}
    <div class="empty band">
      <div style="grid-column:1/-1">
        <div class="empty-icon">✓</div>
        <p>No alerts — all systems healthy</p>
      </div>
    </div>
  {:else}
    <div class="band">
      <table class="data-table" style="grid-column:1/-1">
        <thead>
          <tr>
            <th>Severity</th>
            <th>Workload</th>
            <th>Rule</th>
            <th>Value</th>
            <th>Time</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each alertList as a}
            <tr class={SEV_CLASS[a.severity] || 'info'}>
              <td><span class="sev-badge sev-{a.severity || 'info'}">{a.severity || 'info'}</span></td>
              <td class="mono">{a.host}</td>
              <td class="rule-cell">{a.rule_id}</td>
              <td class="mono">{a.value != null ? Number(a.value).toFixed(2) : '—'}</td>
              <td class="time-cell">{new Date(a.created_at).toLocaleTimeString()}</td>
              <td>
                {#if !a.acknowledged}
                  <button class="btn btn-ghost btn-sm" on:click={() => ack(a.id)}>Ack</button>
                {:else}
                  <span class="acked">✓</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .page-wrap { padding: 32px 24px; overflow-y: auto; height: 100%; display: grid; grid-template-columns: repeat(12,1fr); column-gap: 20px; row-gap: 0; align-content: start; }
  .band { grid-column: 1/-1; display: grid; grid-template-columns: subgrid; column-gap: 20px; align-items: center; margin-bottom: 24px; }
  @supports not (grid-template-columns:subgrid) { .band { grid-template-columns: repeat(12,1fr); } }

  .page-title { font-size: 22px; font-weight: 700; letter-spacing: -0.02em; color: var(--text); }
  .kicker { font-size: 10px; font-weight: 700; letter-spacing: 0.10em; text-transform: uppercase; color: var(--text-3); }
  .btn { display:inline-flex; align-items:center; gap:6px; padding:6px 12px; border-radius:4px; border:1px solid transparent; font-size:12px; font-weight:500; cursor:pointer; font-family:inherit; }
  .btn-ghost { background:transparent; color:var(--text-2); border-color:var(--border-2); }
  .btn-ghost:hover { background:var(--surface-2); color:var(--text); }
  .btn-sm { padding:4px 10px; font-size:11px; }
  .spinner { width:20px; height:20px; border-radius:50%; border:2px solid var(--border-2); border-top-color:var(--accent); animation:spin .7s linear infinite; }
  @keyframes spin { to{transform:rotate(360deg)} }
  .loading { grid-column:1/-1; display:flex; justify-content:center; padding:64px; }
  .error-state { grid-column:1/-1; color:var(--red); padding:24px; font-size:13px; }
  .empty { grid-column:1/-1; }
  .empty-icon { font-size:32px; color:var(--green); margin-bottom:8px; }
  .empty p { font-size:13px; color:var(--text-3); }

  .data-table { width:100%; border-collapse:collapse; font-size:12px; }
  th { text-align:left; padding:6px 10px; font-size:10px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; color:var(--text-3); border-bottom:1px solid var(--border); white-space:nowrap; }
  td { padding:8px 10px; border-bottom:1px solid var(--border); color:var(--text-2); vertical-align:middle; }
  tr.crit td { background:var(--red-dim); }
  tr.warn td { background:var(--amber-dim); }
  .mono { font-family:"DM Mono",monospace; font-variant-numeric:tabular-nums; font-size:11px; }
  .rule-cell { color:var(--text-3); max-width:180px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .time-cell { white-space:nowrap; font-size:11px; color:var(--text-3); }
  .sev-badge { font-size:9px; font-weight:700; letter-spacing:0.06em; text-transform:uppercase; padding:2px 6px; border-radius:3px; }
  .sev-critical,.sev-emergency { background:var(--red-dim); color:var(--red); }
  .sev-warning { background:var(--amber-dim); color:var(--amber); }
  .sev-info { background:var(--surface-3); color:var(--text-3); }
  .acked { color:var(--green); font-size:13px; }
</style>

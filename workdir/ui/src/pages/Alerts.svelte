<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let alertList = [], loading = true, error = ''

  async function load() {
    loading = true
    try {
      const r = await api.alerts()
      alertList = r.data || []
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  async function ack(id) {
    await api.alertAck(id).catch(() => {})
    load()
  }

  async function del(id) {
    await api.alertDelete(id).catch(() => {})
    load()
  }

  onMount(load)
</script>

<div class="page">
  <div class="header">
    <h1>Alerts</h1>
    <button class="btn" on:click={load}>Refresh</button>
  </div>

  {#if loading}
    <p class="muted">Loading…</p>
  {:else if error}
    <p class="err">{error}</p>
  {:else if alertList.length === 0}
    <p class="muted">No alerts</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th>Severity</th><th>Host</th><th>Rule</th><th>Message</th>
          <th>Fired</th><th>Status</th><th></th>
        </tr>
      </thead>
      <tbody>
        {#each alertList as a}
          <tr class="sev-{a.severity}">
            <td><span class="badge sev-{a.severity}">{a.severity || 'info'}</span></td>
            <td>{a.host}</td>
            <td>{a.rule_id}</td>
            <td class="msg">{a.message || '—'}</td>
            <td class="time">{new Date(a.fired_at).toLocaleString()}</td>
            <td>
              {#if a.silenced}
                <span class="tag">silenced</span>
              {:else if a.acknowledged}
                <span class="tag green">acked</span>
              {:else}
                <span class="tag red">active</span>
              {/if}
            </td>
            <td class="actions">
              {#if !a.acknowledged && !a.silenced}
                <button class="btn-sm" on:click={() => ack(a.id)}>Ack</button>
              {/if}
              <button class="btn-sm danger" on:click={() => del(a.id)}>Delete</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .page { padding: 0; }
  .header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
  h1 { margin: 0; font-size: 1.2rem; color: #e2e8f0; }
  .btn { background: #334155; border: none; color: #e2e8f0; padding: 0.35rem 0.75rem; border-radius: 5px; cursor: pointer; font-size: 0.85rem; }
  table { width: 100%; border-collapse: collapse; background: #1e293b; border-radius: 8px; overflow: hidden; font-size: 0.85rem; }
  th { text-align: left; padding: 0.6rem 0.75rem; color: #64748b; font-weight: 500; background: #1a2535; }
  td { padding: 0.5rem 0.75rem; border-top: 1px solid #0f172a; color: #cbd5e1; }
  .msg { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .time { white-space: nowrap; color: #64748b; }
  .badge { font-size: 0.7rem; padding: 2px 6px; border-radius: 10px; text-transform: uppercase; font-weight: 600; background: #334155; }
  .badge.sev-critical { background: #450a0a; color: #f87171; }
  .badge.sev-warning { background: #422006; color: #fb923c; }
  .badge.sev-info { background: #1e3a5f; color: #60a5fa; }
  .tag { font-size: 0.7rem; padding: 1px 5px; border-radius: 4px; background: #334155; color: #94a3b8; }
  .tag.green { background: #052e16; color: #4ade80; }
  .tag.red { background: #450a0a; color: #f87171; }
  .actions { white-space: nowrap; }
  .btn-sm { background: #0284c7; border: none; color: #fff; padding: 2px 8px; border-radius: 4px; cursor: pointer; font-size: 0.75rem; margin-right: 4px; }
  .btn-sm.danger { background: #b91c1c; }
  .muted { color: #64748b; }
  .err { color: #f87171; }
</style>

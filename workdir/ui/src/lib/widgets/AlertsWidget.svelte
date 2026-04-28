<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'
  import { SEVERITY_COLOR, fmtRelative } from '../util/format.js'

  export let widget = {}
  export let refreshTick = 0

  let alerts = []
  let loading = true

  async function load() {
    loading = true
    try {
      const res = await api.alerts()
      const all = res?.data || []
      alerts = widget.severity
        ? all.filter(a => a.severity === widget.severity)
        : all.slice(0, 8)
    } catch { alerts = [] }
    finally { loading = false }
  }

  onMount(load)
  $: if (refreshTick) load()
</script>

{#if loading}
  <div class="aw-msg">loading…</div>
{:else if !alerts.length}
  <div class="aw-msg ok">No active alerts</div>
{:else}
  <div class="aw-list">
    {#each alerts as a}
      <div class="aw-row">
        <span class="aw-dot" style="background: {SEVERITY_COLOR[a.severity] ?? '#94a3b8'}"></span>
        <span class="aw-text">{a.message || a.name}</span>
        <span class="aw-time">{fmtRelative(a.fired_at || a.timestamp)}</span>
      </div>
    {/each}
  </div>
{/if}

<style>
  .aw-msg  { text-align: center; padding: 12px; color: #475569; font-size: 0.8rem; }
  .aw-msg.ok { color: #22c55e; }
  .aw-list { display: flex; flex-direction: column; gap: 4px; padding: 4px 0; }
  .aw-row  { display: flex; align-items: center; gap: 8px; font-size: 0.8rem; }
  .aw-dot  { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .aw-text { flex: 1; color: #e2e8f0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .aw-time { color: #475569; white-space: nowrap; font-size: 0.72rem; }
</style>

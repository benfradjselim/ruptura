<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { timeRange } from '../lib/stores/timeRange.js'
  import { refreshTick } from '../lib/stores/refresh.js'
  import TimeRangePicker from '../lib/components/TimeRangePicker.svelte'
  import RefreshPicker   from '../lib/components/RefreshPicker.svelte'
  import { getWidget }   from '../lib/widgets/index.js'

  export let dashboardId
  export let onBack
  export let onEdit = null

  let dashboard = null
  let loading   = true
  let error     = ''

  async function load() {
    loading = true; error = ''
    try {
      const res = await api.dashboardGet(dashboardId)
      dashboard = res?.data || res
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  onMount(load)
</script>

<div class="dv">
  <!-- Header bar -->
  <div class="dv-header">
    <div class="dv-title-row">
      <button class="back-btn" on:click={onBack}>← Back</button>
      <h2 class="dv-title">{dashboard?.name ?? '…'}</h2>
      {#if dashboard?.description}
        <span class="dv-desc">{dashboard.description}</span>
      {/if}
    </div>
    <div class="dv-controls">
      <TimeRangePicker />
      <RefreshPicker />
      {#if onEdit}
        <button class="edit-btn" on:click={() => onEdit(dashboardId)}>Edit</button>
      {/if}
    </div>
  </div>

  {#if loading}
    <div class="dv-state">Loading dashboard…</div>
  {:else if error}
    <div class="dv-state err">{error}</div>
  {:else if !dashboard?.widgets?.length}
    <div class="dv-state muted">No widgets yet. Click Edit to add panels.</div>
  {:else}
    <div class="widget-grid">
      {#each dashboard.widgets as widget, idx}
        {@const reg = getWidget(widget.type || 'stat')}
        <div
          class="widget-card"
          class:wide={widget.w >= 2}
          class:tall={widget.h >= 2}
          style={widget.w ? `grid-column: span ${Math.min(widget.w, 3)}` : ''}
        >
          <div class="widget-header">
            <span class="widget-icon">{reg.icon}</span>
            <span class="widget-title">{widget.title || reg.label}</span>
            {#if widget.host}
              <span class="widget-host">{widget.host}</span>
            {/if}
          </div>
          <div class="widget-body">
            <svelte:component
              this={reg.component}
              {widget}
              timeRange={$timeRange}
              refreshTick={$refreshTick}
            />
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .dv { display: flex; flex-direction: column; gap: 1rem; }

  .dv-header {
    display: flex; align-items: center; justify-content: space-between;
    flex-wrap: wrap; gap: 0.75rem;
    padding-bottom: 0.75rem; border-bottom: 1px solid #334155;
  }
  .dv-title-row { display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap; }
  .dv-title { font-size: 1.2rem; font-weight: 700; color: #e2e8f0; margin: 0; }
  .dv-desc  { font-size: 0.8rem; color: #64748b; }

  .dv-controls { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }

  .back-btn {
    background: transparent; border: 1px solid #334155; color: #64748b;
    padding: 5px 10px; border-radius: 6px; cursor: pointer; font-size: 0.82rem;
  }
  .back-btn:hover { border-color: #38bdf8; color: #38bdf8; }

  .edit-btn {
    background: #0f3460; border: 1px solid #0284c7; color: #38bdf8;
    padding: 6px 14px; border-radius: 6px; cursor: pointer; font-size: 0.82rem;
  }
  .edit-btn:hover { background: #0369a1; }

  .dv-state { text-align: center; padding: 3rem; color: #475569; font-size: 0.9rem; }
  .err  { color: #ef4444; }
  .muted { color: #334155; }

  /* ── Widget Grid ─────────────────────────────────────────────────────────── */
  .widget-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 1rem;
  }

  .widget-card {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 10px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    transition: border-color 0.15s;
  }
  .widget-card:hover { border-color: #475569; }
  .widget-card.wide { grid-column: span 2; }
  .widget-card.tall { grid-row: span 2; }

  .widget-header {
    display: flex; align-items: center; gap: 6px;
    padding: 10px 14px 6px;
    border-bottom: 1px solid #0f172a;
  }
  .widget-icon  { font-size: 0.85rem; }
  .widget-title { font-size: 0.85rem; font-weight: 600; color: #94a3b8; flex: 1; }
  .widget-host  { font-size: 0.72rem; color: #475569; background: #0f172a; padding: 1px 6px; border-radius: 4px; }

  .widget-body  { padding: 10px 12px 12px; flex: 1; }
</style>

<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { timeRange } from '../lib/stores/timeRange.js'
  import { refreshTick } from '../lib/stores/refresh.js'
  import TimeRangePicker from '../lib/components/TimeRangePicker.svelte'
  import RefreshPicker   from '../lib/components/RefreshPicker.svelte'
  import { getWidget }   from '../lib/widgets/index.js'

  export let dashboardId
  export let onBack    = null   // null = no back button (tab mode)
  export let onEdit    = null
  export let reloadKey = 0      // bump to force a reload from parent

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
  $: reloadKey, load()

  // Detect prediction-dominant board → switch time range to future mode
  $: isPredictionBoard = (() => {
    const ws = dashboard?.widgets ?? []
    if (!ws.length) return false
    const predCount = ws.filter(w => w.type === 'prediction').length
    return predCount / ws.length >= 0.5
  })()

  // Build effective widget: override horizon with time range preset in future mode
  function effectiveWidget(widget) {
    if (!isPredictionBoard || !$timeRange.future) return widget
    return { ...widget, horizon: $timeRange.preset ?? 60 }
  }
</script>

<div class="dv">
  <!-- Header bar -->
  <div class="dv-header">
    <div class="dv-title-row">
      {#if onBack}
        <button class="back-btn" on:click={onBack}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="back-icon">
            <polyline points="15 18 9 12 15 6"/>
          </svg>
          Back
        </button>
      {/if}
      <h2 class="dv-title">{dashboard?.name ?? '…'}</h2>
      {#if isPredictionBoard}
        <span class="pred-badge">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" class="pb-icon">
            <path d="M23 6l-9.5 9.5-5-5L1 18M17 6h6v6"/>
          </svg>
          Forecast
        </span>
      {/if}
      {#if dashboard?.description}
        <span class="dv-desc">{dashboard.description}</span>
      {/if}
    </div>
    <div class="dv-controls">
      <TimeRangePicker futureMode={isPredictionBoard} />
      <RefreshPicker />
      {#if onEdit}
        <button class="edit-btn" on:click={() => onEdit(dashboardId)}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="edit-icon">
            <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
            <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
          </svg>
          Edit
        </button>
      {/if}
    </div>
  </div>

  {#if loading}
    <div class="dv-state">
      <svg class="spin-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M21 12a9 9 0 11-6.219-8.56"/>
      </svg>
      Loading dashboard…
    </div>
  {:else if error}
    <div class="dv-state err">{error}</div>
  {:else if !dashboard?.widgets?.length}
    <div class="dv-state muted">No widgets yet — click Edit to add panels.</div>
  {:else}
    <div class="widget-grid">
      {#each dashboard.widgets as widget}
        {@const reg = getWidget(widget.type || 'stat')}
        {@const ew  = effectiveWidget(widget)}
        <div
          class="widget-card"
          style="grid-column: span {Math.min(widget.w || 1, 4)}; {widget.h && widget.h > 1 ? `grid-row: span ${widget.h};` : ''}"
        >
          <div class="widget-header">
            <svg class="widget-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <path d={reg.icon}/>
            </svg>
            <span class="widget-title">{widget.title || reg.label}</span>
            {#if widget.host}
              <span class="widget-host">{widget.host}</span>
            {/if}
            {#if widget.type === 'prediction'}
              <span class="pred-tag">forecast</span>
            {/if}
          </div>
          <div class="widget-body">
            <svelte:component
              this={reg.component}
              widget={ew}
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
  .dv-title { font-size: 1.1rem; font-weight: 700; color: #e2e8f0; margin: 0; }
  .dv-desc  { font-size: 0.8rem; color: #64748b; }

  .pred-badge {
    display: flex; align-items: center; gap: 4px;
    background: #ec489920; border: 1px solid #ec489955;
    color: #f472b6; font-size: 0.68rem; font-weight: 700;
    padding: 2px 8px; border-radius: 20px; letter-spacing: 0.04em;
  }
  .pb-icon { width: 11px; height: 11px; }

  .dv-controls { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }

  .back-btn {
    display: flex; align-items: center; gap: 4px;
    background: transparent; border: 1px solid #334155; color: #64748b;
    padding: 5px 10px; border-radius: 6px; cursor: pointer; font-size: 0.82rem;
    transition: all 0.15s;
  }
  .back-btn:hover { border-color: #38bdf8; color: #38bdf8; }
  .back-icon { width: 14px; height: 14px; }

  .edit-btn {
    display: flex; align-items: center; gap: 5px;
    background: #0f3460; border: 1px solid #0284c7; color: #38bdf8;
    padding: 6px 14px; border-radius: 6px; cursor: pointer; font-size: 0.82rem;
    transition: background 0.15s;
  }
  .edit-btn:hover { background: #0369a1; }
  .edit-icon { width: 14px; height: 14px; }

  .dv-state {
    display: flex; align-items: center; justify-content: center; gap: 8px;
    text-align: center; padding: 3rem; color: #475569; font-size: 0.9rem;
  }
  .err   { color: #ef4444; }
  .muted { color: #334155; }

  @keyframes spin { to { transform: rotate(360deg); } }
  .spin-icon { width: 18px; height: 18px; animation: spin 1s linear infinite; }

  /* ── Widget Grid ─────────────────────────────────────────────────────────── */
  .widget-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    grid-auto-rows: minmax(180px, auto);
    gap: 1rem;
  }
  @media (max-width: 900px) {
    .widget-grid { grid-template-columns: repeat(2, 1fr); }
  }
  @media (max-width: 500px) {
    .widget-grid { grid-template-columns: 1fr; }
  }

  .widget-card {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 10px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    min-height: 0;
    transition: border-color 0.15s, box-shadow 0.15s;
  }
  .widget-card:hover {
    border-color: #475569;
    box-shadow: 0 2px 12px rgba(0,0,0,0.3);
  }

  .widget-header {
    display: flex; align-items: center; gap: 6px;
    padding: 9px 14px 7px;
    border-bottom: 1px solid #0f172a;
    background: #162032;
  }
  .widget-icon  {
    width: 14px; height: 14px; flex-shrink: 0;
    color: #475569;
  }
  .widget-title { font-size: 0.82rem; font-weight: 600; color: #94a3b8; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .widget-host  { font-size: 0.68rem; color: #475569; background: #0f172a; padding: 1px 6px; border-radius: 4px; flex-shrink: 0; }
  .pred-tag {
    font-size: 0.62rem; font-weight: 700; color: #f472b6;
    background: #ec489915; border: 1px solid #ec489940;
    padding: 1px 5px; border-radius: 3px; flex-shrink: 0;
  }

  .widget-body  { padding: 10px 12px 12px; flex: 1; min-height: 0; overflow: auto; }
</style>

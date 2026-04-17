<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { getWidget } from '../lib/widgets/index.js'
  import WidgetConfigModal from '../lib/components/WidgetConfigModal.svelte'

  export let dashboardId
  export let onBack

  let dashboard = null
  let loading   = true
  let saving    = false
  let error     = ''
  let success   = ''

  let showModal    = false
  let editingWidget = null    // null = new, index = editing existing
  let editingIdx    = -1

  // Drag state
  let dragSrc = -1

  async function load() {
    loading = true; error = ''
    try {
      const res = await api.dashboardGet(dashboardId)
      dashboard = JSON.parse(JSON.stringify(res?.data || res))
      if (!dashboard.widgets) dashboard.widgets = []
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  onMount(load)

  function openNew() {
    editingWidget = null
    editingIdx    = -1
    showModal     = true
  }

  function openEdit(idx) {
    editingWidget = { ...dashboard.widgets[idx] }
    editingIdx    = idx
    showModal     = true
  }

  function handleSave(widgetData) {
    if (editingIdx >= 0) {
      dashboard.widgets[editingIdx] = widgetData
    } else {
      dashboard.widgets = [...dashboard.widgets, widgetData]
    }
    showModal = false
  }

  function removeWidget(idx) {
    dashboard.widgets = dashboard.widgets.filter((_, i) => i !== idx)
  }

  function moveUp(idx) {
    if (idx === 0) return
    const ws = [...dashboard.widgets]
    ;[ws[idx-1], ws[idx]] = [ws[idx], ws[idx-1]]
    dashboard.widgets = ws
  }
  function moveDown(idx) {
    if (idx >= dashboard.widgets.length - 1) return
    const ws = [...dashboard.widgets]
    ;[ws[idx], ws[idx+1]] = [ws[idx+1], ws[idx]]
    dashboard.widgets = ws
  }

  // HTML5 drag-and-drop reorder
  function dragStart(idx) { dragSrc = idx }
  function dragOver(e, idx) { e.preventDefault() }
  function drop(idx) {
    if (dragSrc < 0 || dragSrc === idx) return
    const ws = [...dashboard.widgets]
    const item = ws.splice(dragSrc, 1)[0]
    ws.splice(idx, 0, item)
    dashboard.widgets = ws
    dragSrc = -1
  }

  function resizeWidget(idx, dw, dh) {
    const ws = [...dashboard.widgets]
    const w = { ...ws[idx] }
    w.w = Math.min(4, Math.max(1, (w.w || 1) + dw))
    w.h = Math.min(4, Math.max(1, (w.h || 1) + dh))
    ws[idx] = w
    dashboard.widgets = ws
  }

  async function save() {
    saving = true; success = ''; error = ''
    try {
      await api.dashboardUpdate(dashboardId, dashboard)
      success = 'Saved!'
      setTimeout(() => success = '', 2000)
    } catch (e) { error = e.message }
    finally { saving = false }
  }
</script>

<div class="de">
  <div class="de-header">
    <div class="de-title-row">
      <button class="back-btn" on:click={onBack}>← Back</button>
      <h2>{dashboard?.name ?? '…'}</h2>
    </div>
    <div class="de-actions">
      {#if error}   <span class="de-err">{error}</span>   {/if}
      {#if success} <span class="de-ok">{success}</span>  {/if}
      <button class="add-btn" on:click={openNew}>+ Add Widget</button>
      <button class="save-btn" on:click={save} disabled={saving}>
        {saving ? 'Saving…' : 'Save Dashboard'}
      </button>
    </div>
  </div>

  {#if loading}
    <div class="de-state">Loading…</div>
  {:else if !dashboard?.widgets?.length}
    <div class="de-empty">
      <p>No widgets yet.</p>
      <button class="add-btn" on:click={openNew}>+ Add your first widget</button>
    </div>
  {:else}
    <div class="de-grid">
      {#each dashboard.widgets as widget, idx}
        {@const reg = getWidget(widget.type || 'stat')}
        <div
          class="de-card"
          class:dragging={dragSrc === idx}
          draggable="true"
          on:dragstart={() => dragStart(idx)}
          on:dragover={(e) => dragOver(e, idx)}
          on:drop={() => drop(idx)}
        >
          <div class="de-card-header">
            <span class="drag-handle" title="Drag to reorder">⠿</span>
            <span class="type-badge">{reg.icon} {reg.label}</span>
            <span class="de-wtitle">{widget.title || reg.label}</span>
            <div class="de-card-actions">
              <button title="Narrower (w-1)"  on:click={() => resizeWidget(idx, -1, 0)} disabled={(dashboard.widgets[idx].w||1) <= 1}>◀</button>
              <button title="Wider (w+1)"     on:click={() => resizeWidget(idx,  1, 0)} disabled={(dashboard.widgets[idx].w||1) >= 4}>▶</button>
              <button title="Shorter (h-1)"   on:click={() => resizeWidget(idx,  0,-1)} disabled={(dashboard.widgets[idx].h||1) <= 1}>▲</button>
              <button title="Taller (h+1)"    on:click={() => resizeWidget(idx,  0, 1)} disabled={(dashboard.widgets[idx].h||1) >= 4}>▼</button>
              <button title="Move up"   on:click={() => moveUp(idx)}>↑</button>
              <button title="Move down" on:click={() => moveDown(idx)}>↓</button>
              <button title="Edit"      on:click={() => openEdit(idx)}>✏</button>
              <button title="Remove" class="del-btn" on:click={() => removeWidget(idx)}>✕</button>
            </div>
          </div>
          <div class="de-card-meta">
            {#if widget.metric}<span class="meta-chip">metric: {widget.metric}</span>{/if}
            {#if widget.kpi}   <span class="meta-chip">kpi: {widget.kpi}</span>{/if}
            {#if widget.host}  <span class="meta-chip">host: {widget.host}</span>{/if}
            <span class="meta-chip size-chip">{widget.w||1}w × {widget.h||1}h</span>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

{#if showModal}
  <WidgetConfigModal
    widget={editingWidget}
    onSave={handleSave}
    onClose={() => showModal = false}
  />
{/if}

<style>
  .de { display: flex; flex-direction: column; gap: 1rem; }

  .de-header {
    display: flex; align-items: center; justify-content: space-between;
    flex-wrap: wrap; gap: 0.75rem;
    padding-bottom: 0.75rem; border-bottom: 1px solid #334155;
  }
  .de-title-row { display: flex; align-items: center; gap: 0.75rem; }
  .de-title-row h2 { margin: 0; font-size: 1.2rem; color: #e2e8f0; }

  .de-actions { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }
  .de-err { font-size: 0.8rem; color: #ef4444; }
  .de-ok  { font-size: 0.8rem; color: #22c55e; }

  .back-btn {
    background: transparent; border: 1px solid #334155; color: #64748b;
    padding: 5px 10px; border-radius: 6px; cursor: pointer; font-size: 0.82rem;
  }
  .back-btn:hover { border-color: #38bdf8; color: #38bdf8; }
  .add-btn  { background: #0f3460; border: 1px solid #0284c7; color: #38bdf8; padding: 7px 14px; border-radius: 6px; cursor: pointer; font-size: 0.85rem; }
  .save-btn { background: #0284c7; border: none; color: #fff; padding: 7px 18px; border-radius: 6px; cursor: pointer; font-size: 0.85rem; font-weight: 600; }
  .save-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .de-state, .de-empty { text-align: center; padding: 3rem; color: #475569; }

  .de-grid { display: flex; flex-direction: column; gap: 8px; }

  .de-card {
    background: #1e293b; border: 1px solid #334155; border-radius: 8px;
    padding: 10px 14px; cursor: grab;
    transition: border-color 0.15s, opacity 0.15s;
  }
  .de-card:hover    { border-color: #475569; }
  .de-card.dragging { opacity: 0.4; border-color: #38bdf8; }

  .de-card-header {
    display: flex; align-items: center; gap: 8px;
  }
  .drag-handle { color: #334155; font-size: 1.1rem; cursor: grab; user-select: none; }
  .type-badge  { font-size: 0.72rem; color: #38bdf8; background: #0f3460; padding: 2px 6px; border-radius: 4px; white-space: nowrap; }
  .de-wtitle   { flex: 1; font-size: 0.88rem; color: #e2e8f0; font-weight: 600; }

  .de-card-actions { display: flex; gap: 4px; }
  .de-card-actions button {
    background: transparent; border: 1px solid #334155; color: #64748b;
    padding: 3px 7px; border-radius: 4px; cursor: pointer; font-size: 0.78rem;
  }
  .de-card-actions button:hover { background: #334155; color: #e2e8f0; }
  .del-btn:hover { background: #7f1d1d !important; border-color: #ef4444 !important; color: #ef4444 !important; }

  .de-card-meta { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 6px; }
  .meta-chip { font-size: 0.7rem; color: #64748b; background: #0f172a; padding: 2px 6px; border-radius: 4px; }
  .size-chip { color: #38bdf8; border: 1px solid #0284c730; }
</style>

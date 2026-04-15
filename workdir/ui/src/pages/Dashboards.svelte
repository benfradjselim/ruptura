<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import DashboardView from './DashboardView.svelte'
  import DashboardEdit from './DashboardEdit.svelte'

  let dashboards = [], templates = [], loading = true
  let showNew = false, newName = '', newRefresh = 30, creating = false
  let openId = null    // viewing
  let editId = null    // editing

  async function load() {
    loading = true
    const [dr, tr] = await Promise.all([
      api.dashboards().catch(() => ({ data: [] })),
      api.templates().catch(() => ({ data: [] })),
    ])
    dashboards = dr.data || []
    templates  = tr.data || []
    loading = false
  }

  async function create() {
    creating = true
    await api.dashboardCreate({ name: newName, refresh_seconds: newRefresh }).catch(() => {})
    showNew = false; newName = ''; creating = false
    load()
  }

  async function applyTemplate(id) {
    await api.templateApply(id).catch(() => {})
    load()
  }

  async function del(id) {
    if (!confirm('Delete this dashboard?')) return
    await api.dashboardDelete(id).catch(() => {})
    load()
  }

  onMount(load)
</script>

{#if openId}
  <DashboardView
    dashboardId={openId}
    onBack={() => { openId = null; load() }}
    onEdit={(id) => { openId = null; editId = id }}
  />
{:else if editId}
  <DashboardEdit
    dashboardId={editId}
    onBack={() => { editId = null; load() }}
  />
{:else}
<div class="page">
  <div class="header">
    <h1>Dashboards</h1>
    <button class="btn" on:click={() => showNew = !showNew}>+ New</button>
  </div>

  {#if showNew}
    <div class="new-form card">
      <input bind:value={newName} placeholder="Dashboard name" class="inp"/>
      <label>Refresh (s): <input type="number" bind:value={newRefresh} min="5" max="3600" class="inp-num"/></label>
      <button class="btn-primary" on:click={create} disabled={!newName || creating}>Create</button>
      <button class="btn-ghost" on:click={() => showNew = false}>Cancel</button>
    </div>
  {/if}

  {#if loading}
    <p class="muted">Loading…</p>
  {:else}
    {#if dashboards.length > 0}
      <div class="grid">
        {#each dashboards as d}
          <div class="dash-card card">
            <div class="dash-name">{d.name}</div>
            <div class="dash-meta">
              {d.widgets?.length || 0} widgets · refresh {d.refresh_seconds}s
            </div>
            <div class="dash-actions">
              <button class="btn-sm"        on:click={() => openId = d.id}>View</button>
              <button class="btn-sm edit"   on:click={() => editId  = d.id}>Edit</button>
              <button class="btn-sm danger" on:click={() => del(d.id)}>Delete</button>
            </div>
          </div>
        {/each}
      </div>
    {:else}
      <p class="muted">No dashboards yet. Create one or apply a template.</p>
    {/if}

    {#if templates.length > 0}
      <h2>Built-in Templates</h2>
      <div class="grid">
        {#each templates as t}
          <div class="dash-card card template">
            <div class="dash-name">{t.name}</div>
            <div class="dash-meta">{t.description}</div>
            <div class="dash-actions">
              <button class="btn-sm" on:click={() => applyTemplate(t.id)}>Apply</button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</div>
{/if}

<style>
  .page { padding: 0; }
  .header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
  h1 { margin: 0; font-size: 1.2rem; color: #e2e8f0; }
  h2 { font-size: 0.9rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; margin: 1.5rem 0 0.5rem; }
  .card { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 1rem; }
  .btn { background: #334155; border: none; color: #e2e8f0; padding: 0.35rem 0.75rem; border-radius: 5px; cursor: pointer; font-size: 0.85rem; }
  .btn-primary { background: #0284c7; border: none; color: #fff; padding: 0.35rem 0.75rem; border-radius: 5px; cursor: pointer; }
  .btn-ghost { background: transparent; border: 1px solid #334155; color: #94a3b8; padding: 0.35rem 0.75rem; border-radius: 5px; cursor: pointer; }
  .btn-sm { background: #334155; border: none; color: #e2e8f0; padding: 2px 8px; border-radius: 4px; cursor: pointer; font-size: 0.75rem; }
  .btn-sm.edit   { background: #0f3460; color: #38bdf8; }
  .btn-sm.danger { background: #7f1d1d; color: #fca5a5; }
  .new-form { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 1rem; flex-wrap: wrap; }
  .inp { background: #0f172a; border: 1px solid #334155; color: #e2e8f0; padding: 0.4rem 0.6rem; border-radius: 5px; font-size: 0.85rem; }
  .inp-num { width: 70px; background: #0f172a; border: 1px solid #334155; color: #e2e8f0; padding: 0.3rem 0.4rem; border-radius: 4px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 0.75rem; }
  .dash-card { display: flex; flex-direction: column; gap: 0.4rem; }
  .dash-name { font-weight: 600; color: #e2e8f0; }
  .dash-meta { font-size: 0.75rem; color: #64748b; }
  .dash-actions { margin-top: auto; display: flex; gap: 0.4rem; flex-wrap: wrap; }
  .template { border-color: #1d4ed8; }
  .muted { color: #64748b; }
</style>

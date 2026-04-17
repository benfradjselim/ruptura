<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let orgs    = []
  let error   = null
  let loading = true
  let showForm = false
  let editing  = null
  let form = { name: '', slug: '', description: '' }

  async function load() {
    try {
      const res = await api.orgs()
      orgs = res.data || []
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  async function save() {
    try {
      if (editing) {
        await api.orgUpdate(editing.id, form)
      } else {
        await api.orgCreate(form)
      }
      resetForm()
      await load()
    } catch (e) {
      error = e.message
    }
  }

  async function del(id) {
    if (!confirm('Delete this org?')) return
    try {
      await api.orgDelete(id)
      await load()
    } catch (e) {
      error = e.message
    }
  }

  function startEdit(o) {
    editing = o
    form = { name: o.name, slug: o.slug, description: o.description || '' }
    showForm = true
  }

  function resetForm() {
    editing = null
    form = { name: '', slug: '', description: '' }
    showForm = false
  }

  onMount(load)
</script>

<div class="page">
  <div class="page-header">
    <div>
      <h1>Organisations</h1>
      <p class="subtitle">Tenant workspaces for multi-tenancy</p>
    </div>
    <button class="btn-primary" on:click={() => { resetForm(); showForm = !showForm }}>
      {showForm ? 'Cancel' : '+ New Org'}
    </button>
  </div>

  {#if error}<div class="alert-error">{error}</div>{/if}

  {#if showForm}
    <div class="form-card">
      <h3>{editing ? 'Edit Org' : 'New Org'}</h3>
      <label>Name
        <input bind:value={form.name} placeholder="My Organisation" />
      </label>
      <label>Slug (URL-safe)
        <input bind:value={form.slug} placeholder="my-organisation" />
      </label>
      <label>Description
        <input bind:value={form.description} placeholder="Optional description" />
      </label>
      <div class="form-actions">
        <button class="btn-primary" on:click={save}>{editing ? 'Save' : 'Create'}</button>
        <button class="btn-ghost" on:click={resetForm}>Cancel</button>
      </div>
    </div>
  {/if}

  {#if loading}
    <div class="loading">Loading…</div>
  {:else if orgs.length === 0}
    <div class="empty">No organisations yet. Create the first one above.</div>
  {:else}
    <div class="table-wrap">
      <table class="data-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Slug</th>
            <th>Description</th>
            <th>Created</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each orgs as o}
            <tr>
              <td class="bold">{o.name}</td>
              <td class="mono">{o.slug}</td>
              <td class="muted">{o.description || '—'}</td>
              <td class="muted">{new Date(o.created_at).toLocaleDateString()}</td>
              <td class="actions">
                <button class="btn-icon" on:click={() => startEdit(o)} title="Edit">✎</button>
                <button class="btn-icon danger" on:click={() => del(o.id)} title="Delete">✕</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .page         { padding: 24px; max-width: 900px; }
  .page-header  { display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:24px; }
  h1            { margin:0 0 4px; color:#e2e8f0; font-size:1.4rem; }
  .subtitle     { color:#64748b; font-size:0.85rem; margin:0; }
  .alert-error  { background:#450a0a; border:1px solid #7f1d1d; color:#fca5a5; border-radius:6px; padding:12px 16px; margin-bottom:16px; font-size:0.85rem; }
  .form-card    { background:#1e293b; border:1px solid #334155; border-radius:8px; padding:20px; margin-bottom:24px; }
  .form-card h3 { margin:0 0 16px; color:#e2e8f0; }
  label         { display:block; margin-bottom:12px; color:#94a3b8; font-size:0.82rem; }
  input         { display:block; width:100%; margin-top:4px; padding:8px 10px; background:#0f172a; border:1px solid #334155; border-radius:5px; color:#e2e8f0; font-size:0.9rem; box-sizing:border-box; }
  .form-actions { display:flex; gap:8px; margin-top:16px; }
  .btn-primary  { background:#3b82f6; color:#fff; border:none; padding:8px 16px; border-radius:5px; cursor:pointer; font-size:0.85rem; }
  .btn-primary:hover { background:#2563eb; }
  .btn-ghost    { background:transparent; color:#94a3b8; border:1px solid #334155; padding:8px 16px; border-radius:5px; cursor:pointer; font-size:0.85rem; }
  .loading      { color:#64748b; padding:40px; text-align:center; }
  .empty        { color:#475569; padding:40px; text-align:center; font-size:0.9rem; }
  .table-wrap   { overflow-x:auto; }
  .data-table   { width:100%; border-collapse:collapse; font-size:0.85rem; }
  .data-table th { text-align:left; padding:8px 12px; border-bottom:1px solid #334155; color:#64748b; font-weight:600; }
  .data-table td { padding:10px 12px; border-bottom:1px solid #1e293b; color:#cbd5e1; }
  .bold   { color:#e2e8f0; font-weight:600; }
  .mono   { font-family:monospace; color:#94a3b8; }
  .muted  { color:#64748b; }
  .actions { text-align:right; white-space:nowrap; }
  .btn-icon { background:none; border:none; cursor:pointer; padding:4px 8px; color:#64748b; font-size:1rem; border-radius:4px; }
  .btn-icon:hover { background:#1e293b; color:#94a3b8; }
  .btn-icon.danger:hover { color:#ef4444; }
</style>

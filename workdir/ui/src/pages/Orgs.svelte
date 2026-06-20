<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let orgs = [], loading = true, showForm = false, saving = false
  let form = { name:'', description:'' }

  async function load() {
    loading = true
    try { const r = await api.orgs(); orgs = r.data || [] } catch {}
    finally { loading = false }
  }

  async function save() {
    saving = true
    await api.orgCreate(form).catch(() => {})
    saving = false; showForm = false; load()
  }

  async function del(id) {
    await api.orgDelete(id).catch(() => {})
    load()
  }

  onMount(load)
</script>

<div class="page-wrap">
  <div class="band page-header">
    <div style="grid-column:1/9"><h1 class="page-title">Organizations</h1></div>
    <div style="grid-column:9/13;display:flex;justify-content:flex-end">
      <button class="btn btn-primary btn-sm" on:click={() => showForm=!showForm}>{showForm ? 'Cancel' : '+ New org'}</button>
    </div>
  </div>

  {#if showForm}
    <div class="band">
      <div class="card" style="grid-column:1/7;padding:20px">
        <div class="section-label">New organization</div>
        <div class="field" style="margin-bottom:10px"><label class="field-label">Name</label><input class="input" bind:value={form.name} /></div>
        <div class="field" style="margin-bottom:16px"><label class="field-label">Description</label><input class="input" bind:value={form.description} /></div>
        <button class="btn btn-primary" disabled={saving} on:click={save}>{saving ? 'Creating…' : 'Create'}</button>
      </div>
    </div>
  {/if}

  {#if loading}
    <div class="band"><div class="loading" style="grid-column:1/-1"><div class="spinner"></div></div></div>
  {:else if orgs.length === 0}
    <div class="band"><p class="empty-msg" style="grid-column:1/-1">No organizations yet</p></div>
  {:else}
    <div class="band">
      <table class="data-table" style="grid-column:1/9">
        <thead><tr><th>Name</th><th>Description</th><th>Created</th><th></th></tr></thead>
        <tbody>
          {#each orgs as o}
            <tr>
              <td class="org-name">{o.name}</td>
              <td class="dim">{o.description || '—'}</td>
              <td class="dim mono">{o.created_at ? new Date(o.created_at).toLocaleDateString() : '—'}</td>
              <td><button class="btn btn-danger btn-sm" on:click={() => del(o.id)}>Delete</button></td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .page-wrap { padding:32px 24px; overflow-y:auto; height:100%; display:grid; grid-template-columns:repeat(12,1fr); column-gap:20px; row-gap:0; align-content:start; }
  .band { grid-column:1/-1; display:grid; grid-template-columns:subgrid; column-gap:20px; margin-bottom:24px; align-items:start; }
  @supports not (grid-template-columns:subgrid) { .band { grid-template-columns:repeat(12,1fr); } }
  .page-header { align-items:center; }
  .page-title { font-size:22px; font-weight:700; letter-spacing:-0.02em; color:var(--text); }
  .section-label { font-size:10px; font-weight:700; letter-spacing:0.10em; text-transform:uppercase; color:var(--text-3); margin-bottom:12px; }
  .card { background:var(--surface); border:1px solid var(--border); border-radius:4px; }
  .field { display:flex; flex-direction:column; gap:5px; }
  .field-label { font-size:10px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; color:var(--text-3); }
  .input { background:var(--surface-2); border:1px solid var(--border-2); border-radius:4px; color:var(--text); padding:7px 10px; font-size:13px; font-family:inherit; width:100%; }
  .input:focus { outline:none; border-color:var(--accent); }
  .btn { display:inline-flex; align-items:center; gap:6px; padding:7px 14px; border-radius:4px; border:1px solid transparent; font-size:12px; font-weight:600; cursor:pointer; font-family:inherit; }
  .btn:disabled { opacity:.5; cursor:not-allowed; }
  .btn-primary { background:var(--accent); color:#000; }
  .btn-danger { background:var(--red-dim); color:var(--red); border-color:rgba(244,63,94,.2); }
  .btn-sm { padding:4px 10px; font-size:11px; }
  .loading { display:flex; justify-content:center; padding:32px; }
  .spinner { width:20px; height:20px; border-radius:50%; border:2px solid var(--border-2); border-top-color:var(--accent); animation:spin .7s linear infinite; }
  @keyframes spin { to{transform:rotate(360deg)} }
  .empty-msg { color:var(--text-3); font-size:13px; }
  .data-table { width:100%; border-collapse:collapse; font-size:12px; }
  th { text-align:left; padding:6px 8px; font-size:10px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; color:var(--text-3); border-bottom:1px solid var(--border); }
  td { padding:7px 8px; border-bottom:1px solid var(--border); color:var(--text-2); vertical-align:middle; }
  tr:hover td { background:var(--surface-2); }
  .org-name { font-weight:600; color:var(--text); }
  .dim { color:var(--text-3); font-size:11px; }
  .mono { font-family:"DM Mono",monospace; font-variant-numeric:tabular-nums; }
</style>

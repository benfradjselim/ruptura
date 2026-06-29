<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let sources = [], loading = true, showForm = false, saving = false
  let form = { name:'', type:'prometheus', url:'', token:'', tls_skip:false }

  async function load() {
    loading = true
    try { const r = await api.datasources(); sources = r.data || [] } catch {}
    finally { loading = false }
  }

  async function save() {
    saving = true
    await api.datasourceCreate(form).catch(() => {})
    saving = false; showForm = false; load()
  }

  async function del(id) {
    await api.datasourceDelete(id).catch(() => {})
    load()
  }

  const TYPE_LABELS = { prometheus:'Prometheus', datadog:'Datadog', cloudwatch:'CloudWatch', loki:'Loki', dynatrace:'Dynatrace', azure:'Azure Monitor' }
  onMount(load)
</script>

<div class="page-wrap">
  <div class="band page-header">
    <div style="grid-column:1/9"><h1 class="page-title">Data Sources</h1></div>
    <div style="grid-column:9/13;display:flex;justify-content:flex-end">
      <button class="btn btn-primary btn-sm" on:click={() => showForm=!showForm}>{showForm ? 'Cancel' : '+ Add source'}</button>
    </div>
  </div>

  {#if showForm}
    <div class="band">
      <div class="card" style="grid-column:1/8;padding:20px">
        <div class="section-label">Add data source</div>
        <div class="form-grid">
          <div class="field"><label class="field-label">Name</label><input class="input" bind:value={form.name} /></div>
          <div class="field"><label class="field-label">Type</label>
            <select class="input" bind:value={form.type}>
              {#each Object.entries(TYPE_LABELS) as [k,v]}<option value={k}>{v}</option>{/each}
            </select>
          </div>
          <div class="field full"><label class="field-label">URL</label><input class="input" bind:value={form.url} placeholder="https://…" /></div>
          <div class="field full"><label class="field-label">Token / API key (optional)</label><input class="input" type="password" bind:value={form.token} /></div>
          <div class="field full">
            <label class="check-label"><input type="checkbox" bind:checked={form.tls_skip} /> Skip TLS verification</label>
          </div>
        </div>
        <button class="btn btn-primary" disabled={saving} on:click={save}>{saving ? 'Saving…' : 'Add source'}</button>
      </div>
    </div>
  {/if}

  {#if loading}
    <div class="band"><div class="loading" style="grid-column:1/-1"><div class="spinner"></div></div></div>
  {:else if sources.length === 0}
    <div class="band"><div class="empty-state" style="grid-column:1/-1">
      <div class="empty-icon">🔌</div>
      <p>No external data sources configured.</p>
      <p style="font-size:11px;color:var(--text-3);margin-top:4px;">
        Ruptura ingests telemetry via OTLP (port 4317) and Prometheus remote-write (/api/v2/write).<br/>
        Add an external source to proxy queries through Ruptura dashboards.
      </p>
    </div></div>
  {:else}
    <div class="band sources-grid">
      {#each sources as s}
        <div class="source-card">
          <div class="source-head">
            <span class="source-name">{s.name}</span>
            <span class="type-badge">{TYPE_LABELS[s.type] || s.type}</span>
          </div>
          <div class="source-url mono">{s.url}</div>
          <button class="btn btn-danger btn-sm" on:click={() => del(s.id)}>Remove</button>
        </div>
      {/each}
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
  .form-grid { display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-bottom:16px; }
  .field { display:flex; flex-direction:column; gap:5px; }
  .field.full { grid-column:1/-1; }
  .field-label { font-size:10px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; color:var(--text-3); }
  .check-label { display:flex; align-items:center; gap:8px; font-size:12px; color:var(--text-2); cursor:pointer; }
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
  .empty-state { display:flex; flex-direction:column; align-items:center; gap:12px; padding:48px; text-align:center; color:var(--text-3); }
  .empty-icon { font-size:2rem; opacity:.4; }
  .empty-state p { font-size:13px; max-width:360px; line-height:1.6; }
  .sources-grid { grid-column:1/-1; display:grid; grid-template-columns:repeat(auto-fill,minmax(280px,1fr)); gap:12px; }
  .source-card { background:var(--surface); border:1px solid var(--border); border-radius:4px; padding:16px; display:flex; flex-direction:column; gap:10px; }
  .source-head { display:flex; align-items:center; gap:10px; }
  .source-name { font-size:13px; font-weight:600; color:var(--text); }
  .type-badge { font-size:9px; font-weight:700; letter-spacing:.08em; text-transform:uppercase; background:var(--surface-3); color:var(--text-3); padding:2px 7px; border-radius:3px; }
  .source-url { font-size:11px; color:var(--text-3); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .mono { font-family:"DM Mono",monospace; font-variant-numeric:tabular-nums; }
</style>

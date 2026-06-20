<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let slos = [], loading = true, showForm = false
  let form = { name:'', target:99, window_days:30, metric:'health_score', host:'' }
  let saving = false

  async function load() {
    loading = true
    try { const r = await api.slos(); slos = r.data || [] } catch {}
    finally { loading = false }
  }

  async function save() {
    saving = true
    await api.sloCreate(form).catch(() => {})
    saving = false; showForm = false; load()
  }

  async function del(id) {
    await api.sloDelete(id).catch(() => {})
    load()
  }

  function statusClass(s) {
    if (!s.current_value) return 'unknown'
    return s.current_value >= s.target ? 'healthy' : 'critical'
  }

  onMount(load)
</script>

<div class="page-wrap">
  <div class="band page-header">
    <div style="grid-column:1/9"><h1 class="page-title">SLOs</h1></div>
    <div style="grid-column:9/13;display:flex;justify-content:flex-end">
      <button class="btn btn-primary btn-sm" on:click={() => showForm=!showForm}>
        {showForm ? 'Cancel' : '+ New SLO'}
      </button>
    </div>
  </div>

  {#if showForm}
    <div class="band">
      <div class="card form-card" style="grid-column:1/8">
        <div class="section-label">Define SLO</div>
        <div class="form-grid">
          <div class="field"><label class="field-label">Name</label><input class="input" bind:value={form.name} placeholder="Availability SLO" /></div>
          <div class="field"><label class="field-label">Target %</label><input class="input" type="number" min="0" max="100" step="0.1" bind:value={form.target} /></div>
          <div class="field"><label class="field-label">Window (days)</label><input class="input" type="number" bind:value={form.window_days} /></div>
          <div class="field"><label class="field-label">Metric</label>
            <select class="input" bind:value={form.metric}>
              <option value="health_score">Health Score</option>
              <option value="stress">CPU Pressure</option>
              <option value="fatigue">Memory Pressure</option>
              <option value="error_rate">Error Rate</option>
            </select>
          </div>
          <div class="field"><label class="field-label">Workload (optional)</label><input class="input" bind:value={form.host} placeholder="all" /></div>
        </div>
        <button class="btn btn-primary" disabled={saving} on:click={save}>{saving ? 'Saving…' : 'Create SLO'}</button>
      </div>
    </div>
  {/if}

  {#if loading}
    <div class="band"><div class="loading" style="grid-column:1/-1"><div class="spinner"></div></div></div>
  {:else if slos.length === 0}
    <div class="band"><p class="empty-msg">No SLOs defined yet</p></div>
  {:else}
    <div class="band slos-grid">
      {#each slos as s}
        {@const sc = statusClass(s)}
        <div class="slo-card">
          <div class="slo-head">
            <span class="slo-name">{s.name}</span>
            <span class="status-dot {sc}"></span>
          </div>
          <div class="slo-val num">{s.current_value != null ? s.current_value.toFixed(2) + '%' : '—'}</div>
          <div class="slo-target">Target: {s.target}% · {s.window_days}d window</div>
          <div class="slo-bar-track">
            <div class="slo-bar-fill" style="width:{Math.min(s.current_value || 0, 100)}%; background:{sc==='healthy' ? 'var(--green)' : 'var(--red)'}"></div>
          </div>
          <button class="btn btn-danger btn-sm" on:click={() => del(s.id)}>Delete</button>
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
  .card { background:var(--surface); border:1px solid var(--border); border-radius:4px; padding:16px; }
  .form-card { padding:20px; }
  .form-grid { display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-bottom:16px; }
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
  .empty-msg { color:var(--text-3); font-size:13px; grid-column:1/-1; }
  .slos-grid { grid-column:1/-1; display:grid; grid-template-columns:repeat(auto-fill,minmax(240px,1fr)); gap:12px; }
  .slo-card { background:var(--surface); border:1px solid var(--border); border-radius:4px; padding:16px; display:flex; flex-direction:column; gap:8px; }
  .slo-head { display:flex; align-items:center; justify-content:space-between; }
  .slo-name { font-size:13px; font-weight:600; color:var(--text); }
  .status-dot { width:8px; height:8px; border-radius:50%; flex-shrink:0; }
  .status-dot.healthy { background:var(--green); }
  .status-dot.critical { background:var(--red); }
  .status-dot.unknown { background:var(--text-3); }
  .slo-val { font-family:"DM Mono",monospace; font-size:28px; font-weight:500; line-height:1; color:var(--text); font-variant-numeric:tabular-nums; }
  .num { font-family:"DM Mono",monospace; font-variant-numeric:tabular-nums; }
  .slo-target { font-size:11px; color:var(--text-3); }
  .slo-bar-track { height:3px; background:var(--surface-3); border-radius:2px; overflow:hidden; }
  .slo-bar-fill { height:100%; border-radius:2px; transition:width .4s; }
</style>

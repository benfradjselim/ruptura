<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  const KPI_OPTIONS = ['stress','fatigue','mood','pressure','humidity','contagion','resilience','entropy','velocity','health_score']
  const MET_OPTIONS = ['cpu_percent','memory_percent','disk_percent','request_rate','error_rate']
  const ALL_METRICS = [...KPI_OPTIONS, ...MET_OPTIONS]

  let rules = [], loading = true, saving = false, showForm = false
  let form = { name:'', metric:'health_score', operator:'<', threshold:0.5, severity:'warning', host:'' }
  let editName = null

  async function load() {
    loading = true
    try { const r = await api.alertRules(); rules = r.data || [] } catch {}
    finally { loading = false }
  }

  async function save() {
    saving = true
    try {
      if (editName) await api.alertRuleUpdate(editName, form)
      else await api.alertRuleCreate(form)
      showForm = false; editName = null; load()
    } catch {}
    finally { saving = false }
  }

  async function del(name) {
    await api.alertRuleDelete(name).catch(() => {})
    load()
  }

  function edit(r) {
    form = { ...r }; editName = r.name; showForm = true
  }

  const SEV_CLASS = { critical:'crit', emergency:'crit', warning:'warn', info:'info' }
  onMount(load)
</script>

<div class="page-wrap">
  <div class="band page-header">
    <div style="grid-column:1/9"><h1 class="page-title">Alert Rules</h1></div>
    <div style="grid-column:9/13;display:flex;justify-content:flex-end">
      <button class="btn btn-primary btn-sm" on:click={() => { showForm=!showForm; editName=null; form={name:'',metric:'health_score',operator:'<',threshold:0.5,severity:'warning',host:''} }}>
        {showForm ? 'Cancel' : '+ New rule'}
      </button>
    </div>
  </div>

  {#if showForm}
    <div class="band">
      <div class="card" style="grid-column:1/8;padding:20px">
        <div class="section-label">{editName ? 'Edit rule' : 'New rule'}</div>
        <div class="form-grid">
          <div class="field"><label class="field-label">Name</label><input class="input" bind:value={form.name} /></div>
          <div class="field"><label class="field-label">Severity</label>
            <select class="input" bind:value={form.severity}>
              <option value="info">Info</option><option value="warning">Warning</option>
              <option value="critical">Critical</option><option value="emergency">Emergency</option>
            </select>
          </div>
          <div class="field"><label class="field-label">Metric</label>
            <select class="input" bind:value={form.metric}>
              {#each ALL_METRICS as m}<option value={m}>{m}</option>{/each}
            </select>
          </div>
          <div class="field"><label class="field-label">Operator</label>
            <select class="input" bind:value={form.operator}>
              <option value=">">&gt;</option><option value="<">&lt;</option>
              <option value=">=">&gt;=</option><option value="<=">&lt;=</option>
            </select>
          </div>
          <div class="field"><label class="field-label">Threshold</label><input class="input" type="number" step="0.01" bind:value={form.threshold} /></div>
          <div class="field"><label class="field-label">Workload (optional)</label><input class="input" bind:value={form.host} placeholder="all" /></div>
        </div>
        <button class="btn btn-primary" disabled={saving} on:click={save}>{saving ? 'Saving…' : editName ? 'Update' : 'Create'}</button>
      </div>
    </div>
  {/if}

  {#if loading}
    <div class="band"><div class="loading" style="grid-column:1/-1"><div class="spinner"></div></div></div>
  {:else if rules.length === 0}
    <div class="band"><p class="empty-msg" style="grid-column:1/-1">No alert rules — create one to get notified.</p></div>
  {:else}
    <div class="band">
      <table class="data-table" style="grid-column:1/-1">
        <thead><tr><th>Name</th><th>Condition</th><th>Severity</th><th>Workload</th><th></th></tr></thead>
        <tbody>
          {#each rules as r}
            <tr class={SEV_CLASS[r.severity] || ''}>
              <td class="rule-name">{r.name}</td>
              <td class="mono">{r.metric} {r.operator} {r.threshold}</td>
              <td><span class="sev-badge sev-{r.severity}">{r.severity}</span></td>
              <td class="dim">{r.host || 'all'}</td>
              <td class="actions-cell">
                <button class="btn btn-ghost btn-sm" on:click={() => edit(r)}>Edit</button>
                <button class="btn btn-danger btn-sm" on:click={() => del(r.name)}>Delete</button>
              </td>
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
  .form-grid { display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-bottom:16px; }
  .field { display:flex; flex-direction:column; gap:5px; }
  .field-label { font-size:10px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; color:var(--text-3); }
  .input { background:var(--surface-2); border:1px solid var(--border-2); border-radius:4px; color:var(--text); padding:7px 10px; font-size:13px; font-family:inherit; width:100%; }
  .input:focus { outline:none; border-color:var(--accent); }
  .btn { display:inline-flex; align-items:center; gap:6px; padding:7px 14px; border-radius:4px; border:1px solid transparent; font-size:12px; font-weight:600; cursor:pointer; font-family:inherit; }
  .btn:disabled { opacity:.5; cursor:not-allowed; }
  .btn-primary { background:var(--accent); color:#000; }
  .btn-ghost { background:transparent; color:var(--text-2); border-color:var(--border-2); }
  .btn-ghost:hover { background:var(--surface-2); }
  .btn-danger { background:var(--red-dim); color:var(--red); border-color:rgba(244,63,94,.2); }
  .btn-sm { padding:4px 10px; font-size:11px; }
  .loading { display:flex; justify-content:center; padding:32px; }
  .spinner { width:20px; height:20px; border-radius:50%; border:2px solid var(--border-2); border-top-color:var(--accent); animation:spin .7s linear infinite; }
  @keyframes spin { to{transform:rotate(360deg)} }
  .empty-msg { color:var(--text-3); font-size:13px; padding:24px 0; }
  .data-table { width:100%; border-collapse:collapse; font-size:12px; }
  th { text-align:left; padding:6px 8px; font-size:10px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; color:var(--text-3); border-bottom:1px solid var(--border); }
  td { padding:7px 8px; border-bottom:1px solid var(--border); color:var(--text-2); vertical-align:middle; }
  tr.crit td { background:var(--red-dim); }
  tr.warn td { background:var(--amber-dim); }
  .rule-name { font-weight:600; color:var(--text); }
  .mono { font-family:"DM Mono",monospace; font-size:11px; font-variant-numeric:tabular-nums; }
  .dim { color:var(--text-3); font-size:11px; }
  .actions-cell { display:flex; gap:6px; white-space:nowrap; }
  .sev-badge { font-size:9px; font-weight:700; letter-spacing:0.06em; text-transform:uppercase; padding:2px 6px; border-radius:3px; }
  .sev-critical,.sev-emergency { background:var(--red-dim); color:var(--red); }
  .sev-warning { background:var(--amber-dim); color:var(--amber); }
  .sev-info { background:var(--surface-3); color:var(--text-3); }
</style>

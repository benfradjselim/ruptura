<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let statuses = []
  let loading  = true
  let error    = null
  let showForm = false
  let editingSLO = null
  let form = { name: '', description: '', metric: '', target: 99.9, window: '30d', comparator: 'lte', threshold: 0.8 }

  const windows     = ['7d', '30d', '90d']
  const comparators = [{ v: 'lte', label: '≤ threshold (lower is better, e.g. CPU%)' }, { v: 'gte', label: '≥ threshold (higher is better, e.g. health_score)' }]
  const kpiSuggestions = ['cpu_percent', 'memory_percent', 'disk_percent', 'error_rate', 'stress', 'fatigue', 'health_score', 'resilience', 'contagion']

  const stateColor = { healthy: '#22c55e', at_risk: '#f59e0b', breached: '#ef4444', no_data: '#475569' }
  const stateLabel = { healthy: 'Healthy', at_risk: 'At Risk', breached: 'Breached', no_data: 'No Data' }

  async function load() {
    loading = true; error = null
    try {
      const res = await api.slosStatus()
      statuses = res?.data || []
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  async function save() {
    try {
      if (editingSLO) {
        await api.sloUpdate(editingSLO.id, form)
      } else {
        await api.sloCreate(form)
      }
      resetForm()
      await load()
    } catch (e) { error = e.message }
  }

  async function del(id) {
    if (!confirm('Delete this SLO?')) return
    try {
      await api.sloDelete(id)
      await load()
    } catch (e) { error = e.message }
  }

  function startEdit(s) {
    editingSLO = s.slo
    form = { name: s.slo.name, description: s.slo.description || '', metric: s.slo.metric,
             target: s.slo.target, window: s.slo.window, comparator: s.slo.comparator, threshold: s.slo.threshold }
    showForm = true
  }

  function resetForm() {
    editingSLO = null
    form = { name: '', description: '', metric: '', target: 99.9, window: '30d', comparator: 'lte', threshold: 0.8 }
    showForm = false
  }

  onMount(load)
</script>

<div class="page">
  <div class="page-header">
    <div>
      <h1>Service Level Objectives</h1>
      <p class="subtitle">Define uptime targets, track error budgets and burn rates</p>
    </div>
    <button class="btn-primary" on:click={() => { resetForm(); showForm = !showForm }}>
      {showForm ? 'Cancel' : '+ New SLO'}
    </button>
  </div>

  {#if error}<div class="alert-error">{error}</div>{/if}

  {#if showForm}
    <div class="form-card">
      <h3>{editingSLO ? 'Edit SLO' : 'New SLO'}</h3>
      <div class="form-row">
        <label>Name <input bind:value={form.name} placeholder="API Availability" /></label>
        <label>Metric / KPI
          <input bind:value={form.metric} placeholder="e.g. error_rate, health_score" list="kpi-list"/>
          <datalist id="kpi-list">
            {#each kpiSuggestions as k}<option value={k}/>{/each}
          </datalist>
        </label>
      </div>
      <div class="form-row">
        <label>Target (%)
          <input type="number" bind:value={form.target} min="0" max="100" step="0.1" />
        </label>
        <label>Window
          <select bind:value={form.window}>
            {#each windows as w}<option value={w}>{w}</option>{/each}
          </select>
        </label>
        <label>Good state
          <select bind:value={form.comparator}>
            {#each comparators as c}<option value={c.v}>{c.label}</option>{/each}
          </select>
        </label>
        <label>Threshold
          <input type="number" bind:value={form.threshold} step="0.01" />
        </label>
      </div>
      <label>Description (optional)
        <input bind:value={form.description} placeholder="Brief description of this SLO" />
      </label>
      <div class="form-actions">
        <button class="btn-primary" on:click={save}>{editingSLO ? 'Save' : 'Create SLO'}</button>
        <button class="btn-ghost" on:click={resetForm}>Cancel</button>
      </div>
    </div>
  {/if}

  {#if loading}
    <div class="loading">Loading…</div>
  {:else if statuses.length === 0}
    <div class="empty">
      <p>No SLOs defined yet.</p>
      <p class="hint">Create your first SLO to start tracking error budgets and compliance.</p>
    </div>
  {:else}
    <div class="slo-grid">
      {#each statuses as s}
        {@const color = stateColor[s.state] || '#475569'}
        <div class="slo-card" style="border-color:{color}33">
          <div class="slo-card-header">
            <span class="slo-state-dot" style="background:{color}"></span>
            <div class="slo-card-title">
              <span class="slo-name">{s.slo.name}</span>
              <span class="slo-metric">{s.slo.metric} · {s.slo.window}</span>
            </div>
            <span class="state-badge" style="background:{color}22;color:{color};border-color:{color}44">
              {stateLabel[s.state] || s.state}
            </span>
            <div class="slo-actions">
              <button class="btn-icon" on:click={() => startEdit(s)} title="Edit">✎</button>
              <button class="btn-icon danger" on:click={() => del(s.slo.id)} title="Delete">✕</button>
            </div>
          </div>

          {#if s.state !== 'no_data'}
            <div class="slo-kpis">
              <div class="kpi-block">
                <div class="kpi-val" style="color:{color}">{s.compliance_pct?.toFixed(2)}%</div>
                <div class="kpi-lbl">Compliance</div>
                <div class="kpi-sub">target {s.slo.target}%</div>
              </div>
              <div class="kpi-block">
                <div class="kpi-val" style="color:{s.error_budget_pct > 20 ? '#22c55e' : s.error_budget_pct > 5 ? '#f59e0b' : '#ef4444'}">{s.error_budget_pct?.toFixed(1)}%</div>
                <div class="kpi-lbl">Error Budget</div>
                <div class="kpi-sub">{s.remaining_minutes?.toFixed(0)} min left</div>
              </div>
              <div class="kpi-block">
                <div class="kpi-val" style="color:{s.burn_rate < 1 ? '#22c55e' : s.burn_rate < 2 ? '#f59e0b' : '#ef4444'}">{s.burn_rate?.toFixed(2)}x</div>
                <div class="kpi-lbl">Burn Rate</div>
                <div class="kpi-sub">1x = steady</div>
              </div>
            </div>

            <div class="budget-bar-wrap">
              <div class="budget-bar-track">
                <div class="budget-bar-fill"
                  style="width:{Math.max(0,Math.min(100,s.error_budget_pct||0))}%;
                         background:{s.error_budget_pct>20?'#22c55e':s.error_budget_pct>5?'#f59e0b':'#ef4444'}">
                </div>
              </div>
              <span class="budget-bar-label">Error budget remaining</span>
            </div>
          {:else}
            <div class="no-data">No data collected yet for <code>{s.slo.metric}</code></div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .page        { padding:24px; max-width:1100px; }
  .page-header { display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:24px; }
  h1           { margin:0 0 4px; color:#e2e8f0; font-size:1.4rem; }
  .subtitle    { color:#64748b; font-size:0.85rem; margin:0; }

  .alert-error { background:#450a0a; border:1px solid #7f1d1d; color:#fca5a5; border-radius:6px; padding:12px 16px; margin-bottom:16px; font-size:0.85rem; }

  .form-card   { background:#1e293b; border:1px solid #334155; border-radius:8px; padding:20px; margin-bottom:24px; }
  .form-card h3 { margin:0 0 16px; color:#e2e8f0; font-size:1rem; }
  .form-row    { display:flex; gap:16px; flex-wrap:wrap; margin-bottom:12px; }
  label        { display:flex; flex-direction:column; flex:1; min-width:160px; color:#94a3b8; font-size:0.82rem; gap:4px; }
  input, select { padding:7px 10px; background:#0f172a; border:1px solid #334155; border-radius:5px; color:#e2e8f0; font-size:0.88rem; }
  input:focus, select:focus { border-color:#38bdf8; outline:none; }
  .form-actions { display:flex; gap:8px; margin-top:16px; }
  .btn-primary { background:#3b82f6; color:#fff; border:none; padding:8px 16px; border-radius:5px; cursor:pointer; font-size:0.85rem; }
  .btn-primary:hover { background:#2563eb; }
  .btn-ghost   { background:transparent; color:#94a3b8; border:1px solid #334155; padding:8px 16px; border-radius:5px; cursor:pointer; font-size:0.85rem; }

  .loading     { color:#64748b; padding:40px; text-align:center; }
  .empty       { color:#64748b; padding:40px; text-align:center; }
  .empty p     { margin:0 0 8px; }
  .hint        { font-size:0.82rem; color:#334155; }

  .slo-grid    { display:grid; grid-template-columns:repeat(auto-fill,minmax(340px,1fr)); gap:16px; }

  .slo-card    { background:#1e293b; border:1px solid #334155; border-radius:10px; padding:16px; display:flex; flex-direction:column; gap:12px; transition:border-color 0.2s; }
  .slo-card:hover { border-color:#475569; }

  .slo-card-header { display:flex; align-items:center; gap:8px; }
  .slo-state-dot   { width:10px; height:10px; border-radius:50%; flex-shrink:0; }
  .slo-card-title  { flex:1; min-width:0; }
  .slo-name        { display:block; font-size:0.9rem; font-weight:700; color:#e2e8f0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .slo-metric      { font-size:0.7rem; color:#64748b; }
  .state-badge     { font-size:0.68rem; font-weight:700; padding:2px 8px; border-radius:20px; border:1px solid; white-space:nowrap; flex-shrink:0; }

  .slo-actions { display:flex; gap:4px; }
  .btn-icon    { background:none; border:none; cursor:pointer; padding:3px 7px; color:#64748b; border-radius:4px; }
  .btn-icon:hover { background:#334155; color:#e2e8f0; }
  .btn-icon.danger:hover { color:#ef4444; }

  .slo-kpis    { display:flex; gap:8px; }
  .kpi-block   { flex:1; background:#0f172a; border-radius:6px; padding:10px 6px; text-align:center; }
  .kpi-val     { font-size:1.25rem; font-weight:700; line-height:1; }
  .kpi-lbl     { font-size:0.67rem; color:#64748b; margin-top:3px; }
  .kpi-sub     { font-size:0.6rem; color:#334155; margin-top:2px; }

  .budget-bar-wrap  { }
  .budget-bar-track { height:5px; background:#0f172a; border-radius:3px; overflow:hidden; }
  .budget-bar-fill  { height:100%; border-radius:3px; transition:width 0.5s; }
  .budget-bar-label { font-size:0.65rem; color:#334155; margin-top:3px; display:block; }

  .no-data { color:#475569; font-size:0.82rem; text-align:center; padding:12px 0; }
  code     { background:#0f172a; padding:1px 5px; border-radius:3px; font-size:0.8rem; }
</style>

<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { SEVERITY_COLOR } from '../lib/util/format.js'

  const KPI_OPTIONS  = ['stress','fatigue','mood','pressure','humidity','contagion',
                        'resilience','entropy','velocity','health_score']
  const MET_OPTIONS  = ['cpu_percent','memory_percent','disk_percent','net_rx_bps','net_tx_bps',
                        'request_rate','error_rate','timeout_rate','uptime_seconds']
  const ALL_METRICS  = [...KPI_OPTIONS, ...MET_OPTIONS]

  let rules   = []
  let loading = true
  let error   = ''
  let saving  = false

  let showForm = false
  let form = newForm()
  let editName = null   // non-null → editing existing

  function newForm() {
    return { name: '', metric: '', threshold: 0, severity: 'warning', message: '' }
  }

  async function load() {
    loading = true; error = ''
    try {
      const res = await api.alertRules()
      rules = (res?.data || []).map(r => ({
        Name:      r.Name      || r.name,
        Metric:    r.Metric    || r.metric,
        Threshold: r.Threshold ?? r.threshold,
        Severity:  r.Severity  || r.severity,
        Message:   r.Message   || r.message,
      }))
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  async function saveRule() {
    saving = true; error = ''
    try {
      const payload = {
        name:      form.name,
        metric:    form.metric,
        threshold: parseFloat(form.threshold),
        severity:  form.severity,
        message:   form.message,
      }
      if (editName) {
        await api.alertRuleUpdate(editName, payload)
      } else {
        await api.alertRuleCreate(payload)
      }
      showForm = false; editName = null; form = newForm()
      await load()
    } catch (e) { error = e.message }
    finally { saving = false }
  }

  async function deleteRule(name) {
    if (!confirm(`Delete rule "${name}"?`)) return
    try {
      await api.alertRuleDelete(name)
      await load()
    } catch (e) { error = e.message }
  }

  function startEdit(rule) {
    editName = rule.Name
    form = { name: rule.Name, metric: rule.Metric, threshold: rule.Threshold, severity: rule.Severity, message: rule.Message }
    showForm = true
  }

  function cancelForm() {
    showForm = false; editName = null; form = newForm()
  }

  onMount(load)
</script>

<div class="page">
  <div class="header">
    <div>
      <h1>Alert Rules</h1>
      <p class="sub">Define metric thresholds that trigger alerts</p>
    </div>
    <button class="btn-add" on:click={() => { cancelForm(); showForm = !showForm }}>
      {showForm && !editName ? '✕ Cancel' : '+ New Rule'}
    </button>
  </div>

  {#if error}<div class="err-bar">{error}</div>{/if}

  {#if showForm}
    <div class="form-card">
      <h3>{editName ? 'Edit Rule' : 'New Alert Rule'}</h3>
      <div class="form-grid">
        <label class="field">
          <span>Name *</span>
          <input type="text" bind:value={form.name} placeholder="e.g. high_cpu" />
        </label>
        <label class="field">
          <span>Metric *</span>
          <input list="metric-opts" type="text" bind:value={form.metric} placeholder="metric or kpi name" />
          <datalist id="metric-opts">
            {#each ALL_METRICS as m}<option value={m}></option>{/each}
          </datalist>
        </label>
        <label class="field">
          <span>Threshold</span>
          <input type="number" step="any" bind:value={form.threshold} />
        </label>
        <label class="field">
          <span>Severity</span>
          <select bind:value={form.severity}>
            <option value="info">Info</option>
            <option value="warning">Warning</option>
            <option value="critical">Critical</option>
            <option value="emergency">Emergency</option>
          </select>
        </label>
        <label class="field wide">
          <span>Message</span>
          <input type="text" bind:value={form.message} placeholder="Alert message (optional)" />
        </label>
      </div>
      <div class="form-actions">
        <button class="btn-ghost" on:click={cancelForm}>Cancel</button>
        <button class="btn-save" on:click={saveRule} disabled={!form.name || !form.metric || saving}>
          {saving ? 'Saving…' : editName ? 'Update Rule' : 'Create Rule'}
        </button>
      </div>
    </div>
  {/if}

  {#if loading}
    <p class="muted">Loading rules…</p>
  {:else if !rules.length}
    <div class="empty">
      <p>No custom alert rules. OHE default rules are always active.</p>
    </div>
  {:else}
    <div class="rules-table">
      <div class="rules-head">
        <span>Name</span>
        <span>Metric</span>
        <span>Threshold</span>
        <span>Severity</span>
        <span>Message</span>
        <span></span>
      </div>
      {#each rules as rule}
        <div class="rule-row">
          <span class="rule-name">{rule.Name}</span>
          <code class="rule-metric">{rule.Metric}</code>
          <span class="rule-thresh">≥ {rule.Threshold}</span>
          <span class="sev-badge" style="color: {SEVERITY_COLOR[rule.Severity] ?? '#94a3b8'}">
            {rule.Severity}
          </span>
          <span class="rule-msg">{rule.Message || '—'}</span>
          <div class="rule-actions">
            <button on:click={() => startEdit(rule)}>✏ Edit</button>
            <button class="del" on:click={() => deleteRule(rule.Name)}>✕</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .page   { display: flex; flex-direction: column; gap: 1rem; }
  .header { display: flex; align-items: flex-start; justify-content: space-between; flex-wrap: wrap; gap: 0.5rem; }
  h1 { margin: 0; font-size: 1.2rem; color: #e2e8f0; }
  .sub { margin: 2px 0 0; font-size: 0.8rem; color: #64748b; }

  .btn-add {
    background: #0284c7; border: none; color: #fff;
    padding: 7px 16px; border-radius: 6px; cursor: pointer; font-size: 0.85rem; font-weight: 600;
  }
  .err-bar { background: #7f1d1d; border: 1px solid #ef4444; color: #fca5a5; padding: 8px 14px; border-radius: 6px; font-size: 0.82rem; }

  .form-card {
    background: #1e293b; border: 1px solid #334155; border-radius: 10px; padding: 1.25rem 1.5rem;
  }
  .form-card h3 { margin: 0 0 1rem; font-size: 0.95rem; color: #e2e8f0; }
  .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .field { display: flex; flex-direction: column; gap: 4px; }
  .field.wide { grid-column: 1 / -1; }
  .field span { font-size: 0.75rem; color: #64748b; }
  .field input, .field select {
    background: #0f172a; border: 1px solid #334155; border-radius: 6px;
    color: #e2e8f0; padding: 7px 10px; font-size: 0.85rem;
  }
  .field input:focus, .field select:focus { border-color: #38bdf8; outline: none; }
  .form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 1rem; }
  .btn-ghost { background: transparent; border: 1px solid #334155; color: #94a3b8; padding: 7px 16px; border-radius: 6px; cursor: pointer; }
  .btn-save  { background: #0284c7; border: none; color: #fff; padding: 7px 18px; border-radius: 6px; cursor: pointer; font-weight: 600; }
  .btn-save:disabled { opacity: 0.5; cursor: not-allowed; }

  .empty, .muted { color: #64748b; padding: 1.5rem 0; font-size: 0.9rem; }

  .rules-table { display: flex; flex-direction: column; border: 1px solid #334155; border-radius: 8px; overflow: hidden; }
  .rules-head  { display: grid; grid-template-columns: 1.2fr 1fr 0.8fr 0.8fr 2fr 0.8fr; padding: 8px 14px; background: #0f172a; font-size: 0.72rem; color: #475569; text-transform: uppercase; letter-spacing: 0.05em; gap: 8px; }
  .rule-row    { display: grid; grid-template-columns: 1.2fr 1fr 0.8fr 0.8fr 2fr 0.8fr; padding: 10px 14px; border-top: 1px solid #1e293b; align-items: center; gap: 8px; font-size: 0.82rem; }
  .rule-row:hover { background: #1e293b; }
  .rule-name  { color: #e2e8f0; font-weight: 600; }
  .rule-metric{ color: #38bdf8; background: #0f3460; padding: 2px 6px; border-radius: 4px; font-size: 0.78rem; }
  .rule-thresh{ color: #94a3b8; }
  .sev-badge  { font-size: 0.78rem; font-weight: 600; text-transform: capitalize; }
  .rule-msg   { color: #64748b; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .rule-actions { display: flex; gap: 4px; }
  .rule-actions button { background: transparent; border: 1px solid #334155; color: #64748b; padding: 3px 8px; border-radius: 4px; cursor: pointer; font-size: 0.75rem; }
  .rule-actions button:hover { background: #334155; color: #e2e8f0; }
  .rule-actions .del:hover { background: #7f1d1d; border-color: #ef4444; color: #fca5a5; }
</style>

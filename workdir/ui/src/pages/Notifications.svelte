<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let channels = [], loading = true, error = null, showForm = false, testing = {}
  let form = { name: '', type: 'webhook', url: '', severities: [] }
  const allSeverities = ['info','warning','critical','emergency']
  const channelTypes = ['webhook','slack','pagerduty']

  async function load() {
    try { const r = await api.notifications(); channels = r.data || [] }
    catch(e) { error = e.message }
    finally { loading = false }
  }

  async function save() {
    await api.notificationCreate(form).catch(() => {})
    showForm = false; form = { name:'', type:'webhook', url:'', severities:[] }; load()
  }

  async function del(id) {
    await api.notificationDelete(id).catch(() => {})
    load()
  }

  async function test(id) {
    testing = { ...testing, [id]: true }
    await api.notificationTest(id).catch(() => {})
    testing = { ...testing, [id]: false }
  }

  function toggleSev(s) {
    form.severities = form.severities.includes(s)
      ? form.severities.filter(x => x !== s)
      : [...form.severities, s]
  }

  onMount(load)
</script>

<div class="page-wrap">
  <div class="band page-header">
    <div style="grid-column:1/9"><h1 class="page-title">Alert Channels</h1></div>
    <div style="grid-column:9/13; display:flex; justify-content:flex-end">
      <button class="btn btn-primary btn-sm" on:click={() => showForm=!showForm}>
        {showForm ? 'Cancel' : '+ Add channel'}
      </button>
    </div>
  </div>

  {#if showForm}
    <div class="band">
      <div class="form-card" style="grid-column:1/8">
        <div class="section-label">New channel</div>
        <div class="form-grid">
          <div class="field"><label>Name</label><input class="input" bind:value={form.name} placeholder="My webhook" /></div>
          <div class="field"><label>Type</label>
            <select class="input" bind:value={form.type}>
              {#each channelTypes as t}<option value={t}>{t}</option>{/each}
            </select>
          </div>
          <div class="field full"><label>URL / Integration key</label><input class="input" bind:value={form.url} placeholder="https://…" /></div>
          <div class="field full">
            <label>Severities</label>
            <div class="sev-checks">
              {#each allSeverities as s}
                <label class="check-label">
                  <input type="checkbox" checked={form.severities.includes(s)} on:change={() => toggleSev(s)} />
                  {s}
                </label>
              {/each}
            </div>
          </div>
        </div>
        <button class="btn btn-primary" on:click={save}>Save channel</button>
      </div>
    </div>
  {/if}

  {#if loading}
    <div class="loading band"><div class="spinner"></div></div>
  {:else if channels.length === 0}
    <div class="band"><div style="grid-column:1/-1" class="empty-state">
      <div class="empty-icon">🔔</div>
      <p>No channels yet — add Slack, PagerDuty, or a webhook to receive alerts.</p>
    </div></div>
  {:else}
    <div class="band channels-grid">
      {#each channels as ch}
        <div class="channel-card">
          <div class="ch-head">
            <span class="ch-name">{ch.name}</span>
            <span class="ch-type">{ch.type}</span>
          </div>
          <div class="ch-sevs">
            {#each (ch.severities || []) as s}
              <span class="sev-badge sev-{s}">{s}</span>
            {/each}
          </div>
          <div class="ch-actions">
            <button class="btn btn-ghost btn-sm" disabled={testing[ch.id]} on:click={() => test(ch.id)}>
              {testing[ch.id] ? 'Testing…' : 'Test'}
            </button>
            <button class="btn btn-danger btn-sm" on:click={() => del(ch.id)}>Delete</button>
          </div>
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
  .section-label { font-size:10px; font-weight:700; letter-spacing:0.10em; text-transform:uppercase; color:var(--text-3); margin-bottom:16px; }
  .form-card { background:var(--surface); border:1px solid var(--border); border-radius:4px; padding:20px; }
  .form-grid { display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-bottom:16px; }
  .field { display:flex; flex-direction:column; gap:6px; }
  .field.full { grid-column:1/-1; }
  label { font-size:10px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; color:var(--text-3); }
  .input { background:var(--surface-2); border:1px solid var(--border-2); border-radius:4px; color:var(--text); padding:7px 10px; font-size:13px; font-family:inherit; }
  .input:focus { outline:none; border-color:var(--accent); }
  .sev-checks { display:flex; gap:12px; flex-wrap:wrap; }
  .check-label { display:flex; align-items:center; gap:6px; font-size:12px; color:var(--text-2); cursor:pointer; text-transform:none; letter-spacing:0; }

  .btn { display:inline-flex; align-items:center; gap:6px; padding:7px 14px; border-radius:4px; border:1px solid transparent; font-size:12px; font-weight:600; cursor:pointer; font-family:inherit; }
  .btn:disabled { opacity:.5; cursor:not-allowed; }
  .btn-primary { background:var(--accent); color:#000; }
  .btn-ghost { background:transparent; color:var(--text-2); border-color:var(--border-2); }
  .btn-ghost:hover { background:var(--surface-2); }
  .btn-danger { background:var(--red-dim); color:var(--red); border-color:rgba(244,63,94,.2); }
  .btn-sm { padding:4px 10px; font-size:11px; }

  .channels-grid { grid-column:1/-1; display:grid; grid-template-columns:repeat(auto-fill,minmax(280px,1fr)); gap:12px; }
  .channel-card { background:var(--surface); border:1px solid var(--border); border-radius:4px; padding:16px; display:flex; flex-direction:column; gap:12px; }
  .ch-head { display:flex; align-items:center; gap:10px; }
  .ch-name { font-size:14px; font-weight:600; color:var(--text); }
  .ch-type { font-size:10px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase; background:var(--surface-3); color:var(--text-3); padding:2px 7px; border-radius:3px; }
  .ch-sevs { display:flex; gap:6px; flex-wrap:wrap; min-height:20px; }
  .sev-badge { font-size:9px; font-weight:700; letter-spacing:0.06em; text-transform:uppercase; padding:2px 6px; border-radius:3px; }
  .sev-critical,.sev-emergency { background:var(--red-dim); color:var(--red); }
  .sev-warning { background:var(--amber-dim); color:var(--amber); }
  .sev-info { background:var(--surface-3); color:var(--text-3); }
  .ch-actions { display:flex; gap:8px; }

  .loading { padding:64px; display:flex; justify-content:center; grid-column:1/-1; }
  .spinner { width:20px; height:20px; border-radius:50%; border:2px solid var(--border-2); border-top-color:var(--accent); animation:spin .7s linear infinite; }
  @keyframes spin { to{transform:rotate(360deg)} }
  .empty-state { display:flex; flex-direction:column; align-items:center; gap:12px; padding:48px; text-align:center; color:var(--text-3); }
  .empty-icon { font-size:2rem; opacity:.4; }
  .empty-state p { font-size:13px; max-width:320px; line-height:1.6; }
</style>

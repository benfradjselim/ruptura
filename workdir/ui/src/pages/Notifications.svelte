<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let channels = []
  let error = null
  let loading = true
  let showForm = false
  let testing = {}
  let form = { name: '', type: 'webhook', url: '', severities: [] }

  const allSeverities = ['info', 'warning', 'critical', 'emergency']
  const channelTypes = ['webhook', 'slack', 'pagerduty']

  async function load() {
    try {
      const res = await api.notifications()
      channels = res.data || []
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  async function create() {
    try {
      const payload = { ...form }
      if (payload.severities.length === 0) delete payload.severities
      await api.notificationCreate(payload)
      form = { name: '', type: 'webhook', url: '', severities: [] }
      showForm = false
      await load()
    } catch (e) {
      error = e.message
    }
  }

  async function del(id) {
    if (!confirm('Delete this notification channel?')) return
    try {
      await api.notificationDelete(id)
      await load()
    } catch (e) {
      error = e.message
    }
  }

  async function test(id) {
    testing = { ...testing, [id]: 'sending' }
    try {
      const res = await api.notificationTest(id)
      testing = { ...testing, [id]: res.data?.status === 'ok' ? 'ok' : 'error' }
    } catch (e) {
      testing = { ...testing, [id]: 'error' }
    }
    setTimeout(() => { const t = { ...testing }; delete t[id]; testing = t }, 4000)
  }

  function toggleSeverity(s) {
    if (form.severities.includes(s)) {
      form.severities = form.severities.filter(x => x !== s)
    } else {
      form.severities = [...form.severities, s]
    }
  }

  onMount(load)
</script>

<div class="page">
  <div class="page-header">
    <div>
      <h1>Notification Channels</h1>
      <p class="subtitle">Route alerts to Slack, webhooks, or PagerDuty</p>
    </div>
    <button class="btn-primary" on:click={() => showForm = !showForm}>
      {showForm ? 'Cancel' : '+ Add Channel'}
    </button>
  </div>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  {#if showForm}
    <div class="form-card">
      <h3>New Notification Channel</h3>
      <div class="form-row">
        <label>Name
          <input bind:value={form.name} placeholder="e.g. Ops Slack" />
        </label>
        <label>Type
          <select bind:value={form.type}>
            {#each channelTypes as t}
              <option value={t}>{t}</option>
            {/each}
          </select>
        </label>
      </div>
      <div class="form-row">
        <label class="full">
          {form.type === 'slack' ? 'Slack Incoming Webhook URL' : form.type === 'pagerduty' ? 'PagerDuty Events API URL' : 'Webhook URL'}
          <input bind:value={form.url} placeholder="https://..." />
        </label>
      </div>
      <div class="severity-filter">
        <span class="filter-label">Severity filter (empty = all):</span>
        {#each allSeverities as s}
          <button
            class="sev-btn {s}"
            class:active={form.severities.includes(s)}
            on:click={() => toggleSeverity(s)}
          >{s}</button>
        {/each}
      </div>
      <button class="btn-primary" on:click={create} disabled={!form.name || !form.url}>
        Create Channel
      </button>
    </div>
  {/if}

  {#if loading}
    <div class="loading">Loading...</div>
  {:else if channels.length === 0}
    <div class="empty">No notification channels configured. Add one to receive alert notifications via webhook or Slack.</div>
  {:else}
    <div class="channel-list">
      {#each channels as ch}
        <div class="channel-card">
          <div class="ch-header">
            <div>
              <span class="ch-name">{ch.name}</span>
              <span class="ch-type">{ch.type}</span>
              <span class="ch-status" class:enabled={ch.enabled}>{ch.enabled ? 'enabled' : 'disabled'}</span>
            </div>
            <div class="ch-actions">
              <button
                class="btn-test"
                class:ok={testing[ch.id] === 'ok'}
                class:err={testing[ch.id] === 'error'}
                on:click={() => test(ch.id)}
                disabled={testing[ch.id] === 'sending'}
              >
                {testing[ch.id] === 'sending' ? 'Sending...' : testing[ch.id] === 'ok' ? 'OK' : testing[ch.id] === 'error' ? 'Failed' : 'Test'}
              </button>
              <button class="btn-delete" on:click={() => del(ch.id)}>Delete</button>
            </div>
          </div>
          <div class="ch-url">{ch.url}</div>
          {#if ch.severities && ch.severities.length > 0}
            <div class="ch-severities">
              {#each ch.severities as s}
                <span class="sev-tag {s}">{s}</span>
              {/each}
            </div>
          {:else}
            <div class="ch-severities"><span class="all-tag">all severities</span></div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  <div class="info-box">
    <h4>Integration Guide</h4>
    <ul>
      <li><strong>Webhook</strong> — receives JSON POST with alert payload. Compatible with any HTTP endpoint.</li>
      <li><strong>Slack</strong> — point to a Slack Incoming Webhook URL. OHE formats a text message automatically.</li>
      <li><strong>PagerDuty</strong> — point to PagerDuty Events API v2. Set <code>routing_key</code> in the Headers field.</li>
    </ul>
  </div>
</div>

<style>
  .page { max-width: 900px; }
  .page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1.5rem; }
  h1 { font-size: 1.5rem; font-weight: 700; color: #f1f5f9; margin-bottom: 0.25rem; }
  .subtitle { color: #64748b; font-size: 0.9rem; }

  .loading, .empty { padding: 2rem; text-align: center; color: #64748b; }
  .error { background: #ef444420; border: 1px solid #ef4444; color: #ef4444; padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; }

  .btn-primary {
    background: #0284c7; color: #fff; border: none; padding: 0.6rem 1.2rem;
    border-radius: 6px; cursor: pointer; font-size: 0.9rem; font-weight: 600;
  }
  .btn-primary:hover { background: #0369a1; }
  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

  .form-card {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 10px;
    padding: 1.5rem;
    margin-bottom: 1.5rem;
  }
  .form-card h3 { color: #f1f5f9; margin-bottom: 1rem; }
  .form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1rem; }
  label { display: flex; flex-direction: column; gap: 0.4rem; color: #94a3b8; font-size: 0.85rem; }
  label.full { grid-column: 1 / -1; }
  input, select {
    background: #0f172a; border: 1px solid #334155; color: #f1f5f9;
    padding: 0.5rem 0.75rem; border-radius: 6px; font-size: 0.9rem;
  }

  .severity-filter { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 1rem; }
  .filter-label { color: #64748b; font-size: 0.8rem; }
  .sev-btn {
    padding: 0.25rem 0.75rem; border-radius: 12px; border: 1px solid #334155;
    background: transparent; cursor: pointer; font-size: 0.75rem; font-weight: 600;
    color: #64748b; transition: all 0.1s;
  }
  .sev-btn.info { color: #38bdf8; }
  .sev-btn.warning { color: #eab308; }
  .sev-btn.critical { color: #f97316; }
  .sev-btn.emergency { color: #ef4444; }
  .sev-btn.active { background: #334155; border-color: currentColor; }

  .channel-list { display: flex; flex-direction: column; gap: 0.75rem; margin-bottom: 1.5rem; }
  .channel-card {
    background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 1rem 1.25rem;
  }
  .ch-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.4rem; }
  .ch-name { font-weight: 700; color: #f1f5f9; margin-right: 0.5rem; }
  .ch-type { font-size: 0.75rem; color: #38bdf8; background: #38bdf820; padding: 0.15rem 0.5rem; border-radius: 4px; margin-right: 0.5rem; }
  .ch-status { font-size: 0.75rem; color: #ef4444; }
  .ch-status.enabled { color: #22c55e; }
  .ch-url { font-size: 0.8rem; color: #64748b; font-family: monospace; margin-bottom: 0.5rem; word-break: break-all; }
  .ch-severities { display: flex; gap: 0.4rem; flex-wrap: wrap; }
  .sev-tag { font-size: 0.7rem; font-weight: 600; padding: 0.15rem 0.4rem; border-radius: 4px; }
  .sev-tag.info { background: #38bdf820; color: #38bdf8; }
  .sev-tag.warning { background: #eab30820; color: #eab308; }
  .sev-tag.critical { background: #f9731620; color: #f97316; }
  .sev-tag.emergency { background: #ef444420; color: #ef4444; }
  .all-tag { font-size: 0.7rem; color: #475569; }

  .ch-actions { display: flex; gap: 0.5rem; }
  .btn-test {
    padding: 0.3rem 0.75rem; border-radius: 5px; border: 1px solid #38bdf840;
    background: #38bdf810; color: #38bdf8; cursor: pointer; font-size: 0.8rem;
  }
  .btn-test.ok { background: #22c55e10; border-color: #22c55e40; color: #22c55e; }
  .btn-test.err { background: #ef444410; border-color: #ef444440; color: #ef4444; }
  .btn-delete {
    padding: 0.3rem 0.75rem; border-radius: 5px; border: 1px solid #ef444430;
    background: #ef444410; color: #ef4444; cursor: pointer; font-size: 0.8rem;
  }

  .info-box {
    background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 1.25rem;
    margin-top: 1rem;
  }
  .info-box h4 { color: #94a3b8; margin-bottom: 0.75rem; font-size: 0.9rem; }
  .info-box ul { list-style: disc; padding-left: 1.5rem; display: flex; flex-direction: column; gap: 0.4rem; }
  .info-box li { color: #64748b; font-size: 0.85rem; line-height: 1.4; }
  .info-box strong { color: #94a3b8; }
  .info-box code { background: #0f172a; padding: 0.1rem 0.4rem; border-radius: 3px; font-size: 0.8rem; color: #38bdf8; }
</style>

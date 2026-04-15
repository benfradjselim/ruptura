<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  const DS_TYPES = [
    { value: 'prometheus',     label: 'Prometheus',     icon: '🔥' },
    { value: 'otlp',           label: 'OTLP',           icon: '📡' },
    { value: 'statsd',         label: 'DogStatsD',      icon: '🐕' },
    { value: 'loki',           label: 'Loki',           icon: '📋' },
    { value: 'elasticsearch',  label: 'Elasticsearch',  icon: '🔍' },
    { value: 'datadog',        label: 'Datadog Agent',  icon: '🐾' },
    { value: 'ohe',            label: 'OHE Agent',      icon: '⬡' },
  ]

  let sources  = []
  let loading  = true
  let error    = ''
  let selected = null   // datasource object being viewed/edited
  let isNew    = false
  let saving   = false
  let testResult = null   // {ok, latency_ms, error}

  let form = newForm()

  function newForm() {
    return { name: '', type: 'prometheus', url: '', scrape_interval: 15, auth_type: 'none', username: '', password: '', token: '' }
  }

  async function load() {
    loading = true; error = ''
    try {
      const res = await api.datasources()
      sources = res?.data || []
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  function selectDs(ds) {
    selected = ds
    isNew    = false
    form = { name: ds.name || '', type: ds.type || 'prometheus', url: ds.url || '',
             scrape_interval: ds.scrape_interval || 15, auth_type: ds.auth_type || 'none',
             username: ds.username || '', password: '', token: '' }
    testResult = null
  }

  function startNew() {
    selected = null; isNew = true; form = newForm(); testResult = null
  }

  async function save() {
    saving = true; error = ''
    try {
      const payload = { ...form }
      if (isNew) {
        await api.datasourceCreate(payload)
      } else {
        await api.datasourceUpdate(selected.id, payload)
      }
      isNew = false; selected = null; form = newForm()
      await load()
    } catch (e) { error = e.message }
    finally { saving = false }
  }

  async function del(id) {
    if (!confirm('Delete this datasource?')) return
    try {
      await api.datasourceDelete(id)
      selected = null; await load()
    } catch (e) { error = e.message }
  }

  async function testDs() {
    if (!selected?.id) return
    testResult = null; error = ''
    try {
      const res = await api.datasourceTest(selected.id)
      testResult = res?.data || { ok: true }
    } catch (e) {
      testResult = { ok: false, error: e.message }
    }
  }

  onMount(load)

  $: typeInfo = DS_TYPES.find(t => t.value === form.type) ?? DS_TYPES[0]
</script>

<div class="ds-layout">
  <!-- Left sidebar: list -->
  <aside class="ds-sidebar">
    <div class="ds-sidebar-header">
      <h2>Data Sources</h2>
      <button class="btn-add" on:click={startNew} title="Add datasource">+</button>
    </div>

    {#if loading}
      <div class="ds-msg">Loading…</div>
    {:else if !sources.length}
      <div class="ds-msg muted">No sources configured</div>
    {:else}
      <div class="ds-list">
        {#each sources as ds}
          {@const ti = DS_TYPES.find(t => t.value === ds.type) ?? DS_TYPES[0]}
          <button
            class="ds-item"
            class:active={selected?.id === ds.id}
            on:click={() => selectDs(ds)}
          >
            <span class="ds-icon">{ti.icon}</span>
            <div class="ds-item-info">
              <span class="ds-name">{ds.name}</span>
              <span class="ds-type">{ti.label}</span>
            </div>
          </button>
        {/each}
      </div>
    {/if}
  </aside>

  <!-- Right panel: form -->
  <div class="ds-main">
    {#if error}<div class="err-bar">{error}</div>{/if}

    {#if !selected && !isNew}
      <div class="ds-empty">
        <div class="ds-empty-icon">📡</div>
        <h3>Configure Data Sources</h3>
        <p>Connect OHE to external data sources — Prometheus scrapers, OTLP collectors, Loki, Elasticsearch, and more.</p>
        <button class="btn-primary" on:click={startNew}>+ Add Data Source</button>
      </div>
    {:else}
      <div class="ds-form">
        <div class="ds-form-header">
          <span class="ds-form-icon">{typeInfo.icon}</span>
          <h3>{isNew ? 'New Data Source' : (selected?.name ?? 'Edit')}</h3>
          {#if !isNew && selected}
            <div class="ds-form-actions">
              <button class="btn-test" on:click={testDs}>Test Connection</button>
              <button class="btn-del"  on:click={() => del(selected.id)}>Delete</button>
            </div>
          {/if}
        </div>

        {#if testResult}
          <div class="test-result" class:ok={testResult.ok} class:fail={!testResult.ok}>
            {#if testResult.ok}
              ✓ Connection OK
              {#if testResult.latency_ms} — {testResult.latency_ms}ms {/if}
            {:else}
              ✕ {testResult.error || 'Connection failed'}
            {/if}
          </div>
        {/if}

        <div class="form-grid">
          <label class="field">
            <span>Name *</span>
            <input type="text" bind:value={form.name} placeholder="My Prometheus" />
          </label>
          <label class="field">
            <span>Type *</span>
            <select bind:value={form.type}>
              {#each DS_TYPES as t}
                <option value={t.value}>{t.icon} {t.label}</option>
              {/each}
            </select>
          </label>
          <label class="field wide">
            <span>URL *</span>
            <input type="url" bind:value={form.url} placeholder="http://prometheus:9090" />
          </label>
          <label class="field">
            <span>Scrape interval (s)</span>
            <input type="number" min="5" max="300" bind:value={form.scrape_interval} />
          </label>
          <label class="field">
            <span>Auth type</span>
            <select bind:value={form.auth_type}>
              <option value="none">None</option>
              <option value="basic">Basic Auth</option>
              <option value="bearer">Bearer Token</option>
            </select>
          </label>
          {#if form.auth_type === 'basic'}
            <label class="field">
              <span>Username</span>
              <input type="text" bind:value={form.username} />
            </label>
            <label class="field">
              <span>Password</span>
              <input type="password" bind:value={form.password} />
            </label>
          {/if}
          {#if form.auth_type === 'bearer'}
            <label class="field wide">
              <span>Bearer token</span>
              <input type="password" bind:value={form.token} />
            </label>
          {/if}
        </div>

        <div class="form-actions">
          <button class="btn-ghost" on:click={() => { selected = null; isNew = false }}>Cancel</button>
          <button class="btn-save" on:click={save} disabled={!form.name || !form.url || saving}>
            {saving ? 'Saving…' : isNew ? 'Add Data Source' : 'Update'}
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .ds-layout {
    display: flex; gap: 1rem; height: calc(100vh - 120px); min-height: 400px;
  }

  /* ─ Sidebar ─ */
  .ds-sidebar {
    width: 240px; flex-shrink: 0;
    background: #1e293b; border: 1px solid #334155; border-radius: 10px;
    display: flex; flex-direction: column; overflow: hidden;
  }
  .ds-sidebar-header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 14px 14px 10px; border-bottom: 1px solid #334155;
  }
  .ds-sidebar-header h2 { margin: 0; font-size: 0.9rem; color: #94a3b8; text-transform: uppercase; letter-spacing: 0.05em; }
  .btn-add { background: #0284c7; border: none; color: #fff; width: 26px; height: 26px; border-radius: 6px; cursor: pointer; font-size: 1.1rem; display: flex; align-items: center; justify-content: center; }

  .ds-list { flex: 1; overflow-y: auto; padding: 6px; }
  .ds-item {
    display: flex; align-items: center; gap: 10px; width: 100%;
    background: transparent; border: 1px solid transparent; border-radius: 6px;
    color: #94a3b8; padding: 8px 10px; cursor: pointer; text-align: left; margin-bottom: 2px;
  }
  .ds-item:hover  { background: #0f172a; border-color: #334155; }
  .ds-item.active { background: #0f3460; border-color: #0284c7; color: #e2e8f0; }
  .ds-icon { font-size: 1.1rem; flex-shrink: 0; }
  .ds-item-info { display: flex; flex-direction: column; gap: 1px; overflow: hidden; }
  .ds-name { font-size: 0.85rem; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .ds-type { font-size: 0.7rem; color: #475569; }

  .ds-msg { padding: 16px; font-size: 0.82rem; color: #475569; text-align: center; }
  .ds-msg.muted { color: #334155; }

  /* ─ Main panel ─ */
  .ds-main { flex: 1; overflow-y: auto; }
  .err-bar { background: #7f1d1d; border: 1px solid #ef4444; color: #fca5a5; padding: 8px 14px; border-radius: 6px; font-size: 0.82rem; margin-bottom: 1rem; }

  .ds-empty {
    display: flex; flex-direction: column; align-items: center; justify-content: center;
    text-align: center; padding: 4rem 2rem; gap: 1rem; color: #64748b;
    background: #1e293b; border: 1px solid #334155; border-radius: 10px;
  }
  .ds-empty-icon { font-size: 3rem; }
  .ds-empty h3 { margin: 0; color: #94a3b8; font-size: 1rem; }
  .ds-empty p  { margin: 0; font-size: 0.85rem; max-width: 360px; }
  .btn-primary { background: #0284c7; border: none; color: #fff; padding: 8px 20px; border-radius: 6px; cursor: pointer; font-size: 0.85rem; font-weight: 600; }

  .ds-form { background: #1e293b; border: 1px solid #334155; border-radius: 10px; padding: 1.5rem; }
  .ds-form-header { display: flex; align-items: center; gap: 10px; margin-bottom: 1.25rem; }
  .ds-form-icon { font-size: 1.4rem; }
  .ds-form-header h3 { margin: 0; font-size: 1rem; color: #e2e8f0; flex: 1; }
  .ds-form-actions { display: flex; gap: 6px; }

  .test-result { padding: 8px 12px; border-radius: 6px; font-size: 0.82rem; margin-bottom: 1rem; }
  .test-result.ok   { background: #14532d; border: 1px solid #22c55e; color: #86efac; }
  .test-result.fail { background: #7f1d1d; border: 1px solid #ef4444; color: #fca5a5; }

  .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .field { display: flex; flex-direction: column; gap: 4px; }
  .field.wide { grid-column: 1 / -1; }
  .field span { font-size: 0.75rem; color: #64748b; }
  .field input, .field select {
    background: #0f172a; border: 1px solid #334155; border-radius: 6px;
    color: #e2e8f0; padding: 7px 10px; font-size: 0.85rem;
  }
  .field input:focus, .field select:focus { border-color: #38bdf8; outline: none; }

  .form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 1.25rem; }
  .btn-ghost { background: transparent; border: 1px solid #334155; color: #94a3b8; padding: 7px 16px; border-radius: 6px; cursor: pointer; }
  .btn-save  { background: #0284c7; border: none; color: #fff; padding: 7px 18px; border-radius: 6px; cursor: pointer; font-weight: 600; }
  .btn-save:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn-test { background: #0f3460; border: 1px solid #0284c7; color: #38bdf8; padding: 5px 12px; border-radius: 6px; cursor: pointer; font-size: 0.8rem; }
  .btn-del  { background: transparent; border: 1px solid #b91c1c; color: #ef4444; padding: 5px 10px; border-radius: 6px; cursor: pointer; font-size: 0.8rem; }
</style>

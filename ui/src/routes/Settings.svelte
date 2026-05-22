<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import {
    fetchDataflow, fetchDatasources, createDatasource,
    updateDatasource, deleteDatasource, testDatasource, testDatasourceById,
    fetchRetention, saveRetention, purgeData,
  } from '../lib/api'
  import type { DataflowStats, DatasourceStatus, CreateDatasourceRequest, RetentionConfig } from '../lib/api'

  interface LocalDS extends DatasourceStatus {
    _dirty: boolean
    _saving: boolean
    _testing: boolean
    _testOk: boolean | null
    _testMsg: string
  }

  const DS_TYPE_LABELS: Record<string, string> = {
    prometheus:     'Prometheus',
    direct_metrics: 'Direct /metrics',
    otlp:           'OTLP (push)',
  }

  const DS_ICONS: Record<string, string> = {
    prometheus:     '◎',
    direct_metrics: '⊕',
    otlp:           '⊗',
  }

  let sources: LocalDS[] = []
  let loadingDS = true
  let loadError = ''
  let activeSection = 'datasources'
  let dataflow: DataflowStats | null = null

  // Database / Retention state
  let retention: RetentionConfig = { metrics_days: 2, logs_days: 30, traces_days: 30, snapshots_days: 2 }
  let retentionSaving = false
  let retentionMsg = ''
  let purgeType = 'all'
  let purgeBefore = ''
  let purging = false
  let purgeMsg = ''
  let purgeErr = ''
  let purgeConfirm = false

  async function loadRetention() {
    try { retention = await fetchRetention() } catch { /* use defaults */ }
  }

  async function doSaveRetention() {
    retentionSaving = true; retentionMsg = ''
    try {
      await saveRetention(retention)
      retentionMsg = 'Saved'
      setTimeout(() => retentionMsg = '', 2500)
    } catch (e: unknown) {
      retentionMsg = 'Error: ' + (e instanceof Error ? e.message : 'unknown')
    } finally {
      retentionSaving = false
    }
  }

  async function doPurge() {
    if (!purgeConfirm) { purgeConfirm = true; return }
    purging = true; purgeMsg = ''; purgeErr = ''; purgeConfirm = false
    try {
      const before = purgeBefore ? new Date(purgeBefore).toISOString() : undefined
      const r = await purgeData(purgeType, before)
      const count = r.deleted
      purgeMsg = count === 'all' || count === -1
        ? `All ${purgeType} data purged.`
        : `Purged ${count} record(s).`
    } catch (e: unknown) {
      purgeErr = 'Purge failed: ' + (e instanceof Error ? e.message : 'unknown')
    } finally {
      purging = false
    }
  }
  let dataflowInterval: ReturnType<typeof setInterval>

  let showAddForm = false
  let addSaving = false
  let newDS: CreateDatasourceRequest = blankDS()

  function blankDS(): CreateDatasourceRequest {
    return { name: '', type: 'prometheus', url: '', enabled: true, scrape_interval_seconds: 30, namespace: 'default', workload_key: '' }
  }

  function wrapDS(ds: DatasourceStatus): LocalDS {
    return { ...ds, _dirty: false, _saving: false, _testing: false, _testOk: null, _testMsg: '' }
  }

  async function loadSources() {
    loadingDS = true
    loadError = ''
    try {
      sources = (await fetchDatasources()).map(wrapDS)
    } catch (e: unknown) {
      loadError = e instanceof Error ? e.message : 'Failed to load datasources'
    } finally {
      loadingDS = false
    }
  }

  function markDirty(src: LocalDS) {
    src._dirty = true
    sources = sources
  }

  async function saveSource(src: LocalDS) {
    src._saving = true
    sources = sources
    try {
      const updated = await updateDatasource(src.id, {
        name: src.name,
        type: src.type,
        url: src.url,
        enabled: src.enabled,
        scrape_interval_seconds: src.scrape_interval_seconds,
        workload_key: src.workload_key,
        namespace: src.namespace,
      })
      const idx = sources.findIndex(s => s.id === src.id)
      if (idx >= 0) {
        sources[idx] = { ...wrapDS(updated), _testOk: src._testOk, _testMsg: src._testMsg }
      }
      sources = sources
    } catch (e: unknown) {
      src._saving = false
      sources = sources
      alert(`Save failed: ${e instanceof Error ? e.message : String(e)}`)
    }
  }

  async function removeSource(id: string) {
    if (!confirm('Remove this datasource? Ruptura will stop scraping it.')) return
    try {
      await deleteDatasource(id)
      sources = sources.filter(s => s.id !== id)
    } catch (e: unknown) {
      alert(`Delete failed: ${e instanceof Error ? e.message : String(e)}`)
    }
  }

  async function runTest(src: LocalDS) {
    src._testing = true
    src._testOk = null
    src._testMsg = ''
    sources = sources
    try {
      let res
      if (src._dirty || !src.id) {
        res = await testDatasource({ name: src.name, type: src.type, url: src.url, enabled: src.enabled, scrape_interval_seconds: src.scrape_interval_seconds, namespace: src.namespace, workload_key: src.workload_key })
      } else {
        res = await testDatasourceById(src.id)
      }
      src._testOk = res.ok
      src._testMsg = res.ok
        ? `${res.scraped_metrics ?? 0} metric(s) found`
        : (res.error ?? 'Test failed')
    } catch (e: unknown) {
      src._testOk = false
      src._testMsg = e instanceof Error ? e.message : 'Connection failed'
    } finally {
      src._testing = false
      sources = sources
    }
  }

  async function addSource() {
    if (!newDS.url) { alert('URL is required'); return }
    if (!newDS.name) newDS.name = newDS.url
    addSaving = true
    try {
      const created = await createDatasource(newDS)
      sources = [...sources, wrapDS(created)]
      showAddForm = false
      newDS = blankDS()
    } catch (e: unknown) {
      alert(`Create failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      addSaving = false
    }
  }

  function formatScrapeTime(ts: string): string {
    if (!ts || ts === '0001-01-01T00:00:00Z') return '—'
    try {
      const d = new Date(ts)
      const ago = Math.floor((Date.now() - d.getTime()) / 1000)
      if (ago < 60)  return `${ago}s ago`
      if (ago < 3600) return `${Math.floor(ago / 60)}m ago`
      return d.toLocaleTimeString()
    } catch { return ts }
  }

  async function pollDataflow() {
    try { dataflow = await fetchDataflow() } catch { /* non-fatal */ }
  }

  function onRefreshIntervalChange(e: Event) {
    localStorage.setItem('ruptura:refresh_interval', (e.target as HTMLInputElement).value)
  }

  onMount(() => {
    loadSources()
    pollDataflow()
    loadRetention()
    dataflowInterval = setInterval(pollDataflow, 10_000)
  })

  onDestroy(() => clearInterval(dataflowInterval))
</script>

<div class="settings">
  <div class="page-header">
    <h1 class="page-title">Settings</h1>
    <p class="page-sub">Configure data sources, integrations, and platform preferences.</p>
  </div>

  <div class="settings-layout">
    <!-- sidebar -->
    <nav class="settings-nav">
      {#each [
        ['datasources', '⊕ Data Sources'],
        ['dataflow',    '⟳ Ingest Stats'],
        ['database',    '◫ Database'],
        ['general',     '⊞ General'],
        ['about',       '◈ About'],
      ] as [id, label]}
        <button class="snav-item" class:active={activeSection === id} on:click={() => { activeSection = id }}>
          {label}
        </button>
      {/each}
    </nav>

    <!-- content -->
    <div class="settings-body">

      <!-- ── DATA SOURCES ── -->
      {#if activeSection === 'datasources'}
        <div class="section-header">
          <h2>Data Sources</h2>
          <p>Configure Prometheus servers or direct <code>/metrics</code> endpoints. Ruptura actively scrapes them and feeds data into the health engine — no YAML or external agents needed.</p>
        </div>

        {#if loadingDS}
          <div class="loading-hint">Loading datasources…</div>
        {:else if loadError}
          <div class="err-banner">{loadError} <button class="btn-retry" on:click={loadSources}>Retry</button></div>
        {:else}
          <div class="ds-list">
            {#each sources as src (src.id)}
              <div class="ds-card" class:ds-enabled={src.enabled} class:ds-err={src.status === 'error'}>
                <div class="ds-top">
                  <span class="ds-icon">{DS_ICONS[src.type] ?? '◈'}</span>
                  <input
                    class="ds-label-input"
                    bind:value={src.name}
                    placeholder="Name"
                    on:input={() => markDirty(src)}
                  />
                  <div class="ds-status-badge"
                    class:ok={src.status === 'ok'}
                    class:err={src.status === 'error'}
                    class:pending={src.status === 'pending'}
                    class:disabled={src.status === 'disabled'}
                    title={src.last_error || src.status}
                  >
                    {#if src.status === 'ok'}✓ ok
                    {:else if src.status === 'error'}✗ error
                    {:else if src.status === 'pending'}⟳ pending
                    {:else if src.status === 'disabled'}○ off
                    {:else}— —{/if}
                  </div>
                  <label class="toggle">
                    <input type="checkbox" bind:checked={src.enabled} on:change={() => markDirty(src)} />
                    <span class="toggle-track"></span>
                  </label>
                </div>

                <div class="ds-body">
                  <select class="ds-type-sel" bind:value={src.type} on:change={() => markDirty(src)}>
                    <option value="prometheus">Prometheus</option>
                    <option value="direct_metrics">Direct /metrics</option>
                    <option value="otlp">OTLP (push)</option>
                  </select>
                  <input
                    class="ds-endpoint"
                    bind:value={src.url}
                    placeholder={src.type === 'otlp' ? 'http://node-public-ip:31470' : 'http://host:port'}
                    on:input={() => markDirty(src)}
                  />
                </div>
                {#if src.type === 'otlp'}
                  <div class="otlp-hint">Push-based — your apps push logs/traces to Ruptura's OTLP NodePort. Enter <code>http://&lt;node-public-ip&gt;:31470</code> to enable the connectivity test.</div>
                {/if}

                {#if src.type === 'prometheus'}
                  <div class="ds-extra">
                    <span class="extra-label">\1</span>
                    <input class="ds-extra-input" bind:value={src.namespace} placeholder="all" on:input={() => markDirty(src)} />
                    <span class="extra-label">\1</span>
                    <input class="ds-extra-input w60" type="number" min="5" bind:value={src.scrape_interval_seconds} on:input={() => markDirty(src)} />
                  </div>
                {:else if src.type === 'direct_metrics'}
                  <div class="ds-extra">
                    <span class="extra-label">\1</span>
                    <input class="ds-extra-input" bind:value={src.workload_key} placeholder="namespace/Kind/name" on:input={() => markDirty(src)} />
                    <span class="extra-label">\1</span>
                    <input class="ds-extra-input w60" type="number" min="5" bind:value={src.scrape_interval_seconds} on:input={() => markDirty(src)} />
                  </div>
                {/if}

                <div class="ds-footer">
                  <div class="ds-meta">
                    {#if src.last_scrape && src.last_scrape !== '0001-01-01T00:00:00Z'}
                      <span class="ds-ts">Scraped {formatScrapeTime(src.last_scrape)}</span>
                      {#if src.scraped_metrics > 0}
                        <span class="ds-count">{src.scraped_metrics} metrics</span>
                      {/if}
                    {:else}
                      <span class="ds-ts">Not yet scraped</span>
                    {/if}
                    {#if src._testOk !== null}
                      <span class="test-msg" class:test-ok={src._testOk} class:test-err={!src._testOk}>
                        {src._testMsg}
                      </span>
                    {/if}
                  </div>
                  <div class="ds-actions">
                    {#if src._dirty}
                      <button class="btn-save-inline" on:click={() => saveSource(src)} disabled={src._saving}>
                        {src._saving ? 'Saving…' : 'Save'}
                      </button>
                    {/if}
                    {#if src.type !== 'otlp'}
                      <button class="btn-test" on:click={() => runTest(src)} disabled={src._testing}>
                        {src._testing ? 'Testing…' : 'Test'}
                      </button>
                    {/if}
                    <button class="btn-remove" on:click={() => removeSource(src.id)} title="Remove datasource">✕</button>
                  </div>
                </div>
              </div>
            {/each}

            {#if !showAddForm}
              <button class="btn-add" on:click={() => { showAddForm = true }}>+ Add data source</button>
            {:else}
              <!-- new datasource form -->
              <div class="ds-card ds-add-form">
                <div class="add-form-title">New Data Source</div>

                <div class="ds-body">
                  <select class="ds-type-sel" bind:value={newDS.type}>
                    <option value="prometheus">Prometheus</option>
                    <option value="direct_metrics">Direct /metrics</option>
                    <option value="otlp">OTLP (info only)</option>
                  </select>
                  <input class="ds-endpoint" bind:value={newDS.url} placeholder="http://prometheus:9090" />
                </div>

                <div class="ds-extra">
                  <span class="extra-label">\1</span>
                  <input class="ds-extra-input" bind:value={newDS.name} placeholder="My Prometheus" />
                  {#if newDS.type === 'prometheus'}
                    <span class="extra-label">\1</span>
                    <input class="ds-extra-input" bind:value={newDS.namespace} placeholder="all" />
                  {:else if newDS.type === 'direct_metrics'}
                    <span class="extra-label">\1</span>
                    <input class="ds-extra-input" bind:value={newDS.workload_key} placeholder="default/Deployment/my-app" />
                  {/if}
                  <span class="extra-label">\1</span>
                  <input class="ds-extra-input w60" type="number" min="5" bind:value={newDS.scrape_interval_seconds} />
                </div>

                <div class="ds-footer">
                  <div class="ds-meta"></div>
                  <div class="ds-actions">
                    <button class="btn-cancel" on:click={() => { showAddForm = false; newDS = blankDS() }}>Cancel</button>
                    <button class="btn-save-inline" on:click={addSource} disabled={addSaving}>
                      {addSaving ? 'Adding…' : 'Add'}
                    </button>
                  </div>
                </div>
              </div>
            {/if}
          </div>
        {/if}

      <!-- ── DATAFLOW ── -->
      {:else if activeSection === 'dataflow'}
        <div class="section-header">
          <h2>Ingest Statistics</h2>
          <p>Cumulative data ingested by the Ruptura engine. Updates every 10 seconds.</p>
        </div>
        {#if dataflow}
          <div class="df-grid">
            <div class="df-card">
              <div class="df-icon" style="color:var(--purple)">⊕</div>
              <div class="df-num">{dataflow.metrics.toLocaleString()}</div>
              <div class="df-label">Metrics</div>
            </div>
            <div class="df-card">
              <div class="df-icon" style="color:var(--cyan)">▦</div>
              <div class="df-num">{dataflow.logs.toLocaleString()}</div>
              <div class="df-label">Log Lines</div>
            </div>
            <div class="df-card">
              <div class="df-icon" style="color:var(--yellow)">⌖</div>
              <div class="df-num">{dataflow.traces.toLocaleString()}</div>
              <div class="df-label">Traces</div>
            </div>
          </div>
        {:else}
          <div class="loading-hint">Fetching ingest stats…</div>
        {/if}

      <!-- ── DATABASE ── -->
      {:else if activeSection === 'database'}
        <div class="section-header">
          <h2>Data Retention</h2>
          <p>How long raw ingested data is kept. Applied to new writes immediately after saving.</p>
        </div>

        <div class="retention-grid">
          <label class="ret-field">
            <span class="ret-label">Metrics (days)</span>
            <input class="pref-input" type="number" min="1" max="365" bind:value={retention.metrics_days} />
            <span class="ret-hint">Default: 2</span>
          </label>
          <label class="ret-field">
            <span class="ret-label">Logs (days)</span>
            <input class="pref-input" type="number" min="1" max="365" bind:value={retention.logs_days} />
            <span class="ret-hint">Default: 30</span>
          </label>
          <label class="ret-field">
            <span class="ret-label">Traces (days)</span>
            <input class="pref-input" type="number" min="1" max="365" bind:value={retention.traces_days} />
            <span class="ret-hint">Default: 30</span>
          </label>
          <label class="ret-field">
            <span class="ret-label">KPI Snapshots (days)</span>
            <input class="pref-input" type="number" min="1" max="365" bind:value={retention.snapshots_days} />
            <span class="ret-hint">Default: 2</span>
          </label>
        </div>
        <div class="db-row">
          <button class="btn-save-inline" on:click={doSaveRetention} disabled={retentionSaving}>
            {retentionSaving ? 'Saving…' : 'Save Retention'}
          </button>
          {#if retentionMsg}
            <span class="ret-msg" class:ret-err={retentionMsg.startsWith('Error')}>{retentionMsg}</span>
          {/if}
        </div>

        <div class="section-header" style="margin-top:1rem">
          <h2>Purge Raw Data</h2>
          <p>Permanently delete raw data from BadgerDB. KPI snapshots and computed results are unaffected unless you select "KPI Snapshots".</p>
        </div>

        <div class="purge-row">
          <label class="ret-field">
            <span class="ret-label">Data type</span>
            <select class="ds-type-sel" bind:value={purgeType}>
              <option value="all">All raw data (metrics + logs + traces)</option>
              <option value="metrics">Metrics only</option>
              <option value="logs">Logs only</option>
              <option value="traces">Traces only</option>
              <option value="snapshots">KPI Snapshots</option>
            </select>
          </label>
          <label class="ret-field">
            <span class="ret-label">Before date <small>(empty = purge all)</small></span>
            <input class="pref-input" type="date" bind:value={purgeBefore} />
          </label>
        </div>
        <div class="db-row">
          {#if !purgeConfirm}
            <button class="btn-danger" on:click={doPurge} disabled={purging}>
              {purging ? 'Purging…' : 'Purge'}
            </button>
          {:else}
            <span class="warn-text">This cannot be undone. Are you sure?</span>
            <button class="btn-danger" on:click={doPurge} disabled={purging}>Yes, purge now</button>
            <button class="btn-cancel" on:click={() => purgeConfirm = false}>Cancel</button>
          {/if}
        </div>
        {#if purgeMsg}<p class="ok-msg">{purgeMsg}</p>{/if}
        {#if purgeErr}<p class="err-msg">{purgeErr}</p>{/if}

      <!-- ── GENERAL ── -->
      {:else if activeSection === 'general'}
        <div class="section-header">
          <h2>General</h2>
          <p>Platform-wide preferences stored in this browser.</p>
        </div>

        <div class="general-body">
          <div class="pref-row">
            <div>
              <div class="pref-label">Ruptura API endpoint</div>
              <div class="pref-hint">Proxied through nginx — typically not changed.</div>
            </div>
            <input class="pref-input" value="/api/v2" readonly />
          </div>

          <div class="pref-row">
            <div>
              <div class="pref-label">Fleet refresh interval</div>
              <div class="pref-hint">How often the Fleet view polls for new data (seconds).</div>
            </div>
            <input class="pref-input" type="number" min="5" max="120"
              value={parseInt(localStorage.getItem('ruptura:refresh_interval') ?? '10')}
              on:change={onRefreshIntervalChange}
            />
          </div>
        </div>

      <!-- ── ABOUT ── -->
      {:else if activeSection === 'about'}
        <div class="section-header">
          <h2>About Ruptura</h2>
        </div>
        <div class="about-body">
          <div class="about-logo">
            <img src="/ruptura-icon-64.png" alt="Ruptura" width="48" height="48" style="border-radius:8px" />
            <div class="about-name">RUPTURA</div>
          </div>
          <dl class="about-dl">
            <dt>Platform</dt><dd>Predictive Kubernetes Workload Health</dd>
            <dt>Signals</dt><dd>9 KPI signals + FusedR composite index</dd>
            <dt>Ingest</dt><dd>OTLP (gRPC + HTTP), active Prometheus scrape</dd>
            <dt>Storage</dt><dd>BadgerDB embedded time-series store</dd>
            <dt>License</dt><dd>Proprietary — benfradjselim/ruptura</dd>
          </dl>
          <div class="about-links">
            <a class="about-link" href="https://github.com/benfradjselim/ruptura" target="_blank" rel="noopener">GitHub</a>
          </div>
        </div>
      {/if}

    </div>
  </div>
</div>

<style>
  .settings {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .page-header { max-width: 640px; }
  .page-title { font-size: 22px; font-weight: 700; margin-bottom: 4px; }
  .page-sub { font-size: 13px; color: var(--muted); }

  .settings-layout {
    display: grid;
    grid-template-columns: 180px 1fr;
    gap: 24px;
    align-items: start;
  }
  @media (max-width: 640px) { .settings-layout { grid-template-columns: 1fr; } }

  /* sidebar */
  .settings-nav {
    display: flex;
    flex-direction: column;
    gap: 2px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 8px;
    position: sticky;
    top: 68px;
  }

  .snav-item {
    background: none;
    border: none;
    color: var(--muted);
    text-align: left;
    padding: 8px 12px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
    transition: color 0.15s, background 0.15s;
  }
  .snav-item:hover { color: var(--text); background: var(--surface2); }
  .snav-item.active { color: var(--purple); background: rgba(168,85,247,0.1); }

  /* content */
  .settings-body {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .section-header h2 { font-size: 16px; font-weight: 700; margin-bottom: 6px; }
  .section-header p { font-size: 12px; color: var(--muted); line-height: 1.6; max-width: 540px; }
  .section-header code { font-family: 'JetBrains Mono', monospace; font-size: 11px; background: var(--surface3); padding: 1px 4px; border-radius: 3px; }

  .loading-hint { font-size: 12px; color: var(--muted); font-style: italic; }

  .err-banner {
    font-size: 12px;
    color: var(--red);
    background: rgba(239,68,68,0.08);
    border: 1px solid rgba(239,68,68,0.2);
    border-radius: 8px;
    padding: 10px 14px;
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .btn-retry {
    background: none;
    border: 1px solid rgba(239,68,68,0.4);
    color: var(--red);
    padding: 3px 10px;
    border-radius: 5px;
    cursor: pointer;
    font-size: 11px;
  }

  /* data source cards */
  .ds-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .ds-card {
    background: var(--surface2);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    transition: border-color 0.15s;
  }
  .ds-card.ds-enabled  { border-color: rgba(168,85,247,0.35); }
  .ds-card.ds-err      { border-color: rgba(239,68,68,0.35); }
  .ds-card.ds-add-form { border: 1px dashed var(--border); background: var(--surface3); }

  .add-form-title { font-size: 13px; font-weight: 600; color: var(--purple); }

  .ds-top {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .ds-icon { font-size: 16px; color: var(--purple); flex-shrink: 0; }

  .ds-label-input {
    flex: 1;
    background: none;
    border: none;
    color: var(--text);
    font-size: 14px;
    font-weight: 600;
    outline: none;
    min-width: 0;
  }

  .ds-status-badge {
    font-size: 10px;
    font-weight: 700;
    padding: 2px 7px;
    border-radius: 10px;
    background: var(--surface3);
    color: var(--muted);
    white-space: nowrap;
    font-family: 'JetBrains Mono', monospace;
    flex-shrink: 0;
  }
  .ds-status-badge.ok       { background: rgba(34,197,94,0.12); color: var(--green); }
  .ds-status-badge.err      { background: rgba(239,68,68,0.12); color: var(--red); }
  .ds-status-badge.pending  { background: rgba(234,179,8,0.12);  color: var(--yellow); }
  .ds-status-badge.disabled { opacity: 0.5; }

  /* toggle switch */
  .toggle { position: relative; display: inline-block; width: 32px; height: 18px; flex-shrink: 0; }
  .toggle input { opacity: 0; width: 0; height: 0; }
  .toggle-track {
    position: absolute; inset: 0;
    background: var(--surface3);
    border-radius: 18px;
    cursor: pointer;
    transition: background 0.2s;
    border: 1px solid var(--border);
  }
  .toggle-track::after {
    content: '';
    position: absolute;
    left: 2px; top: 2px;
    width: 12px; height: 12px;
    border-radius: 50%;
    background: var(--muted);
    transition: transform 0.2s, background 0.2s;
  }
  .toggle input:checked + .toggle-track { background: rgba(168,85,247,0.2); border-color: var(--purple); }
  .toggle input:checked + .toggle-track::after { transform: translateX(14px); background: var(--purple); }

  .ds-body {
    display: flex;
    gap: 8px;
  }

  .ds-type-sel {
    background: var(--surface3);
    border: 1px solid var(--border);
    color: var(--muted);
    border-radius: 6px;
    padding: 5px 8px;
    font-size: 11px;
    outline: none;
    cursor: pointer;
    flex-shrink: 0;
  }

  .ds-endpoint {
    flex: 1;
    background: var(--surface3);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: 6px;
    padding: 5px 10px;
    font-size: 12px;
    font-family: 'JetBrains Mono', monospace;
    outline: none;
    min-width: 0;
  }
  .ds-endpoint:focus { border-color: var(--purple); }

  .ds-extra {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
  }

  .extra-label {
    font-size: 10px;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    white-space: nowrap;
  }

  .ds-extra-input {
    background: var(--surface3);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: 6px;
    padding: 4px 8px;
    font-size: 11px;
    font-family: 'JetBrains Mono', monospace;
    outline: none;
    min-width: 100px;
    flex: 1;
  }
  .ds-extra-input.w60 { flex: 0 0 60px; min-width: 60px; }
  .ds-extra-input:focus { border-color: var(--purple); }

  .ds-footer {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .ds-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    flex-wrap: wrap;
  }

  .ds-ts    { font-size: 10px; color: var(--muted); }
  .ds-count { font-size: 10px; color: var(--cyan); font-family: 'JetBrains Mono', monospace; }

  .test-msg { font-size: 10px; font-family: 'JetBrains Mono', monospace; }
  .test-msg.test-ok  { color: var(--green); }
  .test-msg.test-err { color: var(--red); }

  .ds-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }

  .btn-test {
    background: var(--surface3);
    border: 1px solid var(--border);
    color: var(--cyan);
    padding: 4px 12px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 11px;
    font-weight: 600;
    transition: border-color 0.15s;
  }
  .btn-test:hover:not(:disabled) { border-color: var(--cyan); }
  .btn-test:disabled { opacity: 0.5; cursor: default; }

  .btn-save-inline {
    background: var(--purple);
    border: none;
    color: white;
    padding: 4px 12px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 11px;
    font-weight: 600;
    transition: opacity 0.15s;
  }
  .btn-save-inline:hover:not(:disabled) { opacity: 0.85; }
  .btn-save-inline:disabled { opacity: 0.5; cursor: default; }

  .btn-cancel {
    background: none;
    border: 1px solid var(--border);
    color: var(--muted);
    padding: 4px 12px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 11px;
  }
  .btn-cancel:hover { color: var(--text); }

  .btn-remove {
    background: none;
    border: 1px solid transparent;
    color: var(--muted);
    padding: 4px 8px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 11px;
    transition: color 0.15s, border-color 0.15s;
  }
  .btn-remove:hover { color: var(--red); border-color: rgba(239,68,68,0.3); }

  .btn-add {
    background: none;
    border: 1px dashed var(--border);
    color: var(--muted);
    padding: 10px;
    border-radius: 10px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 500;
    transition: color 0.15s, border-color 0.15s;
    width: 100%;
  }
  .btn-add:hover { color: var(--purple); border-color: var(--purple); }

  /* dataflow */
  .df-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
  }

  .df-card {
    background: var(--surface2);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 20px 16px;
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
  }

  .df-icon { font-size: 20px; }
  .df-num { font-size: 28px; font-weight: 700; font-variant-numeric: tabular-nums; font-family: 'JetBrains Mono', monospace; }
  .df-label { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.08em; }

  /* general */
  .general-body { display: flex; flex-direction: column; gap: 16px; }

  .pref-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    padding: 14px;
    background: var(--surface2);
    border: 1px solid var(--border);
    border-radius: 8px;
  }

  .pref-label { font-size: 13px; font-weight: 600; margin-bottom: 2px; }
  .pref-hint { font-size: 11px; color: var(--muted); }

  .pref-input {
    background: var(--surface3);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: 6px;
    padding: 5px 10px;
    font-size: 12px;
    font-family: 'JetBrains Mono', monospace;
    outline: none;
    width: 120px;
    flex-shrink: 0;
  }
  .pref-input:not([readonly]):focus { border-color: var(--purple); }
  .pref-input[readonly] { opacity: 0.6; cursor: default; }

  /* otlp hint */
  .otlp-hint {
    font-size: 11px;
    color: var(--cyan);
    background: rgba(6,182,212,0.06);
    border: 1px solid rgba(6,182,212,0.2);
    border-radius: 6px;
    padding: 7px 12px;
    line-height: 1.5;
  }
  .otlp-hint code { font-family: 'JetBrains Mono', monospace; font-size: 10px; background: var(--surface3); padding: 1px 4px; border-radius: 3px; }

  /* database / retention */
  .retention-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; }
  .ret-field { display: flex; flex-direction: column; gap: 4px; }
  .ret-label { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
  .ret-hint { font-size: 10px; color: var(--muted); opacity: 0.7; }
  .db-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-top: 4px; }
  .purge-row { display: flex; gap: 12px; flex-wrap: wrap; align-items: flex-end; }
  .ret-msg { font-size: 12px; }
  .ret-msg.ret-err { color: var(--red); }
  .btn-danger {
    background: rgba(239,68,68,0.15);
    border: 1px solid rgba(239,68,68,0.4);
    color: var(--red);
    padding: 5px 14px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 600;
  }
  .btn-danger:disabled { opacity: 0.5; cursor: default; }
  .warn-text { font-size: 12px; color: var(--yellow); }
  .ok-msg { font-size: 12px; color: var(--green); margin: 0; }
  .err-msg { font-size: 12px; color: var(--red); margin: 0; }

  /* about */
  .about-body { display: flex; flex-direction: column; gap: 20px; }

  .about-logo {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .about-name {
    font-size: 20px;
    font-weight: 800;
    letter-spacing: 0.12em;
    background: linear-gradient(135deg, var(--purple), var(--cyan));
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .about-dl {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 8px 16px;
    font-size: 13px;
    background: var(--surface2);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 16px;
  }
  dt { color: var(--muted); font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; align-self: center; }
  dd { color: var(--text); font-family: 'JetBrains Mono', monospace; font-size: 12px; }

  .about-links { display: flex; gap: 10px; }
  .about-link {
    color: var(--cyan);
    text-decoration: none;
    font-size: 13px;
    font-weight: 500;
    border-bottom: 1px solid rgba(6,182,212,0.3);
    transition: border-color 0.15s;
  }
  .about-link:hover { border-color: var(--cyan); }
</style>

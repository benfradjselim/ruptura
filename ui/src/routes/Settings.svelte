<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { fetchDataflow } from '../lib/api'
  import type { DataflowStats } from '../lib/api'

  interface DataSource {
    id: string
    type: 'otlp' | 'prometheus' | 'loki' | 'elasticsearch'
    label: string
    endpoint: string
    enabled: boolean
    status: 'unknown' | 'ok' | 'error' | 'testing'
    lastTested?: string
  }

  const DEFAULTS: DataSource[] = [
    { id: 'otlp',   type: 'otlp',          label: 'OTLP gRPC',       endpoint: 'http://otel-collector:4317', enabled: true,  status: 'unknown' },
    { id: 'otlph',  type: 'otlp',          label: 'OTLP HTTP',       endpoint: 'http://otel-collector:4318', enabled: false, status: 'unknown' },
    { id: 'prom',   type: 'prometheus',    label: 'Prometheus',       endpoint: 'http://prometheus:9090',     enabled: false, status: 'unknown' },
    { id: 'loki',   type: 'loki',          label: 'Loki',             endpoint: 'http://loki:3100',           enabled: false, status: 'unknown' },
    { id: 'es',     type: 'elasticsearch', label: 'Elasticsearch',    endpoint: 'http://elasticsearch:9200',  enabled: false, status: 'unknown' },
  ]

  let sources: DataSource[] = []
  let saved = false
  let activeSection = 'datasources'
  let dataflow: DataflowStats | null = null
  let dataflowInterval: ReturnType<typeof setInterval>

  const DS_ICONS: Record<string, string> = {
    otlp:          '⊕',
    prometheus:    '◎',
    loki:          '▦',
    elasticsearch: '⌖',
  }

  function load() {
    try {
      const stored = localStorage.getItem('ruptura:datasources')
      sources = stored ? JSON.parse(stored) : DEFAULTS.map(d => ({ ...d }))
    } catch {
      sources = DEFAULTS.map(d => ({ ...d }))
    }
  }

  function save() {
    localStorage.setItem('ruptura:datasources', JSON.stringify(sources))
    saved = true
    setTimeout(() => { saved = false }, 2000)
  }

  async function testSource(src: DataSource) {
    src.status = 'testing'
    sources = sources
    await new Promise(r => setTimeout(r, 800))
    // Attempt a lightweight fetch — just check reachability.
    // For OTLP gRPC we can't do a browser fetch, so we skip body validation.
    try {
      const url = src.endpoint.startsWith('http') ? src.endpoint : `http://${src.endpoint}`
      const res = await fetch(url, { method: 'HEAD', signal: AbortSignal.timeout(4000) })
      src.status = res.ok || res.status < 500 ? 'ok' : 'error'
    } catch {
      src.status = 'error'
    }
    src.lastTested = new Date().toLocaleTimeString()
    sources = sources
  }

  function addSource() {
    sources = [...sources, { id: `ds-${Date.now()}`, type: 'otlp', label: 'New source', endpoint: '', enabled: false, status: 'unknown' }]
  }

  function removeSource(id: string) {
    sources = sources.filter(s => s.id !== id)
  }

  function onRefreshIntervalChange(e: Event) {
    const val = (e.target as HTMLInputElement).value
    localStorage.setItem('ruptura:refresh_interval', val)
  }

  async function pollDataflow() {
    try { dataflow = await fetchDataflow() } catch { /* non-fatal */ }
  }

  onMount(() => {
    load()
    pollDataflow()
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
          <p>Configure ingest endpoints for metrics, logs, and traces. Ruptura's OTLP receiver is always active on the engine pod. External sources are for future federation support.</p>
        </div>

        <div class="ds-list">
          {#each sources as src (src.id)}
            <div class="ds-card" class:ds-enabled={src.enabled} class:ds-disabled={!src.enabled}>
              <div class="ds-top">
                <span class="ds-icon">{DS_ICONS[src.type] ?? '◈'}</span>
                <input class="ds-label-input" bind:value={src.label} placeholder="Label" />
                <div class="ds-status" class:ok={src.status==='ok'} class:err={src.status==='error'} class:testing={src.status==='testing'}>
                  {#if src.status === 'testing'}⟳{:else if src.status === 'ok'}✓{:else if src.status === 'error'}✗{:else}—{/if}
                </div>
                <label class="toggle">
                  <input type="checkbox" bind:checked={src.enabled} />
                  <span class="toggle-track"></span>
                </label>
              </div>

              <div class="ds-body">
                <select class="ds-type-sel" bind:value={src.type}>
                  <option value="otlp">OTLP</option>
                  <option value="prometheus">Prometheus</option>
                  <option value="loki">Loki</option>
                  <option value="elasticsearch">Elasticsearch</option>
                </select>
                <input class="ds-endpoint" bind:value={src.endpoint} placeholder="http://host:port" />
              </div>

              <div class="ds-footer">
                {#if src.lastTested}
                  <span class="ds-ts">Tested {src.lastTested}</span>
                {/if}
                <button class="btn-test" on:click={() => testSource(src)} disabled={src.status==='testing'}>
                  {src.status === 'testing' ? 'Testing…' : 'Test'}
                </button>
                <button class="btn-remove" on:click={() => removeSource(src.id)} title="Remove">✕</button>
              </div>
            </div>
          {/each}

          <button class="btn-add" on:click={addSource}>+ Add data source</button>
        </div>

        <div class="save-row">
          <button class="btn-save" on:click={save}>
            {saved ? '✓ Saved' : 'Save changes'}
          </button>
        </div>

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

          <div class="pref-row">
            <div>
              <div class="pref-label">Topology refresh interval</div>
              <div class="pref-hint">How often the topology graph redraws (seconds).</div>
            </div>
            <input class="pref-input" type="number" min="10" max="300"
              value="20"
              readonly
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
            <svg viewBox="0 0 40 40" xmlns="http://www.w3.org/2000/svg" width="48" height="48">
              <defs>
                <linearGradient id="rabout" x1="0" y1="0" x2="1" y2="1">
                  <stop offset="0%" stop-color="#a855f7"/>
                  <stop offset="100%" stop-color="#06b6d4"/>
                </linearGradient>
              </defs>
              <polygon points="20,3 35,11.5 35,28.5 20,37 5,28.5 5,11.5" fill="url(#rabout)"/>
              <text x="20" y="26" text-anchor="middle" font-family="monospace" font-weight="700" font-size="17" fill="white">R</text>
            </svg>
            <div class="about-name">RUPTURA</div>
          </div>
          <dl class="about-dl">
            <dt>Platform</dt><dd>Predictive Kubernetes Workload Health</dd>
            <dt>Signals</dt><dd>9 KPI signals + FusedR composite index</dd>
            <dt>Ingest</dt><dd>OTLP (gRPC + HTTP), native Prometheus pull</dd>
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
  .ds-card.ds-enabled { border-color: rgba(168,85,247,0.4); }

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

  .ds-status {
    font-size: 12px;
    font-weight: 700;
    width: 20px;
    text-align: center;
    color: var(--muted);
    font-family: 'JetBrains Mono', monospace;
  }
  .ds-status.ok { color: var(--green); }
  .ds-status.err { color: var(--red); }
  .ds-status.testing { color: var(--yellow); animation: spin 1s linear infinite; }

  @keyframes spin { to { transform: rotate(360deg); } }

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

  .ds-footer {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .ds-ts { font-size: 10px; color: var(--muted); flex: 1; }

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

  .save-row { display: flex; justify-content: flex-end; }

  .btn-save {
    background: var(--purple);
    border: none;
    color: white;
    padding: 8px 20px;
    border-radius: 8px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
    transition: opacity 0.15s;
  }
  .btn-save:hover { opacity: 0.85; }

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

  .loading-hint { font-size: 12px; color: var(--muted); font-style: italic; }

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

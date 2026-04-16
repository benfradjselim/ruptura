<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import DashboardView from './DashboardView.svelte'
  import DashboardEdit from './DashboardEdit.svelte'

  let dashboards = [], templates = [], loading = true
  let showNew = false, newName = '', newRefresh = 30, creating = false

  // ── Tab system ────────────────────────────────────────────────────────────
  // openTabs: [{id, name, dashboardId, reloadKey}]
  let openTabs  = []
  let activeTab = null   // tab.id | null = show list
  let editId    = null   // dashboard id being edited

  function uid() {
    return Math.random().toString(36).slice(2) + Date.now().toString(36)
  }

  function openDashboard(d) {
    const existing = openTabs.find(t => t.dashboardId === d.id)
    if (existing) { activeTab = existing.id; return }
    const tab = { id: uid(), name: d.name, dashboardId: d.id, reloadKey: 0 }
    openTabs  = [...openTabs, tab]
    activeTab = tab.id
  }

  function closeTab(tabId, e) {
    e?.stopPropagation()
    openTabs = openTabs.filter(t => t.id !== tabId)
    if (activeTab === tabId) {
      activeTab = openTabs.length ? openTabs[openTabs.length - 1].id : null
    }
  }

  function editDashboard(dashboardId) {
    editId = dashboardId
  }

  function onEditDone() {
    // Bump reloadKey for the tab that was editing
    openTabs = openTabs.map(t =>
      t.dashboardId === editId ? { ...t, reloadKey: t.reloadKey + 1 } : t
    )
    editId = null
    load()
  }

  // ── Template modal ────────────────────────────────────────────────────────
  let applyModal = null
  let applyName  = ''
  let applyMode  = 'current'
  let applying   = false

  let activeCategory = 'All'

  async function load() {
    loading = true
    const [dr, tr] = await Promise.all([
      api.dashboards().catch(() => ({ data: [] })),
      api.templates().catch(() => ({ data: [] })),
    ])
    dashboards = dr.data || []
    templates  = tr.data || []
    // Update names of open tabs in case renamed
    openTabs = openTabs.map(t => {
      const d = dashboards.find(d => d.id === t.dashboardId)
      return d ? { ...t, name: d.name } : t
    })
    loading = false
  }

  async function create() {
    creating = true
    await api.dashboardCreate({ name: newName, refresh_seconds: newRefresh }).catch(() => {})
    showNew = false; newName = ''; creating = false
    load()
  }

  function openApplyModal(t) { applyModal = t; applyName = ''; applyMode = 'current' }

  async function confirmApply() {
    if (!applyModal) return
    applying = true
    const name = applyName.trim() || undefined
    await api.templateApply(applyModal.id, name, applyMode).catch(() => {})
    applying = false
    applyModal = null
    load()
  }

  async function del(id) {
    if (!confirm('Delete this dashboard?')) return
    // Close any open tab for this dashboard
    openTabs = openTabs.filter(t => t.dashboardId !== id)
    if (openTabs.length === 0) activeTab = null
    else if (!openTabs.find(t => t.id === activeTab)) activeTab = openTabs[openTabs.length - 1].id
    await api.dashboardDelete(id).catch(() => {})
    load()
  }

  onMount(load)

  $: categories = ['All', ...new Set(templates.map(t => t.category).filter(Boolean))]
  $: filtered = activeCategory === 'All'
    ? templates
    : templates.filter(t => t.category === activeCategory)

  // ── Icon paths (Lucide) ───────────────────────────────────────────────────
  const ICONS = {
    'server':         'M20 4H4a2 2 0 00-2 2v3a2 2 0 002 2h16a2 2 0 002-2V6a2 2 0 00-2-2zm0 9H4a2 2 0 00-2 2v3a2 2 0 002 2h16a2 2 0 002-2v-3a2 2 0 00-2-2zM7 7h.01M7 16h.01',
    'activity':       'M22 12h-4l-3 9L9 3l-3 9H2',
    'trending-up':    'M23 6l-9.5 9.5-5-5L1 18M17 6h6v6',
    'box':            'M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 002 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z',
    'layers':         'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5',
    'globe':          'M12 2a10 10 0 100 20A10 10 0 0012 2zM2 12h20M12 2a15.3 15.3 0 010 20M12 2a15.3 15.3 0 000 20',
    'shield':         'M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z',
    'alert-triangle': 'M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0zM12 9v4M12 17h.01',
    'bar-chart-2':    'M18 20V10M12 20V4M6 20v-6',
    'eye':            'M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8zM12 9a3 3 0 110 6 3 3 0 010-6z',
    'award':          'M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z',
    'zap':            'M13 2L3 14h9l-1 8 10-12h-9l1-8z',
    'wifi':           'M5 12.55a11 11 0 0114.08 0M1.42 9a16 16 0 0121.16 0M8.53 16.11a6 6 0 016.95 0M12 20h.01',
    'database':       'M12 2C6.48 2 2 4.24 2 7v10c0 2.76 4.48 5 10 5s10-2.24 10-5V7c0-2.76-4.48-5-10-5zM2 12c0 2.76 4.48 5 10 5s10-2.24 10-5M2 7c0 2.76 4.48 5 10 5s10-2.24 10-5',
    'cpu':            'M18 4H6a2 2 0 00-2 2v12a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2zM9 9h6v6H9V9zM9 1v3M15 1v3M9 20v3M15 20v3M20 9h3M20 15h3M1 9h3M1 15h3',
    'percent':        'M19 5L5 19M6.5 6.5a1 1 0 100 2 1 1 0 000-2zM17.5 15.5a1 1 0 100 2 1 1 0 000-2z',
    'git-branch':     'M6 3v12M18 9a3 3 0 100-6 3 3 0 000 6zM6 21a3 3 0 100-6 3 3 0 000 6zM18 9a9 9 0 01-9 9',
    'brain':          'M9.5 2A2.5 2.5 0 017 4.5v0A2.5 2.5 0 014.5 7H4a2 2 0 00-2 2v0a2 2 0 002 2h.5A2.5 2.5 0 017 13.5v0A2.5 2.5 0 019.5 16h5a2.5 2.5 0 002.5-2.5v0A2.5 2.5 0 0119.5 11H20a2 2 0 002-2v0a2 2 0 00-2-2h-.5A2.5 2.5 0 0117 4.5v0A2.5 2.5 0 0114.5 2H9.5z',
    'package':        'M12 2l9 5v10l-9 5-9-5V7l9-5zM12 12l9-5M12 12v10M12 12L3 7',
    'radio':          'M2 12a10 10 0 1020 0A10 10 0 002 12zM12 12a2 2 0 104 0 2 2 0 00-4 0zM8.93 6.588l-2.29.287-.082.38.45.083c.294.07.352.176.288.469l-.738 3.468c-.194.897.105 1.319.808 1.319.545 0 1.178-.252 1.465-.598l.088-.416c-.2.176-.492.246-.686.246-.275 0-.375-.193-.304-.533L8.93 6.588z',
    'crosshair':      'M12 22a10 10 0 100-20 10 10 0 000 20zM12 2v4M12 18v4M2 12h4M18 12h4',
    'terminal':       'M4 17l6-6-6-6M12 19h8',
  }

  const CATEGORY_COLOR = {
    'Infrastructure': '#0ea5e9',
    'OHE KPIs':       '#a855f7',
    'Kubernetes':     '#3b82f6',
    'SRE':            '#f59e0b',
    'Containers':     '#10b981',
    'Security':       '#ef4444',
    'Applications':   '#6366f1',
    'Prediction':     '#ec4899',
    'MLOps':          '#8b5cf6',
    'IoT':            '#14b8a6',
  }
</script>

<!-- ── Apply Template Modal ──────────────────────────────────────────────── -->
{#if applyModal}
  <div class="modal-backdrop" on:click|self={() => applyModal = null}>
    <div class="modal">
      <div class="modal-header">
        <div class="modal-icon-wrap">
          <svg viewBox="0 0 24 24" class="modal-icon" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d={ICONS[applyModal.icon] || ICONS['activity']}/>
          </svg>
        </div>
        <div>
          <div class="modal-title">{applyModal.name}</div>
          <div class="modal-desc">{applyModal.description}</div>
        </div>
      </div>

      <div class="modal-field">
        <label class="field-label">Dashboard name (optional)</label>
        <input class="inp" bind:value={applyName} placeholder={applyModal.name} />
      </div>

      <div class="modal-field">
        <label class="field-label">Data mode</label>
        <div class="mode-toggle">
          <button class="mode-btn {applyMode === 'current' ? 'active' : ''}" on:click={() => applyMode = 'current'}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mode-icon"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
            Current
            <span class="mode-hint">Live data · timeseries</span>
          </button>
          <button class="mode-btn {applyMode === 'predicted' ? 'active' : ''}" on:click={() => applyMode = 'predicted'}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mode-icon"><path d="M23 6l-9.5 9.5-5-5L1 18M17 6h6v6"/></svg>
            Predicted
            <span class="mode-hint">ML forecast · trend overlay</span>
          </button>
        </div>
      </div>

      <div class="modal-meta">
        <span class="meta-tag" style="background:{CATEGORY_COLOR[applyModal.category] || '#475569'}22;color:{CATEGORY_COLOR[applyModal.category] || '#94a3b8'}">{applyModal.category}</span>
        <span class="meta-count">{applyModal.widget_count} widgets</span>
        {#each (applyModal.tags || []).slice(0, 4) as tag}
          <span class="meta-tag-dim">{tag}</span>
        {/each}
      </div>

      <div class="modal-actions">
        <button class="btn-ghost" on:click={() => applyModal = null}>Cancel</button>
        <button class="btn-primary" on:click={confirmApply} disabled={applying}>
          {applying ? 'Creating…' : 'Apply Template'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- ── Tab bar ────────────────────────────────────────────────────────────── -->
{#if openTabs.length > 0 || activeTab !== null}
  <div class="tab-bar">
    <button
      class="tab-item"
      class:tab-active={activeTab === null && editId === null}
      on:click={() => { activeTab = null }}
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" class="tab-icon">
        <path d="M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z"/>
      </svg>
      Boards
    </button>
    {#each openTabs as tab (tab.id)}
      <div class="tab-item tab-with-close" class:tab-active={activeTab === tab.id && editId === null}>
        <button class="tab-label" on:click={() => activeTab = tab.id}>{tab.name}</button>
        <button class="tab-close" on:click={(e) => closeTab(tab.id, e)} title="Close tab">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="close-icon">
            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
          </svg>
        </button>
      </div>
    {/each}
  </div>
{/if}

<!-- ── Main content area ──────────────────────────────────────────────────── -->
{#if editId}
  <DashboardEdit dashboardId={editId} onBack={onEditDone} />

{:else if activeTab !== null}
  {@const tab = openTabs.find(t => t.id === activeTab)}
  {#if tab}
    {#key tab.id + '_' + tab.reloadKey}
      <DashboardView
        dashboardId={tab.dashboardId}
        onEdit={editDashboard}
      />
    {/key}
  {/if}

{:else}
  <div class="page">
    <div class="header">
      <h1>Dashboards</h1>
      <button class="btn" on:click={() => showNew = !showNew}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="btn-icon"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        New
      </button>
    </div>

    {#if showNew}
      <div class="new-form card">
        <input bind:value={newName} placeholder="Dashboard name" class="inp"/>
        <label class="inline-label">Refresh (s):
          <input type="number" bind:value={newRefresh} min="5" max="3600" class="inp-num"/>
        </label>
        <button class="btn-primary" on:click={create} disabled={!newName || creating}>Create</button>
        <button class="btn-ghost" on:click={() => showNew = false}>Cancel</button>
      </div>
    {/if}

    {#if loading}
      <div class="loading-row">
        <svg class="spin-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 12a9 9 0 11-6.219-8.56"/>
        </svg>
        Loading…
      </div>
    {:else}

      {#if dashboards.length > 0}
        <section>
          <h2>My Dashboards</h2>
          <div class="grid">
            {#each dashboards as d}
              <div class="dash-card card">
                <div class="dash-name">{d.name}</div>
                <div class="dash-meta">{d.widgets?.length || 0} widgets · refresh {d.refresh_seconds}s</div>
                <div class="dash-actions">
                  <button class="btn-sm view" on:click={() => openDashboard(d)}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="sm-icon">
                      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/>
                    </svg>
                    View
                  </button>
                  <button class="btn-sm edit" on:click={() => editDashboard(d.id)}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="sm-icon">
                      <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
                    </svg>
                    Edit
                  </button>
                  <button class="btn-sm danger" on:click={() => del(d.id)}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="sm-icon">
                      <polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/><path d="M10 11v6M14 11v6"/><path d="M9 6V4a1 1 0 011-1h4a1 1 0 011 1v2"/>
                    </svg>
                    Delete
                  </button>
                </div>
              </div>
            {/each}
          </div>
        </section>
      {:else}
        <p class="muted empty">No dashboards yet — apply a template below to get started.</p>
      {/if}

      {#if templates.length > 0}
        <section class="tmpl-section">
          <div class="tmpl-header">
            <h2>Template Gallery</h2>
            <span class="tmpl-count">{templates.length} templates</span>
          </div>

          <div class="category-pills">
            {#each categories as cat}
              <button
                class="pill {activeCategory === cat ? 'pill-active' : ''}"
                style={activeCategory === cat && cat !== 'All' ? `background:${CATEGORY_COLOR[cat] || '#475569'}22;color:${CATEGORY_COLOR[cat] || '#94a3b8'};border-color:${CATEGORY_COLOR[cat] || '#475569'}55` : ''}
                on:click={() => activeCategory = cat}
              >{cat}</button>
            {/each}
          </div>

          <div class="tmpl-grid">
            {#each filtered as t}
              {@const catColor = CATEGORY_COLOR[t.category] || '#475569'}
              <div class="tmpl-card">
                <div class="tmpl-top">
                  <div class="tmpl-icon-wrap" style="background:{catColor}18;color:{catColor}">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"
                         stroke-linecap="round" stroke-linejoin="round" class="tmpl-icon">
                      <path d={ICONS[t.icon] || ICONS['activity']}/>
                    </svg>
                  </div>
                  <div class="tmpl-info">
                    <div class="tmpl-name">{t.name}</div>
                    <span class="tmpl-cat" style="color:{catColor}">{t.category}</span>
                  </div>
                </div>
                <div class="tmpl-desc">{t.description}</div>
                <div class="tmpl-footer">
                  <span class="tmpl-wcount">{t.widget_count} widgets</span>
                  <div class="tmpl-tags">
                    {#each (t.tags || []).slice(0, 3) as tag}
                      <span class="tag">{tag}</span>
                    {/each}
                  </div>
                </div>
                <button class="apply-btn" on:click={() => openApplyModal(t)}>Apply</button>
              </div>
            {/each}
          </div>
        </section>
      {/if}

    {/if}
  </div>
{/if}

<style>
  /* ── Tab bar ─────────────────────────────────────────────────────────────── */
  .tab-bar {
    display: flex; align-items: center; flex-wrap: wrap; gap: 2px;
    background: #0f172a; border-bottom: 1px solid #334155;
    padding: 0 0 0 0; margin: -1.5rem -1.5rem 1rem;
  }
  .tab-item {
    display: flex; align-items: center; gap: 5px;
    background: transparent; border: none; border-bottom: 2px solid transparent;
    color: #64748b; padding: 10px 16px 8px; cursor: pointer; font-size: 0.82rem;
    transition: all 0.15s; white-space: nowrap;
  }
  .tab-item:hover { color: #e2e8f0; background: #1e293b22; }
  .tab-item.tab-active { color: #38bdf8; border-bottom-color: #38bdf8; background: #0f172a; }
  .tab-icon { width: 13px; height: 13px; }

  .tab-with-close { padding-right: 6px; }
  .tab-label { background: none; border: none; color: inherit; cursor: pointer; font-size: inherit; padding: 0; }
  .tab-close {
    background: none; border: none; cursor: pointer; color: #475569;
    padding: 2px; border-radius: 3px; display: flex; align-items: center;
    transition: color 0.1s, background 0.1s; margin-left: 4px;
  }
  .tab-close:hover { color: #ef4444; background: #ef444420; }
  .close-icon { width: 10px; height: 10px; }

  /* ── Page ───────────────────────────────────────────────────────────────── */
  .page { padding: 0; }
  .header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
  h1 { margin: 0; font-size: 1.2rem; color: #e2e8f0; font-weight: 700; }
  h2 { font-size: 0.7rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.08em; margin: 0 0 0.6rem; }
  section { margin-bottom: 2rem; }

  .loading-row { display: flex; align-items: center; gap: 8px; color: #64748b; font-size: 0.85rem; padding: 1rem 0; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .spin-icon { width: 16px; height: 16px; animation: spin 1s linear infinite; flex-shrink: 0; }

  .card { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 1rem; }
  .new-form { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 1rem; flex-wrap: wrap; }
  .inline-label { display: flex; align-items: center; gap: 4px; font-size: 0.8rem; color: #94a3b8; }
  .inp { background: #0f172a; border: 1px solid #334155; color: #e2e8f0; padding: 0.4rem 0.6rem; border-radius: 5px; font-size: 0.85rem; }
  .inp-num { width: 70px; background: #0f172a; border: 1px solid #334155; color: #e2e8f0; padding: 0.3rem 0.4rem; border-radius: 4px; }

  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 0.75rem; }
  .dash-card { display: flex; flex-direction: column; gap: 0.4rem; }
  .dash-name { font-weight: 600; color: #e2e8f0; font-size: 0.9rem; }
  .dash-meta { font-size: 0.72rem; color: #64748b; }
  .dash-actions { margin-top: auto; display: flex; gap: 0.4rem; flex-wrap: wrap; }

  .btn { display: flex; align-items: center; gap: 5px; background: #334155; border: none; color: #e2e8f0; padding: 0.35rem 0.75rem; border-radius: 5px; cursor: pointer; font-size: 0.85rem; }
  .btn:hover { background: #475569; }
  .btn-icon { width: 13px; height: 13px; }

  .btn-primary { background: #0284c7; border: none; color: #fff; padding: 0.4rem 0.9rem; border-radius: 6px; cursor: pointer; font-size: 0.85rem; font-weight: 600; }
  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn-primary:hover:not(:disabled) { background: #0369a1; }
  .btn-ghost { background: transparent; border: 1px solid #334155; color: #94a3b8; padding: 0.4rem 0.9rem; border-radius: 6px; cursor: pointer; font-size: 0.85rem; }
  .btn-ghost:hover { border-color: #475569; color: #e2e8f0; }

  .btn-sm { display: flex; align-items: center; gap: 4px; border: none; padding: 3px 9px; border-radius: 4px; cursor: pointer; font-size: 0.75rem; }
  .btn-sm.view   { background: #0f3460; color: #38bdf8; }
  .btn-sm.view:hover { background: #0369a1; }
  .btn-sm.edit   { background: #1a2744; color: #818cf8; border: 1px solid #334155; }
  .btn-sm.edit:hover { background: #1e3a6e; }
  .btn-sm.danger { background: #7f1d1d20; border: 1px solid #7f1d1d; color: #fca5a5; }
  .btn-sm.danger:hover { background: #7f1d1d; }
  .sm-icon { width: 11px; height: 11px; }

  /* ── Template gallery ───────────────────────────────────────────────────── */
  .tmpl-header { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 0.75rem; }
  .tmpl-count  { font-size: 0.7rem; color: #475569; background: #0f172a; border: 1px solid #334155; border-radius: 10px; padding: 1px 8px; }

  .category-pills { display: flex; flex-wrap: wrap; gap: 0.4rem; margin-bottom: 0.9rem; }
  .pill { background: #1e293b; border: 1px solid #334155; color: #64748b; padding: 3px 10px; border-radius: 20px; cursor: pointer; font-size: 0.72rem; transition: all 0.15s; }
  .pill:hover { border-color: #475569; color: #94a3b8; }
  .pill-active { background: #0f172a; border-color: #475569; color: #e2e8f0; }

  .tmpl-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 0.75rem; }
  .tmpl-card {
    background: #1e293b; border: 1px solid #334155; border-radius: 10px;
    padding: 0.9rem; display: flex; flex-direction: column; gap: 0.55rem;
    transition: border-color 0.15s, transform 0.1s;
  }
  .tmpl-card:hover { border-color: #475569; transform: translateY(-1px); }

  .tmpl-top { display: flex; align-items: flex-start; gap: 0.65rem; }
  .tmpl-icon-wrap { width: 36px; height: 36px; border-radius: 8px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
  .tmpl-icon { width: 18px; height: 18px; }
  .tmpl-info { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
  .tmpl-name { font-weight: 700; color: #e2e8f0; font-size: 0.87rem; line-height: 1.2; }
  .tmpl-cat  { font-size: 0.67rem; font-weight: 600; letter-spacing: 0.04em; }
  .tmpl-desc { font-size: 0.73rem; color: #64748b; line-height: 1.45; flex: 1; }

  .tmpl-footer { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }
  .tmpl-wcount { font-size: 0.67rem; color: #475569; }
  .tmpl-tags { display: flex; flex-wrap: wrap; gap: 3px; }
  .tag { background: #0f172a; border: 1px solid #1e3a5f; color: #38bdf8; font-size: 0.6rem; padding: 1px 5px; border-radius: 3px; }

  .apply-btn {
    width: 100%; background: linear-gradient(135deg, #0284c7, #0369a1);
    border: none; color: #fff; padding: 0.38rem 0; border-radius: 6px;
    cursor: pointer; font-size: 0.8rem; font-weight: 600; transition: opacity 0.15s;
    margin-top: auto;
  }
  .apply-btn:hover { opacity: 0.88; }

  /* ── Modal ──────────────────────────────────────────────────────────────── */
  .modal-backdrop {
    position: fixed; inset: 0; background: rgba(0,0,0,0.65);
    display: flex; align-items: center; justify-content: center; z-index: 200;
  }
  .modal {
    background: #1e293b; border: 1px solid #334155; border-radius: 12px;
    padding: 1.5rem; width: 420px; max-width: 95vw;
    display: flex; flex-direction: column; gap: 1rem;
  }
  .modal-header { display: flex; align-items: flex-start; gap: 0.75rem; }
  .modal-icon-wrap { width: 32px; height: 32px; background: #38bdf820; border-radius: 8px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
  .modal-icon { width: 18px; height: 18px; color: #38bdf8; }
  .modal-title { font-weight: 700; color: #e2e8f0; font-size: 1rem; }
  .modal-desc  { font-size: 0.76rem; color: #64748b; margin-top: 2px; line-height: 1.4; }
  .modal-field { display: flex; flex-direction: column; gap: 0.3rem; }
  .field-label { font-size: 0.7rem; color: #94a3b8; font-weight: 600; text-transform: uppercase; letter-spacing: 0.06em; }
  .modal-field .inp { width: 100%; box-sizing: border-box; }

  .mode-toggle { display: flex; gap: 0.5rem; }
  .mode-btn {
    flex: 1; display: flex; flex-direction: column; align-items: center; gap: 4px;
    padding: 0.65rem 0.4rem; border-radius: 8px;
    background: #0f172a; border: 2px solid #334155; color: #64748b;
    cursor: pointer; font-size: 0.8rem; font-weight: 600; transition: all 0.15s;
  }
  .mode-btn.active { border-color: #0284c7; background: #0284c720; color: #38bdf8; }
  .mode-btn:hover:not(.active) { border-color: #475569; color: #94a3b8; }
  .mode-icon { width: 18px; height: 18px; }
  .mode-hint { font-size: 0.63rem; font-weight: 400; opacity: 0.75; text-align: center; }

  .modal-meta { display: flex; align-items: center; flex-wrap: wrap; gap: 0.4rem; }
  .meta-tag     { font-size: 0.68rem; font-weight: 600; padding: 2px 8px; border-radius: 4px; }
  .meta-tag-dim { font-size: 0.65rem; background: #0f172a; color: #475569; padding: 2px 6px; border-radius: 3px; }
  .meta-count   { font-size: 0.68rem; color: #475569; }
  .modal-actions { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 0.25rem; }

  .muted { color: #64748b; font-size: 0.85rem; }
  .empty { margin: 0.25rem 0 1.5rem; }
</style>

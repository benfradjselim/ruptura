<script>
  import { isLoggedIn, currentPage, theme } from './lib/store.js'
  import Login        from './pages/Login.svelte'
  import Dashboard    from './pages/Dashboard.svelte'
  import Alerts       from './pages/Alerts.svelte'
  import Dashboards   from './pages/Dashboards.svelte'
  import Settings     from './pages/Settings.svelte'
  import Fleet        from './pages/Fleet.svelte'
  import Notifications from './pages/Notifications.svelte'
  import Logs         from './pages/Logs.svelte'
  import Traces       from './pages/Traces.svelte'
  import AlertRules   from './pages/AlertRules.svelte'
  import Datasources  from './pages/Datasources.svelte'
  import Orgs         from './pages/Orgs.svelte'
  import SLOs         from './pages/SLOs.svelte'
  import { api }      from './lib/api.js'
  import { SECURITY_TEMPLATE } from './lib/templates/security.js'

  function toggleTheme() {
    theme.update(t => t === 'dark' ? 'light' : 'dark')
  }

  // SVG icon paths (Lucide-style)
  const NAV_ICONS = {
    orgs:            'M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2M9 7a4 4 0 100 8 4 4 0 000-8zM23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75',
    slos:            'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
    dashboard:       'M3 9l9-7 9 7v11a2 2 0 01-2 2H5a2 2 0 01-2-2zM9 22V12h6v10',
    fleet:           'M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z',
    dashboards:      'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z',
    logs:            'M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8zM14 2v6h6M16 13H8M16 17H8M10 9H8',
    traces:          'M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01',
    alerts:          'M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0zM12 9v4M12 17h.01',
    'alert-rules':   'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4',
    datasources:     'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4M4 12c0 2.21 3.582 4 8 4s8-1.79 8-4',
    notifications:   'M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9M13.73 21a2 2 0 01-3.46 0',
    settings:        'M12 15a3 3 0 100-6 3 3 0 000 6zM19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z',
  }

  const navGroups = [
    {
      label: 'Observe',
      items: [
        { id: 'dashboard',  label: 'Overview'   },
        { id: 'fleet',      label: 'Fleet'      },
        { id: 'dashboards', label: 'Boards'     },
      ],
    },
    {
      label: 'Explore',
      items: [
        { id: 'logs',   label: 'Logs'   },
        { id: 'traces', label: 'Traces' },
      ],
    },
    {
      label: 'Respond',
      items: [
        { id: 'alerts',      label: 'Alerts'      },
        { id: 'alert-rules', label: 'Alert Rules' },
        { id: 'slos',        label: 'SLOs'        },
      ],
    },
    {
      label: 'Configure',
      items: [
        { id: 'datasources',   label: 'Data Sources' },
        { id: 'notifications', label: 'Channels'     },
        { id: 'orgs',          label: 'Orgs'         },
        { id: 'settings',      label: 'Settings'     },
      ],
    },
  ]

  async function applySecurityTemplate() {
    try {
      await api.dashboardCreate(SECURITY_TEMPLATE)
      currentPage.set('dashboards')
    } catch {}
  }

  // Grid overlay toggle — G key or button
  function toggleGrid() {
    document.body.classList.toggle('grid-on')
  }

  // Keyboard shortcut
  function handleKey(e) {
    if ((e.key === 'g' || e.key === 'G') && !e.metaKey && !e.ctrlKey && !e.altKey && e.target.tagName !== 'INPUT') {
      toggleGrid()
    }
  }
</script>

<svelte:window on:keydown={handleKey} />

{#if !$isLoggedIn}
  <Login />
{:else}
  <div class="layout">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-logo">
          <svg viewBox="0 0 24 24" fill="none" stroke="#38bdf8" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="logo-icon">
            <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
          </svg>
        </div>
        <div>
          <span class="brand-text">Ruptura</span>
          <span class="brand-ver">v7</span>
        </div>
      </div>

      <nav>
        {#each navGroups as group}
          <div class="nav-group">
            <span class="nav-group-label">{group.label}</span>
            {#each group.items as item}
              <button
                class="nav-item"
                class:active={$currentPage === item.id}
                on:click={() => currentPage.set(item.id)}
              >
                <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                  <path d={NAV_ICONS[item.id] || ''}/>
                </svg>
                <span class="label">{item.label}</span>
              </button>
            {/each}
          </div>
        {/each}
      </nav>

      <div class="sidebar-footer">
        <!-- Dark / Light mode toggle -->
        <button class="theme-toggle" on:click={toggleTheme} title="{$theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}">
          {#if $theme === 'dark'}
            <!-- Sun icon -->
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="5"/>
              <line x1="12" y1="1" x2="12" y2="3"/>
              <line x1="12" y1="21" x2="12" y2="23"/>
              <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
              <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
              <line x1="1" y1="12" x2="3" y2="12"/>
              <line x1="21" y1="12" x2="23" y2="12"/>
              <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
              <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
            </svg>
            <span class="label">Light mode</span>
          {:else}
            <!-- Moon icon -->
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
            </svg>
            <span class="label">Dark mode</span>
          {/if}
        </button>

        <button class="sec-btn" on:click={applySecurityTemplate} title="Create Security Dashboard">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" class="sec-icon">
            <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
          </svg>
          Security Board
        </button>
      </div>
    </aside>

    <!-- Grid overlay toggle (G key or button) -->
    <button class="grid-toggle" on:click={toggleGrid} aria-label="Toggle grid overlay">
      <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor">
        <rect x="0" y="0" width="4" height="4"/><rect x="6" y="0" width="4" height="4"/>
        <rect x="0" y="6" width="4" height="4"/><rect x="6" y="6" width="4" height="4"/>
      </svg>
      Grid
    </button>

    <main class="content">
      {#if $currentPage === 'dashboard'}
        <Dashboard />
      {:else if $currentPage === 'fleet'}
        <Fleet />
      {:else if $currentPage === 'alerts'}
        <Alerts />
      {:else if $currentPage === 'dashboards'}
        <Dashboards />
      {:else if $currentPage === 'notifications'}
        <Notifications />
      {:else if $currentPage === 'settings'}
        <Settings />
      {:else if $currentPage === 'logs'}
        <Logs />
      {:else if $currentPage === 'traces'}
        <Traces />
      {:else if $currentPage === 'alert-rules'}
        <AlertRules />
      {:else if $currentPage === 'datasources'}
        <Datasources />
      {:else if $currentPage === 'orgs'}
        <Orgs />
      {:else if $currentPage === 'slos'}
        <SLOs />
      {/if}
    </main>
  </div>
{/if}

<style>
  :global(*) { box-sizing: border-box; margin: 0; padding: 0; }
  :global(body) {
    background: var(--bg, #0F172A); color: var(--text, #E2E8F0);
    font-family: "Inter", system-ui, -apple-system, sans-serif;
    font-size: 13px; line-height: 24px; -webkit-font-smoothing: antialiased;
  }
  :global(input:focus), :global(select:focus) { outline: none; border-color: var(--accent, #38BDF8) !important; }

  .layout { display: flex; min-height: 100vh; }

  .sidebar {
    width: 184px;
    background: var(--surface, #1E293B);
    border-right: 1px solid var(--border, rgba(148,163,184,0.10));
    display: flex; flex-direction: column;
    position: sticky; top: 0; height: 100vh;
    flex-shrink: 0; overflow-y: auto;
  }

  .brand {
    display: flex; align-items: center; gap: 10px;
    padding: 20px 16px 16px;
    border-bottom: 1px solid var(--border, rgba(148,163,184,0.10));
  }
  .brand-logo { display: flex; align-items: center; }
  .logo-icon  { width: 20px; height: 20px; }
  .brand-text { font-size: 15px; font-weight: 800; color: var(--accent, #38BDF8); line-height: 1; letter-spacing: -0.02em; }
  .brand-ver  { font-size: 9px; color: var(--text-3, #3F4D5C); font-weight: 600; margin-left: 3px; vertical-align: super; font-family: "DM Mono", monospace; }

  nav { display: flex; flex-direction: column; padding: 8px; flex: 1; gap: 2px; }

  .nav-group { margin-bottom: 8px; }
  .nav-group-label {
    display: block; font-size: 9px; font-weight: 700;
    color: var(--text-3, #3F4D5C); text-transform: uppercase; letter-spacing: 0.12em;
    padding: 8px 8px 4px;
  }

  .nav-item {
    display: flex; align-items: center; gap: 8px; width: 100%;
    background: transparent; border: none; color: var(--text-2, #94A3B8);
    padding: 6px 8px; border-radius: 4px;
    cursor: pointer; font-size: 12px; font-weight: 500; text-align: left;
    transition: background 0.10s, color 0.10s; font-family: inherit; line-height: 1;
  }
  .nav-item:hover  { background: var(--surface-2, #253045); color: var(--text, #E2E8F0); }
  .nav-item.active { background: var(--accent-dim, rgba(56,189,248,0.12)); color: var(--accent, #38BDF8); }
  .nav-icon { width: 13px; height: 13px; flex-shrink: 0; }

  .sidebar-footer {
    padding: 8px; border-top: 1px solid var(--border, rgba(148,163,184,0.10)); margin-top: auto;
    display: flex; flex-direction: column; gap: 4px;
  }

  .theme-toggle {
    display: flex; align-items: center; gap: 6px; width: 100%;
    background: transparent; border: 1px solid var(--border, rgba(148,163,184,0.10));
    color: var(--text-3, #3F4D5C); padding: 6px 8px; border-radius: 4px;
    cursor: pointer; font-size: 11px; text-align: left; font-family: inherit;
    transition: background 0.12s, color 0.12s, border-color 0.12s;
  }
  .theme-toggle:hover { background: var(--surface-2); color: var(--text-2); border-color: var(--border-2); }
  .theme-toggle svg { width: 12px; height: 12px; flex-shrink: 0; }

  .sec-btn {
    display: flex; align-items: center; gap: 6px; width: 100%;
    background: transparent; border: 1px solid var(--border, rgba(148,163,184,0.10));
    color: var(--text-3, #3F4D5C); padding: 6px 8px; border-radius: 4px;
    cursor: pointer; font-size: 11px; text-align: left; font-family: inherit;
    transition: all 0.12s;
  }
  .sec-btn:hover { border-color: var(--accent); color: var(--accent); background: var(--accent-dim); }
  .sec-icon { width: 12px; height: 12px; flex-shrink: 0; }

  /* Grid toggle button */
  .grid-toggle {
    position: fixed; bottom: 24px; right: 24px; z-index: 200;
    display: flex; align-items: center; gap: 6px;
    background: var(--surface-2); border: 1px solid var(--border-2);
    color: var(--text-3); font-family: "DM Mono", monospace;
    font-size: 10px; letter-spacing: 0.1em; text-transform: uppercase;
    padding: 6px 10px; border-radius: 4px; cursor: pointer; transition: all 0.15s;
  }
  .grid-toggle:hover { color: var(--text-2); border-color: var(--border-3); }
  :global(body.grid-on) .grid-toggle { background: var(--accent); color: #000; border-color: transparent; }

  .content { flex: 1; overflow-y: auto; min-width: 0; }
</style>

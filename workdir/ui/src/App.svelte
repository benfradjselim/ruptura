<script>
  import { isLoggedIn, currentPage } from './lib/store.js'
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
  import { api }      from './lib/api.js'
  import { SECURITY_TEMPLATE } from './lib/templates/security.js'

  // SVG icon paths (Lucide-style)
  const NAV_ICONS = {
    orgs:            'M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2M9 7a4 4 0 100 8 4 4 0 000-8zM23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75',
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
</script>

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
          <span class="brand-text">OHE</span>
          <span class="brand-ver">v4</span>
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
        <button class="sec-btn" on:click={applySecurityTemplate} title="Create Security Dashboard">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" class="sec-icon">
            <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
          </svg>
          Security Board
        </button>
      </div>
    </aside>

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
      {/if}
    </main>
  </div>
{/if}

<style>
  :global(*) { box-sizing: border-box; margin: 0; padding: 0; }
  :global(body) { background: #0f172a; color: #e2e8f0; font-family: system-ui, -apple-system, sans-serif; }
  :global(input), :global(select) { outline: none; }
  :global(input:focus), :global(select:focus) { border-color: #38bdf8 !important; }

  .layout { display: flex; min-height: 100vh; }

  .sidebar {
    width: 190px;
    background: #1e293b;
    border-right: 1px solid #334155;
    display: flex;
    flex-direction: column;
    position: sticky;
    top: 0;
    height: 100vh;
    flex-shrink: 0;
    overflow-y: auto;
  }

  .brand {
    display: flex; align-items: center; gap: 8px;
    padding: 0.9rem 1rem 0.75rem;
    border-bottom: 1px solid #334155;
  }
  .brand-logo { display: flex; align-items: center; }
  .logo-icon  { width: 20px; height: 20px; }
  .brand-text { font-size: 1.2rem; font-weight: 800; color: #38bdf8; line-height: 1; }
  .brand-ver  { font-size: 0.6rem; color: #475569; font-weight: 600; margin-left: 3px; vertical-align: super; }

  nav { display: flex; flex-direction: column; gap: 0; padding: 0.5rem 0; flex: 1; }

  .nav-group { padding: 0 0.5rem 0.25rem; }
  .nav-group-label {
    display: block; font-size: 0.6rem; font-weight: 700;
    color: #334155; text-transform: uppercase; letter-spacing: 0.08em;
    padding: 0.6rem 0.75rem 0.2rem;
  }

  .nav-item {
    display: flex; align-items: center; gap: 0.6rem; width: 100%;
    background: transparent; border: none; color: #64748b;
    padding: 0.45rem 0.75rem; border-radius: 6px;
    cursor: pointer; font-size: 0.83rem; text-align: left;
    transition: background 0.12s, color 0.12s;
  }
  .nav-item:hover  { background: #334155; color: #e2e8f0; }
  .nav-item.active { background: #0f3460; color: #38bdf8; }

  .nav-icon { width: 15px; height: 15px; flex-shrink: 0; }

  .sidebar-footer {
    padding: 0.75rem; border-top: 1px solid #334155; margin-top: auto;
  }
  .sec-btn {
    display: flex; align-items: center; gap: 6px;
    width: 100%; background: #1d1f2e; border: 1px solid #334155;
    color: #94a3b8; padding: 7px 10px; border-radius: 6px; cursor: pointer;
    font-size: 0.78rem; text-align: left; transition: all 0.15s;
  }
  .sec-btn:hover { background: #0f3460; border-color: #38bdf8; color: #38bdf8; }
  .sec-icon { width: 13px; height: 13px; flex-shrink: 0; }

  .content {
    flex: 1; padding: 1.5rem; overflow-y: auto;
  }
</style>

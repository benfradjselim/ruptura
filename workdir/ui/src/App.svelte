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
  import { api }      from './lib/api.js'
  import { SECURITY_TEMPLATE } from './lib/templates/security.js'

  const navGroups = [
    {
      label: 'Observe',
      items: [
        { id: 'dashboard',     label: 'Overview',     icon: '◎' },
        { id: 'fleet',         label: 'Fleet',        icon: '⬡' },
        { id: 'dashboards',    label: 'Boards',       icon: '⊞' },
      ],
    },
    {
      label: 'Explore',
      items: [
        { id: 'logs',          label: 'Logs',         icon: '📋' },
        { id: 'traces',        label: 'Traces',       icon: '🔗' },
      ],
    },
    {
      label: 'Respond',
      items: [
        { id: 'alerts',        label: 'Alerts',       icon: '⚡' },
        { id: 'alert-rules',   label: 'Alert Rules',  icon: '⚖' },
      ],
    },
    {
      label: 'Configure',
      items: [
        { id: 'datasources',   label: 'Data Sources', icon: '🔌' },
        { id: 'notifications', label: 'Channels',     icon: '🔔' },
        { id: 'settings',      label: 'Settings',     icon: '⚙' },
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
        <span class="brand-text">OHE</span>
        <span class="brand-ver">v4</span>
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
                <span class="icon">{item.icon}</span>
                <span class="label">{item.label}</span>
              </button>
            {/each}
          </div>
        {/each}
      </nav>

      <!-- Security template shortcut -->
      <div class="sidebar-footer">
        <button class="sec-btn" on:click={applySecurityTemplate} title="Create Security Dashboard">
          🔒 Security Board
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
    display: flex; align-items: baseline; gap: 4px;
    padding: 1rem 1rem 0.75rem;
    border-bottom: 1px solid #334155;
  }
  .brand-text { font-size: 1.4rem; font-weight: 800; color: #38bdf8; }
  .brand-ver  { font-size: 0.65rem; color: #475569; font-weight: 600; }

  nav { display: flex; flex-direction: column; gap: 0; padding: 0.5rem 0; flex: 1; }

  .nav-group { padding: 0 0.5rem 0.25rem; }
  .nav-group-label {
    display: block; font-size: 0.62rem; font-weight: 700;
    color: #334155; text-transform: uppercase; letter-spacing: 0.08em;
    padding: 0.6rem 0.75rem 0.2rem;
  }

  .nav-item {
    display: flex; align-items: center; gap: 0.55rem; width: 100%;
    background: transparent; border: none; color: #64748b;
    padding: 0.45rem 0.75rem; border-radius: 5px;
    cursor: pointer; font-size: 0.85rem; text-align: left;
    transition: background 0.1s, color 0.1s;
  }
  .nav-item:hover  { background: #334155; color: #e2e8f0; }
  .nav-item.active { background: #0f3460; color: #38bdf8; }
  .icon { font-size: 0.9rem; width: 18px; text-align: center; }

  .sidebar-footer {
    padding: 0.75rem; border-top: 1px solid #334155; margin-top: auto;
  }
  .sec-btn {
    width: 100%; background: #1d1f2e; border: 1px solid #334155;
    color: #94a3b8; padding: 7px 8px; border-radius: 6px; cursor: pointer;
    font-size: 0.78rem; text-align: left;
  }
  .sec-btn:hover { background: #0f3460; border-color: #38bdf8; color: #38bdf8; }

  .content {
    flex: 1; padding: 1.5rem; overflow-y: auto;
  }
</style>

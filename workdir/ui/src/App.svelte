<script>
  import { isLoggedIn, currentPage } from './lib/store.js'
  import Login from './pages/Login.svelte'
  import Dashboard from './pages/Dashboard.svelte'
  import Alerts from './pages/Alerts.svelte'
  import Dashboards from './pages/Dashboards.svelte'
  import Settings from './pages/Settings.svelte'

  const nav = [
    { id: 'dashboard', label: 'Overview', icon: '◎' },
    { id: 'alerts',    label: 'Alerts',   icon: '⚡' },
    { id: 'dashboards',label: 'Boards',   icon: '⊞' },
    { id: 'settings',  label: 'Settings', icon: '⚙' },
  ]
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
        {#each nav as item}
          <button
            class="nav-item"
            class:active={$currentPage === item.id}
            on:click={() => currentPage.set(item.id)}
          >
            <span class="icon">{item.icon}</span>
            <span class="label">{item.label}</span>
          </button>
        {/each}
      </nav>
    </aside>

    <main class="content">
      {#if $currentPage === 'dashboard'}
        <Dashboard />
      {:else if $currentPage === 'alerts'}
        <Alerts />
      {:else if $currentPage === 'dashboards'}
        <Dashboards />
      {:else if $currentPage === 'settings'}
        <Settings />
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
    width: 180px;
    background: #1e293b;
    border-right: 1px solid #334155;
    display: flex;
    flex-direction: column;
    padding: 1rem 0;
    position: sticky;
    top: 0;
    height: 100vh;
    flex-shrink: 0;
  }

  .brand {
    display: flex;
    align-items: baseline;
    gap: 4px;
    padding: 0 1rem 1.5rem;
    border-bottom: 1px solid #334155;
    margin-bottom: 0.75rem;
  }
  .brand-text { font-size: 1.5rem; font-weight: 800; color: #38bdf8; }
  .brand-ver { font-size: 0.7rem; color: #475569; font-weight: 600; }

  nav { display: flex; flex-direction: column; gap: 2px; padding: 0 0.5rem; }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    width: 100%;
    background: transparent;
    border: none;
    color: #64748b;
    padding: 0.55rem 0.75rem;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.9rem;
    text-align: left;
    transition: background 0.1s, color 0.1s;
  }
  .nav-item:hover { background: #334155; color: #e2e8f0; }
  .nav-item.active { background: #0f3460; color: #38bdf8; }
  .icon { font-size: 1rem; width: 20px; text-align: center; }

  .content {
    flex: 1;
    padding: 1.5rem;
    overflow-y: auto;
    max-width: 1200px;
  }
</style>

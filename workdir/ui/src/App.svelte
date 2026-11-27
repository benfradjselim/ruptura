<script>
  import { isLoggedIn, currentPage, theme } from './lib/store.js'
  import Login        from './pages/Login.svelte'
  import Settings     from './pages/Settings.svelte'
  import Fleet        from './pages/Fleet.svelte'
  import Failures     from './pages/Failures.svelte'
  import CallGraph    from './pages/CallGraph.svelte'
  import Infra        from './pages/Infra.svelte'
  import Notifications from './pages/Notifications.svelte'
  import AlertRules   from './pages/AlertRules.svelte'
  import Orgs         from './pages/Orgs.svelte'
  import { api }      from './lib/api.js'
  import DemoBanner   from './lib/components/DemoBanner.svelte'
  import { onMount } from 'svelte'
  import Mark from './lib/components/Mark.svelte'

  const HOME_PAGE = 'failures'
  const PRIMARY_NAV = [
    { id: 'failures', label: 'Rupture Horizon' },
    { id: 'callgraph', label: 'Call graph' },
    { id: 'fleet', label: 'Fleet' },
    { id: 'infra', label: 'Infra' },
  ]
  const PRIMARY_IDS = PRIMARY_NAV.map(item => item.id)

  let appVersion = 'v8'
  let moreOpen = false
  $: moreActive = !PRIMARY_IDS.includes($currentPage)

  function openPage(id) {
    currentPage.set(id)
    moreOpen = false
  }

  onMount(async () => {
    try {
      const h = await api.health()
      if (h.version) appVersion = 'v' + h.version
    } catch {}
  })

  function toggleTheme() {
    theme.update(t => t === 'dark' ? 'light' : 'dark')
  }

  // SVG icon paths (Lucide-style)
  const NAV_ICONS = {
    orgs:            'M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2M9 7a4 4 0 100 8 4 4 0 000-8zM23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75',
    callgraph:       'M5 6a2 2 0 100-4 2 2 0 000 4zM19 14a2 2 0 100-4 2 2 0 000 4zM5 22a2 2 0 100-4 2 2 0 000 4zM7 4h4a2 2 0 012 2v4a2 2 0 002 2h2M7 20h4a2 2 0 002-2v-4a2 2 0 012-2',
    failures:        'M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z',
    fleet:           'M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z',
    infra:           'M5 12H3m4-7l-2-2m10 2 2-2M5 19l-2 2m14-2 2 2M12 3v2m0 14v2M7 7l10 10M17 7 7 17',
    'alert-rules':   'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4',
    notifications:   'M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9M13.73 21a2 2 0 01-3.46 0',
    settings:        'M12 15a3 3 0 100-6 3 3 0 000 6zM19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z',
  }

  const navGroups = [
    {
      label: 'Configure',
      items: [
        { id: 'alert-rules', label: 'Alert rules' },
        { id: 'notifications', label: 'Channels' },
        { id: 'settings', label: 'Settings' },
      ],
    },
  ]


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
    <header class="rail">
      <button class="brand" on:click={() => currentPage.set(HOME_PAGE)} aria-label="Ruptura home">
        <Mark size={30} />
        <span class="wordmark">RUPTURA</span>
      </button>

      <nav aria-label="Main">
        {#each PRIMARY_NAV as item}
          <button class="tab" class:active={$currentPage === item.id} on:click={() => openPage(item.id)}>{item.label}</button>
        {/each}
        <div class="more">
          <button class="tab" class:active={moreActive} aria-expanded={moreOpen} on:click={() => (moreOpen = !moreOpen)}>More</button>
          {#if moreOpen}
            <div class="menu">
              {#each navGroups as group}
                <div class="menu-group">
                  <span class="menu-label">{group.label}</span>
                  {#each group.items.filter(item => !PRIMARY_IDS.includes(item.id)) as item}
                    <button class="menu-item" class:active={$currentPage === item.id} on:click={() => openPage(item.id)}>{item.label}</button>
                  {/each}
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </nav>

      <div class="rail-end">
        <span class="version">{appVersion}</span>
        <button class="mode" on:click={toggleTheme}>{$theme === 'dark' ? 'Light' : 'Dark'}</button>
      </div>
    </header>

    <main class="content">
      <DemoBanner />
      {#if $currentPage === 'failures'}
        <Failures />
      {:else if $currentPage === 'callgraph'}
        <CallGraph />
      {:else if $currentPage === 'fleet'}
        <Fleet />
      {:else if $currentPage === 'infra'}
        <Infra />
      {:else if $currentPage === 'notifications'}
        <Notifications />
      {:else if $currentPage === 'settings'}
        <Settings />
      {:else if $currentPage === 'alert-rules'}
        <AlertRules />
      {:else if $currentPage === 'orgs'}
        <Orgs />
      {/if}
    </main>
  </div>
{/if}

<style>
  .layout { min-height: 100vh; display: flex; flex-direction: column; }

  .rail {
    position: sticky; top: 0; z-index: 40;
    display: flex; align-items: center; gap: 28px;
    height: 60px; padding: 0 28px;
    background: var(--bg);
    border-bottom: 1px solid var(--border-2);
  }

  .brand { display: flex; align-items: center; gap: 12px; background: none; border: 0; color: var(--text); cursor: pointer; }
  .wordmark { font-weight: 700; font-size: 17px; letter-spacing: 0.22em; }

  nav { display: flex; align-items: stretch; height: 100%; flex: 1; gap: 4px; }

  .tab {
    position: relative; padding: 0 16px; height: 100%;
    background: none; border: 0; color: var(--text-2);
    font: 600 14px var(--font-ui); letter-spacing: 0.05em; text-transform: uppercase;
    cursor: pointer;
  }
  .tab:hover { color: var(--text); }
  .tab.active { color: var(--accent); }
  .tab.active::after {
    content: ""; position: absolute; left: 10px; right: 10px; bottom: -1px; height: 3px;
    background: var(--accent); transform: skewX(-45deg);
  }
  .tab:focus-visible, .menu-item:focus-visible, .mode:focus-visible, .brand:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }

  .more { position: relative; height: 100%; }
  .menu {
    position: absolute; top: 100%; left: 0; min-width: 200px;
    display: grid; grid-template-columns: 1fr; gap: 18px;
    padding: 18px; background: var(--surface); border: 1px solid var(--border-2);
    clip-path: polygon(0 0, 100% 0, 100% calc(100% - 14px), calc(100% - 14px) 100%, 0 100%);
  }
  .menu-group { display: flex; flex-direction: column; gap: 2px; }
  .menu-label { font: 500 11px var(--font-mono); letter-spacing: 0.08em; text-transform: uppercase; color: var(--text-3); padding: 0 8px 6px; }
  .menu-item { text-align: left; padding: 6px 8px; background: none; border: 0; color: var(--text-2); font: 500 14px var(--font-ui); cursor: pointer; }
  .menu-item:hover { color: var(--text); background: var(--surface-2); }
  .menu-item.active { color: var(--accent); }

  .rail-end { display: flex; align-items: center; gap: 16px; }
  .version { font: 400 12px var(--font-mono); color: var(--text-3); }
  .mode { background: none; border: 1px solid var(--border-2); color: var(--text-2); padding: 4px 10px; font: 500 12px var(--font-mono); text-transform: uppercase; cursor: pointer; }
  .mode:hover { color: var(--text); }

  .content { flex: 1; min-width: 0; }

  @media (max-width: 900px) {
    .rail { gap: 12px; padding: 0 12px; overflow-x: auto; }
    .wordmark, .version { display: none; }
    .menu { position: fixed; left: 12px; right: 12px; min-width: 0; grid-template-columns: 1fr 1fr; }
  }
</style>


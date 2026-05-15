<script lang="ts">
  import { onMount } from 'svelte'
  import NavBar from './components/NavBar.svelte'
  import Fleet from './routes/Fleet.svelte'
  import Map from './routes/Map.svelte'
  import Engine from './routes/Engine.svelte'
  import Nodes from './routes/Nodes.svelte'
  import Alerts from './routes/Alerts.svelte'
  import Settings from './routes/Settings.svelte'

  let route = ''
  let theme = 'dark'

  function readHash() {
    route = window.location.hash.replace(/^#\/?/, '') || 'fleet'
  }

  function toggleTheme() {
    theme = theme === 'dark' ? 'light' : 'dark'
    localStorage.setItem('ruptura:theme', theme)
    document.documentElement.setAttribute('data-theme', theme)
  }

  onMount(() => {
    readHash()
    window.addEventListener('hashchange', readHash)

    const stored = localStorage.getItem('ruptura:theme') ?? 'dark'
    theme = stored
    document.documentElement.setAttribute('data-theme', stored)

    return () => window.removeEventListener('hashchange', readHash)
  })
</script>

<div class="app">
  <NavBar {route} {theme} on:toggleTheme={toggleTheme} />
  <main>
    {#if route === 'fleet' || route === ''}
      <Fleet />
    {:else if route === 'map'}
      <Map />
    {:else if route === 'alerts'}
      <Alerts />
    {:else if route === 'engine'}
      <Engine />
    {:else if route === 'nodes'}
      <Nodes />
    {:else if route === 'settings'}
      <Settings />
    {:else}
      <div class="not-found">Page not found</div>
    {/if}
  </main>
</div>

<style>
  :global(*) {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
  }

  :global(body) {
    background: var(--bg);
    color: var(--text);
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    font-size: 14px;
    line-height: 1.5;
    transition: background 0.2s, color 0.2s;
  }

  :global(:root), :global([data-theme='dark']) {
    --bg:       #0a0d14;
    --surface:  #10151f;
    --surface2: #161c2a;
    --surface3: #1c2438;
    --border:   #1e2d45;
    --text:     #e2e8f0;
    --muted:    #556080;
    --purple:   #a855f7;
    --violet:   #7c3aed;
    --cyan:     #06b6d4;
    --green:    #00e5a0;
    --yellow:   #f59e0b;
    --orange:   #f97316;
    --red:      #ef4444;
    --blue:     #3b82f6;
    --accent:   #a855f7;
  }

  :global([data-theme='light']) {
    --bg:       #f0f4f8;
    --surface:  #ffffff;
    --surface2: #f8fafc;
    --surface3: #edf2f7;
    --border:   #cbd5e0;
    --text:     #1a202c;
    --muted:    #718096;
    --purple:   #7c3aed;
    --violet:   #6d28d9;
    --cyan:     #0891b2;
    --green:    #059669;
    --yellow:   #d97706;
    --orange:   #ea580c;
    --red:      #dc2626;
    --blue:     #2563eb;
    --accent:   #7c3aed;
  }

  :global(::-webkit-scrollbar) { width: 6px; }
  :global(::-webkit-scrollbar-track) { background: var(--bg); }
  :global(::-webkit-scrollbar-thumb) { background: var(--border); border-radius: 3px; }

  .app {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
  }

  main {
    flex: 1;
    padding: 20px 24px 64px;
    max-width: 1440px;
    margin: 0 auto;
    width: 100%;
  }

  .not-found {
    color: var(--muted);
    text-align: center;
    margin-top: 80px;
    font-size: 18px;
  }
</style>

<script lang="ts">
  import { onMount } from 'svelte'
  import NavBar from './components/NavBar.svelte'
  import Fleet from './routes/Fleet.svelte'
  import Map from './routes/Map.svelte'
  import Engine from './routes/Engine.svelte'
  import Nodes from './routes/Nodes.svelte'

  let route = ''

  function readHash() {
    route = window.location.hash.replace(/^#\/?/, '') || 'fleet'
  }

  onMount(() => {
    readHash()
    window.addEventListener('hashchange', readHash)
    return () => window.removeEventListener('hashchange', readHash)
  })
</script>

<div class="app">
  <NavBar {route} />
  <main>
    {#if route === 'fleet' || route === ''}
      <Fleet />
    {:else if route === 'map'}
      <Map />
    {:else if route === 'engine'}
      <Engine />
    {:else if route === 'nodes'}
      <Nodes />
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
    background: #0a0d14;
    color: #e2e8f0;
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    font-size: 14px;
    line-height: 1.5;
  }

  :global(:root) {
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

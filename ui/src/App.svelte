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
    background: #0d1117;
    color: #c9d1d9;
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    font-size: 14px;
    line-height: 1.5;
  }

  :global(:root) {
    --bg: #0d1117;
    --surface: #161b22;
    --surface2: #1c2230;
    --border: #30363d;
    --text: #c9d1d9;
    --muted: #6e7681;
    --cyan: #39d0d8;
    --green: #00e5a0;
    --yellow: #e8c848;
    --orange: #f5a623;
    --red: #e05252;
    --blue: #58a6ff;
    --purple: #bc8cff;
  }

  .app {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
  }

  main {
    flex: 1;
    padding: 24px;
    max-width: 1400px;
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

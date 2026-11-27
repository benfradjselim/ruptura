<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'

  let demoMode = false

  onMount(async () => {
    try {
      const status = await api.health()
      demoMode = !!status.demo_mode
    } catch {}
  })
</script>

{#if demoMode}
  <div class="demo-banner">
    Demo mode: synthetic failures, no cluster connected
  </div>
{/if}

<style>
  .demo-banner {
    background: #f59e0b;
    color: #1a1200;
    font-weight: 600;
    font-size: 0.85rem;
    text-align: center;
    padding: 0.5rem 1rem;
  }
</style>

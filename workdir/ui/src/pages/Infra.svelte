<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'
  import InfraGroupHeatmap from '../lib/components/InfraGroupHeatmap.svelte'
  import PropagationFlow   from '../lib/components/PropagationFlow.svelte'

  let groups      = []
  let propagation = {}
  let loading     = true
  let error       = null
  let interval

  async function load() {
    try {
      const [grpRes, propRes] = await Promise.all([
        api.infraGroups(),
        api.propagation(),
      ])
      groups      = grpRes.groups      || []
      propagation = propRes            || {}
      error       = null
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  onMount(() => {
    load()
    interval = setInterval(load, 30_000)
  })
  onDestroy(() => clearInterval(interval))
</script>

<div class="infra-page">
  <div class="page-header">
    <h1>Infrastructure</h1>
    <span class="subtitle">Infra group health and CGPM propagation pressure</span>
  </div>

  {#if loading}
    <div class="loading"><div class="spinner"></div><p>Loading infra state…</p></div>
  {:else if error}
    <div class="page-error">
      <p>⚠ {error}</p>
      <button on:click={load}>Retry</button>
    </div>
  {:else}
    <section class="panel">
      <h2 class="panel-title">Group Health</h2>
      <p class="panel-desc">One cell per active infra group. Color encodes worst-case signal across all namespaces.</p>
      <InfraGroupHeatmap {groups} />
    </section>

    <section class="panel">
      <h2 class="panel-title">Propagation Pressure</h2>
      <p class="panel-desc">CGPM: which infra groups are pushing pressure into each namespace's workloads.</p>
      <PropagationFlow {propagation} />
    </section>
  {/if}
</div>

<style>
  .infra-page {
    padding: 32px var(--margin, 24px);
    max-width: var(--maxw, 1280px);
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: 32px;
  }

  .page-header   { display: flex; flex-direction: column; gap: 4px; }
  h1             { font-size: 22px; font-weight: 800; color: var(--text); letter-spacing: -0.02em; margin: 0; }
  .subtitle      { font-size: 12px; color: var(--text-3); }

  .loading {
    display: flex; flex-direction: column; align-items: center; gap: 12px;
    padding: 64px 0; color: var(--text-3); font-size: 13px;
  }
  .spinner {
    width: 20px; height: 20px; border-radius: 50%;
    border: 2px solid var(--border-2);
    border-top-color: var(--accent);
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .page-error {
    padding: 24px; background: var(--red-dim); border: 1px solid var(--red);
    border-radius: 4px; color: var(--red); font-size: 12px;
    display: flex; align-items: center; gap: 16px;
  }
  .page-error button {
    background: transparent; border: 1px solid var(--red); color: var(--red);
    padding: 4px 10px; border-radius: 3px; cursor: pointer; font-size: 11px;
  }

  .panel {
    display: flex; flex-direction: column; gap: 12px;
  }
  .panel-title {
    font-size: 13px; font-weight: 700; color: var(--text); letter-spacing: -0.01em; margin: 0;
  }
  .panel-desc  { font-size: 11px; color: var(--text-3); margin: 0; }
</style>

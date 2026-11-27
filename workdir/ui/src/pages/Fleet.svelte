<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'
  import { cardSummary } from '../lib/util/failures.js'

  let fleet = null
  let error = null
  let loading = true
  let interval
  let expandedWorkload = null

  // Infra context, keyed by namespace.
  let infraByNs = {} // { [namespace]: [{ group, health, spread, objectCount }] }

  async function load() {
    try {
      const [fleetRes, infraRes] = await Promise.allSettled([
        api.fleet(),
        api.infraGroups(),
      ])
      if (fleetRes.status === 'fulfilled') fleet = fleetRes.value
      if (infraRes.status === 'fulfilled') {
        const ns = {}
        for (const g of (infraRes.value.groups || [])) {
          const key = g.namespace || ''
          if (!ns[key]) ns[key] = []
          ns[key].push(g)
        }
        infraByNs = ns
      }
      error = null
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  function parseHost(workload) {
    const parts = (workload.host || '').split('/')
    if (parts.length >= 3) return { ns: parts[0], kind: parts[1], name: parts.slice(2).join('/') }
    return { ns: '', kind: 'host', name: workload.host || '' }
  }

  function expand(workload) {
    expandedWorkload = expandedWorkload === workload.host ? null : workload.host
  }

  function stateClass(state) {
    if (!state) return 'unknown'
    const s = state.toLowerCase()
    if (s === 'healthy') return 'healthy'
    if (s === 'degraded') return 'degraded'
    if (s === 'critical') return 'critical'
    return 'unknown'
  }

  function workloadLabel(w) {
    const { ns, name } = parseHost(w)
    return ns ? `${ns}/${name}` : (w.host || 'unknown')
  }

  function infraGroups(w) {
    const ns = parseHost(w).ns || ''
    return [...(infraByNs[ns] || []), ...(infraByNs[''] || [])]
  }

  function infraHealthColor(h) {
    if (h >= 0.9) return 'var(--green)'
    if (h >= 0.6) return 'var(--amber)'
    return 'var(--red)'
  }

  const INFRA_GROUP_LABELS = {
    node: 'Node', network: 'Net', storage: 'Store',
    admission: 'Admission', tenancy: 'Tenancy',
    operators: 'Ops', co: 'CO', mcp: 'MCP',
  }

  onMount(() => {
    load()
    interval = setInterval(load, 15000)
  })
  onDestroy(() => clearInterval(interval))
</script>

<div class="fleet-page">
  <div class="fleet-header">
    <h1>Fleet</h1>
    {#if fleet}
      <span class="fleet-count">{fleet.hosts?.length ?? 0} workloads</span>
    {/if}
  </div>

  {#if loading}
    <div class="fleet-loading">
      <div class="spinner"></div>
      <p>Loading fleet…</p>
    </div>

  {:else if error}
    <div class="fleet-error">
      <p>⚠ Could not load fleet: {error}</p>
      <button on:click={load}>Retry</button>
    </div>

  {:else if !fleet?.hosts?.length}
    <!-- ITEM-017: Empty state -->
    <div class="empty-state">
      <h2>No workloads yet</h2>
      <p>
        Ruptura lists the Deployments, StatefulSets and DaemonSets it finds through the Kubernetes API,
        and the failures it reads from Prometheus.
      </p>
      <div class="empty-actions">
        <a href="https://docs.getruptura.dev/install/" target="_blank" rel="noopener">Installation guide →</a>
        <button on:click={load}>Check again</button>
      </div>
    </div>

  {:else}
    <div class="workload-grid">
      {#each fleet.hosts as w (w.host)}
        {@const key = w.host}
        {@const sc = stateClass(w.state)}
        {@const expanded = expandedWorkload === key}
        {@const summary = cardSummary(w)}

        <div class="workload-card {sc}" class:expanded>
          <!-- Card header -->
          <button class="card-header" on:click={() => expand(w)}>
            <div class="card-identity">
              <span class="state-dot {sc}"></span>
              <span class="workload-name" title={workloadLabel(w)}>{parseHost(w).name || workloadLabel(w)}</span>
              {#if w.state}
                <span class="state-badge {sc}">{w.state}</span>
              {/if}
            </div>
            <div class="card-scores">
              {#if summary.when}
                <div class="score-block">
                  <span class="score-label">{summary.title}</span>
                  <span class="score-value when {sc}">{summary.when}</span>
                </div>
              {/if}
              <span class="expand-icon">{expanded ? '▲' : '▼'}</span>
            </div>
          </button>

          <div class="facts">
            {#if summary.facts.length}
              {#each summary.facts as fact}<p>{fact}</p>{/each}
            {:else if !summary.when}
              <p class="muted-fact">{w.state === 'unknown' ? 'Not watched: no failure source' : 'No known failure'}</p>
            {/if}
          </div>

          <!-- Infra context bar — matched on namespace only (v8.1) -->
          {#if expanded}
            {@const ig = infraGroups(w)}
            {#if ig.length > 0}
              <div class="infra-bar">
                <span class="infra-bar-label">Infra</span>
                {#each ig as g}
                  <div class="infra-chip"
                    style="border-color:{infraHealthColor(g.health)}"
                    title="{INFRA_GROUP_LABELS[g.group] || g.group}: {Math.round(g.health * 100)}% healthy, {g.objectCount} objects">
                    <span class="infra-dot" style="background:{infraHealthColor(g.health)}"></span>
                    <span class="infra-chip-name">{INFRA_GROUP_LABELS[g.group] || g.group}</span>
                    <span class="infra-chip-val" style="color:{infraHealthColor(g.health)}">{Math.round(g.health * 100)}%</span>
                  </div>
                {/each}
              </div>
            {/if}
          {/if}

          {#if expanded && w.failure}
            <div class="card-detail">
              <p class="summary-action">{w.failure.recommendedAction}</p>
              {#if w.failure.affects?.length}
                <p class="summary-affects">Can hit: {w.failure.affects.join(', ')}</p>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
.fleet-page { padding: 1.5rem; max-width: 1200px; margin: 0 auto; }
.fleet-header { display: flex; align-items: baseline; gap: 1rem; margin-bottom: 1.5rem; }
.fleet-header h1 { margin: 0; font-size: 1.5rem; font-weight: 700; }
.fleet-count { color: var(--text-secondary, var(--text-2)); font-size: 0.875rem; }

/* Loading / error */
.fleet-loading, .fleet-error { display: flex; flex-direction: column; align-items: center; gap: 1rem; padding: 4rem; color: var(--text-secondary, var(--text-2)); }
.spinner { width: 24px; height: 24px; border: 2px solid currentColor; border-top-color: transparent; border-radius: 0; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* Empty state */
.empty-state { text-align: center; padding: 4rem 2rem; max-width: 480px; margin: 0 auto; }
.empty-state h2 { font-size: 1.25rem; font-weight: 600; margin: 0 0 0.75rem; }
.empty-state p { color: var(--text-secondary, var(--text-2)); margin: 0 0 0.5rem; line-height: 1.6; }
.empty-actions { display: flex; gap: 1rem; justify-content: center; margin-top: 1.5rem; flex-wrap: wrap; }
.empty-actions a, .empty-actions button { padding: 0.5rem 1rem; border-radius: 0; font-size: 0.875rem; cursor: pointer; text-decoration: none; }
.empty-actions a { background: var(--accent-cyan, #06b6d4); color: #000; font-weight: 600; }
.empty-actions button { background: var(--surface, #1e2535); color: var(--text-primary, #f1f5f9); border: 1px solid var(--border, rgba(255,255,255,0.06)); }

/* Grid */
.workload-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(400px, 1fr)); gap: 1rem; }
.facts { padding: 0 1rem 0.9rem; font-family: var(--font-mono); font-size: 13px; color: var(--text); }
.facts .muted-fact { color: var(--text-3); }
.score-value.when { font-family: var(--font-mono); text-transform: none; }
.workload-card { clip-path: polygon(0 0, 100% 0, 100% calc(100% - 14px), calc(100% - 14px) 100%, 0 100%); }

/* Card */
.workload-card { background: var(--surface, #141824); border: 1px solid var(--border, rgba(255,255,255,0.06)); border-radius: 0; overflow: hidden; transition: border-color 0.15s; }
.workload-card.healthy { border-left: 3px solid var(--green); }
.workload-card.degraded { border-left: 3px solid var(--amber); }
.workload-card.critical { border-left: 3px solid var(--red); }
.workload-card.expanded { grid-column: 1 / -1; }

.card-header { display: flex; align-items: center; justify-content: space-between; width: 100%; padding: 0.875rem 1rem; background: none; border: none; cursor: pointer; text-align: left; color: inherit; gap: 0.5rem; }
.card-identity { display: flex; align-items: center; gap: 0.5rem; min-width: 0; flex: 1; }
.state-dot { width: 8px; height: 8px; border-radius: 0; flex-shrink: 0; }
.state-dot.healthy { background: var(--green); }
.state-dot.degraded { background: var(--amber); }
.state-dot.critical { background: var(--red); animation: pulse-red 1.5s ease-in-out infinite; }
@keyframes pulse-red { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }
.workload-name { font-weight: 600; font-size: 0.875rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.state-badge { font-size: 0.7rem; padding: 0.15em 0.5em; border-radius: 0; font-weight: 500; }
.state-badge.healthy { background: rgba(34,197,94,0.15); color: var(--green); }
.state-badge.degraded { background: rgba(245,158,11,0.15); color: var(--amber); }
.state-badge.critical { background: rgba(239,68,68,0.15); color: var(--red); }

.card-scores { display: flex; align-items: center; gap: 0.75rem; flex-shrink: 0; }
.score-block { display: flex; flex-direction: column; align-items: flex-end; }
.score-label { font-size: 0.65rem; color: var(--text-secondary, var(--text-2)); text-transform: uppercase; letter-spacing: 0.05em; }
.score-value { font-size: 1rem; font-weight: 700; font-variant-numeric: tabular-nums; }
.score-value.healthy { color: var(--green); }
.score-value.degraded { color: var(--amber); }
.score-value.critical { color: var(--red); }
.expand-icon { color: var(--text-secondary, var(--text-2)); font-size: 0.7rem; }


/* Expanded detail */
.card-detail { border-top: 1px solid var(--border, rgba(255,255,255,0.06)); padding: 1rem; }

.summary-action, .summary-affects { margin: 0.35rem 0 0; font-size: 0.8rem; color: var(--text-secondary, var(--text-2)); }




/* Infra context bar */
.infra-bar {
  display: flex; align-items: center; flex-wrap: wrap; gap: 6px;
  padding: 8px 12px;
  border-top: 1px solid var(--border);
  background: var(--surface-3, #1C2540);
}
.infra-bar-label {
  font-size: 9px; font-weight: 700; text-transform: uppercase;
  letter-spacing: 0.12em; color: var(--text-3); margin-right: 4px;
}
.infra-chip {
  display: flex; align-items: center; gap: 4px;
  padding: 2px 7px; border-radius: 0; border: 1px solid;
  background: var(--surface-2);
}
.infra-dot     { width: 5px; height: 5px; border-radius: 0; flex-shrink: 0; }
.infra-chip-name { font-size: 9px; color: var(--text-2); }
.infra-chip-val  { font-family: var(--font-mono); font-size: 9px; font-variant-numeric: tabular-nums; }
</style>

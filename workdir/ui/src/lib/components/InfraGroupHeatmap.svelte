<script>
  // groups: array of { group, namespace, health, spread, gni, agitated, objectCount }
  // health is 0–1 where 1=fully healthy, 0=critical
  export let groups = []

  const GROUP_LABELS = {
    node:        'Nodes',
    network:     'Network',
    storage:     'Storage',
    admission:   'Admission',
    tenancy:     'Tenancy',
    operators:   'Operators',
    co:          'Cluster Ops',
    mcp:         'MachineConfig',
  }

  const GROUP_ORDER = ['node', 'network', 'storage', 'tenancy', 'admission', 'operators', 'co', 'mcp']

  function healthColor(h) {
    if (h >= 0.9) return 'var(--green)'
    if (h >= 0.6) return 'var(--amber)'
    return 'var(--red)'
  }

  function healthClass(h) {
    if (h >= 0.9) return 'stable'
    if (h >= 0.6) return 'elevated'
    return 'critical'
  }

  function healthLabel(h) {
    if (h >= 0.9) return 'Stable'
    if (h >= 0.6) return 'Elevated'
    if (h >= 0.3) return 'Warning'
    return 'Critical'
  }

  // Aggregate groups: one row per group name, worst health across namespaces
  $: byGroup = (() => {
    const m = new Map()
    for (const g of groups) {
      const key = g.group
      const prev = m.get(key)
      if (!prev || g.health < prev.health) {
        m.set(key, { ...g, namespaces: [] })
      }
      const entry = m.get(key)
      if (g.namespace) entry.namespaces.push(g.namespace)
    }
    return GROUP_ORDER
      .filter(k => m.has(k))
      .map(k => m.get(k))
      .concat([...m.values()].filter(v => !GROUP_ORDER.includes(v.group)))
  })()
</script>

<div class="heatmap">
  {#if byGroup.length === 0}
    <div class="empty">No infra collectors active</div>
  {:else}
    <div class="grid">
      {#each byGroup as g (g.group)}
        <div class="cell {healthClass(g.health)}" class:agitated={g.agitated}
          title="{GROUP_LABELS[g.group] || g.group} · health {Math.round(g.health * 100)}% · {g.objectCount} objects{g.namespaces?.length ? '\nNamespaces: ' + g.namespaces.join(', ') : ''}">
          <div class="cell-indicator" style="background:{healthColor(g.health)}"></div>
          <div class="cell-body">
            <span class="cell-name">{GROUP_LABELS[g.group] || g.group}</span>
            <span class="cell-health" style="color:{healthColor(g.health)}">{Math.round(g.health * 100)}%</span>
          </div>
          <div class="cell-meta">
            <span class="cell-label {healthClass(g.health)}">{healthLabel(g.health)}</span>
            <span class="cell-count">{g.objectCount} obj</span>
          </div>
          {#if g.agitated}
            <div class="agitated-dot" title="Agitated"></div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .heatmap { width: 100%; }
  .empty   { color: var(--text-3); font-size: 12px; padding: 24px 0; text-align: center; }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 8px;
  }

  .cell {
    position: relative;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 10px 12px 8px;
    overflow: hidden;
    transition: border-color 0.15s;
  }
  .cell:hover { border-color: var(--border-3); }

  .cell.stable   { border-left: 2px solid var(--green); }
  .cell.elevated { border-left: 2px solid var(--amber); }
  .cell.critical { border-left: 2px solid var(--red);   }

  .cell.agitated { animation: pulse 2s ease-in-out infinite; }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50%       { opacity: 0.75; }
  }

  .cell-indicator {
    position: absolute;
    top: 0; left: 0; right: 0;
    height: 2px;
    opacity: 0.4;
  }

  .cell-body {
    display: flex; justify-content: space-between; align-items: baseline;
    margin-bottom: 4px;
  }
  .cell-name   { font-size: 11px; font-weight: 600; color: var(--text); }
  .cell-health { font-family: var(--font-mono); font-size: 13px; font-weight: 500; font-variant-numeric: tabular-nums; }

  .cell-meta {
    display: flex; justify-content: space-between; align-items: center;
  }
  .cell-label {
    font-size: 9px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.1em;
  }
  .cell-label.stable   { color: var(--green); }
  .cell-label.elevated { color: var(--amber); }
  .cell-label.critical { color: var(--red);   }

  .cell-count  { font-size: 9px; color: var(--text-3); font-family: var(--font-mono); }

  .agitated-dot {
    position: absolute; top: 6px; right: 6px;
    width: 5px; height: 5px; border-radius: 50%;
    background: var(--amber);
    animation: blink 1s step-end infinite;
  }
  @keyframes blink {
    0%, 100% { opacity: 1; }
    50%       { opacity: 0;   }
  }
</style>

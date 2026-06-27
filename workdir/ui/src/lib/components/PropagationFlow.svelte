<script>
  // propagation: { namespaces: { [ns]: { workloadPressure, propPressure: { [group]: float } } } }
  export let propagation = {}

  const GROUP_LABELS = {
    node:        'Node',
    network:     'Network',
    storage:     'Storage',
    admission:   'Admission',
    tenancy:     'Tenancy',
    operators:   'Operators',
    co:          'ClusterOps',
    mcp:         'MachineConfig',
  }

  function pressureColor(v) {
    if (v <= 0.1) return 'var(--green)'
    if (v <= 0.4) return 'var(--amber)'
    return 'var(--red)'
  }

  function pressureClass(v) {
    if (v <= 0.1) return 'low'
    if (v <= 0.4) return 'medium'
    return 'high'
  }

  $: namespaces = Object.entries(propagation.namespaces || {})
    .map(([ns, data]) => ({
      ns,
      workloadPressure: data.workloadPressure ?? 0,
      sources: Object.entries(data.propPressure || {})
        .filter(([, v]) => v > 0)
        .sort((a, b) => b[1] - a[1]),
    }))
    .sort((a, b) => b.workloadPressure - a.workloadPressure)
</script>

<div class="flow">
  {#if namespaces.length === 0}
    <div class="empty">No propagation data — infra collectors may be warming up</div>
  {:else}
    {#each namespaces as item (item.ns)}
      <div class="ns-row">
        <div class="ns-header">
          <span class="ns-name">{item.ns || '(cluster)'}</span>
          <span class="ns-pressure {pressureClass(item.workloadPressure)}"
            title="Workload pressure folded in by CGPM">
            {(item.workloadPressure * 100).toFixed(0)}%
          </span>
        </div>

        {#if item.sources.length > 0}
          <div class="sources">
            {#each item.sources as [group, val]}
              <div class="source-row" title="{GROUP_LABELS[group] || group} propagating {(val*100).toFixed(0)}% pressure">
                <span class="source-label">{GROUP_LABELS[group] || group}</span>
                <div class="bar-track">
                  <div class="bar-fill" style="width:{Math.min(val*100,100)}%;background:{pressureColor(val)}"></div>
                </div>
                <span class="source-val {pressureClass(val)}">{(val * 100).toFixed(0)}%</span>
              </div>
            {/each}
          </div>
        {:else}
          <div class="no-sources">No active pressure sources</div>
        {/if}
      </div>
    {/each}
  {/if}
</div>

<style>
  .flow  { width: 100%; display: flex; flex-direction: column; gap: 12px; }
  .empty { color: var(--text-3); font-size: 12px; padding: 24px 0; text-align: center; }

  .ns-row {
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 10px 12px;
  }

  .ns-header {
    display: flex; justify-content: space-between; align-items: center;
    margin-bottom: 8px;
  }
  .ns-name     { font-size: 12px; font-weight: 600; color: var(--text); font-family: var(--font-mono); }
  .ns-pressure {
    font-family: var(--font-mono); font-size: 12px; font-weight: 500;
    font-variant-numeric: tabular-nums;
    padding: 1px 6px; border-radius: 3px;
  }
  .ns-pressure.low    { color: var(--green); background: var(--green-dim); }
  .ns-pressure.medium { color: var(--amber); background: var(--amber-dim); }
  .ns-pressure.high   { color: var(--red);   background: var(--red-dim);   }

  .sources       { display: flex; flex-direction: column; gap: 5px; }
  .no-sources    { font-size: 11px; color: var(--text-3); }

  .source-row {
    display: grid; grid-template-columns: 80px 1fr 36px; gap: 8px; align-items: center;
  }
  .source-label { font-size: 10px; color: var(--text-2); white-space: nowrap; }

  .bar-track {
    height: 4px; background: var(--border-2); border-radius: 2px; overflow: hidden;
  }
  .bar-fill  { height: 100%; border-radius: 2px; transition: width 0.3s; }

  .source-val {
    font-family: var(--font-mono); font-size: 10px; text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .source-val.low    { color: var(--green); }
  .source-val.medium { color: var(--amber); }
  .source-val.high   { color: var(--red);   }
</style>

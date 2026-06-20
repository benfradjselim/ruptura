<script>
  import { refreshInterval, INTERVALS, manualRefresh } from '../stores/refresh.js'
</script>

<div class="rp">
  <button class="refresh-btn" title="Refresh now" on:click={manualRefresh}>↺</button>
  <select
    class="interval-select"
    bind:value={$refreshInterval}
  >
    {#each INTERVALS as iv}
      <option value={iv.value}>{iv.label}</option>
    {/each}
  </select>
</div>

<style>
  .rp { display: flex; align-items: center; gap: 4px; }

  .refresh-btn {
    background: var(--surface); border: 1px solid var(--border-2);
    color: var(--text-2); padding: 6px 10px; border-radius: 6px;
    cursor: pointer; font-size: 1rem; line-height: 1;
  }
  .refresh-btn:hover { color: var(--accent); border-color: var(--accent); }

  .interval-select {
    background: var(--surface); border: 1px solid var(--border-2);
    color: var(--text-2); padding: 6px 8px; border-radius: 6px;
    font-size: 0.82rem; cursor: pointer;
  }
  .interval-select:hover { border-color: var(--accent); color: var(--text); }
</style>

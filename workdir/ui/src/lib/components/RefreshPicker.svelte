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
    background: #1e293b; border: 1px solid #334155;
    color: #94a3b8; padding: 6px 10px; border-radius: 6px;
    cursor: pointer; font-size: 1rem; line-height: 1;
  }
  .refresh-btn:hover { color: #38bdf8; border-color: #38bdf8; }

  .interval-select {
    background: #1e293b; border: 1px solid #334155;
    color: #94a3b8; padding: 6px 8px; border-radius: 6px;
    font-size: 0.82rem; cursor: pointer;
  }
  .interval-select:hover { border-color: #38bdf8; color: #e2e8f0; }
</style>

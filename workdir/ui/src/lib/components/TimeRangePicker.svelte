<script>
  import { timeRange, PRESETS } from '../stores/timeRange.js'

  let open = false
  let customFrom = ''
  let customTo   = ''

  function selectPreset(minutes) {
    timeRange.set({ preset: minutes, from: null, to: null })
    open = false
  }

  function applyCustom() {
    const from = customFrom ? new Date(customFrom) : null
    const to   = customTo   ? new Date(customTo)   : null
    if (!from || !to || from >= to) return
    timeRange.set({ preset: null, from, to })
    open = false
  }

  $: label = $timeRange.preset
    ? (PRESETS.find(p => p.value === $timeRange.preset)?.label ?? 'Custom')
    : 'Custom'
</script>

<div class="trp" class:open>
  <button class="trp-btn" on:click={() => open = !open}>
    <span class="icon">⏱</span>
    <span>{label}</span>
    <span class="caret">▾</span>
  </button>

  {#if open}
    <div class="trp-dropdown">
      <div class="presets">
        {#each PRESETS as p}
          <button
            class="preset-btn"
            class:active={$timeRange.preset === p.value}
            on:click={() => selectPreset(p.value)}
          >{p.label}</button>
        {/each}
      </div>
      <div class="divider"></div>
      <div class="custom">
        <label>From
          <input type="datetime-local" bind:value={customFrom} />
        </label>
        <label>To
          <input type="datetime-local" bind:value={customTo} />
        </label>
        <button class="apply-btn" on:click={applyCustom}>Apply</button>
      </div>
    </div>
  {/if}
</div>

<svelte:window on:click={(e) => { if (!e.target.closest('.trp')) open = false }} />

<style>
  .trp { position: relative; display: inline-block; }

  .trp-btn {
    display: flex; align-items: center; gap: 6px;
    background: #1e293b; border: 1px solid #334155;
    color: #94a3b8; padding: 6px 12px; border-radius: 6px;
    cursor: pointer; font-size: 0.85rem; white-space: nowrap;
  }
  .trp-btn:hover { border-color: #38bdf8; color: #e2e8f0; }
  .icon { font-size: 0.9rem; }
  .caret { font-size: 0.7rem; margin-left: 2px; }

  .trp-dropdown {
    position: absolute; top: calc(100% + 6px); right: 0; z-index: 100;
    background: #1e293b; border: 1px solid #334155; border-radius: 8px;
    padding: 8px; min-width: 220px; box-shadow: 0 8px 24px rgba(0,0,0,0.4);
  }

  .presets {
    display: grid; grid-template-columns: 1fr 1fr; gap: 4px;
  }
  .preset-btn {
    background: transparent; border: 1px solid #334155;
    color: #94a3b8; padding: 5px 8px; border-radius: 4px;
    cursor: pointer; font-size: 0.8rem; text-align: center;
  }
  .preset-btn:hover { background: #0f3460; color: #e2e8f0; }
  .preset-btn.active { background: #0f3460; border-color: #38bdf8; color: #38bdf8; }

  .divider { height: 1px; background: #334155; margin: 8px 0; }

  .custom { display: flex; flex-direction: column; gap: 6px; }
  .custom label { display: flex; flex-direction: column; gap: 2px; font-size: 0.75rem; color: #64748b; }
  .custom input {
    background: #0f172a; border: 1px solid #334155; border-radius: 4px;
    color: #e2e8f0; padding: 4px 8px; font-size: 0.8rem;
  }
  .apply-btn {
    background: #0284c7; border: none; color: #fff;
    padding: 6px; border-radius: 4px; cursor: pointer; font-size: 0.8rem;
  }
  .apply-btn:hover { background: #0369a1; }
</style>

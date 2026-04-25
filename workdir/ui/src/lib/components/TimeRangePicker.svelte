<script>
  import { timeRange, PRESETS, FUTURE_PRESETS } from '../stores/timeRange.js'

  export let futureMode = false

  let open = false
  let customFrom = ''
  let customTo   = ''

  $: presets = futureMode ? FUTURE_PRESETS : PRESETS

  function selectPreset(minutes) {
    timeRange.set({ preset: minutes, from: null, to: null, future: futureMode })
    open = false
  }

  function applyCustom() {
    if (futureMode) return
    const from = customFrom ? new Date(customFrom) : null
    const to   = customTo   ? new Date(customTo)   : null
    if (!from || !to || from >= to) return
    timeRange.set({ preset: null, from, to, future: false })
    open = false
  }

  // Sync future flag when futureMode prop changes
  $: if (futureMode !== $timeRange.future) {
    const defaultPreset = futureMode
      ? (FUTURE_PRESETS.find(p => p.value === $timeRange.preset) ? $timeRange.preset : 60)
      : (PRESETS.find(p => p.value === $timeRange.preset) ? $timeRange.preset : 60)
    timeRange.set({ preset: defaultPreset, from: null, to: null, future: futureMode })
  }

  $: activeValue = $timeRange.preset
  $: label = activeValue
    ? (presets.find(p => p.value === activeValue)?.label ?? 'Custom')
    : 'Custom'
</script>

<div class="trp" class:open>
  <button class="trp-btn" class:future={futureMode} on:click={() => open = !open}>
    <svg class="trp-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      {#if futureMode}
        <!-- trending-up arrow = forecast -->
        <path d="M23 6l-9.5 9.5-5-5L1 18M17 6h6v6"/>
      {:else}
        <!-- clock = past history -->
        <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
      {/if}
    </svg>
    <span>{label}</span>
    <svg class="caret" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
      <polyline points="6 9 12 15 18 9"/>
    </svg>
  </button>

  {#if open}
    <div class="trp-dropdown">
      {#if futureMode}
        <div class="future-label">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" class="fl-icon">
            <path d="M23 6l-9.5 9.5-5-5L1 18M17 6h6v6"/>
          </svg>
          Forecast horizon
        </div>
      {/if}
      <div class="presets">
        {#each presets as p}
          <button
            class="preset-btn"
            class:active={activeValue === p.value}
            on:click={() => selectPreset(p.value)}
          >{p.label}</button>
        {/each}
      </div>
      {#if !futureMode}
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
      {/if}
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
    transition: border-color 0.15s, color 0.15s;
  }
  .trp-btn:hover { border-color: #38bdf8; color: #e2e8f0; }
  .trp-btn.future { border-color: #ec4899aa; color: #f472b6; }
  .trp-btn.future:hover { border-color: #ec4899; }

  .trp-icon { width: 14px; height: 14px; flex-shrink: 0; }
  .caret    { width: 12px; height: 12px; flex-shrink: 0; margin-left: 2px; }

  .trp-dropdown {
    position: absolute; top: calc(100% + 6px); right: 0; z-index: 100;
    background: #1e293b; border: 1px solid #334155; border-radius: 8px;
    padding: 8px; min-width: 220px; box-shadow: 0 8px 24px rgba(0,0,0,0.4);
  }

  .future-label {
    display: flex; align-items: center; gap: 5px;
    font-size: 0.67rem; font-weight: 700; text-transform: uppercase;
    letter-spacing: 0.08em; color: #ec4899; padding: 2px 2px 6px;
  }
  .fl-icon { width: 12px; height: 12px; }

  .presets { display: grid; grid-template-columns: 1fr 1fr; gap: 4px; }
  .preset-btn {
    background: transparent; border: 1px solid #334155;
    color: #94a3b8; padding: 5px 8px; border-radius: 4px;
    cursor: pointer; font-size: 0.8rem; text-align: center;
    transition: all 0.1s;
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

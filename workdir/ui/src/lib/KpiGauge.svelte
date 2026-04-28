<script>
  import { kpiColor, kpiLabel } from './store.js'

  export let name = ''
  export let value = 0
  export let state = ''

  $: color = kpiColor(name, value)
  $: label = state || kpiLabel(name, value)
  $: pct = Math.round(value * 100)

  // SVG arc helpers
  const R = 40, CX = 50, CY = 54
  function arc(v) {
    const a = Math.PI * (1 + v)  // 180° to 360°
    return `${CX + R * Math.cos(a)},${CY + R * Math.sin(a)}`
  }
  $: arcPath = `M ${arc(0)} A ${R} ${R} 0 ${value > 0.5 ? 1 : 0} 1 ${arc(value)}`
</script>

<div class="gauge">
  <svg viewBox="0 0 100 60" role="img" aria-label="{name}: {pct}%">
    <!-- Track -->
    <path d="M {arc(0)} A {R} {R} 0 1 1 {arc(1)}" fill="none" stroke="#334155" stroke-width="8" stroke-linecap="round"/>
    <!-- Value arc -->
    {#if value > 0}
      <path d={arcPath} fill="none" stroke={color} stroke-width="8" stroke-linecap="round"/>
    {/if}
    <!-- Center text -->
    <text x="50" y="50" text-anchor="middle" font-size="14" font-weight="700" fill={color}>{pct}%</text>
  </svg>
  <div class="name">{name}</div>
  <div class="label" style="color:{color}">{label}</div>
</div>

<style>
  .gauge {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    min-width: 90px;
  }
  svg { width: 90px; height: 60px; }
  .name {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: #94a3b8;
  }
  .label {
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: capitalize;
  }
</style>

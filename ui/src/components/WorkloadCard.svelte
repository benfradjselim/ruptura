<script lang="ts">
  import type { FleetHost } from '../lib/api'

  export let host: FleetHost
  export let selected = false

  $: showRuptureWarning =
    host.state !== 'pending_telemetry' &&
    host.fused_rupture_index > 1.5 &&
    host.health_score > 60

  $: forecastEta = host.health_forecast?.critical_eta_minutes ?? 0
  $: forecastProjected = host.health_forecast?.in_15min ?? 0
  $: forecastLowConfidence = (host.health_forecast?.confidence_window ?? 0) < 60

  function hsColor(v: number): string {
    if (v >= 70) return 'var(--green)'
    if (v >= 40) return 'var(--yellow)'
    return 'var(--red)'
  }

  function stateLabel(s: string): string {
    switch (s) {
      case 'healthy': return 'OK'
      case 'degraded': return 'WARN'
      case 'critical': return 'CRIT'
      case 'pending_telemetry': return 'PENDING'
      default: return s.toUpperCase()
    }
  }

  function stateColor(s: string): string {
    switch (s) {
      case 'healthy': return 'var(--green)'
      case 'degraded': return 'var(--yellow)'
      case 'critical': return 'var(--red)'
      case 'pending_telemetry': return 'var(--blue)'
      default: return 'var(--muted)'
    }
  }

  // Parse "namespace/Kind/name" → display name
  function displayName(h: string): string {
    const parts = h.split('/')
    return parts[parts.length - 1] || h
  }

  function displayMeta(h: string): string {
    const parts = h.split('/')
    if (parts.length === 3) return `${parts[0]} · ${parts[1]}`
    return ''
  }

  function relativeTime(ts: string): string {
    if (!ts) return ''
    const delta = Date.now() - new Date(ts).getTime()
    if (delta < 60_000) return `${Math.round(delta / 1000)}s ago`
    if (delta < 3_600_000) return `${Math.round(delta / 60_000)}m ago`
    return `${Math.round(delta / 3_600_000)}h ago`
  }
</script>

<button
  class="card"
  class:selected
  class:pending={host.state === 'pending_telemetry'}
  on:click
>
  <div class="top">
    <span class="badge" style="color: {stateColor(host.state)}">{stateLabel(host.state)}</span>
    <span class="last-seen">{relativeTime(host.last_seen)}</span>
  </div>

  <div class="name">{displayName(host.host)}</div>
  {#if displayMeta(host.host)}
    <div class="meta">{displayMeta(host.host)}</div>
  {/if}

  {#if host.state !== 'pending_telemetry'}
    <div class="score" style="color: {hsColor(host.health_score)}">
      {Math.round(host.health_score)}
    </div>
    <div class="bars">
      <div class="bar-row">
        <span>Stress</span>
        <div class="bar"><div class="fill" style="width:{host.stress}%;background:var(--orange)"></div></div>
      </div>
      <div class="bar-row">
        <span>Fatigue</span>
        <div class="bar"><div class="fill" style="width:{host.fatigue}%;background:var(--purple)"></div></div>
      </div>
    </div>
    {#if showRuptureWarning}
      <div class="rupture-warn">
        <span class="rupture-warn-title">Early rupture signal · FusedR {host.fused_rupture_index.toFixed(1)}</span>
        <span class="rupture-warn-body">
          HealthScore is still {Math.round(host.health_score)} because KPI signals are lagging indicators.
          {#if forecastEta > 0}
            Expect HealthScore ≈ {Math.round(forecastProjected)} in {forecastEta}m.
            {#if forecastLowConfidence}<span class="low-conf">(low confidence)</span>{/if}
          {/if}
        </span>
      </div>
    {/if}
  {:else}
    <div class="pending-msg">Waiting for first telemetry…</div>
  {/if}
</button>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 4px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 14px 16px;
    cursor: pointer;
    text-align: left;
    color: var(--text);
    transition: border-color 0.15s, background 0.15s;
    width: 100%;
  }

  .card:hover {
    border-color: var(--cyan);
    background: var(--surface2);
  }

  .card.selected {
    border-color: var(--cyan);
    background: rgba(57, 208, 216, 0.06);
  }

  .card.pending {
    opacity: 0.7;
    border-style: dashed;
  }

  .top {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .badge {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.06em;
  }

  .last-seen {
    font-size: 10px;
    color: var(--muted);
  }

  .name {
    font-weight: 600;
    font-size: 14px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta {
    font-size: 11px;
    color: var(--muted);
  }

  .score {
    font-size: 28px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    line-height: 1;
    margin: 4px 0 2px;
  }

  .bars {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 6px;
  }

  .bar-row {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 10px;
    color: var(--muted);
  }

  .bar-row span {
    width: 38px;
    flex-shrink: 0;
  }

  .bar {
    flex: 1;
    height: 3px;
    background: var(--surface2);
    border-radius: 2px;
    overflow: hidden;
  }

  .fill {
    height: 100%;
    border-radius: 2px;
    max-width: 100%;
  }

  .pending-msg {
    font-size: 11px;
    color: var(--blue);
    margin-top: 4px;
    font-style: italic;
  }

  .rupture-warn {
    display: flex;
    flex-direction: column;
    gap: 3px;
    margin-top: 8px;
    padding: 7px 10px;
    background: rgba(255, 85, 85, 0.08);
    border: 1px solid rgba(255, 85, 85, 0.3);
    border-radius: 6px;
  }

  .rupture-warn-title {
    font-size: 10px;
    font-weight: 700;
    color: var(--red);
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .rupture-warn-body {
    font-size: 10px;
    color: var(--muted);
    line-height: 1.5;
  }

  .low-conf {
    color: var(--yellow);
    font-style: italic;
  }
</style>

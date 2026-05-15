<script lang="ts">
  import type { FleetHost, RuptureSnapshot } from '../lib/api'

  export let host: FleetHost
  export let snap: RuptureSnapshot | null = null
  export let selected = false

  const SIG_COLORS: Record<string, string> = {
    stress:     '#ef4444',
    fatigue:    '#f97316',
    mood:       '#06b6d4',
    pressure:   '#f59e0b',
    humidity:   '#3b82f6',
    contagion:  '#ec4899',
    resilience: '#00e5a0',
    entropy:    '#a855f7',
    velocity:   '#fb7185',
    throughput: '#818cf8',
  }

  const SIG_ORDER = ['stress','fatigue','mood','pressure','humidity','contagion','resilience','entropy','velocity','throughput']

  $: isRupture = host.state !== 'pending_telemetry' && host.state !== 'calibrating'
    && host.fused_rupture_index > 1.5 && host.health_score > 60

  $: forecastEta      = host.health_forecast?.critical_eta_minutes ?? 0
  $: forecastIn15     = host.health_forecast?.in_15min ?? 0
  $: forecastLowConf  = (host.health_forecast?.confidence_window ?? 0) < 60

  function hsColor(v: number): string {
    if (v >= 70) return 'var(--green)'
    if (v >= 40) return 'var(--yellow)'
    return 'var(--red)'
  }

  function fuseColor(v: number): string {
    if (v < 1.0) return 'var(--green)'
    if (v < 1.5) return 'var(--yellow)'
    if (v < 2.5) return 'var(--orange)'
    return 'var(--red)'
  }

  function stateLabel(s: string): string {
    switch (s) {
      case 'healthy':          return 'OK'
      case 'degraded':         return 'WARN'
      case 'critical':         return 'CRIT'
      case 'calibrating':      return 'CAL'
      case 'pending_telemetry': return 'PENDING'
      default:                 return s.toUpperCase()
    }
  }

  function stateColor(s: string): string {
    switch (s) {
      case 'healthy':          return 'var(--green)'
      case 'degraded':         return 'var(--yellow)'
      case 'critical':         return 'var(--red)'
      case 'calibrating':      return 'var(--purple)'
      case 'pending_telemetry': return 'var(--muted)'
      default:                 return 'var(--muted)'
    }
  }

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

  // ring arc path for HealthScore
  function arc(score: number): string {
    const r = 16
    const cx = 20
    const cy = 20
    const pct = Math.min(Math.max(score, 0), 100) / 100
    const angle = pct * 2 * Math.PI - Math.PI / 2
    const x = cx + r * Math.cos(angle)
    const y = cy + r * Math.sin(angle)
    const large = pct > 0.5 ? 1 : 0
    if (pct >= 1) return `M ${cx} ${cy - r} A ${r} ${r} 0 1 1 ${cx - 0.001} ${cy - r}`
    return `M ${cx} ${cy - r} A ${r} ${r} 0 ${large} 1 ${x} ${y}`
  }

  function sigVal(sig: string): number {
    if (snap) {
      const kpi = (snap as unknown as Record<string, { value: number } | undefined>)[sig]
      if (kpi?.value !== undefined) return kpi.value
    }
    return ((host as unknown as Record<string, number>)[sig]) ?? 0
  }
</script>

<button
  class="card"
  class:selected
  class:pending={host.state === 'pending_telemetry'}
  class:calibrating={host.state === 'calibrating'}
  on:click
>
  <!-- top row: state badge + timestamp -->
  <div class="top">
    <span class="badge" style="color:{stateColor(host.state)}">{stateLabel(host.state)}</span>
    <span class="ts">{relativeTime(host.last_seen)}</span>
  </div>

  <!-- workload name + namespace/kind -->
  <div class="name">{displayName(host.host)}</div>
  {#if displayMeta(host.host)}
    <div class="meta">{displayMeta(host.host)}</div>
  {/if}

  {#if host.state === 'pending_telemetry'}
    <div class="pending-msg">Waiting for first OTLP telemetry…</div>

  {:else if host.state === 'calibrating'}
    <div class="calib-row">
      <span class="calib-icon">⊙</span>
      <span class="calib-txt">Building baseline…</span>
    </div>
    {#if (host.calibration_progress ?? 0) > 0}
      <div class="calib-progress-wrap">
        <div class="calib-progress-fill" style="width:{host.calibration_progress ?? 0}%"></div>
      </div>
      <div class="calib-pct">{host.calibration_progress ?? 0}%</div>
    {/if}

  {:else}
    <!-- health ring + score + FusedR -->
    <div class="metrics-row">
      <svg class="ring" viewBox="0 0 40 40">
        <!-- track -->
        <circle cx="20" cy="20" r="16" fill="none" stroke="var(--surface3)" stroke-width="3"/>
        <!-- arc -->
        <path d={arc(host.health_score)} fill="none" stroke={hsColor(host.health_score)} stroke-width="3" stroke-linecap="round"/>
      </svg>
      <div class="score-block">
        <span class="score" style="color:{hsColor(host.health_score)}">{Math.round(host.health_score)}</span>
        <span class="score-label">Health</span>
      </div>
      <div class="fused-block">
        <span class="fused-val" style="color:{fuseColor(host.fused_rupture_index)}">{host.fused_rupture_index.toFixed(2)}</span>
        <span class="fused-label">FusedR</span>
      </div>
    </div>

    <!-- 9 signal mini-bars -->
    <div class="sigs">
      {#each SIG_ORDER as sig}
        <div class="sig-bar" title="{sig}: {sigVal(sig).toFixed(1)}">
          <div class="sig-fill" style="height:{Math.min(sigVal(sig),100)}%;background:{SIG_COLORS[sig]}"></div>
        </div>
      {/each}
    </div>
    <div class="sig-labels">
      {#each SIG_ORDER as sig}
        <span>{sig.slice(0,3)}</span>
      {/each}
    </div>

    <!-- early rupture warning -->
    {#if isRupture}
      <div class="rupture-warn">
        <span class="rw-title">◉ Early rupture · FusedR {host.fused_rupture_index.toFixed(1)}</span>
        {#if forecastEta > 0}
          <span class="rw-body">→ HealthScore ≈ {Math.round(forecastIn15)} in 15m
            {#if forecastLowConf}<em>(low conf.)</em>{/if}
          </span>
        {/if}
      </div>
    {/if}
  {/if}
</button>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 5px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 14px 14px 10px;
    cursor: pointer;
    text-align: left;
    color: var(--text);
    transition: border-color 0.15s, background 0.15s, box-shadow 0.15s;
    width: 100%;
  }

  .card:hover {
    border-color: rgba(168, 85, 247, 0.5);
    background: var(--surface2);
    box-shadow: 0 0 0 1px rgba(168,85,247,0.1);
  }

  .card.selected {
    border-color: var(--purple);
    background: rgba(168, 85, 247, 0.06);
    box-shadow: 0 0 0 1px rgba(168,85,247,0.15);
  }

  .card.pending { opacity: 0.65; border-style: dashed; }
  .card.calibrating { border-color: rgba(168, 85, 247, 0.3); }

  .top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1px;
  }

  .badge {
    font-size: 9px;
    font-weight: 800;
    letter-spacing: 0.08em;
    font-family: 'JetBrains Mono', monospace;
  }

  .ts {
    font-size: 10px;
    color: var(--muted);
  }

  .name {
    font-weight: 700;
    font-size: 14px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta {
    font-size: 10px;
    color: var(--muted);
    margin-bottom: 2px;
  }

  /* metrics row: ring + health + fusedR */
  .metrics-row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 4px 0 2px;
  }

  .ring {
    width: 38px;
    height: 38px;
    flex-shrink: 0;
  }

  .score-block, .fused-block {
    display: flex;
    flex-direction: column;
  }

  .score {
    font-size: 22px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    line-height: 1;
  }

  .score-label, .fused-label {
    font-size: 9px;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .fused-val {
    font-size: 15px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    font-family: 'JetBrains Mono', monospace;
    line-height: 1;
  }

  /* 9-signal bars */
  .sigs {
    display: flex;
    gap: 3px;
    height: 18px;
    align-items: flex-end;
    margin-top: 6px;
  }

  .sig-bar {
    flex: 1;
    height: 100%;
    background: var(--surface3);
    border-radius: 2px;
    overflow: hidden;
    display: flex;
    align-items: flex-end;
  }

  .sig-fill {
    width: 100%;
    border-radius: 2px;
    min-height: 2px;
    transition: height 0.3s;
  }

  .sig-labels {
    display: flex;
    gap: 3px;
    margin-top: 2px;
  }

  .sig-labels span {
    flex: 1;
    font-size: 7px;
    color: var(--muted);
    text-align: center;
    overflow: hidden;
    font-family: 'JetBrains Mono', monospace;
  }

  /* calibrating */
  .calib-row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 0 2px;
  }

  .calib-icon {
    color: var(--purple);
    font-size: 14px;
    animation: spin 2s linear infinite;
  }

  @keyframes spin { to { transform: rotate(360deg); } }

  .calib-txt {
    font-size: 11px;
    color: var(--purple);
    font-style: italic;
  }

  .calib-progress-wrap {
    height: 3px;
    background: var(--surface3);
    border-radius: 2px;
    overflow: hidden;
    margin-top: 4px;
  }

  .calib-progress-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--violet), var(--purple));
    border-radius: 2px;
    transition: width 0.5s ease;
  }

  .calib-pct {
    font-size: 9px;
    color: var(--muted);
    text-align: right;
    margin-top: 2px;
    font-family: 'JetBrains Mono', monospace;
  }

  /* pending */
  .pending-msg {
    font-size: 11px;
    color: var(--muted);
    margin-top: 4px;
    font-style: italic;
  }

  /* rupture warning */
  .rupture-warn {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin-top: 6px;
    padding: 6px 8px;
    background: rgba(239, 68, 68, 0.07);
    border: 1px solid rgba(239, 68, 68, 0.25);
    border-radius: 6px;
  }

  .rw-title {
    font-size: 9px;
    font-weight: 700;
    color: var(--red);
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .rw-body {
    font-size: 10px;
    color: var(--muted);
    line-height: 1.4;
  }

  .rw-body em { color: var(--yellow); font-style: italic; }
</style>

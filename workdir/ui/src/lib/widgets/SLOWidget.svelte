<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'

  export let widget = {}
  export let refreshTick = 0

  let status = null
  let loading = true
  let error = ''

  async function load() {
    loading = true; error = ''
    try {
      const sloId = widget.options?.slo_id || widget.slo_id || ''
      if (sloId) {
        const res = await api.sloStatus(sloId)
        status = res?.data || null
      } else {
        // Show all SLOs summary
        const res = await api.slosStatus()
        const all = res?.data || []
        if (all.length === 1) {
          status = all[0]
        } else {
          status = { _all: all }
        }
      }
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  onMount(load)
  $: if (refreshTick) load()

  const stateColor = { healthy: '#22c55e', at_risk: '#f59e0b', breached: '#ef4444', no_data: '#475569' }
  const stateLabel = { healthy: 'Healthy', at_risk: 'At Risk', breached: 'Breached', no_data: 'No Data' }
</script>

{#if loading}
  <div class="slo-state">loading…</div>
{:else if error}
  <div class="slo-state err">{error}</div>
{:else if !status}
  <div class="slo-state muted">No SLO configured. Set slo_id in widget options.</div>
{:else if status._all}
  <!-- Multi-SLO summary list -->
  <div class="slo-list">
    {#each status._all as s}
      <div class="slo-row">
        <span class="slo-dot" style="background:{stateColor[s.state] || '#475569'}"></span>
        <span class="slo-name">{s.slo?.name || '—'}</span>
        <span class="slo-comp" style="color:{stateColor[s.state]}">{s.compliance_pct?.toFixed(2)}%</span>
        <span class="slo-target muted">/{s.slo?.target}%</span>
        <span class="slo-budget" title="Error budget remaining">{s.error_budget_pct?.toFixed(1)}% budget</span>
      </div>
    {/each}
  </div>
{:else}
  <!-- Single SLO detail -->
  {@const s = status}
  {@const color = stateColor[s.state] || '#475569'}
  <div class="slo-detail">
    <div class="slo-toprow">
      <span class="slo-badge" style="background:{color}22; color:{color}; border-color:{color}55">
        {stateLabel[s.state] || s.state}
      </span>
      <span class="slo-name-big">{s.slo?.name}</span>
    </div>

    <div class="slo-metrics">
      <div class="slo-metric">
        <div class="sm-val" style="color:{color}">{s.compliance_pct?.toFixed(2)}%</div>
        <div class="sm-label">Compliance</div>
        <div class="sm-sub">target {s.slo?.target}%</div>
      </div>
      <div class="slo-metric">
        <div class="sm-val" style="color:{s.error_budget_pct > 20 ? '#22c55e' : s.error_budget_pct > 5 ? '#f59e0b' : '#ef4444'}">{s.error_budget_pct?.toFixed(1)}%</div>
        <div class="sm-label">Error Budget</div>
        <div class="sm-sub">{s.remaining_minutes?.toFixed(0)} min left</div>
      </div>
      <div class="slo-metric">
        <div class="sm-val" style="color:{s.burn_rate < 1 ? '#22c55e' : s.burn_rate < 2 ? '#f59e0b' : '#ef4444'}">{s.burn_rate?.toFixed(2)}x</div>
        <div class="sm-label">Burn Rate</div>
        <div class="sm-sub">1x = steady</div>
      </div>
    </div>

    <!-- Budget bar -->
    <div class="budget-bar-wrap">
      <div class="budget-bar-track">
        <div class="budget-bar-fill"
          style="width:{Math.max(0, Math.min(100, s.error_budget_pct || 0))}%;
                 background:{s.error_budget_pct > 20 ? '#22c55e' : s.error_budget_pct > 5 ? '#f59e0b' : '#ef4444'}">
        </div>
      </div>
      <span class="budget-bar-label">{s.slo?.window} window · {s.slo?.metric}</span>
    </div>
  </div>
{/if}

<style>
  .slo-state   { display:flex; align-items:center; justify-content:center; height:100%; color:#475569; font-size:0.8rem; }
  .err         { color:#ef4444; }
  .muted       { color:#475569; }

  /* List mode */
  .slo-list    { display:flex; flex-direction:column; gap:6px; padding:4px 2px; }
  .slo-row     { display:flex; align-items:center; gap:8px; font-size:0.78rem; }
  .slo-dot     { width:8px; height:8px; border-radius:50%; flex-shrink:0; }
  .slo-name    { flex:1; color:#e2e8f0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .slo-comp    { font-weight:700; font-variant-numeric:tabular-nums; }
  .slo-target  { color:#475569; }
  .slo-budget  { font-size:0.7rem; color:#64748b; }

  /* Detail mode */
  .slo-detail  { display:flex; flex-direction:column; gap:10px; padding:4px; height:100%; }
  .slo-toprow  { display:flex; align-items:center; gap:8px; }
  .slo-badge   { font-size:0.68rem; font-weight:700; padding:2px 8px; border-radius:20px; border:1px solid; white-space:nowrap; }
  .slo-name-big { font-size:0.88rem; font-weight:600; color:#e2e8f0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }

  .slo-metrics  { display:flex; gap:12px; }
  .slo-metric   { flex:1; text-align:center; background:#0f172a; border-radius:6px; padding:8px 4px; }
  .sm-val       { font-size:1.3rem; font-weight:700; line-height:1; }
  .sm-label     { font-size:0.68rem; color:#64748b; margin-top:2px; }
  .sm-sub       { font-size:0.62rem; color:#334155; margin-top:1px; }

  .budget-bar-wrap  { margin-top:auto; }
  .budget-bar-track { height:6px; background:#1e293b; border-radius:3px; overflow:hidden; }
  .budget-bar-fill  { height:100%; border-radius:3px; transition:width 0.5s; }
  .budget-bar-label { font-size:0.65rem; color:#475569; margin-top:4px; display:block; }
</style>

<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'
  import { formatEta, sortFailures, missingRoles } from '../lib/util/failures.js'

  const REFRESH_MS = 15000

  let failures = [], missing = [], loading = true, error = '', timer

  async function load() {
    try {
      const [found, inputs] = await Promise.all([api.failures(), api.inputs()])
      failures = sortFailures(found || [])
      missing = missingRoles(inputs || [])
      error = ''
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  const name = target => target.split('/').pop()
  const scope = target => target.split('/').slice(0, -1).join(' / ')
  const label = mode => mode.replace(/_/g, ' ')

  onMount(() => { load(); timer = setInterval(load, REFRESH_MS) })
  onDestroy(() => clearInterval(timer))
</script>

<div class="page-wrap">
  <div class="page-header band">
    <div style="grid-column:1/9">
      <h1 class="page-title">Failures</h1>
    </div>
    <div style="grid-column:9/13; display:flex; align-items:center; justify-content:flex-end; gap:12px">
      <span class="kicker">{failures.length} active</span>
    </div>
  </div>

  {#if loading}
    <div class="loading"><div class="spinner"></div></div>
  {:else if error}
    <div class="error-state">{error}</div>
  {:else if failures.length === 0}
    <div class="empty band"><p>No known failure right now.</p></div>
  {:else}
    <div class="band">
      <table class="failures">
        <thead>
          <tr><th>Target</th><th>Failure</th><th>Kind</th><th>State</th><th>Time left</th><th>Next step</th></tr>
        </thead>
        <tbody>
          {#each failures as failure (failure.target + failure.failureMode)}
            <tr>
              <td><strong>{name(failure.target)}</strong><div class="muted">{scope(failure.target)}</div></td>
              <td>{label(failure.failureMode)}</td>
              <td><span class="tag">{failure.kind}</span></td>
              <td class={failure.state}>{failure.state}</td>
              <td class="num">{formatEta(failure.etaSeconds)}</td>
              <td>{failure.recommendedAction}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  {#if missing.length > 0}
    <div class="band notice">
      <strong>Not visible right now:</strong> {missing.join(', ')}
    </div>
  {/if}
</div>

<style>
  .failures { width: 100%; border-collapse: collapse; }
  .failures th { text-align: left; padding: 10px 12px; font-size: 12px; opacity: .7; }
  .failures td { padding: 12px; border-top: 1px solid var(--border, rgba(127,127,127,.25)); vertical-align: top; }
  .muted { font-size: 12px; opacity: .65; }
  .num { font-variant-numeric: tabular-nums; white-space: nowrap; }
  .tag { font-size: 12px; padding: 2px 8px; border-radius: 999px; border: 1px solid currentColor; }
  .critical { color: var(--critical, #e5484d); font-weight: 600; }
  .degraded { color: var(--warning, #f5a524); font-weight: 600; }
  .notice { margin-top: 16px; font-size: 13px; opacity: .85; }
</style>

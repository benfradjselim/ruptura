<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'
  import { anomalyRows } from '../lib/util/anomalies.js'

  const REFRESH_MS = 30000

  let rows = [], loading = true, error = '', timer

  async function load() {
    try {
      rows = anomalyRows((await api.alerts()) || [], Date.now())
      error = ''
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  onMount(() => { load(); timer = setInterval(load, REFRESH_MS) })
  onDestroy(() => clearInterval(timer))
</script>

<section class="anomalies">
  <p class="eyebrow">Anomalies · last 60 minutes</p>
  <h1>{rows.length === 0 ? 'Nothing unusual' : `${rows.length} unusual measurement${rows.length > 1 ? 's' : ''}`}</h1>
  <p class="lead">A measurement far from what is usual for that workload. It is a signal to look at, not a failure. Known failures are on the Rupture Horizon.</p>

  {#if loading}
    <p class="quiet">Reading the cluster</p>
  {:else if error}
    <p class="fault-note">{error}</p>
  {:else if rows.length > 0}
    <table>
      <thead>
        <tr><th>When</th><th>Workload</th><th>Measurement</th><th>Observed</th><th>Usual</th><th>Change</th></tr>
      </thead>
      <tbody>
        {#each rows as row}
          <tr class={row.severity}>
            <td class="num">{row.minutesAgo === 0 ? 'now' : `${row.minutesAgo} min ago`}</td>
            <td><strong>{row.name}</strong><span class="scope">{row.scope}</span></td>
            <td>{row.metric}</td>
            <td class="num">{row.observed}</td>
            <td class="num usual">{row.usual}</td>
            <td class="change">{row.change}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>

<style>
  .anomalies { max-width: 1240px; margin: 0 auto; padding: 36px 28px 64px; }
  .eyebrow { font: 500 12px var(--font-mono); letter-spacing: 0.14em; text-transform: uppercase; color: var(--accent); }
  h1 { font-size: 34px; line-height: 40px; font-weight: 700; margin-top: 6px; }
  .lead { margin-top: 8px; max-width: 760px; color: var(--text-2); }
  table { width: 100%; margin-top: 28px; border-collapse: collapse; }
  th { text-align: left; padding: 8px 12px; border-bottom: 1px solid var(--border-2); }
  td { padding: 12px; border-bottom: 1px solid var(--border); vertical-align: top; }
  tr.critical td:first-child { box-shadow: inset 3px 0 0 var(--red); }
  tr.warning td:first-child { box-shadow: inset 3px 0 0 var(--amber); }
  .num { font-family: var(--font-mono); font-variant-numeric: tabular-nums; white-space: nowrap; }
  .usual { color: var(--text-3); }
  .scope { display: block; font: 400 12px var(--font-mono); color: var(--text-3); }
  .change { font-family: var(--font-mono); color: var(--text); white-space: nowrap; }
  .quiet { margin-top: 28px; color: var(--text-3); }
  .fault-note { margin-top: 28px; color: var(--red); }
  @media (max-width: 760px) { h1 { font-size: 26px; line-height: 32px; } table { font-size: 13px; } }
</style>

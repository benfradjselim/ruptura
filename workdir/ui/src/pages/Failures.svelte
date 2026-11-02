<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'
  import { formatEta, sortFailures, missingRoles, horizonPlacement, pastPlacement, relatedEvents } from '../lib/util/failures.js'

  const REFRESH_MS = 15000
  const HORIZON_SECONDS = 30 * 60
  const PAST_MINUTES = 30
  const TICKS = ['-30 min', '-15 min', 'Now', '+15 min', '+30 min']
  const LANES = 3

  let failures = [], missing = [], record = null, protection = null, history = [], selected = null, loading = true, error = '', timer

  async function load() {
    try {
      const [found, inputs, track, coverage, past] = await Promise.all([
        api.failures(),
        api.inputs().catch(() => []),
        api.trackRecord().catch(() => null),
        api.coverage().catch(() => null),
        api.eventHistory().catch(() => []),
      ])
      failures = sortFailures(found || [])
      missing = missingRoles(inputs || [])
      record = track
      protection = coverage
      history = past || []
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

  $: predicted = failures.filter(f => f.kind === 'predicted').length
  $: detected = failures.length - predicted
  $: critical = failures.filter(f => f.state === 'critical').length

  onMount(() => { load(); timer = setInterval(load, REFRESH_MS) })
  onDestroy(() => clearInterval(timer))
</script>

<section class="horizon-page">
  <header class="heading">
    <div>
      <p class="eyebrow">Rupture horizon</p>
      <h1>{failures.length === 0 ? 'No rupture ahead' : `${failures.length} rupture${failures.length > 1 ? 's' : ''} in sight`}</h1>
    </div>
    <dl class="tally">
      <div class:hot={critical > 0}><dt>Critical</dt><dd>{critical}</dd></div>
      <div><dt>Predicted</dt><dd>{predicted}</dd></div>
      <div><dt>Detected</dt><dd>{detected}</dd></div>
    </dl>
  </header>

  {#if loading}
    <p class="quiet">Reading the cluster</p>
  {:else if error}
    <p class="fault-note">{error}</p>
  {:else}
    <div class="axis" role="img" aria-label="Failures placed on a 30 minute horizon">
      <div class="line"></div>
      {#each TICKS as tick, i}
        <span class="tick" style="left:{(i / (TICKS.length - 1)) * 100}%">{tick}</span>
      {/each}
      <div class="now"></div>
      {#each history as event}
        <span class="past {event.type}" style="left:{pastPlacement(event.minutesAgo, PAST_MINUTES)}%" title="{name(event.target)}: {event.type} {event.minutesAgo} min ago {event.detail || ''}"></span>
      {/each}
      {#each failures as failure, i (failure.target + failure.failureMode)}
        {@const place = horizonPlacement(failure, HORIZON_SECONDS)}
        <div class="node {failure.state}" style="left:{place.position}%; --lane:{i % LANES}" title="{name(failure.target)}: {label(failure.failureMode)}">
          <span class="stem"></span>
          <span class="dot"></span>
          <span class="tag">{name(failure.target)}{place.beyond ? ' →' : ''}</span>
        </div>
      {/each}
    </div>

    {#if failures.length > 0}
      <ol class="ruptures">
        {#each failures as failure (failure.target + failure.failureMode)}
          <li class={failure.state}>
            <button class="open" on:click={() => (selected = failure)} aria-label="Details of {name(failure.target)}"></button>
            <div class="time">
              <strong>{failure.etaSeconds ? formatEta(failure.etaSeconds) : 'Now'}</strong>
              <span>{failure.kind}</span>
            </div>
            <div class="what">
              <h2>{name(failure.target)}</h2>
              <p>{scope(failure.target)}</p>
            </div>
            <div class="why">
              <h3>{label(failure.failureMode)}</h3>
              {#each failure.evidence || [] as fact}
                <p class="fact">{fact}</p>
              {/each}
              <p>{failure.recommendedAction}</p>
            </div>
          </li>
        {/each}
      </ol>
    {/if}
  {/if}

  {#if selected}
    {@const events = relatedEvents(history, selected.target)}
    <aside class="drawer" aria-label="Failure details">
      <button class="close" on:click={() => (selected = null)}>Close</button>
      <p class="eyebrow">{label(selected.failureMode)} · {selected.kind}</p>
      <h2>{name(selected.target)}</h2>
      <p class="scope">{scope(selected.target)}</p>
      <h3>Why Ruptura says this</h3>
      {#each selected.evidence || [] as fact}<p class="fact">{fact}</p>{/each}
      {#if selected.pod}<p class="scope">Pod {selected.pod.split('/').pop()}</p>{/if}
      <h3>What to do</h3>
      <p>{selected.recommendedAction}</p>
      <h3>Last 30 minutes</h3>
      {#if events.length === 0}
        <p class="scope">No change or failure recorded.</p>
      {:else}
        <ol class="events">
          {#each events as event}
            <li class={event.type}><span>{event.minutesAgo} min ago</span> {event.type}{event.detail ? `: ${event.detail}` : ''}</li>
          {/each}
        </ol>
      {/if}
    </aside>
  {/if}

  <div class="proof">
    <div>
      <h2>Track record <span>last 24 hours</span></h2>
      {#if record && record.failures > 0}
        <dl>
          <div><dt>Failures seen</dt><dd>{record.failures}</dd></div>
          <div><dt>Warned 5 min ahead</dt><dd>{record.warned}</dd></div>
          <div><dt>Median warning</dt><dd>{formatEta(record.medianLeadSeconds)}</dd></div>
        </dl>
      {:else}
        <p class="quiet">No failure recorded yet.</p>
      {/if}
    </div>

    <div>
      <h2>Blind spots</h2>
      {#if protection && protection.watched > 0}
        <dl>
          <div><dt>Pods watched</dt><dd>{protection.watched}</dd></div>
          <div class:warn={protection.protected < protection.watched}><dt>No memory limit</dt><dd>{protection.watched - protection.protected}</dd></div>
        </dl>
        {#if protection.protected < protection.watched}
          <p class="quiet">Without a limit, Ruptura cannot forecast memory exhaustion on these pods.</p>
        {/if}
      {/if}
      {#if missing.length > 0}
        <p class="quiet">Data not found: {missing.join(' · ')}</p>
      {/if}
    </div>
  </div>
</section>

<style>
  .horizon-page { max-width: 1240px; margin: 0 auto; padding: 36px 28px 64px; }

  .heading { display: flex; justify-content: space-between; align-items: flex-end; gap: 24px; flex-wrap: wrap; }
  .eyebrow { font: 500 12px var(--font-mono); letter-spacing: 0.14em; text-transform: uppercase; color: var(--accent); }
  h1 { font-size: 40px; line-height: 46px; font-weight: 700; margin-top: 6px; }

  .tally { display: flex; gap: 36px; }
  .tally dt { font: 500 11px var(--font-mono); letter-spacing: 0.1em; text-transform: uppercase; color: var(--text-3); }
  .tally dd { font: 500 32px/38px var(--font-mono); color: var(--text); font-variant-numeric: tabular-nums; }
  .tally .hot dd { color: var(--red); }

  .axis { position: relative; height: 190px; margin: 34px 14px 12px; }
  .line { position: absolute; left: 0; right: 0; bottom: 30px; height: 2px; background: var(--accent); }
  .tick { position: absolute; bottom: 0; transform: translateX(-50%); font: 400 12px var(--font-mono); color: var(--text-3); white-space: nowrap; }
  .tick:first-of-type { transform: none; }
  .tick:nth-last-of-type(1) { transform: translateX(-100%); }
  .now { position: absolute; left: 50%; bottom: 22px; height: 170px; width: 1px; background: var(--accent); opacity: 0.45; }
  .past { position: absolute; bottom: 26px; width: 10px; height: 10px; margin-left: -5px; transform: rotate(45deg); background: var(--surface-3); border: 1px solid var(--text-3); }
  .past.failure { background: var(--red); border-color: var(--red); opacity: 0.55; }
  .past.change { background: var(--accent); border-color: var(--accent); transform: none; width: 3px; height: 20px; margin-left: -1px; bottom: 21px; }

  .node { position: absolute; bottom: 30px; width: 0; }
  .stem { position: absolute; bottom: 0; left: 0; width: 1px; height: calc(44px + var(--lane) * 40px); background: var(--border-3); }
  .dot { position: absolute; bottom: -6px; left: -7px; width: 14px; height: 14px; transform: rotate(45deg); background: var(--amber); }
  .node.critical { z-index: 2; }
  .node.critical .dot { background: var(--red); }
  .tag {
    position: absolute; left: 6px; bottom: calc(30px + var(--lane) * 40px);
    font: 600 14px var(--font-ui); white-space: nowrap; color: var(--text);
  }

  .ruptures { list-style: none; margin-top: 30px; display: grid; gap: 10px; }
  .ruptures li {
    position: relative;
    display: grid; grid-template-columns: 150px 1fr 1.6fr; gap: 28px; align-items: center;
    padding: 16px 24px; background: var(--surface);
    border-left: 4px solid var(--amber);
    clip-path: polygon(0 0, 100% 0, 100% calc(100% - 14px), calc(100% - 14px) 100%, 0 100%);
  }
  .ruptures li.critical { border-left-color: var(--red); }
  .time strong { display: block; font: 500 24px/30px var(--font-mono); font-variant-numeric: tabular-nums; }
  .critical .time strong { color: var(--red); }
  .time span { font: 400 12px var(--font-mono); color: var(--text-3); text-transform: uppercase; letter-spacing: 0.08em; }
  h2 { font-size: 19px; line-height: 24px; font-weight: 700; }
  .what p { font: 400 12px var(--font-mono); color: var(--text-3); }
  h3 { font-size: 13px; line-height: 20px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--accent); font-weight: 600; }
  .why p { color: var(--text-2); }

  .open { position: absolute; inset: 0; background: none; border: 0; cursor: pointer; }
  .open:hover { background: var(--accent-dim); }
  .open:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
  .drawer {
    position: fixed; top: 60px; right: 0; bottom: 0; width: 420px; z-index: 30; overflow-y: auto;
    padding: 28px; background: var(--surface); border-left: 2px solid var(--accent);
  }
  .drawer h2 { font-size: 26px; line-height: 32px; margin-top: 4px; }
  .drawer h3 { margin-top: 24px; margin-bottom: 4px; }
  .drawer .scope { font: 400 12px var(--font-mono); color: var(--text-3); }
  .close { float: right; background: none; border: 1px solid var(--border-2); color: var(--text-2); padding: 4px 10px; font: 500 12px var(--font-mono); text-transform: uppercase; cursor: pointer; }
  .events { list-style: none; font: 400 13px var(--font-mono); }
  .events li { padding: 6px 0; border-bottom: 1px solid var(--border); }
  .events span { color: var(--text-3); margin-right: 8px; }
  .events .failure { color: var(--red); }
  .fact { color: var(--text); font-family: var(--font-mono); font-size: 13px; }
  .proof { display: grid; grid-template-columns: 1fr 1fr; gap: 36px; margin-top: 44px; padding-top: 24px; border-top: 1px solid var(--border-2); }
  .proof h2 { font-size: 16px; text-transform: uppercase; letter-spacing: 0.06em; }
  .proof h2 span { font: 400 12px var(--font-mono); color: var(--text-3); text-transform: none; letter-spacing: 0; margin-left: 8px; }
  .proof dl { display: flex; gap: 36px; margin-top: 14px; flex-wrap: wrap; }
  .proof dt { font: 500 11px var(--font-mono); letter-spacing: 0.08em; text-transform: uppercase; color: var(--text-3); }
  .proof dd { font: 500 26px/32px var(--font-mono); font-variant-numeric: tabular-nums; }
  .proof .warn dd { color: var(--amber); }
  .quiet { margin-top: 12px; color: var(--text-3); }
  .fault-note { margin-top: 40px; color: var(--red); }

  @media (max-width: 760px) {
    .drawer { width: 100%; }
    .proof { grid-template-columns: 1fr; }
    h1 { font-size: 28px; line-height: 34px; }
    .ruptures li { grid-template-columns: 1fr; gap: 8px; }
  }
</style>

<script>
  import { forecastGeometry } from '../util/forecastChart.js'

  export let pods = []
  export let failure
  export let failureTimes = []
  export let now = Date.now()
  export let height = 220
  export let compact = false

  let width = 0
  const short = pod => pod.split('/').pop()

  $: pad = compact ? { left: 34, right: 8, top: 8, bottom: 16 } : { left: 44, right: 14, top: 14, bottom: 24 }
  $: g = width > 0 ? forecastGeometry({ pods, failure, failureTimes, now, width, height, pad }) : null
  $: siblings = g ? g.lines.filter(l => !l.highlight).length : 0
</script>

<figure class="forecast" class:compact bind:clientWidth={width}>
  {#if g && !g.empty}
    <svg {width} {height} role="img"
         aria-label="Memory of {failure.pod ? short(failure.pod) : 'the workload'} as a share of its limit, last 30 minutes and forecast">
      <rect class="future" x={g.nowX} y={pad.top} width={Math.max(0, width - pad.right - g.nowX)} height={height - pad.top - pad.bottom} />
      {#each g.yTicks as tick}
        <line class="grid" x1={pad.left} x2={width - pad.right} y1={tick.y} y2={tick.y} />
        <text class="label" x={pad.left - 6} y={tick.y + 4} text-anchor="end">{tick.label}</text>
      {/each}
      <line class="limit" x1={pad.left} x2={width - pad.right} y1={g.limitY} y2={g.limitY} />
      {#if !compact}<text class="limit-label" x={pad.left + 6} y={g.limitY - 6}>memory limit</text>{/if}
      <line class="now" x1={g.nowX} x2={g.nowX} y1={pad.top} y2={height - pad.bottom} />
      {#each g.failures as fx}
        <line class="past-failure" x1={fx} x2={fx} y1={pad.top} y2={height - pad.bottom} />
      {/each}
      {#each g.lines as line (line.pod)}
        <path class="series" class:hot={line.highlight} d={line.d}><title>{short(line.pod)}</title></path>
      {/each}
      {#if g.band}<polygon class="band" points={g.band} />{/if}
      {#if g.projection}
        <line class="projection" x1={g.projection.x1} y1={g.projection.y1} x2={g.projection.x2} y2={g.projection.y2} />
      {/if}
      {#if g.eta}
        <line class="eta" x1={g.eta.x} x2={g.eta.x} y1={pad.top} y2={height - pad.bottom} />
        <rect class="eta-dot" x={g.eta.x - 5} y={g.limitY - 5} width="10" height="10" transform="rotate(45 {g.eta.x} {g.limitY})" />
        {#if !compact}<text class="eta-label" x={g.eta.x - 6} y={pad.top + 12} text-anchor="end">failure ~{g.eta.label}</text>{/if}
      {/if}
      {#each g.xTicks as tick, i}
        <text class="label" x={tick.x} y={height - 6} text-anchor={i === 0 ? 'start' : i === g.xTicks.length - 1 ? 'end' : 'middle'}>{tick.label}</text>
      {/each}
    </svg>
    {#if !compact && g.range}
      <p class="range">Failure expected between <strong>{g.range.from}</strong> and <strong>{g.range.to || 'later'}</strong>, most likely <strong>{g.eta ? g.eta.label : ''}</strong></p>
    {/if}
    {#if !compact}
      <figcaption>
        <span class="key hot"></span>{failure.pod ? short(failure.pod) : 'workload'}
        {#if siblings > 0}<span class="key"></span>{siblings} other replica{siblings > 1 ? 's' : ''}{/if}
        {#if g.projection}<span class="key dash"></span>trend Ruptura reads{/if}
        {#if g.band}<span class="key area"></span>range of trends{/if}
        {#if g.failures.length}<span class="key fail"></span>past failure{/if}
      </figcaption>
    {/if}
  {:else if g}
    <p class="none">No memory readings for this workload yet.</p>
  {/if}
</figure>

<style>
  .forecast { margin: 0; width: 100%; min-width: 0; }
  svg { display: block; overflow: visible; }
  .future { fill: var(--accent-dim); opacity: 0.35; }
  .grid { stroke: var(--border); stroke-width: 1; }
  .label { font: 400 10px var(--font-mono); fill: var(--text-3); font-variant-numeric: tabular-nums; }
  .limit { stroke: var(--red); stroke-width: 1.5; stroke-dasharray: 2 3; }
  .limit-label { font: 500 10px var(--font-mono); fill: var(--red); text-transform: uppercase; letter-spacing: 0.06em; }
  .now { stroke: var(--accent); stroke-width: 1; opacity: 0.6; }
  .past-failure { stroke: var(--red); stroke-width: 2; opacity: 0.45; }
  .series { fill: none; stroke: var(--text-3); stroke-width: 1.5; opacity: 0.7; stroke-linejoin: round; }
  .series.hot { stroke: var(--amber); stroke-width: 2.5; opacity: 1; }
  .band { fill: var(--amber); opacity: 0.16; }
  .range { margin-top: 8px; font: 400 12px var(--font-mono); color: var(--text-2); font-variant-numeric: tabular-nums; }
  .range strong { color: var(--text); font-weight: 600; }
  .key.area { height: 10px; border-top: 0; background: var(--amber); opacity: 0.3; }
  .projection { stroke: var(--amber); stroke-width: 2; stroke-dasharray: 6 5; }
  .eta { stroke: var(--red); stroke-width: 1; opacity: 0.5; }
  .eta-dot { fill: var(--red); }
  .eta-label { font: 600 11px var(--font-mono); fill: var(--red); font-variant-numeric: tabular-nums; }
  figcaption { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 14px; margin-top: 8px; font: 400 11px var(--font-mono); color: var(--text-3); }
  .key { display: inline-block; width: 16px; height: 0; border-top: 2px solid var(--text-3); margin-right: -8px; vertical-align: middle; }
  .key.hot { border-top: 3px solid var(--amber); }
  .key.dash { border-top: 2px dashed var(--amber); }
  .key.fail { width: 0; height: 12px; border-top: 0; border-left: 2px solid var(--red); }
  .none { font: 400 12px var(--font-mono); color: var(--text-3); }
</style>

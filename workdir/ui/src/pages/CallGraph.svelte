<script>
  import { onMount, onDestroy } from 'svelte'
  import { api } from '../lib/api.js'
  import { formatEta } from '../lib/util/failures.js'
  import { isPlatform, filterGraph, layout, blastRadius, hitEdges, NODE_W, NODE_H } from '../lib/util/callGraph.js'

  const REFRESH_MS = 30000
  const SOURCE_LABEL = { env: 'declared in env', route: 'Route', ingress: 'Ingress' }

  let graph = { nodes: [], edges: [] }, loading = true, error = '', timer
  let selected = null, showPlatform = false, chosen = null

  async function load() {
    try {
      graph = await api.callGraph()
      error = ''
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  const label = mode => (mode || '').replace(/_/g, ' ')

  $: namespaces = [...new Set(graph.nodes.map(n => n.namespace))].sort()
  $: visibleNamespaces = chosen || new Set(namespaces.filter(ns => showPlatform || !isPlatform(ns)))
  $: view = layout(filterGraph(graph, visibleNamespaces))
  $: failing = view.nodes.filter(n => n.state !== 'ok')
  $: focus = selected ? [selected.id] : failing.map(n => n.id)
  $: radius = new Set(focus.flatMap(id => [...blastRadius(view.edges, id)]))
  $: hot = new Set(focus.flatMap(id => [...hitEdges(view.edges, id, blastRadius(view.edges, id))]))
  $: calls = selected ? view.edges.filter(e => e.from === selected.id) : []
  $: calledBy = selected ? view.edges.filter(e => e.to === selected.id) : []
  $: selectedRadius = selected ? [...blastRadius(view.edges, selected.id)] : []
  $: byId = new Map(view.nodes.map(n => [n.id, n]))

  function toggle(ns) {
    const next = new Set(visibleNamespaces)
    next.has(ns) ? next.delete(ns) : next.add(ns)
    chosen = next
  }

  onMount(() => { load(); timer = setInterval(load, REFRESH_MS) })
  onDestroy(() => clearInterval(timer))
</script>

<section class="graph-page">
  <header class="heading">
    <div>
      <p class="eyebrow">Call graph</p>
      <h1>Who calls whom</h1>
      <p class="lede">Read from what the cluster declares: services named in container environment, and Routes or Ingresses. A failure turns its callers red, they are what it can take down.</p>
    </div>
    <dl class="tally">
      <div><dt>Workloads</dt><dd>{view.nodes.filter(n => n.type === 'workload').length}</dd></div>
      <div><dt>Calls</dt><dd>{view.edges.length}</dd></div>
      <div class:hot={failing.length > 0}><dt>Failing</dt><dd>{failing.length}</dd></div>
    </dl>
  </header>

  {#if namespaces.length > 0}
    <div class="filters">
      {#each namespaces.filter(ns => showPlatform || !isPlatform(ns)) as ns}
        <button class="chip" class:on={visibleNamespaces.has(ns)} on:click={() => toggle(ns)}>{ns}</button>
      {/each}
      <label class="platform"><input type="checkbox" bind:checked={showPlatform} on:change={() => (chosen = null)} /> Platform namespaces</label>
    </div>
  {/if}

  {#if loading}
    <p class="quiet">Reading the cluster</p>
  {:else if error}
    <p class="fault-note">{error}</p>
  {:else if view.nodes.length === 0}
    <p class="quiet">No call found in these namespaces. Ruptura sees a call when a container names a Service in its environment, or a Route sends traffic to one. Observed calls from traces and network flows come later.</p>
  {:else}
    <div class="canvas">
      <svg width={view.width} height={view.height} role="img" aria-label="Call graph of {view.nodes.length} workloads">
        {#each view.edges as edge (edge.from + edge.to)}
          <path class="edge {edge.source}" class:hit={hot.has(edge.from + '|' + edge.to)} d={edge.d}>
            <title>{byId.get(edge.from)?.name} calls {byId.get(edge.to)?.name}, {SOURCE_LABEL[edge.source]}{edge.via ? `: ${edge.via}` : ''}</title>
          </path>
        {/each}
        {#each view.nodes as n (n.id)}
          <g class="node {n.state} {n.type}" class:radius={radius.has(n.id)} class:picked={selected?.id === n.id}
             transform="translate({n.x},{n.y})" role="button" tabindex="0"
             on:click={() => (selected = selected?.id === n.id ? null : n)}
             on:keydown={e => (e.key === 'Enter' || e.key === ' ') && (selected = selected?.id === n.id ? null : n)}>
            <rect width={NODE_W} height={NODE_H} />
            <text class="name" x="12" y="22">{n.name.length > 22 ? n.name.slice(0, 21) + '…' : n.name}</text>
            <text class="sub" x="12" y="40">{n.type === 'entry' ? n.kind : n.namespace}{n.failureMode ? ` · ${label(n.failureMode)}` : ''}</text>
          </g>
        {/each}
      </svg>
    </div>
    <p class="legend">
      <span class="key env"></span>declared in env
      <span class="key route"></span>Route or Ingress
      <span class="key hit"></span>can be hit by a failure
      <span class="key box crit"></span>failing
    </p>
  {/if}

  {#if selected}
    <aside class="drawer" aria-label="Workload details">
      <button class="close" on:click={() => (selected = null)}>Close</button>
      <p class="eyebrow">{selected.kind} · {selected.namespace}</p>
      <h2>{selected.name}</h2>
      {#if selected.host}<p class="scope">{selected.host}</p>{/if}
      {#if selected.failureMode}
        <h3>Failure</h3>
        <p class="fact">{label(selected.failureMode)}, {selected.failureKind}{selected.etaSeconds ? ` in ${formatEta(selected.etaSeconds)}` : ''}</p>
      {/if}
      <h3>Calls</h3>
      {#if calls.length === 0}<p class="scope">Nothing declared.</p>{/if}
      {#each calls as e}<p class="fact">{byId.get(e.to)?.name} <span>{SOURCE_LABEL[e.source]}{e.via ? `: ${e.via}` : ''}</span></p>{/each}
      <h3>Called by</h3>
      {#if calledBy.length === 0}<p class="scope">No caller found.</p>{/if}
      {#each calledBy as e}<p class="fact">{byId.get(e.from)?.name} <span>{SOURCE_LABEL[e.source]}{e.via ? `: ${e.via}` : ''}</span></p>{/each}
      <h3>If it fails, it can hit</h3>
      {#if selectedRadius.length === 0}
        <p class="scope">No caller depends on it.</p>
      {:else}
        <p class="fact">{selectedRadius.map(id => byId.get(id)?.name).join(', ')}</p>
      {/if}
    </aside>
  {/if}
</section>

<style>
  .graph-page { max-width: 1240px; margin: 0 auto; padding: 36px 28px 64px; }
  .heading { display: flex; justify-content: space-between; align-items: flex-end; gap: 24px; flex-wrap: wrap; }
  .eyebrow { font: 500 12px var(--font-mono); letter-spacing: 0.14em; text-transform: uppercase; color: var(--accent); }
  h1 { font-size: 40px; line-height: 46px; font-weight: 700; margin-top: 6px; }
  .lede { margin-top: 8px; max-width: 640px; color: var(--text-2); }
  .tally { display: flex; gap: 36px; }
  .tally dt { font: 500 11px var(--font-mono); letter-spacing: 0.1em; text-transform: uppercase; color: var(--text-3); }
  .tally dd { font: 500 32px/38px var(--font-mono); font-variant-numeric: tabular-nums; }
  .tally .hot dd { color: var(--red); }

  .filters { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 26px; align-items: center; }
  .chip { background: none; border: 1px solid var(--border-2); color: var(--text-3); padding: 4px 10px; font: 500 12px var(--font-mono); cursor: pointer; }
  .chip.on { border-color: var(--accent); color: var(--text); background: var(--accent-dim); }
  .platform { margin-left: auto; font: 400 12px var(--font-mono); color: var(--text-3); display: flex; gap: 6px; align-items: center; }

  .canvas { margin-top: 20px; overflow: auto; background: var(--surface); border: 1px solid var(--border); max-height: 70vh; }
  svg { display: block; }
  .edge { fill: none; stroke: var(--text-3); stroke-width: 1.5; opacity: 0.7; stroke-dasharray: 6 6; animation: flow 1.2s linear infinite; }
  .edge.route, .edge.ingress { stroke: var(--accent); }
  .edge.hit { stroke: var(--red); stroke-width: 2.5; opacity: 1; animation-duration: 0.6s; }
  @keyframes flow { to { stroke-dashoffset: -12; } }
  @media (prefers-reduced-motion: reduce) { .edge { animation: none; } }

  .node { cursor: pointer; }
  .node rect { fill: var(--surface-2); stroke: var(--border-3); stroke-width: 1; }
  .node.entry rect { stroke: var(--accent); stroke-dasharray: 4 3; }
  .node.radius rect { stroke: var(--red); stroke-width: 1.5; }
  .node.degraded rect, .node.warning rect { fill: var(--amber-dim); stroke: var(--amber); stroke-width: 2; }
  .node.critical rect { fill: var(--red-dim); stroke: var(--red); stroke-width: 2; }
  .node.picked rect { stroke: var(--accent); stroke-width: 2.5; }
  .node:focus-visible { outline: none; }
  .node:focus-visible rect { stroke: var(--accent); stroke-width: 3; }
  .name { font: 600 13px var(--font-ui); fill: var(--text); }
  .sub { font: 400 11px var(--font-mono); fill: var(--text-3); }

  .legend { display: flex; flex-wrap: wrap; gap: 6px 16px; align-items: center; margin-top: 10px; font: 400 12px var(--font-mono); color: var(--text-3); }
  .key { display: inline-block; width: 18px; border-top: 2px dashed var(--text-3); margin-right: -10px; }
  .key.route { border-top-color: var(--accent); }
  .key.hit { border-top: 2px solid var(--red); }
  .key.box { width: 12px; height: 12px; border: 2px solid var(--red); background: var(--red-dim); }

  .drawer { position: fixed; top: 60px; right: 0; bottom: 0; width: 420px; z-index: 30; overflow-y: auto; padding: 28px; background: var(--surface); border-left: 2px solid var(--accent); }
  .drawer h2 { font-size: 26px; line-height: 32px; margin-top: 4px; }
  .drawer h3 { margin-top: 22px; margin-bottom: 4px; font-size: 13px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--accent); font-weight: 600; }
  .scope { font: 400 12px var(--font-mono); color: var(--text-3); }
  .fact { font: 400 13px var(--font-mono); color: var(--text); padding: 2px 0; }
  .fact span { color: var(--text-3); }
  .close { float: right; background: none; border: 1px solid var(--border-2); color: var(--text-2); padding: 4px 10px; font: 500 12px var(--font-mono); text-transform: uppercase; cursor: pointer; }
  .quiet { margin-top: 24px; color: var(--text-3); max-width: 720px; }
  .fault-note { margin-top: 40px; color: var(--red); }

  @media (max-width: 760px) {
    .drawer { width: 100%; }
    h1 { font-size: 28px; line-height: 34px; }
    .platform { margin-left: 0; }
  }
</style>

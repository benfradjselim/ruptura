<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte'
  import { fetchHealth, fetchEngineStatus } from '../lib/api'

  export let route: string
  export let theme = 'dark'

  const dispatch = createEventDispatcher()

  let version = ''
  let connected = false
  let calibrating = 0
  let metricsRps = 0
  let interval: ReturnType<typeof setInterval>

  async function poll() {
    try {
      const [h, s] = await Promise.all([fetchHealth(), fetchEngineStatus()])
      version = h.version
      connected = h.status !== 'unreachable'
      calibrating = s.analyzer.calibrating_workloads
      metricsRps = Math.round(s.ingest.metrics_per_sec)
    } catch {
      connected = false
    }
  }

  onMount(() => {
    poll()
    interval = setInterval(poll, 10_000)
  })
  onDestroy(() => clearInterval(interval))

  const navLinks = [
    { id: 'fleet',    label: 'Fleet',    icon: '⬡' },
    { id: 'map',      label: 'Topology', icon: '⎋' },
    { id: 'alerts',   label: 'Alerts',   icon: '◉' },
    { id: 'engine',   label: 'Engine',   icon: '⚙' },
    { id: 'nodes',    label: 'Nodes',    icon: '◫' },
    { id: 'settings', label: 'Settings', icon: '⊞' },
  ]
</script>

<nav>
  <a class="brand" href="#fleet">
    <svg class="logo-img" viewBox="0 0 40 40" xmlns="http://www.w3.org/2000/svg" width="26" height="26" aria-label="Ruptura">
      <defs>
        <linearGradient id="rnav" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stop-color="#a855f7"/>
          <stop offset="100%" stop-color="#06b6d4"/>
        </linearGradient>
      </defs>
      <polygon points="20,3 35,11.5 35,28.5 20,37 5,28.5 5,11.5" fill="url(#rnav)"/>
      <text x="20" y="26" text-anchor="middle" font-family="monospace" font-weight="700" font-size="17" fill="white">R</text>
    </svg>
    <span class="name">RUPTURA</span>
    {#if version}<span class="version">v{version}</span>{/if}
  </a>

  <div class="links">
    {#each navLinks as link}
      <a
        href="#{link.id}"
        class:active={route === link.id || (route === '' && link.id === 'fleet')}
      >
        <span class="icon">{link.icon}</span>
        {link.label}
      </a>
    {/each}
  </div>

  <div class="right">
    {#if calibrating > 0}
      <span class="chip calib" title="{calibrating} workload(s) calibrating">⊙ {calibrating} cal.</span>
    {/if}
    {#if metricsRps > 0}
      <span class="chip ingest">{metricsRps}/s</span>
    {/if}
    <button class="theme-btn" on:click={() => dispatch('toggleTheme')} title="Toggle light/dark mode">
      {theme === 'dark' ? '☀' : '🌙'}
    </button>
    <div class="pulse-wrap" title="Engine: {connected ? 'connected' : 'unreachable'}">
      <div class="dot" class:connected></div>
      {#if connected}<div class="dot-ring"></div>{/if}
    </div>
  </div>
</nav>

<style>
  nav {
    display: flex;
    align-items: center;
    gap: 0;
    padding: 0 20px;
    height: 52px;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
    z-index: 100;
    backdrop-filter: blur(12px);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 9px;
    text-decoration: none;
    flex-shrink: 0;
    margin-right: 24px;
  }

  .logo-img {
    flex-shrink: 0;
    border-radius: 5px;
    display: block;
  }

  .name {
    font-weight: 800;
    font-size: 13px;
    letter-spacing: 0.12em;
    background: linear-gradient(135deg, var(--purple), var(--cyan));
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .version {
    font-size: 10px;
    color: var(--muted);
    background: var(--surface2);
    padding: 1px 6px;
    border-radius: 4px;
    border: 1px solid var(--border);
    font-family: 'JetBrains Mono', monospace;
    -webkit-text-fill-color: var(--muted);
  }

  .links {
    display: flex;
    gap: 2px;
    flex: 1;
  }

  a {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 12px;
    border-radius: 6px;
    color: var(--muted);
    text-decoration: none;
    font-size: 13px;
    font-weight: 500;
    transition: color 0.15s, background 0.15s;
  }

  a:hover {
    color: var(--text);
    background: var(--surface2);
  }

  a.active {
    color: var(--purple);
    background: rgba(168, 85, 247, 0.12);
  }

  .icon { font-size: 14px; opacity: 0.8; }

  .right {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-left: auto;
  }

  .chip {
    font-size: 10px;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: 20px;
    font-family: 'JetBrains Mono', monospace;
    letter-spacing: 0.04em;
  }

  .chip.calib {
    background: rgba(245, 158, 11, 0.12);
    color: var(--yellow);
    border: 1px solid rgba(245, 158, 11, 0.3);
  }

  .chip.ingest {
    background: rgba(168, 85, 247, 0.1);
    color: var(--purple);
    border: 1px solid rgba(168, 85, 247, 0.25);
  }

  .pulse-wrap {
    position: relative;
    width: 12px;
    height: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--muted);
    position: relative;
    z-index: 1;
    transition: background 0.3s;
  }

  .dot.connected { background: var(--green); }

  .dot-ring {
    position: absolute;
    inset: -2px;
    border-radius: 50%;
    background: transparent;
    border: 2px solid var(--green);
    animation: pulse-ring 2s ease-out infinite;
  }

  @keyframes pulse-ring {
    0%   { transform: scale(0.8); opacity: 0.8; }
    100% { transform: scale(2);   opacity: 0; }
  }

  .theme-btn {
    background: none;
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--muted);
    cursor: pointer;
    padding: 4px 8px;
    font-size: 14px;
    line-height: 1;
    transition: color 0.15s, border-color 0.15s;
  }
  .theme-btn:hover { color: var(--text); border-color: var(--muted); }
</style>

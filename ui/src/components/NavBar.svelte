<script lang="ts">
  import { onMount } from 'svelte'
  import { fetchHealth } from '../lib/api'

  export let route: string

  let version = ''
  let status = ''

  onMount(async () => {
    try {
      const h = await fetchHealth()
      version = h.version
      status = h.status
    } catch {
      status = 'unreachable'
    }
  })

  const navLinks = [
    { id: 'fleet',  label: 'Fleet',   icon: '⬡' },
    { id: 'map',    label: 'Topology', icon: '⎋' },
    { id: 'engine', label: 'Engine',  icon: '⚙' },
    { id: 'nodes',  label: 'Nodes',   icon: '◫' },
  ]
</script>

<nav>
  <div class="brand">
    <span class="logo">◈</span>
    <span class="name">ruptura</span>
    {#if version}
      <span class="version">v{version}</span>
    {/if}
  </div>

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

  <div class="status-dot" class:ok={status === 'ok'} title="Engine: {status}"></div>
</nav>

<style>
  nav {
    display: flex;
    align-items: center;
    gap: 24px;
    padding: 0 24px;
    height: 52px;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
    z-index: 100;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }

  .logo {
    color: var(--cyan);
    font-size: 20px;
  }

  .name {
    font-weight: 700;
    font-size: 15px;
    letter-spacing: 0.04em;
    color: var(--text);
  }

  .version {
    font-size: 11px;
    color: var(--muted);
    background: var(--surface2);
    padding: 1px 6px;
    border-radius: 4px;
    border: 1px solid var(--border);
  }

  .links {
    display: flex;
    gap: 4px;
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
    color: var(--cyan);
    background: rgba(57, 208, 216, 0.1);
  }

  .icon {
    font-size: 14px;
    opacity: 0.7;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--muted);
    flex-shrink: 0;
    margin-left: auto;
  }

  .status-dot.ok {
    background: var(--green);
    box-shadow: 0 0 6px var(--green);
  }
</style>

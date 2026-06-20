<script>
  export let entry = null
  export let onClose = () => {}

  $: parsed = (() => {
    if (!entry) return null
    if (typeof entry === 'string') {
      try { return JSON.parse(entry) } catch { return { message: entry } }
    }
    return entry
  })()

  $: pairs = parsed ? Object.entries(parsed).filter(([k]) => k !== 'message') : []
</script>

{#if parsed}
  <div class="panel">
    <div class="panel-header">
      <span class="panel-title">Log Detail</span>
      <button class="close-btn" on:click={onClose}>✕</button>
    </div>
    <div class="panel-body">
      <div class="msg-block">
        <code>{parsed.message || parsed.body || '(no message)'}</code>
      </div>
      <div class="pairs">
        {#each pairs as [k, v]}
          <div class="pair">
            <span class="pair-key">{k}</span>
            <span class="pair-val">{typeof v === 'object' ? JSON.stringify(v) : String(v)}</span>
          </div>
        {/each}
      </div>
      <div class="raw-wrap">
        <details>
          <summary class="raw-toggle">Raw JSON</summary>
          <pre class="raw">{JSON.stringify(parsed, null, 2)}</pre>
        </details>
        <button class="copy-btn" on:click={() => navigator.clipboard.writeText(JSON.stringify(parsed, null, 2))}>Copy</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .panel {
    width: 380px; background: var(--surface); border-left: 1px solid var(--border-2);
    display: flex; flex-direction: column; height: 100%;
    flex-shrink: 0;
  }
  .panel-header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 10px 14px; border-bottom: 1px solid var(--border-2);
  }
  .panel-title { font-size: 0.82rem; font-weight: 600; color: var(--text-2); }
  .close-btn   { background: none; border: none; color: var(--text-3); cursor: pointer; font-size: 1rem; }
  .close-btn:hover { color: var(--text); }

  .panel-body { flex: 1; overflow-y: auto; padding: 12px; display: flex; flex-direction: column; gap: 12px; }

  .msg-block code {
    display: block; background: var(--bg); border: 1px solid var(--border-2);
    border-radius: 6px; padding: 10px; font-size: 0.8rem; color: var(--text);
    white-space: pre-wrap; word-break: break-all;
  }

  .pairs { display: flex; flex-direction: column; gap: 4px; }
  .pair  { display: flex; gap: 8px; font-size: 0.78rem; align-items: flex-start; }
  .pair-key { min-width: 90px; color: var(--accent); font-weight: 600; flex-shrink: 0; word-break: break-all; }
  .pair-val { color: var(--text-2); word-break: break-all; }

  .raw-wrap { position: relative; }
  .raw-toggle { font-size: 0.75rem; color: #475569; cursor: pointer; }
  .raw { background: var(--bg); border: 1px solid var(--border-2); border-radius: 6px; padding: 8px; font-size: 0.72rem; color: var(--text-3); overflow-x: auto; margin-top: 6px; }
  .copy-btn { position: absolute; top: 0; right: 0; background: var(--border-2); border: none; color: var(--text-2); padding: 3px 8px; border-radius: 4px; cursor: pointer; font-size: 0.72rem; }
  .copy-btn:hover { color: var(--text); }
</style>

<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte'
  import {
    fetchSuppressions,
    createSuppression,
    deleteSuppression,
  } from '../lib/api'
  import type { Suppression } from '../lib/api'

  export let defaultWorkload = ''

  const dispatch = createEventDispatcher<{ close: void }>()

  let windows: Suppression[] = []
  let loadErr = ''
  let saving = false
  let saveErr = ''

  // form fields
  let workload = defaultWorkload
  let startVal = dtLocal(new Date())
  let endVal = dtLocal(new Date(Date.now() + 3_600_000))
  let reason = ''

  function dtLocal(d: Date): string {
    // datetime-local format: YYYY-MM-DDTHH:mm
    return d.toISOString().slice(0, 16)
  }

  function fmtRange(s: string, e: string): string {
    const fmt = (iso: string) =>
      new Date(iso).toLocaleString(undefined, {
        month: 'short', day: 'numeric',
        hour: '2-digit', minute: '2-digit',
      })
    return `${fmt(s)} → ${fmt(e)}`
  }

  function isActive(end: string): boolean {
    return new Date(end) > new Date()
  }

  async function load() {
    try {
      windows = await fetchSuppressions()
    } catch (e) {
      loadErr = e instanceof Error ? e.message : String(e)
    }
  }

  async function submit() {
    if (!workload.trim()) { saveErr = 'Workload is required'; return }
    const start = new Date(startVal)
    const end = new Date(endVal)
    if (end <= start) { saveErr = 'End must be after start'; return }
    saving = true
    saveErr = ''
    try {
      await createSuppression({
        workload: workload.trim(),
        start: start.toISOString(),
        end: end.toISOString(),
        reason: reason.trim(),
      })
      reason = ''
      workload = defaultWorkload
      await load()
    } catch (e) {
      saveErr = e instanceof Error ? e.message : String(e)
    } finally {
      saving = false
    }
  }

  async function remove(id: string) {
    try {
      await deleteSuppression(id)
      windows = windows.filter(w => w.id !== id)
    } catch (e) {
      loadErr = e instanceof Error ? e.message : String(e)
    }
  }

  onMount(load)
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="backdrop" on:click|self={() => dispatch('close')}>
  <div class="modal" role="dialog" aria-modal="true" aria-label="Maintenance windows">
    <div class="header">
      <span class="title">🔕 Maintenance Windows</span>
      <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close">✕</button>
    </div>

    <!-- existing windows -->
    <section class="section">
      <div class="section-label">Active & Scheduled</div>
      {#if loadErr}
        <div class="msg-err">{loadErr}</div>
      {:else if windows.length === 0}
        <div class="msg-empty">No suppression windows configured.</div>
      {:else}
        <ul class="window-list">
          {#each windows as w (w.id)}
            <li class="window-row" class:active={isActive(w.end)}>
              <div class="window-info">
                <span class="window-workload">{w.workload || '(all)'}</span>
                <span class="window-range">{fmtRange(w.start, w.end)}</span>
                {#if w.reason}
                  <span class="window-reason">{w.reason}</span>
                {/if}
              </div>
              <button
                class="del-btn"
                on:click={() => remove(w.id)}
                aria-label="Delete suppression"
              >✕</button>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <!-- add form -->
    <section class="section">
      <div class="section-label">Add Window</div>
      <form on:submit|preventDefault={submit} class="form">
        <label class="field">
          <span>Workload key <span class="hint">(namespace/kind/name or * for all)</span></span>
          <input type="text" bind:value={workload} placeholder="production/Deployment/api" />
        </label>

        <div class="row-2">
          <label class="field">
            <span>Start</span>
            <input type="datetime-local" bind:value={startVal} />
          </label>
          <label class="field">
            <span>End</span>
            <input type="datetime-local" bind:value={endVal} />
          </label>
        </div>

        <label class="field">
          <span>Reason <span class="hint">(optional)</span></span>
          <input type="text" bind:value={reason} placeholder="Planned maintenance, deploy window…" />
        </label>

        {#if saveErr}
          <div class="msg-err">{saveErr}</div>
        {/if}

        <button type="submit" class="submit-btn" disabled={saving}>
          {saving ? 'Saving…' : 'Create window'}
        </button>
      </form>
    </section>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 16px;
  }

  .modal {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    width: 100%;
    max-width: 560px;
    max-height: 90vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
    background: var(--surface);
    z-index: 1;
  }

  .title {
    font-weight: 700;
    font-size: 15px;
  }

  .close-btn {
    background: none;
    border: none;
    color: var(--muted);
    cursor: pointer;
    font-size: 16px;
    padding: 2px 6px;
    border-radius: 4px;
  }

  .close-btn:hover { color: var(--text); }

  .section {
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
  }

  .section:last-child { border-bottom: none; }

  .section-label {
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: var(--muted);
    margin-bottom: 12px;
  }

  .msg-empty {
    font-size: 12px;
    color: var(--muted);
    font-style: italic;
  }

  .msg-err {
    font-size: 12px;
    color: var(--red);
    background: rgba(224, 82, 82, 0.08);
    border: 1px solid rgba(224, 82, 82, 0.3);
    border-radius: 6px;
    padding: 6px 10px;
    margin-bottom: 8px;
  }

  .window-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .window-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    background: var(--surface2);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 10px 12px;
  }

  .window-row.active {
    border-color: rgba(88, 166, 255, 0.4);
  }

  .window-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .window-workload {
    font-weight: 600;
    font-size: 13px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--blue);
  }

  .window-range {
    font-size: 11px;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }

  .window-reason {
    font-size: 11px;
    color: var(--muted);
    font-style: italic;
  }

  .del-btn {
    flex-shrink: 0;
    background: none;
    border: 1px solid var(--border);
    color: var(--muted);
    cursor: pointer;
    font-size: 12px;
    padding: 4px 8px;
    border-radius: 4px;
    transition: color 0.15s, border-color 0.15s;
  }

  .del-btn:hover {
    color: var(--red);
    border-color: var(--red);
  }

  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
    color: var(--muted);
  }

  .hint {
    font-weight: 400;
    opacity: 0.7;
  }

  .field input {
    background: var(--surface2);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 8px 10px;
    color: var(--text);
    font-size: 13px;
    outline: none;
    transition: border-color 0.15s;
    width: 100%;
  }

  .field input:focus {
    border-color: var(--cyan);
  }

  .row-2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }

  .submit-btn {
    background: var(--cyan);
    color: #0d1117;
    border: none;
    border-radius: 8px;
    padding: 10px 18px;
    font-size: 13px;
    font-weight: 700;
    cursor: pointer;
    transition: opacity 0.15s;
    align-self: flex-start;
  }

  .submit-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .submit-btn:hover:not(:disabled) {
    opacity: 0.85;
  }
</style>

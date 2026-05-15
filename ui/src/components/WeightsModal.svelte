<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte'
  import { fetchWeights, saveWeights } from '../lib/api'
  import type { SignalWeights } from '../lib/api'

  const dispatch = createEventDispatcher<{ close: void }>()

  const SIGNALS: Array<keyof Omit<SignalWeights, 'selector'>> = [
    'stress', 'fatigue', 'mood', 'pressure', 'humidity', 'contagion',
  ]

  const DEFAULT_WEIGHTS = { stress: 0.25, fatigue: 0.20, mood: 0.20, pressure: 0.15, humidity: 0.10, contagion: 0.10 }

  let rows: SignalWeights[] = []
  let loadErr = ''
  let saving = false
  let saveErr = ''
  let saved = false

  function newRow(): SignalWeights {
    return { selector: '*', ...DEFAULT_WEIGHTS }
  }

  function sum(row: SignalWeights): number {
    return SIGNALS.reduce((acc, k) => acc + (row[k] as number), 0)
  }

  function sumColor(s: number): string {
    const d = Math.abs(s - 1.0)
    if (d < 0.01) return 'var(--green)'
    if (d < 0.05) return 'var(--yellow)'
    return 'var(--red)'
  }

  function addRow() {
    rows = [...rows, newRow()]
  }

  function removeRow(i: number) {
    rows = rows.filter((_, idx) => idx !== i)
  }

  async function load() {
    try {
      const data = await fetchWeights()
      rows = data.length > 0 ? data : [newRow()]
    } catch (e) {
      loadErr = e instanceof Error ? e.message : String(e)
      rows = [newRow()]
    }
  }

  async function save() {
    saving = true
    saveErr = ''
    saved = false
    try {
      await saveWeights(rows)
      saved = true
      setTimeout(() => { saved = false }, 2500)
    } catch (e) {
      saveErr = e instanceof Error ? e.message : String(e)
    } finally {
      saving = false
    }
  }

  onMount(load)
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="backdrop" on:click|self={() => dispatch('close')}>
  <div class="modal" role="dialog" aria-modal="true" aria-label="Signal weight overrides">
    <div class="header">
      <span class="title">⚖ Signal Weight Overrides</span>
      <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close">✕</button>
    </div>

    <div class="info-bar">
      Weights are normalised to sum to 1.0 by the engine. Entries are evaluated in order — first matching selector wins.
      Selector supports glob patterns: <code>payments/*</code>, <code>batch/*</code>, <code>*</code>.
    </div>

    {#if loadErr}
      <div class="msg-err padded">{loadErr}</div>
    {/if}

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th class="th-sel">Selector</th>
            {#each SIGNALS as sig}
              <th>{sig.slice(0, 3)}</th>
            {/each}
            <th>Sum</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each rows as row, i (i)}
            <tr>
              <td>
                <input
                  class="cell-input sel-input"
                  type="text"
                  bind:value={row.selector}
                  placeholder="*"
                  aria-label="Selector for row {i + 1}"
                />
              </td>
              {#each SIGNALS as sig}
                <td>
                  <input
                    class="cell-input num-input"
                    type="number"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={row[sig]}
                    aria-label="{sig} weight for row {i + 1}"
                  />
                </td>
              {/each}
              <td class="sum-cell" style="color:{sumColor(sum(row))}">{sum(row).toFixed(2)}</td>
              <td>
                <button
                  class="del-btn"
                  on:click={() => removeRow(i)}
                  aria-label="Remove row {i + 1}"
                  title="Remove"
                >✕</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="footer">
      <button class="add-btn" on:click={addRow}>+ Add rule</button>
      <div class="footer-right">
        {#if saveErr}
          <span class="msg-err inline">{saveErr}</span>
        {/if}
        {#if saved}
          <span class="msg-ok">✓ Saved</span>
        {/if}
        <button class="save-btn" on:click={save} disabled={saving}>
          {saving ? 'Saving…' : 'Save all'}
        </button>
      </div>
    </div>
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
    max-width: 860px;
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

  .title { font-weight: 700; font-size: 15px; }

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

  .info-bar {
    padding: 10px 20px;
    font-size: 11px;
    color: var(--muted);
    background: var(--surface2);
    border-bottom: 1px solid var(--border);
    line-height: 1.6;
  }

  .info-bar code {
    font-family: monospace;
    background: rgba(57, 208, 216, 0.1);
    color: var(--cyan);
    padding: 1px 4px;
    border-radius: 3px;
  }

  .msg-err {
    font-size: 12px;
    color: var(--red);
    background: rgba(224, 82, 82, 0.08);
    border: 1px solid rgba(224, 82, 82, 0.3);
    border-radius: 6px;
    padding: 6px 10px;
  }

  .msg-err.padded { margin: 12px 20px; }
  .msg-err.inline { white-space: nowrap; }

  .msg-ok {
    font-size: 12px;
    color: var(--green);
    font-weight: 600;
  }

  .table-wrap {
    overflow-x: auto;
    flex: 1;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }

  th {
    text-align: center;
    padding: 8px 6px;
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    background: var(--surface2);
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
  }

  th.th-sel { text-align: left; padding-left: 12px; min-width: 180px; }

  td {
    padding: 6px 4px;
    border-bottom: 1px solid rgba(48, 54, 61, 0.5);
    text-align: center;
  }

  td:first-child { padding-left: 8px; }

  tr:last-child td { border-bottom: none; }

  tr:hover td { background: rgba(255,255,255,0.02); }

  .cell-input {
    background: transparent;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--text);
    padding: 4px 6px;
    font-size: 12px;
    outline: none;
    transition: border-color 0.12s, background 0.12s;
    width: 100%;
  }

  .cell-input:hover { border-color: var(--border); }
  .cell-input:focus { border-color: var(--cyan); background: var(--surface2); }

  .sel-input { text-align: left; min-width: 160px; font-family: monospace; }
  .num-input { text-align: center; width: 60px; }

  /* hide browser number spinners */
  .num-input::-webkit-inner-spin-button,
  .num-input::-webkit-outer-spin-button { -webkit-appearance: none; }
  .num-input { -moz-appearance: textfield; }

  .sum-cell {
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    font-size: 11px;
  }

  .del-btn {
    background: none;
    border: 1px solid transparent;
    color: var(--muted);
    cursor: pointer;
    font-size: 11px;
    padding: 3px 7px;
    border-radius: 4px;
    transition: color 0.15s, border-color 0.15s;
  }

  .del-btn:hover { color: var(--red); border-color: rgba(224, 82, 82, 0.4); }

  .footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 20px;
    border-top: 1px solid var(--border);
    position: sticky;
    bottom: 0;
    background: var(--surface);
    gap: 12px;
  }

  .footer-right {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .add-btn {
    background: none;
    border: 1px solid var(--border);
    color: var(--muted);
    cursor: pointer;
    font-size: 12px;
    padding: 7px 14px;
    border-radius: 7px;
    transition: color 0.15s, border-color 0.15s;
  }

  .add-btn:hover { color: var(--cyan); border-color: var(--cyan); }

  .save-btn {
    background: var(--cyan);
    color: #0d1117;
    border: none;
    border-radius: 7px;
    padding: 8px 20px;
    font-size: 13px;
    font-weight: 700;
    cursor: pointer;
    transition: opacity 0.15s;
  }

  .save-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .save-btn:hover:not(:disabled) { opacity: 0.85; }
</style>

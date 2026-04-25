<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'
  import { WIDGET_TYPES } from '../widgets/index.js'

  export let widget = null      // null = new widget, object = edit existing
  export let onSave = () => {}
  export let onClose = () => {}

  const KPI_OPTIONS = ['stress','fatigue','mood','pressure','humidity','contagion',
                       'resilience','entropy','velocity','health_score']

  let form = {
    type:      'timeseries',
    title:     '',
    metric:    '',
    kpi:       '',
    host:      '',
    threshold: '',
    horizon:   60,
    severity:  '',
    label_key: 'src_ip',
    limit:     10,
    max:       '',
    w:         1,
    h:         1,
  }

  let metricList = []

  onMount(async () => {
    if (widget) {
      form = { ...form, ...widget }
      if (typeof form.threshold === 'number') form.threshold = String(form.threshold)
    }
    try {
      const res = await api.metrics()
      metricList = Object.keys(res?.data || {})
    } catch { metricList = [] }
  })

  $: typeInfo = WIDGET_TYPES[form.type] ?? WIDGET_TYPES.stat
  $: fields   = typeInfo?.fields ?? []

  function save() {
    const out = { ...form }
    if (out.threshold !== '') out.threshold = parseFloat(out.threshold) || 0
    else delete out.threshold
    if (!out.max) delete out.max
    if (!out.host) delete out.host
    onSave(out)
  }
</script>

<div class="modal-overlay" on:click|self={onClose}>
  <div class="modal">
    <div class="modal-header">
      <h3>{widget ? 'Edit Widget' : 'Add Widget'}</h3>
      <button class="close-btn" on:click={onClose}>✕</button>
    </div>

    <div class="modal-body">
      <!-- Widget type -->
      <label class="field">
        <span>Type</span>
        <select bind:value={form.type}>
          {#each Object.entries(WIDGET_TYPES) as [key, meta]}
            <option value={key}>{meta.icon} {meta.label}</option>
          {/each}
        </select>
      </label>

      <!-- Title -->
      <label class="field">
        <span>Title</span>
        <input type="text" bind:value={form.title} placeholder={typeInfo.label} />
      </label>

      {#if fields.includes('metric')}
        <label class="field">
          <span>Metric</span>
          <input list="metric-list" type="text" bind:value={form.metric} placeholder="e.g. cpu_percent" />
          <datalist id="metric-list">
            {#each metricList as m}<option value={m}></option>{/each}
          </datalist>
        </label>
      {/if}

      {#if fields.includes('kpi')}
        <label class="field">
          <span>KPI</span>
          <select bind:value={form.kpi}>
            <option value="">— none —</option>
            {#each KPI_OPTIONS as k}<option value={k}>{k}</option>{/each}
          </select>
        </label>
      {/if}

      {#if fields.includes('host')}
        <label class="field">
          <span>Host</span>
          <input type="text" bind:value={form.host} placeholder="hostname or empty for default" />
        </label>
      {/if}

      {#if fields.includes('threshold')}
        <label class="field">
          <span>Threshold</span>
          <input type="number" step="any" bind:value={form.threshold} placeholder="optional alert threshold" />
        </label>
      {/if}

      {#if fields.includes('horizon')}
        <label class="field">
          <span>Forecast horizon (min)</span>
          <input type="number" min="10" max="1440" bind:value={form.horizon} />
        </label>
      {/if}

      {#if fields.includes('severity')}
        <label class="field">
          <span>Severity filter</span>
          <select bind:value={form.severity}>
            <option value="">All</option>
            <option value="info">Info</option>
            <option value="warning">Warning</option>
            <option value="critical">Critical</option>
            <option value="emergency">Emergency</option>
          </select>
        </label>
      {/if}

      {#if fields.includes('max')}
        <label class="field">
          <span>Max value</span>
          <input type="number" step="any" bind:value={form.max} placeholder="auto" />
        </label>
      {/if}

      {#if fields.includes('label_key')}
        <label class="field">
          <span>Label key</span>
          <input type="text" bind:value={form.label_key} placeholder="e.g. src_ip, service" />
        </label>
      {/if}

      {#if fields.includes('limit')}
        <label class="field">
          <span>Top N</span>
          <input type="number" min="1" max="50" bind:value={form.limit} />
        </label>
      {/if}

      <!-- Grid size -->
      <div class="field-row">
        <label class="field half">
          <span>Width (columns)</span>
          <select bind:value={form.w}>
            <option value={1}>1</option>
            <option value={2}>2</option>
            <option value={3}>3</option>
          </select>
        </label>
        <label class="field half">
          <span>Height (rows)</span>
          <select bind:value={form.h}>
            <option value={1}>1</option>
            <option value={2}>2</option>
          </select>
        </label>
      </div>
    </div>

    <div class="modal-footer">
      <button class="cancel-btn" on:click={onClose}>Cancel</button>
      <button class="save-btn" on:click={save}>Save Widget</button>
    </div>
  </div>
</div>

<style>
  .modal-overlay {
    position: fixed; inset: 0; background: rgba(0,0,0,0.7);
    display: flex; align-items: center; justify-content: center; z-index: 200;
  }
  .modal {
    background: #1e293b; border: 1px solid #334155; border-radius: 12px;
    width: 480px; max-width: 95vw; max-height: 90vh;
    display: flex; flex-direction: column; box-shadow: 0 16px 48px rgba(0,0,0,0.5);
  }
  .modal-header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 16px 20px; border-bottom: 1px solid #334155;
  }
  .modal-header h3 { margin: 0; font-size: 1rem; color: #e2e8f0; }
  .close-btn { background: none; border: none; color: #64748b; font-size: 1.1rem; cursor: pointer; }
  .close-btn:hover { color: #e2e8f0; }

  .modal-body { padding: 16px 20px; overflow-y: auto; display: flex; flex-direction: column; gap: 12px; }

  .field { display: flex; flex-direction: column; gap: 4px; }
  .field span { font-size: 0.78rem; color: #64748b; }
  .field input, .field select {
    background: #0f172a; border: 1px solid #334155; border-radius: 6px;
    color: #e2e8f0; padding: 7px 10px; font-size: 0.85rem;
  }
  .field input:focus, .field select:focus { border-color: #38bdf8; outline: none; }

  .field-row { display: flex; gap: 12px; }
  .field.half { flex: 1; }

  .modal-footer {
    display: flex; justify-content: flex-end; gap: 8px;
    padding: 14px 20px; border-top: 1px solid #334155;
  }
  .cancel-btn {
    background: transparent; border: 1px solid #334155; color: #94a3b8;
    padding: 7px 16px; border-radius: 6px; cursor: pointer; font-size: 0.85rem;
  }
  .save-btn {
    background: #0284c7; border: none; color: #fff;
    padding: 7px 18px; border-radius: 6px; cursor: pointer; font-size: 0.85rem;
    font-weight: 600;
  }
  .save-btn:hover { background: #0369a1; }
</style>

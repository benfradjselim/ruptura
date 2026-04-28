<script>
  import { onMount } from 'svelte'
  import { api } from '../api.js'

  export let widget = {}       // {label_key, limit, from, title}
  export let refreshTick = 0

  let items  = []
  let max    = 1
  let loading = true
  let error   = ''

  async function load() {
    loading = true; error = ''
    try {
      const res = await api.logs({ limit: 1000, from: widget.from || '' })
      const raw = res?.data || []
      const labelKey = widget.label_key || 'src_ip'
      const counts = {}
      for (const entry of raw) {
        let labels = {}
        try { labels = typeof entry === 'string' ? JSON.parse(entry) : entry }
        catch { continue }
        const v = labels[labelKey] || (labels.labels && labels.labels[labelKey]) || ''
        if (v) counts[v] = (counts[v] || 0) + 1
      }
      const sorted = Object.entries(counts).sort((a, b) => b[1] - a[1]).slice(0, widget.limit || 10)
      items = sorted.map(([k, v]) => ({ key: k, count: v }))
      max   = items[0]?.count || 1
    } catch (e) { error = e.message }
    finally { loading = false }
  }

  onMount(load)
  $: if (refreshTick) load()
</script>

{#if loading}
  <div class="topn-msg">loading…</div>
{:else if error}
  <div class="topn-msg err">{error}</div>
{:else if !items.length}
  <div class="topn-msg">No data</div>
{:else}
  <div class="topn-list">
    {#each items as item, i}
      <div class="topn-row">
        <span class="topn-rank">#{i+1}</span>
        <span class="topn-key">{item.key}</span>
        <div class="topn-bar-wrap">
          <div class="topn-bar" style="width: {(item.count/max*100).toFixed(1)}%"></div>
        </div>
        <span class="topn-count">{item.count}</span>
      </div>
    {/each}
  </div>
{/if}

<style>
  .topn-msg  { text-align: center; padding: 12px; color: #475569; font-size: 0.8rem; }
  .err       { color: #ef4444; }
  .topn-list { display: flex; flex-direction: column; gap: 4px; }
  .topn-row  { display: flex; align-items: center; gap: 6px; font-size: 0.78rem; }
  .topn-rank { width: 20px; color: #475569; }
  .topn-key  { width: 90px; color: #e2e8f0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .topn-bar-wrap { flex: 1; background: #1e293b; border-radius: 2px; height: 8px; }
  .topn-bar      { background: #ef4444; height: 100%; border-radius: 2px; transition: width 0.3s; }
  .topn-count { width: 30px; text-align: right; color: #64748b; }
</style>

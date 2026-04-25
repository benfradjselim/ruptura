<script>
  export let data = []   // array of numbers
  export let color = '#38bdf8'
  export let height = 40
  export let width = 120

  $: points = (() => {
    if (data.length < 2) return ''
    const min = Math.min(...data)
    const max = Math.max(...data)
    const range = max - min || 1
    return data.map((v, i) => {
      const x = (i / (data.length - 1)) * width
      const y = height - ((v - min) / range) * height
      return `${x},${y}`
    }).join(' ')
  })()
</script>

{#if points}
  <svg viewBox="0 0 {width} {height}" style="width:{width}px;height:{height}px">
    <polyline fill="none" stroke={color} stroke-width="1.5" points={points}/>
  </svg>
{:else}
  <svg viewBox="0 0 {width} {height}" style="width:{width}px;height:{height}px">
    <line x1="0" y1={height/2} x2={width} y2={height/2} stroke="#334155" stroke-width="1"/>
  </svg>
{/if}

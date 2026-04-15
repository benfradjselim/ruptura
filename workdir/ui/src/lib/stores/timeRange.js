import { writable, derived } from 'svelte/store'

// Preset durations in minutes
export const PRESETS = [
  { label: 'Last 5m',  value: 5   },
  { label: 'Last 15m', value: 15  },
  { label: 'Last 1h',  value: 60  },
  { label: 'Last 6h',  value: 360 },
  { label: 'Last 24h', value: 1440 },
  { label: 'Last 7d',  value: 10080 },
  { label: 'Last 30d', value: 43200 },
]

// {preset: number|null, from: Date|null, to: Date|null}
// preset=null means custom from/to
export const timeRange = writable({ preset: 60, from: null, to: null })

// Derived: {from: Date, to: Date} — always absolute
export const absRange = derived(timeRange, ($tr) => {
  const to = $tr.to || new Date()
  const from = $tr.from || (
    $tr.preset
      ? new Date(to.getTime() - $tr.preset * 60_000)
      : new Date(to.getTime() - 60 * 60_000)
  )
  return { from, to }
})

// Returns ISO string pair for API calls: {from: string, to: string}
export function getRangeParams(range) {
  return {
    from: range.from.toISOString(),
    to:   range.to.toISOString(),
  }
}

// Returns a "from" relative offset string compatible with /metrics/{name}?from=
// e.g. "-60m", "-1h", "-24h"
export function toFromParam($tr) {
  if ($tr.preset) {
    const m = $tr.preset
    if (m < 60)   return `-${m}m`
    if (m < 1440) return `-${Math.round(m/60)}h`
    return `-${Math.round(m/1440)}d`
  }
  // custom: return ISO
  return $tr.from ? $tr.from.toISOString() : '-1h'
}

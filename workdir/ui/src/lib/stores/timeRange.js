import { writable, derived } from 'svelte/store'

// Preset durations in minutes — past
export const PRESETS = [
  { label: 'Last 5m',  value: 5     },
  { label: 'Last 15m', value: 15    },
  { label: 'Last 1h',  value: 60    },
  { label: 'Last 6h',  value: 360   },
  { label: 'Last 24h', value: 1440  },
  { label: 'Last 7d',  value: 10080 },
  { label: 'Last 30d', value: 43200 },
]

// Preset durations in minutes — future (forecast horizon)
export const FUTURE_PRESETS = [
  { label: 'Next 30m', value: 30    },
  { label: 'Next 1h',  value: 60    },
  { label: 'Next 6h',  value: 360   },
  { label: 'Next 24h', value: 1440  },
  { label: 'Next 7d',  value: 10080 },
]

// {preset: number|null, from: Date|null, to: Date|null, future: boolean}
// future=true  → preset is the forecast horizon in minutes
// future=false → preset is how far back to look
export const timeRange = writable({ preset: 60, from: null, to: null, future: false })

// Derived: {from: Date, to: Date} — always absolute
export const absRange = derived(timeRange, ($tr) => {
  const now  = new Date()
  if ($tr.future) {
    return { from: now, to: new Date(now.getTime() + ($tr.preset ?? 60) * 60_000) }
  }
  const to   = $tr.to   || now
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

// Returns a relative offset string for /metrics/{name}?from=
// Past: "-60m", "-1h", "-24h"
// Future: not used for historical fetch — returns "-1h" as safe fallback
export function toFromParam($tr) {
  if ($tr.future) return '-1h'   // fetch last 1h of history for context
  if ($tr.preset) {
    const m = $tr.preset
    if (m < 60)   return `-${m}m`
    if (m < 1440) return `-${Math.round(m / 60)}h`
    return `-${Math.round(m / 1440)}d`
  }
  return $tr.from ? $tr.from.toISOString() : '-1h'
}

// Returns forecast horizon in minutes when in future mode, else 60
export function toHorizon($tr) {
  return $tr.future ? ($tr.preset ?? 60) : 60
}

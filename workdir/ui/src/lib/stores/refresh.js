import { writable } from 'svelte/store'

// Auto-refresh intervals in seconds (0 = off)
export const INTERVALS = [
  { label: 'Off',  value: 0  },
  { label: '5s',   value: 5  },
  { label: '10s',  value: 10 },
  { label: '30s',  value: 30 },
  { label: '1m',   value: 60 },
  { label: '5m',   value: 300 },
]

export const refreshInterval = writable(0)  // seconds; 0 = paused
export const refreshTick     = writable(0)  // incremented on each refresh

let _timer = null

refreshInterval.subscribe((interval) => {
  if (_timer) { clearInterval(_timer); _timer = null }
  if (interval > 0) {
    _timer = setInterval(() => {
      refreshTick.update(n => n + 1)
    }, interval * 1000)
  }
})

export function manualRefresh() {
  refreshTick.update(n => n + 1)
}

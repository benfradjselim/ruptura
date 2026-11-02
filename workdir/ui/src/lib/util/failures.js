export function formatEta(seconds) {
  if (!seconds) return '—'
  if (seconds < 60) return '< 1 min'
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `${minutes} min`
  return `${Math.floor(minutes / 60)} h ${minutes % 60} min`
}

const eta = failure => failure.etaSeconds || 0

export function sortFailures(failures) {
  const rank = state => (state === 'critical' ? 0 : 1)
  return [...failures].sort((a, b) => rank(a.state) - rank(b.state) || eta(a) - eta(b))
}

export function missingRoles(inputs) {
  return inputs.filter(input => !input.available).map(input => input.role)
}

const RECENT_MS = 15 * 60 * 1000

export function recentAnomalies(anomalies, now) {
  return anomalies.filter(a => !a.acknowledged && now - Date.parse(a.timestamp) <= RECENT_MS)
}

export function measuredSignal(workload, key, value) {
  return (workload.unavailable_signals || []).includes(key) ? null : value
}

export function horizonPlacement(failure, horizonSeconds) {
  const seconds = failure.etaSeconds || 0
  return { position: Math.min(100, (seconds / horizonSeconds) * 100), beyond: seconds > horizonSeconds }
}

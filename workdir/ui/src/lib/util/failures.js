export function formatEta(seconds) {
  if (!seconds) return '—'
  if (seconds < 60) return '< 1 min'
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `${minutes} min`
  return `${Math.floor(minutes / 60)} h ${minutes % 60} min`
}

const eta = failure => failure.etaSeconds || Number.MAX_SAFE_INTEGER

export function sortFailures(failures) {
  const rank = state => (state === 'critical' ? 0 : 1)
  return [...failures].sort((a, b) => rank(a.state) - rank(b.state) || eta(a) - eta(b))
}

export function missingRoles(inputs) {
  return inputs.filter(input => !input.available).map(input => input.role)
}

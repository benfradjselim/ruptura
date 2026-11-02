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
  return { position: 50 + Math.min(50, (seconds / horizonSeconds) * 50), beyond: seconds > horizonSeconds }
}

// Failures closer than CLUSTER_GAP percent of the axis share one marker, so
// their labels never overlap. A label opens to the left past RIGHT_EDGE, or
// when another label follows within LABEL_SPAN and none precedes within it.
const CLUSTER_GAP = 12
const RIGHT_EDGE = 75
const LABEL_SPAN = 50
const LANES = 2

export function horizonClusters(failures, horizonSeconds) {
  const placed = failures
    .map(failure => ({ failure, ...horizonPlacement(failure, horizonSeconds) }))
    .sort((a, b) => a.position - b.position)
  const clusters = []
  for (const item of placed) {
    const last = clusters[clusters.length - 1]
    if (last && item.position - last.position < CLUSTER_GAP) {
      last.failures.push(item.failure)
      last.beyond = last.beyond || item.beyond
      continue
    }
    clusters.push({ position: item.position, beyond: item.beyond, failures: [item.failure] })
  }
  return clusters.map((cluster, i) => ({
    ...cluster,
    lane: i % LANES,
    alignRight: cluster.position > RIGHT_EDGE || opensLeft(clusters, i),
    state: cluster.failures.some(f => f.state === 'critical') ? 'critical' : 'warning',
  }))
}

function opensLeft(clusters, i) {
  const next = clusters[i + 1], prev = clusters[i - 1]
  const crowdedRight = next && next.position - clusters[i].position < LABEL_SPAN
  const freeLeft = !prev || clusters[i].position - prev.position >= LABEL_SPAN
  return Boolean(crowdedRight && freeLeft)
}

export function pastPlacement(minutesAgo, windowMinutes) {
  return Math.max(0, 50 - (minutesAgo / windowMinutes) * 50)
}

export function relatedEvents(events, target) {
  return events.filter(event => event.target === target).sort((a, b) => a.minutesAgo - b.minutesAgo)
}

export function cardSummary(workload) {
  const failure = workload.failure
  if (!failure) return { title: 'No known failure', when: '', facts: [] }
  return {
    title: failure.failureMode.replace(/_/g, ' '),
    when: failure.kind === 'predicted' && failure.etaSeconds ? `in ${formatEta(failure.etaSeconds)}` : 'now',
    facts: failure.evidence || [],
  }
}

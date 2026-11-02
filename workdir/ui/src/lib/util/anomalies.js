const METRICS = {
  'k8s.pod.network.io': { label: 'Network traffic', kind: 'rate' },
  'k8s.pod.memory_limit_utilization': { label: 'Memory used of limit', kind: 'ratio' },
  'k8s.pod.cpu_limit_utilization': { label: 'CPU used of limit', kind: 'ratio' },
  'k8s.pod.cpu.utilization': { label: 'CPU in use', kind: 'cores' },
  'k8s.pod.cpu.time': { label: 'CPU time', kind: 'cores' },
}

const RECENT_MINUTES = 60

const GROUPS = {
  network: 'Network', controlplane: 'Control plane', node: 'Nodes', storage: 'Storage',
  admission: 'Admission', tenancy: 'Tenancy', operators: 'Operators', co: 'Cluster operators', mcp: 'Machine config',
}

export function metricInfo(metric) {
  return METRICS[metric] || { label: metric, kind: 'plain' }
}

function bytes(value) {
  if (value >= 1e6) return `${(value / 1048576).toFixed(1)} MB/s`
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KB/s`
  return `${Math.round(value)} B/s`
}

export function formatMeasure(metric, value) {
  switch (metricInfo(metric).kind) {
    case 'rate': return bytes(value)
    case 'ratio': return `${Math.round(value * 100)}%`
    case 'cores': return `${value.toFixed(2)} cores`
  }
  return Number(value).toFixed(2)
}

function comparison(value, expected) {
  if (!expected) return ''
  const ratio = value / expected
  if (ratio >= 1.5) return `${Math.round(ratio)}× usual`
  if (ratio <= 0.67) return ratio <= 0.55 && ratio >= 0.45 ? 'half of usual' : `${Math.round(ratio * 100)}% of usual`
  return 'near usual'
}

export function anomalyRows(anomalies, now) {
  return anomalies
    .map(a => ({ ...a, minutesAgo: Math.round((now - Date.parse(a.timestamp)) / 60000) }))
    .filter(a => a.minutesAgo >= 0 && a.minutesAgo <= RECENT_MINUTES)
    .sort((a, b) => a.minutesAgo - b.minutesAgo)
    .map(a => {
      const parts = a.host.split('/')
      return {
        name: parts[parts.length - 1],
        scope: parts[0],
        metric: metricInfo(a.metric).label,
        observed: formatMeasure(a.metric, a.value),
        usual: formatMeasure(a.metric, a.expected),
        change: comparison(a.value, a.expected),
        minutesAgo: a.minutesAgo,
        severity: a.severity,
      }
    })
}

export function pressuredNamespaces(propagation) {
  const kept = {}
  for (const [ns, data] of Object.entries(propagation.namespaces || {})) {
    const sources = Object.values(data.propPressure || {})
    if ((data.workloadPressure || 0) > 0 || sources.some(v => v > 0)) kept[ns] = data
  }
  return { ...propagation, namespaces: kept }
}

export function groupName(group) {
  const key = group.replace(/^grp\./, '')
  return GROUPS[key] || group
}

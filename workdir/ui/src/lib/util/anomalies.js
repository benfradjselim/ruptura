const GROUPS = {
  network: 'Network', controlplane: 'Control plane', node: 'Nodes', storage: 'Storage',
  admission: 'Admission', tenancy: 'Tenancy', operators: 'Operators', co: 'Cluster operators', mcp: 'Machine config',
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

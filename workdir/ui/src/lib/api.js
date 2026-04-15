// OHE API client — thin wrapper around fetch
const BASE = '/api/v1'

let _token = localStorage.getItem('ohe_token') || ''

export function setToken(t) {
  _token = t
  if (t) localStorage.setItem('ohe_token', t)
  else localStorage.removeItem('ohe_token')
}

export function getToken() { return _token }

async function req(method, path, body) {
  const headers = { 'Content-Type': 'application/json' }
  if (_token) headers['Authorization'] = 'Bearer ' + _token
  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body != null ? JSON.stringify(body) : undefined,
  })
  const json = await res.json().catch(() => ({}))
  if (!res.ok) throw Object.assign(new Error(json?.error?.message || res.statusText), { status: res.status, json })
  return json
}

export const api = {
  // ── Auth ───────────────────────────────────────────────────────────────────
  setup:   (username, password) => req('POST', '/auth/setup', { username, password }),
  login:   (username, password) => req('POST', '/auth/login', { username, password }),
  logout:  ()                   => req('POST', '/auth/logout'),
  refresh: ()                   => req('POST', '/auth/refresh'),

  // ── Health ─────────────────────────────────────────────────────────────────
  health: () => req('GET', '/health'),

  // ── KPIs ───────────────────────────────────────────────────────────────────
  kpis:      (host)       => req('GET', '/kpis' + (host ? '?host=' + encodeURIComponent(host) : '')),
  kpi:       (name, host) => req('GET', `/kpis/${name}` + (host ? '?host=' + encodeURIComponent(host) : '')),
  kpisMulti: (hosts)      => { const q = hosts.map(h => 'host=' + encodeURIComponent(h)).join('&'); return req('GET', '/kpis/multi' + (q ? '?' + q : '')) },

  // ── Metrics ────────────────────────────────────────────────────────────────
  metrics:     (host)              => req('GET', '/metrics' + (host ? '?host=' + encodeURIComponent(host) : '')),
  metricRange: (name, host, from)  =>
    req('GET', `/metrics/${name}?host=${encodeURIComponent(host)}&from=${encodeURIComponent(from)}`),
  // Full time-range version: from/to are ISO strings or relative offset
  metricRangeFull: (name, host, from, to) => {
    let path = `/metrics/${name}?host=${encodeURIComponent(host)}&from=${encodeURIComponent(from)}`
    if (to) path += `&to=${encodeURIComponent(to)}`
    return req('GET', path)
  },

  // ── Alerts ─────────────────────────────────────────────────────────────────
  alerts:      ()    => req('GET', '/alerts'),
  alertGet:    (id)  => req('GET', `/alerts/${id}`),
  alertDelete: (id)  => req('DELETE', `/alerts/${id}`),
  alertAck:    (id)  => req('POST', `/alerts/${id}/acknowledge`),
  alertSilence:(id)  => req('POST', `/alerts/${id}/silence`),

  // ── Alert Rules ────────────────────────────────────────────────────────────
  alertRules:       ()             => req('GET', '/alert-rules'),
  alertRuleCreate:  (rule)         => req('POST', '/alert-rules', rule),
  alertRuleUpdate:  (name, rule)   => req('PUT', `/alert-rules/${encodeURIComponent(name)}`, rule),
  alertRuleDelete:  (name)         => req('DELETE', `/alert-rules/${encodeURIComponent(name)}`),

  // ── Dashboards ─────────────────────────────────────────────────────────────
  dashboards:      ()       => req('GET', '/dashboards'),
  dashboardCreate: (d)      => req('POST', '/dashboards', d),
  dashboardGet:    (id)     => req('GET', `/dashboards/${id}`),
  dashboardUpdate: (id, d)  => req('PUT', `/dashboards/${id}`, d),
  dashboardDelete: (id)     => req('DELETE', `/dashboards/${id}`),
  dashboardExport: (id)     => req('GET', `/dashboards/${id}/export`),
  dashboardImport: (json)   => req('POST', '/dashboards/import', json),

  // ── Templates ──────────────────────────────────────────────────────────────
  templates:     ()   => req('GET', '/templates'),
  templateApply: (id) => req('POST', `/templates/${id}/apply`),

  // ── DataSources ────────────────────────────────────────────────────────────
  datasources:       ()         => req('GET', '/datasources'),
  datasourceCreate:  (ds)       => req('POST', '/datasources', ds),
  datasourceGet:     (id)       => req('GET', `/datasources/${id}`),
  datasourceUpdate:  (id, ds)   => req('PUT', `/datasources/${id}`, ds),
  datasourceDelete:  (id)       => req('DELETE', `/datasources/${id}`),
  datasourceTest:    (id)       => req('POST', `/datasources/${id}/test`),

  // ── Users (admin) ──────────────────────────────────────────────────────────
  users:      ()         => req('GET', '/auth/users'),
  userCreate: (u)        => req('POST', '/auth/users', u),
  userDelete: (username) => req('DELETE', `/auth/users/${username}`),

  // ── Predict ────────────────────────────────────────────────────────────────
  predict: (host, metric, horizon) => {
    let path = `/predict?host=${encodeURIComponent(host)}&horizon=${horizon}`
    if (metric) path += `&metric=${encodeURIComponent(metric)}`
    return req('GET', path)
  },

  // ── Fleet ──────────────────────────────────────────────────────────────────
  fleet: () => req('GET', '/fleet'),

  // ── Notifications ──────────────────────────────────────────────────────────
  notifications:        ()         => req('GET', '/notifications'),
  notificationCreate:   (ch)       => req('POST', '/notifications', ch),
  notificationUpdate:   (id, ch)   => req('PUT', `/notifications/${id}`, ch),
  notificationDelete:   (id)       => req('DELETE', `/notifications/${id}`),
  notificationTest:     (id)       => req('POST', `/notifications/${id}/test`),

  // ── Topology ───────────────────────────────────────────────────────────────
  topology: () => req('GET', '/topology'),

  // ── Logs ───────────────────────────────────────────────────────────────────
  logs: ({ service = '', severity = '', q = '', from = '', to = '', limit = 200 } = {}) => {
    const params = new URLSearchParams()
    if (service)  params.set('service', service)
    if (severity) params.set('severity', severity)
    if (q)        params.set('q', q)
    if (from)     params.set('from', from)
    if (to)       params.set('to', to)
    params.set('limit', String(limit))
    return req('GET', '/logs?' + params.toString())
  },

  /** Returns an EventSource for live log streaming */
  logStream: ({ service = '', severity = '', q = '' } = {}) => {
    const params = new URLSearchParams()
    if (service)  params.set('service', service)
    if (severity) params.set('severity', severity)
    if (q)        params.set('q', q)
    const url = BASE + '/logs/stream?' + params.toString()
    // Append auth token as query param since EventSource doesn't support headers
    const full = _token ? url + '&token=' + encodeURIComponent(_token) : url
    return new EventSource(full)
  },

  // ── Traces ─────────────────────────────────────────────────────────────────
  traceSearch: ({ service = '', limit = 50 } = {}) => {
    const params = new URLSearchParams()
    if (service) params.set('service', service)
    params.set('limit', String(limit))
    return req('GET', '/traces?' + params.toString())
  },
  traceGet: (traceID) => req('GET', `/traces/${encodeURIComponent(traceID)}`),
}

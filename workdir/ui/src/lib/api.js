// OHE API client — thin wrapper around fetch
const BASE = '/api/v1'

let _token = localStorage.getItem('ohe_token') || ''

export function setToken(t) {
  _token = t
  if (t) localStorage.setItem('ohe_token', t)
  else localStorage.removeItem('ohe_token')
}

export function getToken() {
  return _token
}

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
  // Auth
  setup: (username, password) => req('POST', '/auth/setup', { username, password }),
  login: (username, password) => req('POST', '/auth/login', { username, password }),
  logout: () => req('POST', '/auth/logout'),
  refresh: () => req('POST', '/auth/refresh'),

  // Health
  health: () => req('GET', '/health'),

  // KPIs
  kpis: (host) => req('GET', '/kpis' + (host ? '?host=' + encodeURIComponent(host) : '')),
  kpi: (name, host) => req('GET', `/kpis/${name}` + (host ? '?host=' + encodeURIComponent(host) : '')),

  // Metrics
  metrics: (host) => req('GET', '/metrics' + (host ? '?host=' + encodeURIComponent(host) : '')),
  metricRange: (name, host, from) =>
    req('GET', `/metrics/${name}?host=${encodeURIComponent(host)}&from=${encodeURIComponent(from)}`),

  // Alerts
  alerts: () => req('GET', '/alerts'),
  alertGet: (id) => req('GET', `/alerts/${id}`),
  alertDelete: (id) => req('DELETE', `/alerts/${id}`),
  alertAck: (id) => req('POST', `/alerts/${id}/acknowledge`),
  alertSilence: (id) => req('POST', `/alerts/${id}/silence`),

  // Dashboards
  dashboards: () => req('GET', '/dashboards'),
  dashboardCreate: (d) => req('POST', '/dashboards', d),
  dashboardGet: (id) => req('GET', `/dashboards/${id}`),
  dashboardUpdate: (id, d) => req('PUT', `/dashboards/${id}`, d),
  dashboardDelete: (id) => req('DELETE', `/dashboards/${id}`),

  // Templates
  templates: () => req('GET', '/templates'),
  templateApply: (id) => req('POST', `/templates/${id}/apply`),

  // Users (admin)
  users: () => req('GET', '/auth/users'),
  userCreate: (u) => req('POST', '/auth/users', u),
  userDelete: (username) => req('DELETE', `/auth/users/${username}`),

  // Predict
  predict: (host, metric, horizon) =>
    req('GET', `/predict?host=${encodeURIComponent(host)}&metric=${encodeURIComponent(metric)}&horizon=${horizon}`),
}

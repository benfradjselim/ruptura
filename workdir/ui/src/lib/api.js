// Ruptura API client — thin wrapper around fetch
const BASE = '/api/v2'

let _token = localStorage.getItem('ruptura_token') || window.__RUPTURA_KEY__ || ''

export function setToken(t) {
  _token = t
  if (t) localStorage.setItem('ruptura_token', t)
  else localStorage.removeItem('ruptura_token')
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
  engineStatus: () => req('GET', '/engine/status'),

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

  // ── Orgs ────────────────────────────────────────────────────────────────────
  orgs:       ()         => req('GET', '/orgs'),
  orgCreate:  (o)        => req('POST', '/orgs', o),
  orgGet:     (id)       => req('GET', `/orgs/${id}`),
  orgUpdate:  (id, o)    => req('PUT', `/orgs/${id}`, o),
  orgDelete:  (id)       => req('DELETE', `/orgs/${id}`),

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
  failures: () => req('GET', '/failures'),
  callGraph: () => req('GET', '/callgraph'),
  failureClosest: () => req('GET', '/failures/closest'),
  failureSeries: (workload) => req('GET', `/failures/series?workload=${encodeURIComponent(workload)}`),
  inputs: () => req('GET', '/inputs'),
  trackRecord: () => req('GET', '/track-record'),
  coverage: () => req('GET', '/coverage'),
  eventHistory: () => req('GET', '/event-history'),

  // ── History ────────────────────────────────────────────────────────────────
  history: (workload, from, to) => {
    let path = workload ? `/history/${encodeURIComponent(workload)}` : '/history'
    const params = new URLSearchParams()
    if (from) params.set('from', from)
    if (to)   params.set('to', to)
    const qs = params.toString()
    return req('GET', path + (qs ? '?' + qs : ''))
  },

  // ── Ruptures ───────────────────────────────────────────────────────────────
  ruptures:        ()       => req('GET', '/ruptures'),
  rupture:         (host)   => req('GET', `/rupture/${encodeURIComponent(host)}`),
  ruptureHistory:  (host)   => req('GET', `/rupture/${encodeURIComponent(host)}/history`),
  ruptureProfile:  (host)   => req('GET', `/rupture/${encodeURIComponent(host)}/profile`),

  // ── Forecast ───────────────────────────────────────────────────────────────
  forecastWorkload: (metric, host) =>
    req('GET', `/forecast/${encodeURIComponent(metric)}/${encodeURIComponent(host)}`),

  // ── Workload summary (forecast-as-hero headline) ────────────────────────────
  workloadSummary: (ns, kind, name) =>
    req('GET', `/workloads/${encodeURIComponent(ns)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}/summary`),

  // ── Notifications ──────────────────────────────────────────────────────────
  notifications:        ()         => req('GET', '/notifications'),
  notificationCreate:   (ch)       => req('POST', '/notifications', ch),
  notificationUpdate:   (id, ch)   => req('PUT', `/notifications/${id}`, ch),
  notificationDelete:   (id)       => req('DELETE', `/notifications/${id}`),
  notificationTest:     (id)       => req('POST', `/notifications/${id}/test`),

  // ── Topology ───────────────────────────────────────────────────────────────
  topology: () => req('GET', '/topology'),
  dataflow: () => req('GET', '/dataflow'),

  // ── Infra ──────────────────────────────────────────────────────────────────
  infraGroups:    () => req('GET', '/infra/groups'),
  infraNodes:     () => req('GET', '/infra/nodes'),
  infraNetwork:   () => req('GET', '/infra/network'),
  infraStorage:   () => req('GET', '/infra/storage'),
  infraAdmission: () => req('GET', '/infra/admission'),
  infraTenancy:   () => req('GET', '/infra/tenancy'),
  propagation:    () => req('GET', '/propagation'),
}

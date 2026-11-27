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

  // ── Failures ───────────────────────────────────────────────────────────────
  fleet:          () => req('GET', '/fleet'),
  failures:       () => req('GET', '/failures'),
  failureClosest: () => req('GET', '/failures/closest'),
  failureSeries:  (workload) => req('GET', `/failures/series?workload=${encodeURIComponent(workload)}`),
  callGraph:      () => req('GET', '/callgraph'),
  inputs:         () => req('GET', '/inputs'),
  trackRecord:    () => req('GET', '/track-record'),
  coverage:       () => req('GET', '/coverage'),
  eventHistory:   () => req('GET', '/event-history'),

  // ── Alert rules ────────────────────────────────────────────────────────────
  alertRules:       ()             => req('GET', '/alert-rules'),
  alertRuleCreate:  (rule)         => req('POST', '/alert-rules', rule),
  alertRuleUpdate:  (name, rule)   => req('PUT', `/alert-rules/${encodeURIComponent(name)}`, rule),
  alertRuleDelete:  (name)         => req('DELETE', `/alert-rules/${encodeURIComponent(name)}`),

  // ── Notifications ──────────────────────────────────────────────────────────
  notifications:        ()         => req('GET', '/notifications'),
  notificationCreate:   (ch)       => req('POST', '/notifications', ch),
  notificationUpdate:   (id, ch)   => req('PUT', `/notifications/${id}`, ch),
  notificationDelete:   (id)       => req('DELETE', `/notifications/${id}`),
  notificationTest:     (id)       => req('POST', `/notifications/${id}/test`),

  // ── Orgs ───────────────────────────────────────────────────────────────────
  orgs:       ()         => req('GET', '/orgs'),
  orgCreate:  (o)        => req('POST', '/orgs', o),
  orgGet:     (id)       => req('GET', `/orgs/${id}`),
  orgUpdate:  (id, o)    => req('PUT', `/orgs/${id}`, o),
  orgDelete:  (id)       => req('DELETE', `/orgs/${id}`),

  // ── Users (admin) ──────────────────────────────────────────────────────────
  users:      ()         => req('GET', '/auth/users'),
  userCreate: (u)        => req('POST', '/auth/users', u),
  userDelete: (username) => req('DELETE', `/auth/users/${username}`),

  // ── Infra ──────────────────────────────────────────────────────────────────
  infraGroups:    () => req('GET', '/infra/groups'),
  infraNodes:     () => req('GET', '/infra/nodes'),
  infraNetwork:   () => req('GET', '/infra/network'),
  infraStorage:   () => req('GET', '/infra/storage'),
  infraAdmission: () => req('GET', '/infra/admission'),
  infraTenancy:   () => req('GET', '/infra/tenancy'),
  propagation:    () => req('GET', '/propagation'),
}

// api.ts — typed client for all Ruptura v2 REST endpoints.
// The UI pod proxies /api/ to the Go backend via nginx, so paths are relative.

export interface KPI {
  name: string
  value: number
  state: string   // "ok" | "warning" | "critical"
  trend: string   // "rising" | "falling" | "stable"
}

export interface KPIMap {
  stress: KPI
  fatigue: KPI
  mood: KPI
  pressure: KPI
  humidity: KPI
  contagion: KPI
  resilience: KPI
  entropy: KPI
  velocity: KPI
  health_score: KPI
}

export interface WorkloadRef {
  namespace: string
  kind: string
  name: string
  node: string
}

export interface HealthForecast {
  trend: 'stable' | 'improving' | 'degrading'
  in_15min: number
  in_30min: number
  critical_eta_minutes: number
  confidence_window: number
}

export interface FleetHost {
  host: string
  state: 'healthy' | 'degraded' | 'critical' | 'pending_telemetry'
  health_score: number
  stress: number
  fatigue: number
  contagion: number
  active_alerts: number
  last_seen: string
  fused_rupture_index: number
  health_forecast?: HealthForecast
}

export interface FleetResponse {
  total_hosts: number
  healthy_hosts: number
  degraded_hosts: number
  critical_hosts: number
  hosts: FleetHost[]
}

export interface KPISnapshot {
  host: string
  workload: WorkloadRef
  timestamp: string
  workload_status: string
  fused_rupture_index: number
  health_score: KPI
  stress: KPI
  fatigue: KPI
  mood: KPI
  pressure: KPI
  humidity: KPI
  contagion: KPI
  resilience: KPI
  entropy: KPI
  velocity: KPI
}

export interface HealthResponse {
  status: string
  version: string
  uptime_seconds: number
  storage: { status: string }
  ingest: { metrics: number; logs: number; traces: number }
}

export interface Alert {
  id: string
  host: string
  metric: string
  value: number
  severity: string
  message: string
  created_at: string
  resolved_at: string | null
}

// v7 planned endpoints (returned as null until implemented server-side)
export interface TopologyNode {
  id: string
  health_score: number
  fused_r: number
  state: string
}

export interface TopologyEdge {
  source: string
  target: string
  call_rate: number
  error_rate: number
  p99_latency_ms: number
}

export interface TopologyResponse {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

export interface EngineStatus {
  analyzer: {
    tick_interval_ms: number
    last_tick_ago_ms: number
    active_workloads: number
    calibrating_workloads: number
  }
  ingest: {
    metrics_per_sec: number
    logs_per_sec: number
    traces_per_sec: number
  }
  actions: {
    pending_tier1: number
    pending_tier2: number
    executed_last_hour: number
  }
  version: string
  edition: string
  uptime_seconds: number
}

export interface ClusterNode {
  name: string
  cpu_pct: number
  memory_pct: number
  disk_pressure: boolean
  workload_count: number
  worst_fused_r: number
}

export interface Suppression {
  id: string
  workload: string
  start: string
  end: string
  reason: string
}

export interface CreateSuppressionRequest {
  workload: string
  start: string   // ISO 8601
  end: string     // ISO 8601
  reason: string
}

// 6-signal weight override for a workload selector glob.
export interface SignalWeights {
  selector: string
  stress: number
  fatigue: number
  mood: number
  pressure: number
  humidity: number
  contagion: number
}

// ── fetch helpers ────────────────────────────────────────────────────────────

async function get<T>(path: string, apiKey?: string): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (apiKey) headers['Authorization'] = `Bearer ${apiKey}`
  const res = await fetch(path, { headers })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText} — ${path}`)
  return res.json() as Promise<T>
}

// ── public API ───────────────────────────────────────────────────────────────

export function fetchHealth(apiKey?: string) {
  return get<HealthResponse>('/api/v2/health', apiKey)
}

export function fetchFleet(apiKey?: string) {
  return get<FleetResponse>('/api/v2/fleet', apiKey)
}

export function fetchKPIs(host: string, apiKey?: string) {
  return get<KPIMap>(`/api/v2/kpis?host=${encodeURIComponent(host)}`, apiKey)
}

export function fetchSnapshot(host: string, apiKey?: string) {
  return get<KPISnapshot>(`/api/v2/kpi?host=${encodeURIComponent(host)}`, apiKey)
}

export function fetchAlerts(apiKey?: string) {
  return get<Alert[]>('/api/v2/alerts', apiKey)
}

export function fetchTopology(apiKey?: string) {
  return get<TopologyResponse>('/api/v2/topology', apiKey)
}

export function fetchEngineStatus(apiKey?: string) {
  return get<EngineStatus>('/api/v2/engine/status', apiKey)
}

export function fetchNodes(apiKey?: string) {
  return get<ClusterNode[]>('/api/v2/nodes', apiKey)
}

// ── write helpers ────────────────────────────────────────────────────────────

async function post<T>(path: string, body: unknown, apiKey?: string): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (apiKey) headers['Authorization'] = `Bearer ${apiKey}`
  const res = await fetch(path, { method: 'POST', headers, body: JSON.stringify(body) })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status} — ${text}`)
  }
  return res.json() as Promise<T>
}

async function del(path: string, apiKey?: string): Promise<void> {
  const headers: Record<string, string> = {}
  if (apiKey) headers['Authorization'] = `Bearer ${apiKey}`
  const res = await fetch(path, { method: 'DELETE', headers })
  if (!res.ok && res.status !== 204) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status} — ${text}`)
  }
}

// ── suppressions ─────────────────────────────────────────────────────────────

export function fetchSuppressions(apiKey?: string) {
  return get<Suppression[]>('/api/v2/suppressions', apiKey)
}

export function createSuppression(req: CreateSuppressionRequest, apiKey?: string) {
  return post<Suppression>('/api/v2/suppressions', req, apiKey)
}

export function deleteSuppression(id: string, apiKey?: string) {
  return del(`/api/v2/suppressions/${encodeURIComponent(id)}`, apiKey)
}

// ── signal weights ────────────────────────────────────────────────────────────

export function fetchWeights(apiKey?: string) {
  return get<SignalWeights[]>('/api/v2/config/weights', apiKey)
}

export function saveWeights(weights: SignalWeights[], apiKey?: string) {
  return post<{ applied: number }>('/api/v2/config/weights', weights, apiKey)
}

// ── topology ──────────────────────────────────────────────────────────────────

export interface TopologyNode {
  id: string
  health_score: number
  fused_r: number
  state: 'healthy' | 'degraded' | 'critical' | 'pending_telemetry'
}

export interface TopologyEdge {
  source: string
  target: string
  call_rate: number
  error_rate: number
  p99_latency_ms: number
}

export interface TopologyGraph {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

export function fetchTopology(apiKey?: string) {
  return get<TopologyGraph>('/api/v2/topology', apiKey)
}

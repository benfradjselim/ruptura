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
  throughput: KPI
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

export interface PatternMatch {
  similarity: number
  matched_rupture_id: string
  matched_at: string
  resolution: string
}

export interface BusinessSignals {
  slo_burn_velocity: number
  blast_radius: number
  recovery_debt: number
}

export interface FleetHost {
  host: string
  state: 'healthy' | 'degraded' | 'critical' | 'pending_telemetry' | 'calibrating'
  health_score: number
  stress: number
  fatigue: number
  contagion: number
  active_alerts: number
  last_seen: string
  fused_rupture_index: number
  health_forecast?: HealthForecast
  calibration_progress?: number
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
  status: string           // "calibrating" | "active" | "pending_telemetry"
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
  throughput: KPI
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

export interface EngineStatus {
  analyzer: {
    tick_interval_ms: number
    last_tick_ago_ms: number
    active_workloads: number
    calibrating_workloads: number
    pending_workloads: number
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
  start: string
  end: string
  reason: string
}

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

function safeJson<T>(text: string, path: string): T {
  const trimmed = text.trim()
  if (!trimmed || trimmed === 'null') return null as unknown as T
  try {
    return JSON.parse(trimmed) as T
  } catch {
    throw new Error(`Invalid JSON from ${path}: ${trimmed.slice(0, 120)}`)
  }
}

// get — object endpoint; returns parsed value (null if body is empty/null)
async function get<T>(path: string): Promise<T> {
  const res = await fetch(path)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText} — ${path}`)
  const text = await res.text()
  return safeJson<T>(text, path)
}

// getArray — list endpoint; always returns an array, never null
async function getArray<T>(path: string): Promise<T[]> {
  const res = await fetch(path)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText} — ${path}`)
  const text = await res.text()
  const parsed = safeJson<T[] | null>(text, path)
  return Array.isArray(parsed) ? parsed : []
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status} — ${text}`)
  }
  const text = await res.text()
  return safeJson<T>(text, path)
}

async function del(path: string): Promise<void> {
  const res = await fetch(path, { method: 'DELETE' })
  if (!res.ok && res.status !== 204) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status} — ${text}`)
  }
}

// ── health / version ─────────────────────────────────────────────────────────

export function fetchHealth() {
  return get<HealthResponse>('/api/v2/health')
}

// ── fleet ────────────────────────────────────────────────────────────────────

export async function fetchFleet(): Promise<FleetResponse> {
  const r = await get<FleetResponse | null>('/api/v2/fleet')
  return r ?? { total_hosts: 0, healthy_hosts: 0, degraded_hosts: 0, critical_hosts: 0, hosts: [] }
}

// ── kpis ─────────────────────────────────────────────────────────────────────

export function fetchKPIs(host: string) {
  return get<KPIMap>(`/api/v2/kpis?host=${encodeURIComponent(host)}`)
}

// ── alerts ────────────────────────────────────────────────────────────────────

export function fetchAlerts() {
  return getArray<Alert>('/api/v2/alerts')
}

// ── engine status / storage ───────────────────────────────────────────────────

export function fetchEngineStatus() {
  return get<EngineStatus>('/api/v2/engine/status')
}

export interface EngineStorage {
  badger: {
    disk_bytes: number
    vlog_size_bytes: number
    num_tables: number
    keys: number
  }
}

export function fetchEngineStorage() {
  return get<EngineStorage>('/api/v2/engine/storage')
}

// ── topology ─────────────────────────────────────────────────────────────────

export interface TopologyNode {
  id: string
  label: string
  namespace: string
  kind: string
  health_score: number
  fused_r: number
  state: string
  stress: number
  fatigue: number
  contagion: number
  mood: number
  velocity: number
  entropy: number
}

export interface TopologyEdge {
  source: string
  target: string
  call_rate: number
  error_rate: number
  p99_latency_ms: number
  edge_type: 'trace' | 'inferred'
  strength: number
}

export interface TopologyGraph {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

export async function fetchTopology(): Promise<TopologyGraph> {
  const r = await get<TopologyGraph | null>('/api/v2/topology')
  return r ?? { nodes: [], edges: [] }
}

// ── nodes ────────────────────────────────────────────────────────────────────

export function fetchNodes() {
  return getArray<ClusterNode>('/api/v2/nodes')
}

export interface NodeWorkload {
  ref: string
  health_score: number
  fused_r: number
  status: string
}

export interface NodeDetail {
  name: string
  cpu_pct: number
  memory_pct: number
  disk_pressure: boolean
  workload_count: number
  worst_fused_r: number
  workloads: NodeWorkload[]
}

export function fetchNodeDetail(node: string) {
  return get<NodeDetail>(`/api/v2/nodes/${encodeURIComponent(node)}`)
}

// ── suppressions ─────────────────────────────────────────────────────────────

export function fetchSuppressions() {
  return getArray<Suppression>('/api/v2/suppressions')
}

export function createSuppression(req: CreateSuppressionRequest) {
  return post<Suppression>('/api/v2/suppressions', req)
}

export function deleteSuppression(id: string) {
  return del(`/api/v2/suppressions/${encodeURIComponent(id)}`)
}

// ── signal weights ────────────────────────────────────────────────────────────

export function fetchWeights() {
  return getArray<SignalWeights>('/api/v2/config/weights')
}

export function saveWeights(weights: SignalWeights[]) {
  return post<{ applied: number }>('/api/v2/config/weights', weights)
}

// ── workload k8s metadata ─────────────────────────────────────────────────────

export interface PodInfo {
  name: string
  node: string
  status: string
  restarts: number
  age: string
}

export interface ResourceSet {
  cpu: string
  memory: string
}

export interface WorkloadK8sMeta {
  namespace: string
  kind: string
  name: string
  replicas: { desired: number; ready: number; available: number }
  image: string
  resources: { requests: ResourceSet; limits: ResourceSet }
  labels: Record<string, string>
  last_deploy: string
  pods: PodInfo[]
}

export function fetchWorkloadK8s(namespace: string, kind: string, name: string) {
  return get<WorkloadK8sMeta>(
    `/api/v2/workloads/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}/k8s`,
  )
}

// ── history / time-series ────────────────────────────────────────────────────

export interface HistoryPoint {
  ts: string
  health_score: number
  fused_r: number
  stress: number
  fatigue: number
  mood: number
  pressure: number
  humidity: number
  contagion: number
  resilience: number
  entropy: number
  velocity: number
  throughput: number
}

export function fetchHistory(wlRef: string) {
  return getArray<HistoryPoint>(`/api/v2/history/${encodeURIComponent(wlRef)}`)
}

// ── actions ───────────────────────────────────────────────────────────────────

export interface Action {
  id: string
  host: string
  tier: number
  kind: string
  description: string
  created_at: string
  state: 'pending' | 'approved' | 'rejected' | 'executed'
}

export function fetchActions() {
  return getArray<Action>('/api/v2/actions')
}

export function approveAction(id: string) {
  return post<{ ok: boolean }>(`/api/v2/actions/${encodeURIComponent(id)}/approve`, {})
}

export function rejectAction(id: string) {
  return post<{ ok: boolean }>(`/api/v2/actions/${encodeURIComponent(id)}/reject`, {})
}

export function emergencyStop() {
  return post<{ ok: boolean }>('/api/v2/actions/emergency-stop', {})
}

// ── explain ───────────────────────────────────────────────────────────────────

export type ExplainMode = 'narrative' | 'formula' | 'pipeline'

export interface ExplainResult {
  narrative?: string
  formula?: string
  [key: string]: unknown
}

export function fetchExplain(ruptureId: string, mode: ExplainMode) {
  return get<ExplainResult>(
    `/api/v2/explain/${encodeURIComponent(ruptureId)}/${mode}`,
  )
}

// ── ruptures (full snapshot list) ────────────────────────────────────────────

export interface RuptureSnapshot {
  host: string
  workload: WorkloadRef
  timestamp: string
  status: string              // "calibrating" | "active" | "pending_telemetry"
  calibration_progress: number
  calibration_eta_minutes: number
  fused_rupture_index: number
  health_score: KPI
  health_forecast?: HealthForecast
  rupture_events?: Array<{ id: string; ts: string; severity: string }>
  pattern_match?: PatternMatch
  business?: BusinessSignals
  stress: KPI
  fatigue: KPI
  mood: KPI
  pressure: KPI
  humidity: KPI
  contagion: KPI
  resilience: KPI
  entropy: KPI
  velocity: KPI
  throughput: KPI
}

export function fetchRuptures() {
  return getArray<RuptureSnapshot>('/api/v2/ruptures')
}

// ── predictions ───────────────────────────────────────────────────────────────

export interface PredictionEntry {
  target: string
  current: number
  predicted: number
  trend: 'stable' | 'improving' | 'degrading' | 'rising' | 'falling'
  horizon_minutes: number
}

export interface PredictResponse {
  predictions: PredictionEntry[]
}

export function fetchPredictions(host: string, horizon = 120) {
  return get<PredictResponse>(
    `/api/v2/predict?host=${encodeURIComponent(host)}&horizon=${horizon}`,
  )
}

export interface ForecastPoint {
  offset_minutes: number
  mean: number
  lower_80: number
  upper_80: number
  lower_95: number
  upper_95: number
}

export interface ForecastResult {
  host: string
  metric: string
  current: number
  trend: string
  confidence: number
  warming_up?: boolean
  points: ForecastPoint[]
  timestamp: string
}

export function fetchForecast(host: string, metric: string, horizon = 1440) {
  return get<ForecastResult>(
    `/api/v2/forecast/${encodeURIComponent(metric)}/${encodeURIComponent(host)}?horizon=${horizon}`,
  )
}

// ── logs ─────────────────────────────────────────────────────────────────────

export interface LogEntry {
  timestamp: string
  severity: string
  body: string
  service: string
  attributes: Record<string, string>
}

export function fetchLogs(service: string, fromMs?: number, toMs?: number, limit = 200) {
  const params = new URLSearchParams({ limit: String(limit) })
  if (service) params.set('service', service)
  if (fromMs)  params.set('from', String(fromMs))
  if (toMs)    params.set('to', String(toMs))
  return getArray<LogEntry>(`/api/v2/logs?${params}`)
}

// ── dataflow (ingest totals) ──────────────────────────────────────────────────

export interface DataflowStats {
  metrics: number
  logs: number
  traces: number
}

export function fetchDataflow() {
  return get<DataflowStats>('/api/v2/dataflow')
}

// ── fusion state ─────────────────────────────────────────────────────────────

export interface FusionState {
  [key: string]: unknown
}

export function fetchFusion(namespace: string, kind: string, name: string) {
  return get<FusionState>(
    `/api/v2/engine/fusion/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`,
  )
}

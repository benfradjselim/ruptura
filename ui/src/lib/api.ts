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

// ── stale-while-revalidate cache ─────────────────────────────────────────────
// Serves the last-known value immediately, then refreshes in background.
// GET-only endpoints benefit; POST/PUT/DELETE bypass the cache entirely.

interface CacheEntry { value: unknown; ts: number }
const _cache = new Map<string, CacheEntry>()
const CACHE_TTL_MS = 30_000

function cacheGet<T>(key: string): T | undefined {
  const e = _cache.get(key)
  if (!e) return undefined
  if (Date.now() - e.ts > CACHE_TTL_MS * 3) { _cache.delete(key); return undefined }
  return e.value as T
}
function cacheSet(key: string, value: unknown) {
  _cache.set(key, { value, ts: Date.now() })
}
function cacheStale(key: string): boolean {
  const e = _cache.get(key)
  return !!e && Date.now() - e.ts > CACHE_TTL_MS
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

// get — object endpoint with stale-while-revalidate
async function get<T>(path: string): Promise<T> {
  const cached = cacheGet<T>(path)
  if (cached !== undefined && !cacheStale(path)) return cached
  const fetchFresh = async () => {
    const res = await fetch(path)
    if (!res.ok) throw new Error(`${res.status} ${res.statusText} — ${path}`)
    const val = safeJson<T>(await res.text(), path)
    cacheSet(path, val)
    return val
  }
  if (cached !== undefined) {
    // stale: return immediately, refresh in background
    fetchFresh().catch(() => {})
    return cached
  }
  return fetchFresh()
}

// getArray — list endpoint with stale-while-revalidate
async function getArray<T>(path: string): Promise<T[]> {
  const cached = cacheGet<T[]>(path)
  if (cached !== undefined && !cacheStale(path)) return cached
  const fetchFresh = async () => {
    const res = await fetch(path)
    if (!res.ok) throw new Error(`${res.status} ${res.statusText} — ${path}`)
    const parsed = safeJson<T[] | null>(await res.text(), path)
    const val = Array.isArray(parsed) ? parsed : []
    cacheSet(path, val)
    return val
  }
  if (cached !== undefined) {
    fetchFresh().catch(() => {})
    return cached
  }
  return fetchFresh()
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

async function put<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'PUT',
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

export async function fetchEngineStatus(): Promise<EngineStatus> {
  const r = await get<EngineStatus | null>('/api/v2/engine/status')
  return r ?? {
    analyzer: { tick_interval_ms: 0, last_tick_ago_ms: 0, active_workloads: 0, calibrating_workloads: 0, pending_workloads: 0 },
    ingest: { metrics_per_sec: 0, logs_per_sec: 0, traces_per_sec: 0 },
    actions: { pending_tier1: 0, pending_tier2: 0, executed_last_hour: 0 },
    version: '', edition: '', uptime_seconds: 0,
  }
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

export interface ModelContribution {
  name: string
  weight: number
  mean: number
}

export interface ForecastResult {
  host: string
  metric: string
  current: number
  trend: string
  confidence: number
  warming_up?: boolean
  points?: ForecastPoint[]
  models?: ModelContribution[]
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

// ── SSE event stream ──────────────────────────────────────────────────────────

export interface RuptureEvent {
  type: 'rupture' | 'recovery' | 'heartbeat'
  workload?: string
  fused_r?: number
  state?: string
  ts: string
}

export function fetchRecentEvents(limit = 50) {
  return getArray<RuptureEvent>(`/api/v2/events?limit=${limit}`)
}

// ── datasources ───────────────────────────────────────────────────────────────

export interface DatasourceConfig {
  id: string
  name: string
  type: 'prometheus' | 'direct_metrics' | 'otlp'
  url: string
  enabled: boolean
  scrape_interval_seconds: number
  workload_key: string
  namespace: string
  created_at: string
  updated_at: string
}

export interface DatasourceStatus extends DatasourceConfig {
  status: 'ok' | 'error' | 'pending' | 'disabled' | 'unknown'
  last_scrape: string
  last_error: string
  scraped_metrics: number
}

export interface CreateDatasourceRequest {
  name: string
  type: string
  url: string
  enabled: boolean
  scrape_interval_seconds?: number
  workload_key?: string
  namespace?: string
}

export function fetchDatasources() {
  return getArray<DatasourceStatus>('/api/v2/datasources')
}

export function createDatasource(req: CreateDatasourceRequest) {
  return post<DatasourceStatus>('/api/v2/datasources', req)
}

export function updateDatasource(id: string, req: CreateDatasourceRequest) {
  return put<DatasourceStatus>(`/api/v2/datasources/${encodeURIComponent(id)}`, req)
}

export function deleteDatasource(id: string) {
  return del(`/api/v2/datasources/${encodeURIComponent(id)}`)
}

export interface TestDatasourceResult {
  ok: boolean
  scraped_metrics?: number
  error?: string
}

export function testDatasource(req: CreateDatasourceRequest) {
  return post<TestDatasourceResult>('/api/v2/datasources/test', req)
}

export function testDatasourceById(id: string) {
  return post<TestDatasourceResult>(`/api/v2/datasources/${encodeURIComponent(id)}/test`, {})
}

// ── Database / Retention ──────────────────────────────────────────────────

export interface RetentionConfig {
  metrics_days:   number
  logs_days:      number
  traces_days:    number
  snapshots_days: number
}

export function fetchRetention(): Promise<RetentionConfig> {
  return get<RetentionConfig>('/api/v2/config/retention')
}

export function saveRetention(cfg: RetentionConfig): Promise<RetentionConfig> {
  return put<RetentionConfig>('/api/v2/config/retention', cfg)
}

export async function purgeData(type: string, before?: string): Promise<{ deleted: number | string }> {
  const params = new URLSearchParams()
  params.set('type', type)
  if (before) params.set('before', before)
  const res = await fetch(`/api/v2/ingest/purge?${params}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 204) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status} — ${text}`)
  }
  const json = await res.json().catch(() => ({})) as { data?: { deleted?: number | string }, deleted?: number | string }
  const d = json.data ?? json
  return { deleted: d.deleted ?? -1 }
}

export function openEventStream(
  onEvent: (e: RuptureEvent) => void,
  opts?: { namespace?: string; minFusedR?: number },
): EventSource {
  const params = new URLSearchParams()
  if (opts?.namespace) params.set('namespace', opts.namespace)
  if (opts?.minFusedR != null) params.set('min_fused_r', String(opts.minFusedR))
  const es = new EventSource(`/api/v2/events?${params}`)
  es.onmessage = (ev) => {
    try {
      const data = JSON.parse(ev.data) as RuptureEvent
      onEvent(data)
    } catch { /* ignore malformed */ }
  }
  return es
}

// Shapes of the engine read API, as served under /api/v2.

export type FailureKind = 'predicted' | 'detected';

/** One failure the engine names, from GET /api/v2/failures. */
export interface Failure {
  level: string;
  target: string;
  /** namespace/Kind/name */
  workload: string;
  pod?: string;
  failureMode: string;
  kind: FailureKind;
  state: string;
  etaSeconds?: number;
  etaEarlySeconds?: number;
  etaLateSeconds?: number;
  confidence?: number;
  recommendedAction?: string;
  evidence?: string[];
  /** Workloads this failure can take down, as namespace/Kind/name. */
  affects?: string[];
}

/** One memory reading of a pod, as a share of its limit. */
export interface SeriesPoint {
  t: number;
  usage: number;
}

/** Memory readings of one pod, from GET /api/v2/failures/series. */
export interface PodSeries {
  pod: string;
  points: SeriesPoint[];
}

/** Response of GET /api/v2/failures/series?workload=... */
export interface FailureSeries {
  workload: string;
  pods: PodSeries[];
}

/** A workload reference parsed from namespace/Kind/name. */
export interface WorkloadRef {
  namespace: string;
  kind: string;
  name: string;
}

/** One pod of GET /api/v2/failures/closest: memory as a share of its limit. */
export interface ClosestPod {
  workload: string;
  /** namespace/pod */
  pod: string;
  usage: number;
  /** Change of usage per hour, as a share of the limit. */
  trendPerHour?: number;
}

/** What the plugin reads of GET /api/v2/health. */
export interface EngineHealth {
  version?: string;
  uptime_seconds?: number;
}

// Pure helpers of the Ruptures page: the 30 minute horizon, the engine
// warm-up, the pods closest to their limit and the filters. Kept in line with
// the Ruptura web console (workdir/ui/src/lib/util).
import { ClosestPod, Failure } from '../types';
import { formatEta } from './failures';
import { parseWorkload } from './workload';

/** Span of the horizon axis after now, in seconds. */
export const HORIZON_SECONDS = 30 * 60;

// Failures closer than CLUSTER_GAP percent of the axis share one marker.
const CLUSTER_GAP = 6;

export interface HorizonMarker {
  /** Position on the axis, 0 at now and 100 at the horizon. */
  position: number;
  /** True when a failure of the marker is beyond the horizon. */
  beyond: boolean;
  critical: boolean;
  failures: Failure[];
}

/** Places failures on the horizon: detected ones at now, forecasts at their time. */
export function horizonMarkers(failures: Failure[], horizonSeconds = HORIZON_SECONDS): HorizonMarker[] {
  const placed = failures
    .map((failure) => {
      const seconds = failure.kind === 'predicted' ? failure.etaSeconds || 0 : 0;
      return { failure, position: Math.min(100, (seconds / horizonSeconds) * 100), beyond: seconds > horizonSeconds };
    })
    .sort((a, b) => a.position - b.position);
  const markers: HorizonMarker[] = [];
  for (const item of placed) {
    const last = markers[markers.length - 1];
    if (last && item.position - last.position < CLUSTER_GAP) {
      last.failures.push(item.failure);
      last.beyond = last.beyond || item.beyond;
      last.critical = last.critical || item.failure.state === 'critical';
      continue;
    }
    markers.push({ position: item.position, beyond: item.beyond, critical: item.failure.state === 'critical', failures: [item.failure] });
  }
  return markers;
}

// Detected conditions come back within 5 minutes of a start, forecasts after
// up to 30 minutes of readings, sooner when the engine restored its state.
export const DETECTION_WARMUP_SECONDS = 5 * 60;
export const FORECAST_WARMUP_SECONDS = 30 * 60;

export interface Warmup {
  startedMinutes: number;
  detecting: boolean;
  /** Clock time by which forecasts are back, HH:MM. */
  forecastsBy: string;
}

/** The warm-up after an engine start, or null once it is over or unknown. */
export function warmup(uptimeSeconds: number | undefined, now: number): Warmup | null {
  if (uptimeSeconds === undefined || uptimeSeconds === null || uptimeSeconds >= FORECAST_WARMUP_SECONDS) return null;
  const by = new Date(now + (FORECAST_WARMUP_SECONDS - uptimeSeconds) * 1000);
  return {
    startedMinutes: Math.floor(uptimeSeconds / 60),
    detecting: uptimeSeconds >= DETECTION_WARMUP_SECONDS,
    forecastsBy: `${String(by.getHours()).padStart(2, '0')}:${String(by.getMinutes()).padStart(2, '0')}`,
  };
}

// A trend below this share of the limit per hour reads as flat.
const FLAT_TREND_PER_HOUR = 0.01;

export interface ClosestRow {
  pod: string;
  namespace: string;
  workload: string;
  /** Usage as a share of the limit, 0 to 1. */
  usage: number;
  share: string;
  trend: string;
  rising: boolean;
  reachesIn: string;
}

/** One row of the pods closest to their memory limit. */
export function closestRow(p: ClosestPod): ClosestRow {
  const [namespace, pod] = p.pod.includes('/') ? p.pod.split('/') : ['', p.pod];
  const trend = p.trendPerHour || 0;
  const rising = trend >= FLAT_TREND_PER_HOUR;
  let trendText = 'flat';
  if (rising) trendText = `+${Math.round(trend * 100)}%/h`;
  else if (trend <= -FLAT_TREND_PER_HOUR) trendText = `${Math.round(trend * 100)}%/h`;
  const left = 1 - p.usage;
  const reachesIn = rising && left > 0 ? `limit in ${formatEta(Math.round((left / trend) * 3600))} at this pace` : '';
  return { pod, namespace, workload: p.workload, usage: p.usage, share: `${Math.round(p.usage * 100)}%`, trend: trendText, rising, reachesIn };
}

export type KindFilter = 'all' | 'predicted' | 'detected';

export interface FailureFilter {
  /** Words matched against workload, pod, failure and evidence. */
  text: string;
  /** Namespace, or "" for every namespace. */
  namespace: string;
  kind: KindFilter;
  criticalOnly: boolean;
}

export const NO_FILTER: FailureFilter = { text: '', namespace: '', kind: 'all', criticalOnly: false };

/** Failures that match every part of the filter. */
export function filterFailures(failures: Failure[], filter: FailureFilter): Failure[] {
  const words = filter.text.toLowerCase().split(/\s+/).filter(Boolean);
  return failures.filter((f) => {
    if (filter.kind !== 'all' && f.kind !== filter.kind) return false;
    if (filter.criticalOnly && f.state !== 'critical') return false;
    if (filter.namespace && parseWorkload(f.workload)?.namespace !== filter.namespace) return false;
    if (words.length === 0) return true;
    const haystack = [f.workload, f.pod || '', f.failureMode.replace(/_/g, ' '), ...(f.evidence || [])].join(' ').toLowerCase();
    return words.every((w) => haystack.includes(w));
  });
}

/** Namespaces of the failures, sorted. */
export function namespacesOf(failures: Failure[]): string[] {
  const names = new Set<string>();
  for (const f of failures) {
    const ref = parseWorkload(f.workload);
    if (ref) names.add(ref.namespace);
  }
  return [...names].sort();
}

/** "12 s ago", "3 min ago": how old the data on screen is. */
export function ageLabel(readAt: number, now: number): string {
  if (!readAt) return '';
  const seconds = Math.max(0, Math.round((now - readAt) / 1000));
  if (seconds < 60) return `${seconds} s ago`;
  return `${Math.floor(seconds / 60)} min ago`;
}

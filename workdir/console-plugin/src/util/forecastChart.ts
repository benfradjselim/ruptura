// Geometry of the forecast chart: memory of each pod as a share of its limit
// over the last 30 minutes, and the engine's trend carried to the failure
// time. A port of workdir/ui/src/lib/util/forecastChart.js.
import { Failure, PodSeries, SeriesPoint } from '../types';

const MINUTE = 60 * 1000;
const PAST_MS = 30 * MINUTE;
const MIN_FUTURE_MS = 10 * MINUTE;
const MAX_FUTURE_MS = 3 * 60 * MINUTE;
const FUTURE_MARGIN = 1.15;
const LIMIT = 1;
const TOP_HEADROOM = 1.1;
const FLOOR_MARGIN = 0.1;

export const TREND_MODES = ['saturation', 'replica_divergence', 'release_regression'];
const CHART_MODES = [...TREND_MODES, 'recurrent_failure'];

/** Whether a failure comes with a memory chart: a forecast of a memory mode. */
export function hasForecastChart(failure: Pick<Failure, 'failureMode' | 'kind'>): boolean {
  return failure.kind === 'predicted' && CHART_MODES.includes(failure.failureMode);
}

export interface Padding {
  left: number;
  right: number;
  top: number;
  bottom: number;
}

export type ChartFailure = Pick<Failure, 'failureMode' | 'pod' | 'etaSeconds' | 'etaEarlySeconds' | 'etaLateSeconds'>;

export interface GeometryInput {
  pods: PodSeries[];
  failure: ChartFailure;
  failureTimes?: number[];
  now: number;
  width: number;
  height: number;
  pad?: Padding;
}

export interface Tick {
  x?: number;
  y?: number;
  label: string;
}

export interface Line {
  pod: string;
  d: string;
  highlight: boolean;
}

export interface Projection {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

export interface Geometry {
  width: number;
  height: number;
  pad: Padding;
  lines: Line[];
  projection: Projection | null;
  band: string | null;
  range: { from: string; to: string | null } | null;
  limitY: number;
  nowX: number;
  eta: { x: number; label: string } | null;
  failures: number[];
  yTicks: { y: number; label: string }[];
  xTicks: { x: number; label: string }[];
  empty: boolean;
}

const DEFAULT_PAD: Padding = { left: 44, right: 14, top: 12, bottom: 22 };

const clamp = (v: number, lo: number, hi: number): number => Math.min(hi, Math.max(lo, v));

function futureSpan(etaMs: number): number {
  return etaMs > 0 ? clamp(etaMs * FUTURE_MARGIN, MIN_FUTURE_MS, MAX_FUTURE_MS) : MIN_FUTURE_MS;
}

function clock(t: number): string {
  return new Date(t).toTimeString().slice(0, 5);
}

function relative(ms: number): string {
  const minutes = Math.round(ms / MINUTE);
  if (minutes === 0) return 'Now';
  const sign = minutes > 0 ? '+' : '−';
  const abs = Math.abs(minutes);
  return abs < 60 ? `${sign}${abs} min` : `${sign}${Math.floor(abs / 60)} h${abs % 60 ? ` ${abs % 60}` : ''}`;
}

// Point of a straight trend from last that reaches the limit at failAt, at t.
function onTrend(last: SeriesPoint, failAt: number, t: number): number {
  return failAt === last.t ? LIMIT : last.usage + ((LIMIT - last.usage) * (t - last.t)) / (failAt - last.t);
}

// Area between the fastest and slowest trends the readings agree on, capped by
// the limit and the right edge. An unknown latest failure stays flat.
function bandPolygon(
  last: SeriesPoint,
  earlyAt: number,
  lateAt: number | null,
  t1: number,
  x: (t: number) => number,
  y: (u: number) => number,
): string {
  const end = (t: number): number => Math.min(t, t1);
  const pts: [number, number][] = [
    [last.t, last.usage],
    [end(earlyAt), onTrend(last, earlyAt, end(earlyAt))],
  ];
  if (lateAt === null) {
    if (earlyAt < t1) pts.push([t1, LIMIT]);
    pts.push([t1, last.usage]);
  } else {
    if (earlyAt < t1) pts.push([end(lateAt), LIMIT]);
    pts.push([end(lateAt), onTrend(last, lateAt, end(lateAt))]);
  }
  return pts.map(([t, u]) => `${x(t).toFixed(1)},${y(u).toFixed(1)}`).join(' ');
}

/** Computes everything the chart draws, in pixels of a width x height box. */
export function forecastGeometry({ pods, failure, failureTimes = [], now, width, height, pad = DEFAULT_PAD }: GeometryInput): Geometry {
  const etaMs = (failure.etaSeconds || 0) * 1000;
  const earlyMs = (failure.etaEarlySeconds || 0) * 1000;
  const lateMs = (failure.etaLateSeconds || 0) * 1000;
  const t0 = now - PAST_MS;
  const t1 = now + futureSpan(etaMs);
  const visible = pods.map((p) => ({ ...p, points: p.points.filter((pt) => pt.t >= t0 && pt.t <= now) }));
  const usages = visible.flatMap((p) => p.points.map((pt) => pt.usage));
  const lo = usages.length ? Math.max(0, Math.floor((Math.min(...usages, LIMIT) - FLOOR_MARGIN) * 10) / 10) : 0;
  const hi = Math.max(TOP_HEADROOM, ...usages);

  const x = (t: number): number => pad.left + ((t - t0) / (t1 - t0)) * (width - pad.left - pad.right);
  const y = (u: number): number => height - pad.bottom - ((u - lo) / (hi - lo)) * (height - pad.top - pad.bottom);
  const path = (points: SeriesPoint[]): string =>
    points.map((pt, i) => `${i ? 'L' : 'M'}${x(pt.t).toFixed(1)},${y(pt.usage).toFixed(1)}`).join(' ');

  const withPoints = visible.filter((p) => p.points.length > 0);
  const single = withPoints.length === 1;
  const lines: Line[] = withPoints
    .map((p) => ({ pod: p.pod, d: path(p.points), highlight: p.pod === failure.pod || (single && !failure.pod) }))
    .sort((a, b) => Number(a.highlight) - Number(b.highlight));

  const failAt = etaMs > 0 ? now + etaMs : null;
  let projection: Projection | null = null;
  let band: string | null = null;
  const lead = withPoints.find((p) => p.pod === failure.pod || (single && !failure.pod));
  if (failAt && TREND_MODES.includes(failure.failureMode) && lead) {
    const last = lead.points[lead.points.length - 1];
    const endT = Math.min(failAt, t1);
    const endU = onTrend(last, failAt, endT);
    projection = { x1: x(last.t), y1: y(last.usage), x2: x(endT), y2: y(endU) };
    if (earlyMs > 0) band = bandPolygon(last, now + earlyMs, lateMs > 0 ? now + lateMs : null, t1, x, y);
  }

  return {
    width,
    height,
    pad,
    lines,
    projection,
    band,
    range: band && earlyMs > 0 ? { from: clock(now + earlyMs), to: lateMs > 0 ? clock(now + lateMs) : null } : null,
    limitY: y(LIMIT),
    nowX: x(now),
    eta: failAt && failAt <= t1 ? { x: x(failAt), label: clock(failAt) } : null,
    failures: failureTimes.filter((t) => t >= t0 && t <= now).map((t) => x(t)),
    yTicks: [lo, (lo + LIMIT) / 2, LIMIT].map((u) => ({ y: y(u), label: `${Math.round(u * 100)}%` })),
    xTicks: [t0, now, t1].map((t) => ({ x: x(t), label: relative(t - now) })),
    empty: lines.length === 0,
  };
}

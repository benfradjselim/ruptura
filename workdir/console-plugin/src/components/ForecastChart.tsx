// Memory of each pod as a share of its limit over the last 30 minutes, with
// the trend Ruptura reads carried to the failure time. Same drawing as the
// Ruptura web console, in console colors.
import * as React from 'react';
import { PodSeries } from '../types';
import { ChartFailure, forecastGeometry, Padding } from '../util/forecastChart';
import { shortPod } from '../util/workload';
import './plugin.css';

const PAD: Padding = { left: 44, right: 14, top: 14, bottom: 24 };

interface ForecastChartProps {
  pods: PodSeries[];
  failure: ChartFailure;
  now: number;
  height?: number;
}

// Width of the figure, followed as the console layout changes.
function useWidth(ref: React.RefObject<HTMLElement>): number {
  const [width, setWidth] = React.useState(0);
  React.useEffect(() => {
    const el = ref.current;
    if (!el) return undefined;
    const measure = (): void => setWidth(el.clientWidth);
    measure();
    if (typeof ResizeObserver === 'undefined') return undefined;
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [ref]);
  return width;
}

export default function ForecastChart({ pods, failure, now, height = 220 }: ForecastChartProps): JSX.Element {
  const ref = React.useRef<HTMLElement>(null);
  const width = useWidth(ref);
  const g = width > 0 ? forecastGeometry({ pods, failure, now, width, height, pad: PAD }) : null;
  const siblings = g ? g.lines.filter((l) => !l.highlight).length : 0;
  const subject = failure.pod ? shortPod(failure.pod) : 'the workload';
  const anchor = (i: number, count: number): 'start' | 'middle' | 'end' => (i === 0 ? 'start' : i === count - 1 ? 'end' : 'middle');

  return (
    <figure className="ruptura-forecast" ref={ref}>
      {g && !g.empty && (
        <>
          <svg width={width} height={height} role="img" aria-label={`Memory of ${subject} as a share of its limit, last 30 minutes and forecast`}>
            <rect className="future" x={g.nowX} y={PAD.top} width={Math.max(0, width - PAD.right - g.nowX)} height={height - PAD.top - PAD.bottom} />
            {g.yTicks.map((tick) => (
              <g key={tick.label}>
                <line className="grid" x1={PAD.left} x2={width - PAD.right} y1={tick.y} y2={tick.y} />
                <text className="label" x={PAD.left - 6} y={tick.y + 4} textAnchor="end">
                  {tick.label}
                </text>
              </g>
            ))}
            <line className="limit" x1={PAD.left} x2={width - PAD.right} y1={g.limitY} y2={g.limitY} />
            <text className="limit-label" x={PAD.left + 6} y={g.limitY - 6}>
              memory limit
            </text>
            <line className="now" x1={g.nowX} x2={g.nowX} y1={PAD.top} y2={height - PAD.bottom} />
            {g.lines.map((line) => (
              <path key={line.pod} className={line.highlight ? 'series hot' : 'series'} d={line.d}>
                <title>{shortPod(line.pod)}</title>
              </path>
            ))}
            {g.band && <polygon className="band" points={g.band} />}
            {g.projection && <line className="projection" x1={g.projection.x1} y1={g.projection.y1} x2={g.projection.x2} y2={g.projection.y2} />}
            {g.eta && (
              <>
                <line className="eta" x1={g.eta.x} x2={g.eta.x} y1={PAD.top} y2={height - PAD.bottom} />
                <rect className="eta-dot" x={g.eta.x - 5} y={g.limitY - 5} width="10" height="10" transform={`rotate(45 ${g.eta.x} ${g.limitY})`} />
                <text className="eta-label" x={g.eta.x - 6} y={PAD.top + 12} textAnchor="end">
                  failure ~{g.eta.label}
                </text>
              </>
            )}
            {g.xTicks.map((tick, i) => (
              <text key={tick.label} className="label" x={tick.x} y={height - 6} textAnchor={anchor(i, g.xTicks.length)}>
                {tick.label}
              </text>
            ))}
          </svg>
          {g.range && (
            <p className="range">
              Failure expected between <strong>{g.range.from}</strong> and <strong>{g.range.to || 'later'}</strong>, most likely{' '}
              <strong>{g.eta ? g.eta.label : ''}</strong>
            </p>
          )}
          <figcaption>
            <span className="key hot" />
            {failure.pod ? shortPod(failure.pod) : 'workload'}
            {siblings > 0 && (
              <>
                <span className="key" />
                {siblings} other replica{siblings > 1 ? 's' : ''}
              </>
            )}
            {g.projection && (
              <>
                <span className="key dash" />
                trend Ruptura reads
              </>
            )}
            {g.band && (
              <>
                <span className="key area" />
                range of trends
              </>
            )}
          </figcaption>
        </>
      )}
      {g && g.empty && <p className="ruptura-fact">No memory readings for this workload yet.</p>}
    </figure>
  );
}

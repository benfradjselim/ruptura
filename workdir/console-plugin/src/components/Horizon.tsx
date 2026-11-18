// The next 30 minutes on one axis: what is failing now on the left, each
// forecast at its expected time, so the order of the trouble reads at once.
import * as React from 'react';
import { Failure } from '../types';
import { modeLabel } from '../util/failures';
import { horizonMarkers } from '../util/horizon';
import { parseWorkload } from '../util/workload';
import './plugin.css';

const TICKS = ['Now', '+10 min', '+20 min', '+30 min'];
const MAX_NAMES = 3;

const shortName = (f: Failure): string => parseWorkload(f.workload)?.name || f.workload;

export default function Horizon({ failures, onSelect }: { failures: Failure[]; onSelect: (f: Failure) => void }): JSX.Element {
  const markers = horizonMarkers(failures);
  return (
    <section className="ruptura-horizon" aria-label="Failures placed on the next 30 minutes">
      <div className="ruptura-horizon__track">
        <div className="ruptura-horizon__line" />
        {TICKS.map((tick, i) => (
          <span key={tick} className="ruptura-horizon__tick" style={{ left: `${(i / (TICKS.length - 1)) * 100}%` }}>
            {tick}
          </span>
        ))}
        {markers.map((m, i) => (
          <button
            type="button"
            key={`${m.position}-${i}`}
            className={`ruptura-horizon__marker${m.critical ? ' is-critical' : ''}${i % 2 ? ' is-low' : ''}${m.position > 70 ? ' opens-left' : ''}`}
            style={{ left: `${m.position}%` }}
            onClick={() => onSelect(m.failures[0])}
            title={m.failures.map((f) => `${shortName(f)}: ${modeLabel(f.failureMode)}`).join('\n')}
          >
            <span className="ruptura-horizon__dot" aria-hidden="true" />
            <span className="ruptura-horizon__label">
              {m.failures.slice(0, MAX_NAMES).map(shortName).join(', ')}
              {m.failures.length > MAX_NAMES && ` +${m.failures.length - MAX_NAMES}`}
              {m.beyond && ' →'}
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}

// The pods nearest to their memory limit, with their pace: what to watch
// before Ruptura names a failure.
import * as React from 'react';
import { ClosestRow } from '../util/horizon';
import { WorkloadLink } from './ui';
import './plugin.css';

export default function Closest({ rows }: { rows: ClosestRow[] }): JSX.Element {
  return (
    <section className="ruptura-card" aria-label="Pods closest to their memory limit">
      <header className="ruptura-card__head">
        <h2>Closest to their memory limit</h2>
        <span className="ruptura-muted">A forecast appears when a pod climbs steadily for 10 minutes.</span>
      </header>
      <table className="ruptura-closest">
        <thead>
          <tr>
            <th>Pod</th>
            <th>Workload</th>
            <th className="is-num">Of limit</th>
            <th className="is-num">Trend</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={`${row.namespace}/${row.pod}`} className={row.rising ? 'is-rising' : ''}>
              <td data-label="Pod">
                {row.pod}
                {row.reachesIn && <span className="ruptura-muted">{row.reachesIn}</span>}
              </td>
              <td data-label="Workload">
                <WorkloadLink workload={row.workload} />
              </td>
              <td data-label="Of limit" className="is-num">
                <span className="ruptura-meter" aria-hidden="true">
                  <span className="ruptura-meter__fill" style={{ width: `${Math.min(100, Math.round(row.usage * 100))}%` }} />
                </span>
                {row.share}
              </td>
              <td data-label="Trend" className="is-num">
                {row.trend}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

// The failures as rows: when, workload, failure with its first evidence, and
// the action. A row opens its details. On a narrow screen rows become cards.
import * as React from 'react';
import { Failure } from '../types';
import { etaRange, modeLabel, whenLabel } from '../util/failures';
import { shortPod } from '../util/workload';
import { KindBadge, severity, WorkloadLink } from './ui';
import './plugin.css';

interface FailureListProps {
  failures: Failure[];
  onSelect: (failure: Failure) => void;
  selected?: Failure | null;
  /** Hide the workload column on a page that is about one workload. */
  withWorkload?: boolean;
}

export const failureKey = (f: Failure): string => `${f.target}/${f.failureMode}`;

export default function FailureList({ failures, onSelect, selected, withWorkload = true }: FailureListProps): JSX.Element {
  const open = (f: Failure) => (e: React.KeyboardEvent | React.MouseEvent): void => {
    if ('key' in e && e.key !== 'Enter' && e.key !== ' ') return;
    if ((e.target as HTMLElement).closest('a')) return;
    e.preventDefault();
    onSelect(f);
  };
  return (
    <table className="ruptura-list" aria-label="Failures Ruptura names">
      <thead>
        <tr>
          <th className="ruptura-list__when">When</th>
          {withWorkload && <th>Workload</th>}
          <th>Failure</th>
          <th>Recommended action</th>
        </tr>
      </thead>
      <tbody>
        {failures.map((f) => {
          const isSelected = selected ? failureKey(selected) === failureKey(f) : false;
          return (
            <tr
              key={failureKey(f)}
              className={`ruptura-list__row is-${severity(f)}${isSelected ? ' is-selected' : ''}`}
              tabIndex={0}
              aria-selected={isSelected}
              onClick={open(f)}
              onKeyDown={open(f)}
            >
              <td className="ruptura-list__when" data-label="When">
                <span className="ruptura-list__eta">{whenLabel(f)}</span>
                <KindBadge failure={f} />
                {etaRange(f) && <span className="ruptura-muted">{etaRange(f)}</span>}
              </td>
              {withWorkload && (
                <td data-label="Workload">
                  <WorkloadLink workload={f.workload} />
                  {f.pod && <span className="ruptura-muted">pod {shortPod(f.pod)}</span>}
                </td>
              )}
              <td data-label="Failure">
                <span className="ruptura-list__mode">{modeLabel(f.failureMode)}</span>
                {(f.evidence || []).slice(0, 2).map((fact) => (
                  <span key={fact} className="ruptura-muted">
                    {fact}
                  </span>
                ))}
              </td>
              <td data-label="Action" className="ruptura-list__action">
                {f.recommendedAction || '—'}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

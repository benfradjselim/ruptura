// Search and filters of the Ruptures page.
import * as React from 'react';
import { FailureFilter, KindFilter, NO_FILTER } from '../util/horizon';
import './plugin.css';

const KINDS: [KindFilter, string][] = [
  ['all', 'All'],
  ['predicted', 'Predicted'],
  ['detected', 'Detected'],
];

interface ToolbarProps {
  filter: FailureFilter;
  namespaces: string[];
  shown: number;
  total: number;
  onChange: (filter: FailureFilter) => void;
}

export default function Toolbar({ filter, namespaces, shown, total, onChange }: ToolbarProps): JSX.Element {
  const set = (over: Partial<FailureFilter>): void => onChange({ ...filter, ...over });
  const filtered = shown !== total;
  return (
    <div className="ruptura-toolbar" role="search">
      <input
        className="ruptura-input ruptura-toolbar__search"
        type="search"
        placeholder="Search workload, pod or evidence"
        aria-label="Search failures"
        value={filter.text}
        onChange={(e) => set({ text: e.target.value })}
      />
      <select className="ruptura-input" aria-label="Namespace" value={filter.namespace} onChange={(e) => set({ namespace: e.target.value })}>
        <option value="">All namespaces</option>
        {namespaces.map((ns) => (
          <option key={ns} value={ns}>
            {ns}
          </option>
        ))}
      </select>
      <div className="ruptura-segmented" role="group" aria-label="Kind">
        {KINDS.map(([kind, label]) => (
          <button key={kind} type="button" className={filter.kind === kind ? 'is-active' : ''} aria-pressed={filter.kind === kind} onClick={() => set({ kind })}>
            {label}
          </button>
        ))}
      </div>
      <label className="ruptura-check">
        <input type="checkbox" checked={filter.criticalOnly} onChange={(e) => set({ criticalOnly: e.target.checked })} />
        Critical only
      </label>
      <span className="ruptura-muted ruptura-toolbar__count">
        {filtered ? `${shown} of ${total}` : `${total} failure${total === 1 ? '' : 's'}`}
        {filtered && (
          <button type="button" className="ruptura-link" onClick={() => onChange(NO_FILTER)}>
            Clear filters
          </button>
        )}
      </span>
    </div>
  );
}

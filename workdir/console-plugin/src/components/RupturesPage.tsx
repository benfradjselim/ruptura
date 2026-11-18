// Observe > Ruptures: every failure Ruptura names in the namespaces the user
// may see, on a 30 minute horizon, with filters, details and the pods closest
// to their limit.
import * as React from 'react';
import { K8sResourceCommon, useK8sWatchResource } from '@openshift-console/dynamic-plugin-sdk';
import { fetchClosest, fetchFailures, fetchHealth, usePolling } from '../api';
import { Failure } from '../types';
import { predictedFirst } from '../util/failures';
import { ageLabel, closestRow, FailureFilter, filterFailures, namespacesOf, NO_FILTER, warmup } from '../util/horizon';
import { parseWorkload, visibleFailures } from '../util/workload';
import Closest from './Closest';
import Drawer from './Drawer';
import FailureDetail from './FailureDetail';
import FailureList, { failureKey } from './FailureList';
import Horizon from './Horizon';
import Toolbar from './Toolbar';
import { Banner, Empty, Loading } from './ui';
import './plugin.css';

// Projects are the namespaces the console lists for the user, whatever their
// role, so the page shows exactly what the user may see.
const PROJECT = { group: 'project.openshift.io', version: 'v1', kind: 'Project' };
const MAX_CLOSEST = 8;

function Stat({ label, value, tone }: { label: string; value: number; tone?: 'critical' | 'quiet' }): JSX.Element {
  return (
    <div className={`ruptura-stat${tone ? ` is-${tone}` : ''}`}>
      <span className="ruptura-stat__value">{value}</span>
      <span className="ruptura-stat__label">{label}</span>
    </div>
  );
}

// Re-renders every few seconds so "updated 12 s ago" stays true.
function useNow(intervalMs: number): number {
  const [now, setNow] = React.useState(Date.now());
  React.useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(timer);
  }, [intervalMs]);
  return now;
}

export default function RupturesPage(): JSX.Element {
  const [projects, projectsLoaded, projectsError] = useK8sWatchResource<K8sResourceCommon[]>({ groupVersionKind: PROJECT, isList: true });
  const { data, error, loaded, readAt } = usePolling(fetchFailures, []);
  const closest = usePolling(fetchClosest, []);
  const health = usePolling(fetchHealth, []);
  const [filter, setFilter] = React.useState<FailureFilter>(NO_FILTER);
  const [selectedKey, setSelectedKey] = React.useState<string | null>(null);
  const now = useNow(5000);

  const namespaces = projectsLoaded && !projectsError ? projects.map((p) => p.metadata?.name || '') : null;
  const failures = predictedFirst(visibleFailures(data || [], namespaces));
  const shown = filterFailures(failures, filter);
  const selected = failures.find((f) => failureKey(f) === selectedKey) || null;
  const select = (f: Failure): void => setSelectedKey(failureKey(f));
  const close = React.useCallback(() => setSelectedKey(null), []);

  const allowed = new Set(namespaces || []);
  const closestRows = (closest.data || [])
    .filter((p) => allowed.has(parseWorkload(p.workload)?.namespace || ''))
    .slice(0, MAX_CLOSEST)
    .map(closestRow);
  const warm = warmup(health.data?.uptime_seconds, now);
  const predicted = failures.filter((f) => f.kind === 'predicted').length;
  const critical = failures.filter((f) => f.state === 'critical').length;
  const workloads = new Set(failures.map((f) => f.workload)).size;
  const ready = loaded && projectsLoaded;

  return (
    <div className="ruptura-page">
      <header className="ruptura-header">
        <div className="ruptura-header__text">
          <h1 className="ruptura-header__title">Ruptures</h1>
          <p className="ruptura-header__lead">Failures Ruptura predicts or detects, with the evidence and the action to take.</p>
        </div>
        <p className="ruptura-header__meta">
          {health.data?.version && <span>Engine {health.data.version}</span>}
          {readAt > 0 && <span>Updated {ageLabel(readAt, now)}</span>}
        </p>
      </header>

      <div className="ruptura-body">
        {error && <Banner variant="danger" title="Ruptura engine unreachable">{error}</Banner>}
        {projectsError && (
          <Banner variant="warning" title="Projects unavailable">
            The list of projects you can see could not be read, so no failure is shown.
          </Banner>
        )}
        {warm && (
          <Banner variant="info" title={`Engine started ${warm.startedMinutes} min ago`}>
            {warm.detecting ? 'Detected failures are back.' : 'Detected failures return within 5 minutes.'} Forecasts return by {warm.forecastsBy} at the latest.
            Until then, no forecast does not mean no risk.
          </Banner>
        )}
        {!ready && !error && <Loading label="Reading failures" />}

        {ready && (
          <>
            <div className="ruptura-stats">
              <Stat label="Critical" value={critical} tone={critical > 0 ? 'critical' : 'quiet'} />
              <Stat label="Predicted" value={predicted} />
              <Stat label="Detected" value={failures.length - predicted} />
              <Stat label="Workloads affected" value={workloads} />
            </div>

            {failures.length > 0 && <Horizon failures={shown} onSelect={select} />}

            {failures.length === 0 ? (
              <Empty title="No failure ahead">Ruptura names no failure in the projects you can see.</Empty>
            ) : (
              <section className="ruptura-card">
                <Toolbar filter={filter} namespaces={namespacesOf(failures)} shown={shown.length} total={failures.length} onChange={setFilter} />
                {shown.length === 0 ? (
                  <Empty title="No failure matches">Change or clear the filters.</Empty>
                ) : (
                  <FailureList failures={shown} onSelect={select} selected={selected} />
                )}
              </section>
            )}

            {closestRows.length > 0 && <Closest rows={closestRows} />}
          </>
        )}
      </div>

      {selected && (
        <Drawer onClose={close} label="Failure details">
          <FailureDetail failure={selected} />
        </Drawer>
      )}
    </div>
  );
}

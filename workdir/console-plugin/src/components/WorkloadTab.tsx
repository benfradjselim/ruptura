// The Ruptura tab of a Deployment, StatefulSet or DaemonSet: each failure of
// the workload in full, with the memory chart for a forecast.
import * as React from 'react';
import { fetchFailures, usePolling } from '../api';
import { predictedFirst } from '../util/failures';
import { ConsoleObject, failuresOf, workloadKey } from '../util/workload';
import FailureDetail from './FailureDetail';
import { failureKey } from './FailureList';
import { Banner, Empty, Loading } from './ui';
import './plugin.css';

export default function WorkloadTab({ obj }: { obj: ConsoleObject }): JSX.Element {
  const key = workloadKey(obj);
  const { data, error, loaded } = usePolling(fetchFailures, [key]);
  const failures = predictedFirst(failuresOf(data || [], key));
  return (
    <div className="ruptura-page ruptura-body">
      {error && <Banner variant="danger" title="Ruptura engine unreachable">{error}</Banner>}
      {!loaded && !error && <Loading label="Reading failures" />}
      {loaded && !error && failures.length === 0 && <Empty title="No known failure">Ruptura names no failure for this workload.</Empty>}
      {failures.map((f) => (
        <section key={failureKey(f)} className="ruptura-card">
          <FailureDetail failure={f} withWorkload={false} />
        </section>
      ))}
    </div>
  );
}

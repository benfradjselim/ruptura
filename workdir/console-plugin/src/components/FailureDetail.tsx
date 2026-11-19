// Everything Ruptura says about one failure: when, why, what to do, what it
// can take down and, for a memory forecast, the chart with the trend.
import * as React from 'react';
import { fetchSeries, usePolling } from '../api';
import { Failure } from '../types';
import { confidenceLabel, etaRange, formatEta, modeLabel } from '../util/failures';
import { hasForecastChart } from '../util/forecastChart';
import { shortPod } from '../util/workload';
import ForecastChart from './ForecastChart';
import { Banner, KindBadge, severity, WorkloadLink } from './ui';
import './plugin.css';

function Chart({ failure }: { failure: Failure }): JSX.Element | null {
  const { data, error, readAt } = usePolling(() => fetchSeries(failure.workload), [failure.workload]);
  if (error) return <Banner variant="warning" title="Memory readings unavailable">{error}</Banner>;
  if (!data) return null;
  return <ForecastChart pods={data.pods} failure={failure} now={readAt} />;
}

interface FailureDetailProps {
  failure: Failure;
  /** Show the workload link; off on the workload's own tab. */
  withWorkload?: boolean;
}

export default function FailureDetail({ failure, withWorkload = true }: FailureDetailProps): JSX.Element {
  const range = etaRange(failure);
  const confidence = confidenceLabel(failure);
  const affects = failure.affects || [];
  return (
    <article className={`ruptura-detail is-${severity(failure)}`}>
      <header className="ruptura-detail__head">
        <KindBadge failure={failure} />
        <h2 className="ruptura-detail__title">{modeLabel(failure.failureMode)}</h2>
        {withWorkload && <WorkloadLink workload={failure.workload} />}
        {failure.pod && <span className="ruptura-muted">Pod {shortPod(failure.pod)}</span>}
      </header>

      <dl className="ruptura-facts">
        <div>
          <dt>{failure.kind === 'predicted' ? 'Expected in' : 'Since'}</dt>
          <dd>{failure.kind === 'predicted' && failure.etaSeconds ? formatEta(failure.etaSeconds) : 'Now'}</dd>
        </div>
        {range && (
          <div>
            <dt>Window</dt>
            <dd>{range}</dd>
          </div>
        )}
        {confidence && (
          <div>
            <dt>Confidence</dt>
            <dd>{confidence}</dd>
          </div>
        )}
        <div>
          <dt>State</dt>
          <dd>{failure.state}</dd>
        </div>
      </dl>

      {hasForecastChart(failure) && (
        <section className="ruptura-detail__section">
          <h3>Memory and forecast</h3>
          <Chart failure={failure} />
        </section>
      )}

      <section className="ruptura-detail__section">
        <h3>Why Ruptura says this</h3>
        {(failure.evidence || []).length === 0 ? (
          <p className="ruptura-muted">No evidence reported.</p>
        ) : (
          <ul className="ruptura-evidence">
            {(failure.evidence || []).map((fact) => (
              <li key={fact}>{fact}</li>
            ))}
          </ul>
        )}
      </section>

      <section className="ruptura-detail__section">
        <h3>What to do</h3>
        <p className="ruptura-action">{failure.recommendedAction || 'No action recommended.'}</p>
      </section>

      {affects.length > 0 && (
        <section className="ruptura-detail__section">
          <h3>Can take down</h3>
          <ul className="ruptura-affects">
            {affects.map((caller) => (
              <li key={caller}>
                <WorkloadLink workload={caller} />
              </li>
            ))}
          </ul>
        </section>
      )}
    </article>
  );
}

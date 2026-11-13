// The Ruptura tab of a Deployment, StatefulSet or DaemonSet: its failures and,
// for a predicted memory failure, the memory chart with the forecast.
import * as React from 'react';
import { Alert, EmptyState, EmptyStateBody, EmptyStateHeader, PageSection, Spinner, Title } from '@patternfly/react-core';
import { fetchFailures, fetchSeries, usePolling } from '../api';
import { Failure } from '../types';
import { predictedFirst } from '../util/failures';
import { hasForecastChart } from '../util/forecastChart';
import { ConsoleObject, failuresOf, workloadKey } from '../util/workload';
import FailureTable from './FailureTable';
import ForecastChart from './ForecastChart';

interface WorkloadTabProps {
  obj: ConsoleObject;
}

function Forecast({ workload, failure }: { workload: string; failure: Failure }): JSX.Element | null {
  const { data, error, readAt } = usePolling(() => fetchSeries(workload), [workload]);
  if (error) {
    return (
      <Alert variant="warning" isInline title="Memory readings unavailable">
        {error}
      </Alert>
    );
  }
  if (!data) return null;
  return (
    <>
      <Title headingLevel="h3" size="md" style={{ marginTop: 24, marginBottom: 8 }}>
        Memory and forecast
      </Title>
      <ForecastChart pods={data.pods} failure={failure} now={readAt} />
    </>
  );
}

export default function WorkloadTab({ obj }: WorkloadTabProps): JSX.Element {
  const key = workloadKey(obj);
  const { data, error, loaded } = usePolling(fetchFailures, [key]);
  const failures = predictedFirst(failuresOf(data || [], key));
  const charted = failures.find(hasForecastChart);

  return (
    <PageSection>
      {error && (
        <Alert variant="danger" isInline title="Ruptura engine unreachable" style={{ marginBottom: 16 }}>
          {error}
        </Alert>
      )}
      {!loaded && !error && <Spinner aria-label="Reading failures" />}
      {loaded && failures.length === 0 && !error && (
        <EmptyState>
          <EmptyStateHeader titleText="No known failure" headingLevel="h2" />
          <EmptyStateBody>Ruptura names no failure for this workload.</EmptyStateBody>
        </EmptyState>
      )}
      {failures.length > 0 && <FailureTable failures={failures} withWorkload={false} />}
      {charted && <Forecast workload={key} failure={charted} />}
    </PageSection>
  );
}

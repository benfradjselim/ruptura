// Observe > Ruptures: every failure Ruptura names, predicted first, in the
// namespaces the user may see.
import * as React from 'react';
import { Alert, EmptyState, EmptyStateBody, EmptyStateHeader, PageSection, Spinner } from '@patternfly/react-core';
import { K8sResourceCommon, useK8sWatchResource } from '@openshift-console/dynamic-plugin-sdk';
import { fetchFailures, usePolling } from '../api';
import { predictedFirst } from '../util/failures';
import { visibleFailures } from '../util/workload';
import FailureTable from './FailureTable';
import './plugin.css';

// Projects are the namespaces the console lists for the user, whatever their
// role, so the page shows exactly what the user may see.
const PROJECT = { group: 'project.openshift.io', version: 'v1', kind: 'Project' };

function Count({ label, value }: { label: string; value: number }): JSX.Element {
  return (
    <div className="ruptura-count">
      <span className="ruptura-count__value">{value}</span>
      <span className="ruptura-count__label">{label}</span>
    </div>
  );
}

export default function RupturesPage(): JSX.Element {
  const [projects, projectsLoaded, projectsError] = useK8sWatchResource<K8sResourceCommon[]>({ groupVersionKind: PROJECT, isList: true });
  const { data, error, loaded } = usePolling(fetchFailures, []);

  const namespaces = projectsLoaded && !projectsError ? projects.map((p) => p.metadata?.name || '') : null;
  const failures = predictedFirst(visibleFailures(data || [], namespaces));
  const predicted = failures.filter((f) => f.kind === 'predicted').length;
  const critical = failures.filter((f) => f.state === 'critical').length;
  const ready = loaded && projectsLoaded;

  return (
    <>
      <header className="ruptura-header">
        <h1 className="ruptura-header__title">Ruptures</h1>
        <p className="ruptura-header__lead">Failures Ruptura names before and as they happen, with evidence and a recommended action.</p>
      </header>
      <PageSection>
        {error && <Alert variant="danger" isInline title="Ruptura engine unreachable" style={{ marginBottom: 16 }}>{error}</Alert>}
        {projectsError && (
          <Alert variant="warning" isInline title="Projects unavailable" style={{ marginBottom: 16 }}>
            Failures are shown for the projects you can see; the list of projects could not be read.
          </Alert>
        )}
        {!ready && !error && <Spinner aria-label="Reading failures" />}
        {ready && (
          <div className="ruptura-counts">
            <Count label="Predicted" value={predicted} />
            <Count label="Detected" value={failures.length - predicted} />
            <Count label="Critical" value={critical} />
          </div>
        )}
        {ready && failures.length === 0 && (
          <EmptyState>
            <EmptyStateHeader titleText="No known failure" headingLevel="h2" />
            <EmptyStateBody>Ruptura names no failure in the projects you can see.</EmptyStateBody>
          </EmptyState>
        )}
        {ready && failures.length > 0 && <FailureTable failures={failures} />}
      </PageSection>
    </>
  );
}

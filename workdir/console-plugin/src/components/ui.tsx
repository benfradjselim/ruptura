// Small building blocks drawn with the plugin's own CSS and console tokens, so
// they render the same on PatternFly 5 and PatternFly 6 consoles.
import * as React from 'react';
import { ResourceLink } from '@openshift-console/dynamic-plugin-sdk';
import { Failure } from '../types';
import { parseWorkload } from '../util/workload';
import './plugin.css';

/** A link to the workload in the console, or its key when it is not parsable. */
export function WorkloadLink({ workload }: { workload: string }): JSX.Element {
  const ref = parseWorkload(workload);
  if (!ref) return <span>{workload}</span>;
  return <ResourceLink groupVersionKind={{ group: 'apps', version: 'v1', kind: ref.kind }} name={ref.name} namespace={ref.namespace} />;
}

/** critical, degraded or other, as a CSS modifier. */
export function severity(failure: Pick<Failure, 'state'>): 'critical' | 'degraded' | 'info' {
  if (failure.state === 'critical') return 'critical';
  if (failure.state === 'degraded') return 'degraded';
  return 'info';
}

/** Predicted or detected, colored by severity. */
export function KindBadge({ failure }: { failure: Failure }): JSX.Element {
  return <span className={`ruptura-badge ruptura-badge--${severity(failure)}`}>{failure.kind}</span>;
}

type BannerVariant = 'danger' | 'warning' | 'info';

/** An inline message with a title. */
export function Banner({ variant, title, children }: { variant: BannerVariant; title: string; children?: React.ReactNode }): JSX.Element {
  return (
    <div className={`ruptura-banner ruptura-banner--${variant}`} role={variant === 'danger' ? 'alert' : 'status'}>
      <strong className="ruptura-banner__title">{title}</strong>
      {children && <div className="ruptura-banner__body">{children}</div>}
    </div>
  );
}

/** A quiet line shown while data is read. */
export function Loading({ label }: { label: string }): JSX.Element {
  return (
    <p className="ruptura-loading" role="status">
      <span className="ruptura-loading__dot" aria-hidden="true" />
      {label}
    </p>
  );
}

/** A centered message when there is nothing to list. */
export function Empty({ title, children }: { title: string; children?: React.ReactNode }): JSX.Element {
  return (
    <div className="ruptura-empty">
      <h2 className="ruptura-empty__title">{title}</h2>
      {children && <p className="ruptura-empty__body">{children}</p>}
    </div>
  );
}

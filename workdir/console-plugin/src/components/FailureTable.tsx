// The failures as a table: when, workload, failure with its evidence, the
// recommended action and the callers it can take down.
import * as React from 'react';
import { Label } from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';
import { ResourceLink } from '@openshift-console/dynamic-plugin-sdk';
import { Failure } from '../types';
import { confidenceLabel, etaRange, modeLabel, whenLabel } from '../util/failures';
import { parseWorkload } from '../util/workload';
import './plugin.css';

interface FailureTableProps {
  failures: Failure[];
  /** Hide the workload column on a page that is about one workload. */
  withWorkload?: boolean;
}

/** A link to the workload in the console, or its key when it is not parsable. */
export function WorkloadLink({ workload }: { workload: string }): JSX.Element {
  const ref = parseWorkload(workload);
  if (!ref) return <span>{workload}</span>;
  return <ResourceLink groupVersionKind={{ group: 'apps', version: 'v1', kind: ref.kind }} name={ref.name} namespace={ref.namespace} />;
}

function stateColor(state: string): 'red' | 'orange' | 'grey' {
  if (state === 'critical') return 'red';
  if (state === 'degraded') return 'orange';
  return 'grey';
}

function When({ failure }: { failure: Failure }): JSX.Element {
  const range = etaRange(failure);
  const confidence = confidenceLabel(failure);
  return (
    <div className="ruptura-when">
      <strong>{whenLabel(failure)}</strong>
      <div>
        <Label isCompact color={stateColor(failure.state)}>
          {failure.kind}
        </Label>
      </div>
      {range && <p className="ruptura-fact">{range}</p>}
      {confidence && <p className="ruptura-fact">{confidence}</p>}
    </div>
  );
}

export default function FailureTable({ failures, withWorkload = true }: FailureTableProps): JSX.Element {
  return (
    <Table aria-label="Failures Ruptura names" variant="compact">
      <Thead>
        <Tr>
          <Th>When</Th>
          {withWorkload && <Th>Workload</Th>}
          <Th>Failure</Th>
          <Th>Recommended action</Th>
          <Th>Can take down</Th>
        </Tr>
      </Thead>
      <Tbody>
        {failures.map((failure) => (
          <Tr key={`${failure.target}/${failure.failureMode}`}>
            <Td dataLabel="When">
              <When failure={failure} />
            </Td>
            {withWorkload && (
              <Td dataLabel="Workload">
                <WorkloadLink workload={failure.workload} />
              </Td>
            )}
            <Td dataLabel="Failure">
              <strong>{modeLabel(failure.failureMode)}</strong>
              {(failure.evidence || []).map((fact) => (
                <p key={fact} className="ruptura-fact">
                  {fact}
                </p>
              ))}
            </Td>
            <Td dataLabel="Recommended action">{failure.recommendedAction || '—'}</Td>
            <Td dataLabel="Can take down">
              {(failure.affects || []).length === 0
                ? '—'
                : (failure.affects || []).map((caller) => (
                    <div key={caller}>
                      <WorkloadLink workload={caller} />
                    </div>
                  ))}
            </Td>
          </Tr>
        ))}
      </Tbody>
    </Table>
  );
}

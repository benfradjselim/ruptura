// Mapping between console resources and the engine's workload keys, and the
// namespace filter that keeps the page within what the user may see.
import { Failure, WorkloadRef } from '../types';

/** A Kubernetes object as the console passes it to a tab. */
export interface ConsoleObject {
  kind?: string;
  metadata?: { name?: string; namespace?: string };
}

/** Parses namespace/Kind/name; null when the key has another shape. */
export function parseWorkload(key: string): WorkloadRef | null {
  const parts = key.split('/');
  if (parts.length !== 3 || parts.some((p) => p === '')) return null;
  const [namespace, kind, name] = parts;
  return { namespace, kind, name };
}

/** The engine key of a console object: namespace/Kind/name. */
export function workloadKey(obj: ConsoleObject): string {
  const namespace = obj.metadata?.namespace || '';
  const name = obj.metadata?.name || '';
  return `${namespace}/${obj.kind || ''}/${name}`;
}

/** Failures of one workload. */
export function failuresOf(failures: Failure[], key: string): Failure[] {
  return failures.filter((f) => f.workload === key);
}

/**
 * Failures in namespaces the user may see. The list of namespaces comes from
 * the projects the console lists for the user; while it is unknown nothing is
 * shown, so a user without access to a namespace never sees its failures.
 */
export function visibleFailures(failures: Failure[], namespaces: string[] | null): Failure[] {
  if (namespaces === null) return [];
  const allowed = new Set(namespaces);
  return failures.filter((f) => {
    const ref = parseWorkload(f.workload);
    return ref !== null && allowed.has(ref.namespace);
  });
}

/** The pod name without its namespace. */
export function shortPod(pod: string): string {
  return pod.split('/').pop() || pod;
}

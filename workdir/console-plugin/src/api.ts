// Reads the engine through the console proxy of the ConsolePlugin. The proxy
// reaches the plugin pod, whose nginx adds the API key: the browser only ever
// sees console URLs and the console session cookie.
import { useEffect, useState } from 'react';
import { consoleFetchJSON } from '@openshift-console/dynamic-plugin-sdk';
import { Failure, FailureSeries } from './types';

/** Console proxy of the plugin, as declared in the ConsolePlugin spec.proxy. */
export const PROXY_BASE = '/api/proxy/plugin/ruptura/engine';

/** How often the pages ask the engine again. */
export const REFRESH_MS = 30 * 1000;

export function fetchFailures(): Promise<Failure[]> {
  return consoleFetchJSON(`${PROXY_BASE}/api/v2/failures`).then((data: unknown) => (Array.isArray(data) ? (data as Failure[]) : []));
}

export function fetchSeries(workload: string): Promise<FailureSeries> {
  return consoleFetchJSON(`${PROXY_BASE}/api/v2/failures/series?workload=${encodeURIComponent(workload)}`) as Promise<FailureSeries>;
}

/** Message of an error the console fetch helpers throw. */
export function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === 'string') return error;
  return 'unexpected error';
}

export interface Polled<T> {
  data: T | null;
  error: string | null;
  loaded: boolean;
  /** When the data was last read, in milliseconds since the epoch. */
  readAt: number;
}

/**
 * Reads with fetcher now and every REFRESH_MS, until the component goes away.
 * A failed read keeps the last data and reports the error.
 */
export function usePolling<T>(fetcher: () => Promise<T>, deps: unknown[]): Polled<T> {
  const [state, setState] = useState<Polled<T>>({ data: null, error: null, loaded: false, readAt: 0 });
  useEffect(() => {
    let active = true;
    const read = (): void => {
      fetcher().then(
        (data) => active && setState({ data, error: null, loaded: true, readAt: Date.now() }),
        (error: unknown) => active && setState((previous) => ({ ...previous, error: errorMessage(error), loaded: true })),
      );
    };
    read();
    const timer = window.setInterval(read, REFRESH_MS);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
  return state;
}

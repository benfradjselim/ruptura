// Wording and ordering of failures, kept identical to the Ruptura web console
// (workdir/ui/src/lib/util/failures.js and horizonStatus.js) so both say the
// same thing.
import { Failure } from '../types';

/** Formats seconds as the console does: "< 1 min", "12 min", "1 h 30 min". */
export function formatEta(seconds?: number): string {
  if (!seconds) return '—';
  if (seconds < 60) return '< 1 min';
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} min`;
  return `${Math.floor(minutes / 60)} h ${minutes % 60} min`;
}

const eta = (failure: Failure): number => failure.etaSeconds || 0;

/** Critical first, then what is happening now, then the nearest failure. */
export function sortFailures(failures: Failure[]): Failure[] {
  const rank = (state: string): number => (state === 'critical' ? 0 : 1);
  return [...failures].sort((a, b) => rank(a.state) - rank(b.state) || eta(a) - eta(b));
}

/** Predicted failures first, in their order; then detected ones. */
export function predictedFirst(failures: Failure[]): Failure[] {
  const sorted = sortFailures(failures);
  return [...sorted.filter((f) => f.kind === 'predicted'), ...sorted.filter((f) => f.kind !== 'predicted')];
}

// Only these modes measure their confidence; the others always report 1.
const MEASURED_CONFIDENCE: Record<string, string> = {
  saturation: 'trend agreement',
  recurrent_failure: 'rhythm regularity',
};

/** "trend agreement 80%" for a predicted failure with a measured confidence. */
export function confidenceLabel(failure: Pick<Failure, 'failureMode' | 'kind' | 'confidence'>): string {
  const name = MEASURED_CONFIDENCE[failure.failureMode];
  if (failure.kind !== 'predicted' || !name || !(failure.confidence && failure.confidence > 0)) return '';
  return `${name} ${Math.round(failure.confidence * 100)}%`;
}

/** The window a predicted failure is expected in: "8 min to 9 min". */
export function etaRange(failure: Pick<Failure, 'etaEarlySeconds' | 'etaLateSeconds'>): string {
  if (!failure.etaEarlySeconds) return '';
  if (!failure.etaLateSeconds) return `from ${formatEta(failure.etaEarlySeconds)}`;
  return `${formatEta(failure.etaEarlySeconds)} to ${formatEta(failure.etaLateSeconds)}`;
}

/** "saturation" -> "saturation", "crash_loop" -> "crash loop". */
export function modeLabel(mode: string): string {
  return mode.replace(/_/g, ' ');
}

/** "in 8 min" for a forecast, "now" for a detected failure. */
export function whenLabel(failure: Pick<Failure, 'kind' | 'etaSeconds'>): string {
  return failure.kind === 'predicted' && failure.etaSeconds ? `in ${formatEta(failure.etaSeconds)}` : 'now';
}

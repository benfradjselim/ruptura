import { confidenceLabel, etaRange, formatEta, modeLabel, predictedFirst, sortFailures, whenLabel } from './failures';
import { Failure } from '../types';

const failure = (over: Partial<Failure>): Failure => ({
  level: 'workload',
  target: 'ns/Deployment/a',
  workload: 'ns/Deployment/a',
  failureMode: 'saturation',
  kind: 'detected',
  state: 'critical',
  ...over,
});

describe('formatEta', () => {
  it.each<[number | undefined, string]>([
    [undefined, '—'],
    [0, '—'],
    [45, '< 1 min'],
    [720, '12 min'],
    [5400, '1 h 30 min'],
  ])('formats %s', (seconds, want) => {
    expect(formatEta(seconds)).toBe(want);
  });
});

describe('sortFailures', () => {
  it('puts critical first, then what is happening now, then the nearest failure', () => {
    const got = sortFailures([
      failure({ target: 'a', state: 'degraded', etaSeconds: 60 }),
      failure({ target: 'b', state: 'critical', etaSeconds: 900 }),
      failure({ target: 'c', state: 'critical', etaSeconds: 300 }),
      failure({ target: 'd', state: 'critical' }),
    ]);
    expect(got.map((f) => f.target)).toEqual(['d', 'c', 'b', 'a']);
  });

  it('does not change its input', () => {
    const input = [failure({ target: 'a', state: 'degraded' }), failure({ target: 'b', state: 'critical' })];
    sortFailures(input);
    expect(input[0].target).toBe('a');
  });
});

describe('predictedFirst', () => {
  it('lists forecasts before detected failures, each group in console order', () => {
    const got = predictedFirst([
      failure({ target: 'a', kind: 'detected', state: 'critical' }),
      failure({ target: 'b', kind: 'predicted', state: 'degraded', etaSeconds: 1200 }),
      failure({ target: 'c', kind: 'predicted', state: 'critical', etaSeconds: 600 }),
      failure({ target: 'd', kind: 'detected', state: 'degraded' }),
    ]);
    expect(got.map((f) => f.target)).toEqual(['c', 'b', 'a', 'd']);
  });
});

describe('confidenceLabel', () => {
  it.each<[Pick<Failure, 'failureMode' | 'kind' | 'confidence'>, string]>([
    [{ failureMode: 'saturation', kind: 'predicted', confidence: 0.8 }, 'trend agreement 80%'],
    [{ failureMode: 'recurrent_failure', kind: 'predicted', confidence: 0.5 }, 'rhythm regularity 50%'],
    [{ failureMode: 'crash_loop', kind: 'predicted', confidence: 1 }, ''],
    [{ failureMode: 'saturation', kind: 'detected', confidence: 1 }, ''],
    [{ failureMode: 'saturation', kind: 'predicted' }, ''],
  ])('%o -> %s', (f, want) => {
    expect(confidenceLabel(f)).toBe(want);
  });
});

describe('etaRange', () => {
  it.each<[Pick<Failure, 'etaEarlySeconds' | 'etaLateSeconds'>, string]>([
    [{}, ''],
    [{ etaEarlySeconds: 480 }, 'from 8 min'],
    [{ etaEarlySeconds: 480, etaLateSeconds: 540 }, '8 min to 9 min'],
  ])('%o -> %s', (f, want) => {
    expect(etaRange(f)).toBe(want);
  });
});

describe('labels', () => {
  it('reads a failure mode as words', () => {
    expect(modeLabel('crash_loop')).toBe('crash loop');
  });

  it('says when the failure is expected', () => {
    expect(whenLabel({ kind: 'predicted', etaSeconds: 498 })).toBe('in 8 min');
    expect(whenLabel({ kind: 'detected' })).toBe('now');
    expect(whenLabel({ kind: 'predicted' })).toBe('now');
  });
});

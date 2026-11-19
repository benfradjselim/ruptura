import { Failure } from '../types';
import { ageLabel, closestRow, filterFailures, horizonMarkers, namespacesOf, NO_FILTER, warmup } from './horizon';

const failure = (over: Partial<Failure>): Failure => ({
  level: 'workload',
  target: 'shop/Deployment/a',
  workload: 'shop/Deployment/a',
  failureMode: 'saturation',
  kind: 'predicted',
  state: 'degraded',
  ...over,
});

describe('horizonMarkers', () => {
  it('puts detected failures at now and forecasts at their time', () => {
    const got = horizonMarkers([failure({ kind: 'detected', state: 'critical' }), failure({ etaSeconds: 900 })]);
    expect(got.map((m) => [m.position, m.critical])).toEqual([
      [0, true],
      [50, false],
    ]);
  });

  it('groups close failures and pins far ones at the edge', () => {
    const got = horizonMarkers([failure({ etaSeconds: 600 }), failure({ etaSeconds: 620, state: 'critical' }), failure({ etaSeconds: 7200 })]);
    expect(got).toHaveLength(2);
    expect(got[0].failures).toHaveLength(2);
    expect(got[0].critical).toBe(true);
    expect(got[1]).toMatchObject({ position: 100, beyond: true });
  });
});

describe('warmup', () => {
  const now = new Date(2026, 10, 18, 10, 0, 0).getTime();
  it.each<[number | undefined, boolean]>([
    [undefined, false],
    [1800, false],
    [3600, false],
    [120, true],
  ])('uptime %s gives a warm-up: %s', (uptime, want) => {
    expect(warmup(uptime, now) !== null).toBe(want);
  });

  it('says when forecasts are back', () => {
    expect(warmup(600, now)).toEqual({ startedMinutes: 10, detecting: true, forecastsBy: '10:20' });
    expect(warmup(60, now)?.detecting).toBe(false);
  });
});

describe('closestRow', () => {
  it('reads a rising pod', () => {
    expect(closestRow({ workload: 'shop/Deployment/a', pod: 'shop/a-1', usage: 0.8, trendPerHour: 0.2 })).toMatchObject({
      pod: 'a-1',
      namespace: 'shop',
      share: '80%',
      trend: '+20%/h',
      rising: true,
      reachesIn: 'limit in 1 h 0 min at this pace',
    });
  });

  it('reads flat and falling pods without a time', () => {
    expect(closestRow({ workload: 'w', pod: 'p', usage: 0.5 })).toMatchObject({ trend: 'flat', rising: false, reachesIn: '' });
    expect(closestRow({ workload: 'w', pod: 'p', usage: 0.5, trendPerHour: -0.05 }).trend).toBe('-5%/h');
  });
});

describe('filterFailures', () => {
  const all = [
    failure({ workload: 'shop/Deployment/cart', evidence: ['Memory at 80% of its limit'] }),
    failure({ workload: 'pay/Deployment/api', kind: 'detected', state: 'critical', failureMode: 'crash_loop' }),
  ];
  it.each<[string, Partial<typeof NO_FILTER>, string[]]>([
    ['no filter', {}, ['shop/Deployment/cart', 'pay/Deployment/api']],
    ['kind', { kind: 'detected' }, ['pay/Deployment/api']],
    ['critical only', { criticalOnly: true }, ['pay/Deployment/api']],
    ['namespace', { namespace: 'shop' }, ['shop/Deployment/cart']],
    ['words across fields', { text: 'cart 80%' }, ['shop/Deployment/cart']],
    ['mode words', { text: 'crash loop' }, ['pay/Deployment/api']],
    ['no match', { text: 'nothing' }, []],
  ])('%s', (_, over, want) => {
    expect(filterFailures(all, { ...NO_FILTER, ...over }).map((f) => f.workload)).toEqual(want);
  });

  it('lists namespaces once, sorted', () => {
    expect(namespacesOf([...all, ...all])).toEqual(['pay', 'shop']);
  });
});

describe('ageLabel', () => {
  it.each<[number, string]>([
    [0, ''],
    [995000, '5 s ago'],
    [880000, '2 min ago'],
  ])('read at %s', (readAt, want) => {
    expect(ageLabel(readAt, 1000000)).toBe(want);
  });
});
